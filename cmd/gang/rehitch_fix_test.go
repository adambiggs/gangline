//go:build acceptance

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

func TestRehitchDefersTeamLockContention(t *testing.T) {
	f, a, p := rehitchFixture(t)
	a.Directory = t.TempDir()
	f.setAgent(t, a, func(current *core.Agent) { *current = a })
	if err := f.run.record(a, core.Event{Type: "rehitch_requested"}); err != nil {
		t.Fatal(err)
	}
	lock, err := f.run.lockTeam()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := f.run.rehitchPending(a.ID); err != nil {
		t.Fatalf("contending restart failed: %v", err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rehitch == nil || got.Rehitch.Text != a.Rehitch.Text || got.Status != core.Active || got.Pane != a.Pane || len(pending) != 0 || loggedEvents(t, f, "rehitch_failed") != 0 {
		t.Fatalf("contention consumed restart: %+v notices=%+v", got, pending)
	}
}

type rehitchTeardownFailure struct {
	*inputFixture
	failAfterStop bool
	removed       bool
}

func (b *rehitchTeardownFailure) AcquireTree(context.Context, substrate.PaneID, tmux.Identity) (*tmux.Owned, error) {
	if !b.failAfterStop {
		return nil, errors.New("synthetic process acquisition failure")
	}
	return &tmux.Owned{}, nil
}
func (b *rehitchTeardownFailure) RemoveRegisteredNativePane(context.Context, tmux.PaneIdentity, tmux.Identity) error {
	b.removed = true
	return errors.New("synthetic pane removal failure")
}

func TestRehitchTeardownFailureKeepsLiveHarnessActive(t *testing.T) {
	for _, afterStop := range []bool{false, true} {
		t.Run(map[bool]string{false: "before stop", true: "after stop"}[afterStop], func(t *testing.T) {
			f, a, p := rehitchFixture(t)
			backend := &rehitchTeardownFailure{inputFixture: f.input, failAfterStop: afterStop}
			f.run.cmd.paneBackend = backend
			next := a
			next.Rehitch = nil
			locked, err := f.run.prepareRehitch(a, &next)
			if locked != nil {
				_ = locked.Close()
				t.Fatal("failed teardown returned locked replacement")
			}
			if err == nil {
				t.Fatal("teardown failure accepted")
			}
			got, readErr := p.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			wantStatus := core.Active
			if afterStop {
				wantStatus = core.Failed
			}
			if got.Status != wantStatus || got.Rehitch != nil || got.Pane != a.Pane || backend.removed != afterStop {
				t.Fatalf("teardown result: %+v removed=%t err=%v", got, backend.removed, err)
			}
		})
	}
}

func TestRehitchLiveRelaunchNeedsAttentionRatherThanFailure(t *testing.T) {
	for _, variant := range []string{"blocked startup", "unverified startup", "watchdog error", "failed launch", "original harness"} {
		t.Run(variant, func(t *testing.T) {
			f, before, p := rehitchFixture(t)
			current := before
			current.Rehitch = nil
			current.Registration.Generation = strings.Repeat("b", 64)
			switch variant {
			case "blocked startup":
				current.Status, current.Activity = core.Booting, core.Blocked
			case "unverified startup":
				current.Status, current.Activity = core.Active, core.Unknown
			case "watchdog error":
				current.Status, current.Activity = core.Active, core.Busy
			case "failed launch":
				current.Status, current.Pane = core.Failed, ""
			case "original harness":
				current = before
			}
			f.setAgent(t, current, func(a *core.Agent) { *a = current })
			failure := errors.New("synthetic " + variant)
			if err := f.run.rehitchError(before, failure); !errors.Is(err, failure) {
				t.Fatalf("lost command error: %v", err)
			}
			notices, err := p.ListNew()
			if err != nil || len(notices) != 1 {
				t.Fatalf("notice: %+v: %v", notices, err)
			}
			attention := variant != "failed launch" && variant != "original harness"
			if attention {
				if !strings.Contains(notices[0].Message.Text, "restarted its harness but needs attention") || strings.Contains(notices[0].Message.Text, "could not restart") || loggedEvents(t, f, "rehitch_failed") != 0 || loggedEvents(t, f, "rehitch_attention") != 1 {
					t.Fatalf("live relaunch reported as failed: %s", notices[0].Message.Text)
				}
			} else if !strings.Contains(notices[0].Message.Text, "could not restart") || loggedEvents(t, f, "rehitch_failed") != 1 {
				t.Fatalf("failed restart not reported: %s", notices[0].Message.Text)
			}
		})
	}
}
