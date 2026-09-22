package main

import (
	"github.com/adambiggs/gangline/core"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDeliveryLifetimeCannotBeConfiguredToExpire(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(file, []byte("GANG_DELIVERY_TIMEOUT=12m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := readConfiguration(file)
	if err == nil {
		t.Fatalf("removed expiry setting was accepted: %v, %v", settings, err)
	}
}

func TestDeliveryBackoffCapsUntilDeliverySettles(t *testing.T) {
	now := time.Unix(0, 0)
	acceptAt := now.Add(100 * time.Second)
	state := core.NewState(core.Team{ID: "team"})
	state.Deliveries["message"] = core.Delivery{Status: core.DeliveryQueued}
	var delays []time.Duration
	got, err := awaitDelivery(state, "message", func(delay time.Duration) {
		delays = append(delays, delay)
		now = now.Add(delay)
	}, func() (core.State, error) {
		if !now.Before(acceptAt) {
			d := state.Deliveries["message"]
			d.Status = core.DeliveryDelivered
			state.Deliveries["message"] = d
		}
		return state, nil
	})
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond, 6400 * time.Millisecond, 12800 * time.Millisecond, 25600 * time.Millisecond, 30 * time.Second, 30 * time.Second}
	if err != nil || got.Deliveries["message"].Status != core.DeliveryDelivered || !reflect.DeepEqual(delays, want) || now.Before(acceptAt) {
		t.Fatalf("backoff = %v, now = %v, error = %v", delays, now, err)
	}
}

func TestLiveRetrySurvivesHoursPastLegacyDeadline(t *testing.T) {
	now := time.Unix(1000, 0)
	state := core.NewState(core.Team{ID: "team"})
	state.Deliveries["message"] = core.Delivery{Status: core.DeliveryQueued, Deadline: now.Add(time.Minute)}
	attempts := 0
	_, err := awaitDelivery(state, "message", func(time.Duration) { now = now.Add(6 * time.Hour) }, func() (core.State, error) {
		attempts++
		if attempts == 3 {
			d := state.Deliveries["message"]
			d.Status = core.DeliveryDelivered
			state.Deliveries["message"] = d
		}
		return state, nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("live message expired: attempts=%d error=%v", attempts, err)
	}
}
