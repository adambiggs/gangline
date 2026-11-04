//go:build acceptance

package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func modelFixture(t *testing.T) (*LockedAgent, core.Agent) {
	t.Helper()
	a := testAgent("model-fixture", "worker")
	a.Registration = core.PaneRegistration{Generation: "server", Session: "session", TokenHash: "launch-one"}
	a.ConfiguredModel, a.ConfiguredEffort = "vendor/configured", "high"
	a.Native.Model, a.Native.Effort = "vendor/observed", "low"
	l, err := testTeam(t).CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, a
}

func TestModelMetadataKeepsLifecycleSchemaCompatible(t *testing.T) {
	l, a := modelFixture(t)
	raw, err := os.ReadFile(l.Paths.State)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["configured_model"] != nil || fields["configured_effort"] != nil || strings.Contains(string(fields["native"]), `"effort"`) {
		t.Fatalf("strict schema extended: %s", raw)
	}
	got, err := l.Paths.Read()
	if err != nil || got.ConfiguredModel != a.ConfiguredModel || got.ConfiguredEffort != a.ConfiguredEffort || got.Native.Effort != a.Native.Effort {
		t.Fatalf("side record = %+v, %v", got, err)
	}
	info, err := os.Stat(filepath.Join(l.Paths.Directory, "model-metadata.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private metadata: %v %v", info, err)
	}
}

func TestModelMetadataRefreshesEveryNewWriterSave(t *testing.T) {
	l, _ := modelFixture(t)
	for i := 1; i <= 4; i++ {
		a, err := l.Paths.Read()
		if err != nil {
			t.Fatal(err)
		}
		if a.Native.Effort != "low" {
			t.Fatalf("observation lost before save %d", i)
		}
		a.Native.Offset = int64(i)
		if err := l.Save(a); err != nil {
			t.Fatal(err)
		}
		got, err := l.Paths.Read()
		if err != nil || got.Native.Effort != "low" || got.ConfiguredModel != "vendor/configured" {
			t.Fatalf("save %d = %+v %v", i, got, err)
		}
	}
	t.Log("observed effort retained in 4/4 successive new-writer state saves")
}

func TestModelMetadataLegacyWritesInvalidateOnlyObservation(t *testing.T) {
	l, _ := modelFixture(t)
	// Decode/write only the lifecycle schema, as a predecessor does.
	var legacy core.Agent
	if err := readJSON(l.Paths.State, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.Native.Offset++
	if err := atomicJSON(l.Paths.State, legacy); err != nil {
		t.Fatal(err)
	}
	a, err := l.Paths.Read()
	if err != nil || a.Native.Effort != "" || a.ConfiguredModel != "vendor/configured" {
		t.Fatalf("legacy save = %+v %v", a, err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	a, err = l.Paths.Read()
	if err != nil || a.Native.Effort != "" {
		t.Fatalf("unknown resurrected: %+v %v", a, err)
	}
	a.Native.Effort = "high" // a subsequent native observation
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	a, err = l.Paths.Read()
	if err != nil || a.Native.Effort != "high" {
		t.Fatalf("new observation = %+v %v", a, err)
	}
	t.Log("legacy cursor change: effort unknown; new save preserves unknown; next observation restores coverage")
}

func TestModelMetadataClearAndRehitchIdentity(t *testing.T) {
	for _, kind := range []string{"clear", "rehitch", "unregistered", "missing nonce", "missing server", "other hitch"} {
		t.Run(kind, func(t *testing.T) {
			l, _ := modelFixture(t)
			var a core.Agent
			if err := readJSON(l.Paths.State, &a); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "clear":
				a.Native = core.NativeState{SessionID: "new-session"}
			case "rehitch":
				a.Registration.TokenHash = "launch-two" // same ID, name, CreatedAt, server and session
			case "unregistered":
				a.Registration = core.PaneRegistration{}
			case "missing nonce":
				a.Registration.TokenHash = ""
			case "missing server":
				a.Registration.Generation = ""
			case "other hitch":
				var metadata modelMetadata
				path := filepath.Join(l.Paths.Directory, "model-metadata.json")
				if err := readJSON(path, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata.HitchID = "other"
				if err := atomicJSON(path, metadata); err != nil {
					t.Fatal(err)
				}
			}
			if err := atomicJSON(l.Paths.State, a); err != nil {
				t.Fatal(err)
			}
			got, err := l.Paths.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Native.Effort != "" {
				t.Fatalf("stale observation attached: %+v", got)
			}
			if kind == "clear" {
				if got.ConfiguredModel != "vendor/configured" {
					t.Fatal("clear lost launch config")
				}
			} else if got.ConfiguredModel != "" || got.ConfiguredEffort != "" {
				t.Fatalf("old declaration attached: %+v", got)
			}
		})
	}
}

func TestModelMetadataInterruptedWriteDoesNotMixState(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			l, a := modelFixture(t)
			a.Native.Offset = 100
			a.Native.Model, a.Native.Effort = "vendor/new", "high"
			count := 0
			stop := errors.New("fixture write interrupted")
			err := l.Paths.saveModelState(a, func(path string, v any) error {
				count++
				if count == failAt {
					return stop
				}
				return atomicJSON(path, v)
			})
			if !errors.Is(err, stop) {
				t.Fatalf("interruption: %v", err)
			}
			got, err := l.Paths.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Native.Model != "vendor/observed" || got.Native.Offset != 0 {
				t.Fatalf("state changed: %+v", got)
			}
			want := "low"
			if failAt == 2 {
				want = ""
			}
			if got.Native.Effort != want || got.ConfiguredModel != "vendor/configured" {
				t.Fatalf("mixed snapshot: %+v", got)
			}
		})
	}
}

func TestModelMetadataUnavailableIsUnknown(t *testing.T) {
	l, _ := modelFixture(t)
	path := filepath.Join(l.Paths.Directory, "model-metadata.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	a, err := l.Paths.Read()
	if err != nil || a.ConfiguredModel != "" || a.Native.Effort != "" {
		t.Fatalf("missing: %+v %v", a, err)
	}
	for _, raw := range []string{"broken", `{"version":2}`, `{"version":1,"future_field":true}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		a, err := l.Paths.Read()
		if err != nil || a.ConfiguredModel != "" || a.ConfiguredEffort != "" || a.Native.Effort != "" || a.Native.Model != "vendor/observed" {
			t.Fatalf("unavailable optional metadata %q: %+v %v", raw, a, err)
		}
	}
}
