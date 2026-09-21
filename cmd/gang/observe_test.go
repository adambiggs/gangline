package main

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestPaneLossEventRequiresGrace(t *testing.T) {
	hitch := core.Hitch{ID: "h-1", Pane: "%2"}
	observedAt := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	observation := paneLossObservation{MissingSince: observedAt, Evidence: "pane %2 is absent"}

	if event := paneLossEvent(hitch, observation, observedAt.Add(paneLossGrace-time.Nanosecond)); event != nil {
		t.Fatalf("event before grace = %#v", event)
	}
	event := paneLossEvent(hitch, observation, observedAt.Add(paneLossGrace))
	if event == nil || event.HitchID != hitch.ID || event.Evidence != observation.Evidence {
		t.Fatalf("event after grace = %#v", event)
	}
}

func TestReconcilePanesRecordsVanishedPane(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is required")
	}
	root := t.TempDir()
	run := &runtime{
		cmd: command{},
		settings: settings{
			Session:   "pane-loss-test",
			StateRoot: filepath.Join(root, "state"),
			Socket:    filepath.Join(root, "tmux.sock"),
		},
	}
	observedAt := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	state, err := run.drive(core.AdoptRequested{
		At: observedAt, Pane: "%9",
		Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: root},
	})
	if err != nil {
		t.Fatal(err)
	}

	state, err = run.reconcilePanes(state, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if state.Hitches["h-1"].Status != core.HitchActive {
		t.Fatalf("first observation changed hitch = %#v", state.Hitches["h-1"])
	}
	state, err = run.reconcilePanes(state, observedAt.Add(paneLossGrace))
	if err != nil {
		t.Fatal(err)
	}
	hitch := state.Hitches["h-1"]
	if hitch.Status != core.HitchFailed || hitch.Activity != core.ActivityWedged || hitch.WedgeEvidence == "" {
		t.Fatalf("reconciled hitch = %#v", hitch)
	}
}

func TestRecoverReconcilesVanishedPane(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is required")
	}
	root := t.TempDir()
	run := &runtime{
		cmd: command{},
		settings: settings{
			Session:   "pane-loss-recover-test",
			StateRoot: filepath.Join(root, "state"),
			Socket:    filepath.Join(root, "tmux.sock"),
		},
	}
	observedAt := time.Now().Add(-paneLossGrace)
	state, err := run.drive(core.AdoptRequested{
		At: observedAt, Pane: "%9",
		Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.reconcilePanes(state, observedAt); err != nil {
		t.Fatal(err)
	}

	state, err = run.recover()
	if err != nil {
		t.Fatal(err)
	}
	hitch := state.Hitches["h-1"]
	if hitch.Status != core.HitchFailed || hitch.Activity != core.ActivityWedged {
		t.Fatalf("recovered hitch = %#v, want failed and wedged", hitch)
	}
}
