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
