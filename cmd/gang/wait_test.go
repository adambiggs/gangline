package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

type controlledAppendWait struct {
	release <-chan struct{}
}

func (wait controlledAppendWait) Wait(ctx context.Context) error {
	select {
	case <-wait.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (controlledAppendWait) Close() error { return nil }

func TestWaitZeroRecordsTimeoutWithoutChangingHitch(t *testing.T) {
	root := t.TempDir()
	values := map[string]string{
		"GANG_SESSION":    "wait-test",
		"GANG_STATE_ROOT": filepath.Join(root, "state"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
	}
	cmd := command{
		getenv: func(name string) string { return values[name] },
		lookupEnv: func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		},
		userHomeDir: func() (string, error) { return root, nil },
	}
	settings, err := cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	run := &runtime{cmd: cmd, settings: settings}
	now := time.Now()
	state, err := run.drive(core.AdoptRequested{
		At: now, Pane: "%1",
		Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err = run.drive(core.TurnStarted{At: now, HitchID: "h-1"})
	if err != nil {
		t.Fatal(err)
	}

	err = cmd.wait([]string{"worker", "--timeout", "0"})
	var commandErr commandError
	if !errors.As(err, &commandErr) || commandErr.status != exitRefused {
		t.Fatalf("wait error = %#v, want refused timeout", err)
	}
	state, err = run.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Hitches["h-1"].Activity != core.ActivityBusy {
		t.Fatalf("wait timeout changed hitch = %#v", state.Hitches["h-1"])
	}
	locked, err := run.paths().Lock(settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	entries, err := locked.Log()
	if err != nil {
		t.Fatal(err)
	}
	last, ok := entries[len(entries)-1].Event.(core.OperationTimedOut)
	if !ok || last.Operation != core.TimeoutWait || last.ID != "h-1" {
		t.Fatalf("last event = %#v, want wait timeout", entries[len(entries)-1].Event)
	}
}

func TestWaitBlocksForAnIdleBoundary(t *testing.T) {
	root := t.TempDir()
	values := map[string]string{
		"GANG_SESSION":    "wait-boundary-test",
		"GANG_STATE_ROOT": filepath.Join(root, "state"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
	}
	registered := make(chan struct{})
	release := make(chan struct{})
	cmd := command{
		getenv: func(name string) string { return values[name] },
		lookupEnv: func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		},
		userHomeDir: func() (string, error) { return root, nil },
		newAppendWait: func(string, int64) (appendWait, error) {
			close(registered)
			return controlledAppendWait{release: release}, nil
		},
	}
	settings, err := cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	run := &runtime{cmd: cmd, settings: settings}
	now := time.Now()
	state, err := run.drive(core.AdoptRequested{
		At: now, Pane: "%1",
		Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run.drive(core.TurnStarted{At: now, HitchID: "h-1"}); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() { result <- cmd.wait([]string{"worker"}) }()
	<-registered
	if _, err = run.drive(core.TurnBoundaryReached{At: now, HitchID: "h-1"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err = <-result; err != nil {
		t.Fatalf("wait returned before the idle boundary: %v", err)
	}
	state, err = run.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Hitches["h-1"].Activity != core.ActivityIdle {
		t.Fatalf("wait completed with hitch = %#v", state.Hitches["h-1"])
	}
}
