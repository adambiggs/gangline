package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
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
