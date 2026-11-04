//go:build acceptance

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOptionalModelMetadataDoesNotBlockInventoryOrDrop(t *testing.T) {
	for _, raw := range []string{"broken", `{"version":2}`, `{"version":1,"future_field":true}`} {
		t.Run(raw, func(t *testing.T) {
			f, _, p := clearFixture(t)
			if err := os.WriteFile(filepath.Join(p.Directory, "model-metadata.json"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			agents, err := f.run.team.ListAgents()
			if err != nil || len(agents) != 1 {
				t.Fatalf("inventory blocked by optional metadata: %+v %v", agents, err)
			}
			if agents[0].ConfiguredModel != "" || agents[0].ConfiguredEffort != "" || agents[0].Native.Effort != "" {
				t.Fatal("unavailable metadata was presented as known")
			}
			if err := f.cmd.drop([]string{"worker"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(p.Directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("agent directory retained after drop: %v", err)
			}
		})
	}
}
