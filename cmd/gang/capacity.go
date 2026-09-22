package main

import (
	"context"
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// Only Gangline's synthetic recovery prompt has a submission budget. A user
// delivery's legacy Deadline is deliberately ignored.
func capacitySubmissionBudget(state core.State, id core.EnvelopeID, now time.Time) (time.Duration, error) {
	delivery := state.Deliveries[id]
	if !delivery.CapacityRecovery {
		return deliveryTimeout, nil
	}
	for _, hitch := range state.Hitches {
		if hitch.Name != delivery.Envelope.To || hitch.Status != core.HitchActive {
			continue
		}
		if hitch.Capacity.Deadline.IsZero() {
			return 0, fmt.Errorf("provider capacity recovery has no recorded deadline")
		}
		remaining := hitch.Capacity.Deadline.Sub(now)
		if remaining <= 0 {
			return 0, fmt.Errorf("provider capacity recovery deadline elapsed")
		}
		return min(deliveryTimeout, remaining), nil
	}
	return 0, fmt.Errorf("provider capacity recovery recipient is no longer active")
}

// The explicit tick command is the owner. Codex omits Stop on a terminal
// provider error, so neither ordinary activity hooks nor a resident watcher
// start this loop. Later ticks resume its persisted schedule.
func (run *runtime) refreshCapacity(state core.State, now time.Time) (core.State, error) {
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return core.State{}, err
	}
	for id, hitch := range state.Hitches {
		if hitch.Status != core.HitchActive || (hitch.Activity != core.ActivityBusy && hitch.Activity != core.ActivityIdle) || hitch.PendingCompactID != "" {
			continue
		}
		collar, err := loadCollar(hitch.Collar, run.settings)
		if err != nil {
			return core.State{}, err
		}
		if collar.Primitives.Capacity == nil {
			continue
		}
		screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
		if err != nil {
			return core.State{}, fmt.Errorf("observe provider capacity for %s: %w", hitch.Name, err)
		}
		capacity, found, err := harness.DetectCapacity(collar, screen)
		if err != nil {
			return core.State{}, fmt.Errorf("observe provider capacity for %s: %w", hitch.Name, err)
		}
		if found && capacity.Fingerprint != hitch.Capacity.Fingerprint {
			state, err = run.drive(core.CapacityDetected{At: now, HitchID: id, Fingerprint: capacity.Fingerprint, Evidence: capacity.Evidence, Deadline: now.Add(run.settings.CapacityTimeout)})
			if err != nil {
				return core.State{}, err
			}
			hitch = state.Hitches[id]
		}
		if hitch.Capacity.Fingerprint == "" {
			continue
		}
		if !now.Before(hitch.Capacity.Deadline) {
			state, err = run.drive(core.CapacityExpired{At: now, HitchID: id})
		} else if found && hitch.Activity == core.ActivityIdle && !now.Before(hitch.Capacity.NextAt) {
			state, err = run.drive(core.CapacityRetryRequested{At: now, HitchID: id, Fingerprint: capacity.Fingerprint})
		}
		if err != nil {
			return core.State{}, err
		}
	}
	return state, nil
}

func (run *runtime) finishCapacityRecovery(state core.State) error {
	_, err := awaitCapacityRecovery(state, time.Now, func(delay time.Duration) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
	}, func() (core.State, error) {
		state, err := run.load()
		if err != nil {
			return core.State{}, err
		}
		state, err = run.reconcilePanes(state, time.Now())
		if err != nil {
			return core.State{}, err
		}
		return run.refreshCapacity(state, time.Now())
	})
	return err
}

// Supplied time and waiting keep the bounded owner deterministic in tests.
func awaitCapacityRecovery(state core.State, now func() time.Time, wait func(time.Duration), refresh func() (core.State, error)) (core.State, error) {
	for {
		delay, pending, err := capacityRecoveryDelay(state, now())
		if err != nil || !pending {
			return state, err
		}
		wait(delay)
		state, err = refresh()
		if err != nil {
			return core.State{}, err
		}
	}
}

func capacityRecoveryDelay(state core.State, now time.Time) (time.Duration, bool, error) {
	delay, pending := maximumRetryDelay, false
	for _, hitch := range state.Hitches {
		if hitch.Status != core.HitchActive || hitch.Capacity.Fingerprint == "" {
			continue
		}
		if hitch.Capacity.Expired {
			return 0, false, refuseError("provider capacity recovery deadline elapsed for %s; inspect the pane", hitch.Name)
		}
		if hitch.Activity != core.ActivityIdle && hitch.Activity != core.ActivityBusy {
			return 0, false, refuseError("provider capacity recovery for %s stopped at %s; inspect the pane", hitch.Name, hitch.Activity)
		}
		pending = true
		next := min(hitch.Capacity.Deadline.Sub(now), maximumRetryDelay)
		if hitch.Activity == core.ActivityIdle {
			next = min(next, max(core.RetryDelay(0), hitch.Capacity.NextAt.Sub(now)))
		} else if hitch.Capacity.NextAt.After(now) {
			next = min(next, hitch.Capacity.NextAt.Sub(now))
		}
		delay = min(delay, max(0, next))
	}
	return delay, pending, nil
}
