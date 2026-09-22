package main

import (
	"errors"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestCapacityOwnerBackoffUsesSuppliedClockAndKeepsDeadline(t *testing.T) {
	now := time.Unix(1000, 0)
	state := core.NewState(core.Team{})
	state.Hitches["worker"] = core.Hitch{ID: "worker", Name: "worker", Status: core.HitchActive, Activity: core.ActivityIdle, Capacity: core.CapacityRecovery{Fingerprint: "one", Deadline: now.Add(time.Minute), NextAt: now.Add(100 * time.Millisecond)}}
	var waits []time.Duration
	_, err := awaitCapacityRecovery(state, func() time.Time { return now }, func(delay time.Duration) { waits = append(waits, delay); now = now.Add(delay) }, func() (core.State, error) {
		hitch := state.Hitches["worker"]
		switch len(waits) {
		case 1:
			hitch.Capacity.NextAt = now.Add(200 * time.Millisecond)
		case 2:
			hitch.Capacity.NextAt = now.Add(400 * time.Millisecond)
		default:
			hitch.Capacity = core.CapacityRecovery{}
		}
		state.Hitches["worker"] = hitch
		return state, nil
	})
	if err != nil || len(waits) != 3 || waits[0] != 100*time.Millisecond || waits[1] != 200*time.Millisecond || waits[2] != 400*time.Millisecond {
		t.Fatalf("waits=%v error=%v", waits, err)
	}
}

func TestCapacityOwnerStopsOnExpiryDropOrObservationFailure(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, outcome := range []string{"expired", "dropped", "capture error"} {
		t.Run(outcome, func(t *testing.T) {
			state := core.NewState(core.Team{})
			state.Hitches["worker"] = core.Hitch{ID: "worker", Name: "worker", Status: core.HitchActive, Activity: core.ActivityBusy, Capacity: core.CapacityRecovery{Fingerprint: "one", Deadline: now.Add(time.Second)}}
			calls := 0
			_, err := awaitCapacityRecovery(state, func() time.Time { return now }, func(delay time.Duration) { now = now.Add(delay) }, func() (core.State, error) {
				calls++
				if calls > 1 {
					t.Fatal("owner did not terminate after its terminal observation")
				}
				hitch := state.Hitches["worker"]
				switch outcome {
				case "expired":
					hitch.Capacity.Expired = true
				case "dropped":
					hitch.Status = core.HitchDropped
				case "capture error":
					return core.State{}, errors.New("capture failed")
				}
				state.Hitches["worker"] = hitch
				return state, nil
			})
			if (err == nil) != (outcome == "dropped") || calls != 1 {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestCapacityTimeoutConfiguration(t *testing.T) {
	for _, value := range []string{"17m", "0", "-1m", "not a duration"} {
		root := t.TempDir()
		cmd := command{getenv: func(name string) string {
			if name == "GANG_CAPACITY_TIMEOUT" {
				return value
			}
			if name == "XDG_CONFIG_HOME" {
				return root
			}
			return ""
		}, userHomeDir: func() (string, error) { return root, nil }}
		got, err := cmd.settings()
		if value == "17m" {
			if err != nil || got.CapacityTimeout != 17*time.Minute {
				t.Fatalf("settings=%+v error=%v", got, err)
			}
		} else if err == nil {
			t.Fatalf("invalid capacity timeout %q accepted", value)
		}
	}
}

func TestCapacityOwnerExpiresBusyEpisodeThroughReducer(t *testing.T) {
	now := time.Unix(1000, 0)
	state := core.NewState(core.Team{})
	state.Hitches["worker"] = core.Hitch{ID: "worker", Name: "worker", Status: core.HitchActive, Activity: core.ActivityBusy, Capacity: core.CapacityRecovery{Fingerprint: "one", Deadline: now.Add(time.Second)}}
	refreshes := 0
	result, err := awaitCapacityRecovery(state, func() time.Time { return now }, func(delay time.Duration) { now = now.Add(delay) }, func() (core.State, error) {
		refreshes++
		if refreshes > 1 {
			t.Fatal("busy capacity expiry did not stop the command")
		}
		state, _ = core.Step(state, core.CapacityExpired{At: now, HitchID: "worker"})
		return state, nil
	})
	if err == nil || !result.Hitches["worker"].Capacity.Expired || result.Hitches["worker"].Activity != core.ActivityBusy {
		t.Fatalf("busy expiry result=%+v error=%v", result.Hitches["worker"], err)
	}
}

func TestCapacitySubmissionBudgetNeverExpiresUserSends(t *testing.T) {
	now := time.Unix(1000, 0)
	state := core.NewState(core.Team{})
	state.Hitches["worker"] = core.Hitch{Name: "worker", Status: core.HitchActive, Capacity: core.CapacityRecovery{Deadline: now.Add(time.Second)}}
	state.Deliveries["user"] = core.Delivery{Envelope: core.Envelope{To: "worker"}, Deadline: now.Add(-time.Hour)}
	state.Deliveries["recovery"] = core.Delivery{Envelope: core.Envelope{To: "worker"}, CapacityRecovery: true}
	if got, err := capacitySubmissionBudget(state, "recovery", now); err != nil || got != time.Second {
		t.Fatalf("recovery budget=%v error=%v", got, err)
	}
	if _, err := capacitySubmissionBudget(state, "recovery", now.Add(time.Second)); err == nil {
		t.Fatal("recovery submission passed its deadline")
	}
	if got, err := capacitySubmissionBudget(state, "user", now.Add(24*time.Hour)); err != nil || got != deliveryTimeout {
		t.Fatalf("user message gained expiry: budget=%v error=%v", got, err)
	}
}

func TestCapacityObservationGapDoesNotSpin(t *testing.T) {
	now := time.Unix(1000, 0)
	state := core.NewState(core.Team{})
	state.Hitches["worker"] = core.Hitch{ID: "worker", Name: "worker", Status: core.HitchActive, Activity: core.ActivityIdle, Capacity: core.CapacityRecovery{Fingerprint: "one", Deadline: now.Add(time.Minute), NextAt: now.Add(-time.Second)}}
	delay, pending, err := capacityRecoveryDelay(state, now)
	if err != nil || !pending || delay <= 0 {
		t.Fatalf("missing terminal evidence permits a busy loop: delay=%v pending=%v error=%v", delay, pending, err)
	}
}
