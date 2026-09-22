package store

import (
	"github.com/adambiggs/gangline/core"
	"os"
	"path/filepath"
	"testing"
)

func TestObserveLogDistinguishesPartialAndInvalidRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	initial := core.NewState(core.Team{ID: "team"})
	event, err := core.EncodeEvent(core.AdoptRequested{At: storeNow, Hitch: core.Hitch{ID: "worker", Name: "worker", Collar: "codex", Directory: "/work"}, Pane: "%1"})
	if err != nil {
		t.Fatal(err)
	}
	prefix := append(event, '\n')
	for _, test := range []struct {
		name, suffix    string
		complete, fails bool
	}{
		{"complete", "", true, false},
		{"incomplete", "{", false, false},
		{"malformed", "{\n", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := append(append([]byte{}, prefix...), test.suffix...)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			state, size, complete, err := ObserveLog(path, initial)
			if (err != nil) != test.fails {
				t.Fatalf("error = %v", err)
			}
			if !test.fails && (size != int64(len(data)) || complete != test.complete || state.Hitches["worker"].Activity != core.ActivityIdle) {
				t.Fatalf("size = %d, complete = %v, hitch = %#v", size, complete, state.Hitches["worker"])
			}
		})
	}
}
