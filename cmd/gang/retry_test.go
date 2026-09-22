package main

import (
	"github.com/adambiggs/gangline/core"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDeferredDeliveryDeadlineIsOperatorConfigurable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(file, []byte("GANG_DELIVERY_TIMEOUT=12m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := readConfiguration(file)
	if err != nil || settings["GANG_DELIVERY_TIMEOUT"] != "12m" {
		t.Fatalf("operator delivery deadline rejected: %v, %v", settings, err)
	}
}

func TestDeliveryBackoffCapsAndStopsAtDeadline(t *testing.T) {
	now := time.Unix(0, 0)
	deadline := now.Add(100 * time.Second)
	state := core.NewState(core.Team{ID: "team"})
	state.Deliveries["message"] = core.Delivery{Status: core.DeliveryQueued, Deadline: deadline}
	var delays []time.Duration
	got, err := awaitDelivery(state, "message", func() time.Time { return now }, func(delay time.Duration) {
		delays = append(delays, delay)
		now = now.Add(delay)
	}, func() (core.State, error) {
		if !now.Before(deadline) {
			d := state.Deliveries["message"]
			d.Status = core.DeliveryFailed
			state.Deliveries["message"] = d
		}
		return state, nil
	})
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond, 6400 * time.Millisecond, 12800 * time.Millisecond, 25600 * time.Millisecond, 30 * time.Second, 18900 * time.Millisecond}
	if err != nil || got.Deliveries["message"].Status != core.DeliveryFailed || !reflect.DeepEqual(delays, want) || !now.Equal(deadline) {
		t.Fatalf("backoff = %v, now = %v, error = %v", delays, now, err)
	}
}
