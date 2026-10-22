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

func TestRehitchRequiresRecordedOrExplicitLaunchChoices(t *testing.T) {
	for _, variant := range []string{"legacy", "missing effort", "missing model", "explicit", "recorded", "override"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := rehitchFixture(t)
			a.Rehitch = nil
			a.Native.Model = "observed-model"
			args := []string{"--resume", "continue"}
			switch variant {
			case "explicit":
				args = append(args, "--model", "chosen-model", "--effort", "high")
			case "missing effort":
				a.LaunchModel = "recorded-model"
			case "missing model":
				a.LaunchEffort = "high"
			case "recorded", "override":
				a.LaunchModel, a.LaunchEffort = "recorded-model", "medium"
			}
			if variant == "override" {
				args = append(args, "-m", "chosen-model", "-e", "high")
			}
			f.setAgent(t, a, func(current *core.Agent) { *current = a })
			err := f.cmd.rehitch(args)
			got, readErr := p.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if variant == "legacy" || variant == "missing effort" || variant == "missing model" {
				if err == nil || errorStatus(err) != exitRefused || got.Rehitch != nil || got.Pane != a.Pane {
					t.Fatalf("missing launch choices accepted: %+v: %v", got, err)
				}
				if !strings.Contains(err.Error(), "--model") || !strings.Contains(err.Error(), "--effort") {
					t.Fatalf("refusal lacks recovery: %v", err)
				}
				return
			}
			wantModel, wantEffort := "chosen-model", "high"
			if variant == "recorded" {
				wantModel, wantEffort = "recorded-model", "medium"
			}
			if err != nil || got.Rehitch == nil || got.LaunchModel != wantModel || got.LaunchEffort != wantEffort {
				t.Fatalf("launch choices lost: %+v: %v", got, err)
			}
		})
	}
}

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
