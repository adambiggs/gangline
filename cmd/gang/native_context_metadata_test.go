//go:build acceptance

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func contextFixtureWithoutCachedState(t *testing.T, records string, mutate func(*core.Agent)) *stateFixture {
	t.Helper()
	fixture := nativeContextFixture(t, "rollout-test-parent.jsonl", 0, records)
	transcript := filepath.Join(fixture.env["GANG_STATE_ROOT"], "rollout-test-parent.jsonl")
	saveAgent(t, fixture, "a", func(agent *core.Agent) {
		agent.Native = core.NativeState{SessionID: "parent", BoundSession: "parent", Transcript: transcript}
		agent.ContextBands, agent.Compaction = core.ContextBandState{}, nil
		mutate(agent)
	})
	paths, err := fixture.run.team.Agent("a")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Witness, paths.CompactionWitness} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	fixture.cmd.detach = func(string, hookNotice) error { return nil }
	return fixture
}

func TestContextResumedAgentWithoutTranscriptStaysUnknown(t *testing.T) {
	fixture := contextFixtureWithoutCachedState(t, parentContextRecords, func(agent *core.Agent) {
		agent.Native.Transcript, agent.Native.BoundSession = "", ""
	})
	err := fixture.cmd.context([]string{"worker", "--json"})
	if !strings.Contains(fixture.out.String(), `"status":"unknown"`) {
		t.Fatalf("expected unknown context, got error=%v output=%q", err, fixture.out.String())
	}
}

func TestContextTailWithoutTurnContextKeepsModel(t *testing.T) {
	padding := strings.Repeat(`{"timestamp":"2026-09-22T10:00:00Z","type":"response_item","payload":{"x":"`+strings.Repeat("y", 4000)+`"}}`+"\n", 1100)
	records := `{"timestamp":"2026-09-22T10:00:00Z","type":"turn_context","payload":{"model":"parent-model"}}` + "\n" + padding +
		`{"timestamp":"2026-09-22T10:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":150},"model_context_window":1000}}}` + "\n"
	fixture := contextFixtureWithoutCachedState(t, records, func(agent *core.Agent) {
		agent.Native.Model = "parent-model"
	})
	if err := fixture.cmd.context([]string{"worker", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got contextJSON
	decodeOutput(t, fixture, &got)
	if got.Model != "parent-model" || got.Band == nil || *got.Band != "yellow" {
		t.Fatalf("model=%q band=%v; want parent-model/yellow", got.Model, got.Band)
	}
}

func contextStateSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			files[path] = target
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func nativeContextFixture(t *testing.T, binding string, offset int64, records string) *stateFixture {
	t.Helper()
	fixture := newStateFixture(t)
	agent := fixture.add(t, "a", "worker", "codex")
	root := fixture.env["GANG_STATE_ROOT"]
	for name, body := range map[string]string{
		"rollout-test-parent.jsonl": `{"type":"session_meta","payload":{"id":"parent"}}` + "\n" + records,
		"child.jsonl":               `{"type":"session_meta","payload":{"id":"child"}}` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	future := fixture.cmd.now().Add(2 * time.Hour)
	used, limit, percent := int64(999), int64(1000), 99.9
	saveAgent(t, fixture, agent.ID, func(agent *core.Agent) {
		agent.Native = core.NativeState{
			SessionID: "parent", BoundSession: "parent", Transcript: filepath.Join(root, binding), Offset: offset,
			HookSequence: 42, TurnID: "parent-turn", SubmittedAt: fixture.cmd.now(),
			CompactedAt: future, ConfirmedCompactedAt: future,
			LastError: "child error", LastErrorAt: future, TurnFailure: "child failure", FailedTurn: "child-turn",
			Model: "child-model", Context: core.Reading{At: &future, Status: "observed", Used: &used, Limit: &limit, Percent: &percent},
			Limits: core.Reading{At: &future, Status: "observed"},
		}
		agent.ContextBands = core.ContextBandState{Model: "child-model", Percent: 99.9, CompactedAt: future, Sequence: 7,
			Pending: []core.ContextBandNote{{Band: "child-band", Envelope: core.Envelope{
				ID: "context-7", Token: "0123456789abcdef", Recipient: agent.ID, To: agent.Name,
				From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, CreatedAt: fixture.cmd.now(),
				Message: core.Message{Text: "child context band"},
			}}}}
		agent.Compaction = &core.Compaction{ID: "compact", Status: "submitted", StartedAt: fixture.cmd.now(), CompletedAt: future, Continuation: true}
		agent.ContextBandThresholds = &core.ContextBandThresholds{Early: .1, Late: .2}
	})
	paths, err := fixture.run.team.Agent(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.WriteWitness(store.Witness{ID: "new-parent-submit", At: fixture.cmd.now().Add(time.Minute), SessionID: "parent", Transcript: filepath.Join(root, "rollout-test-parent.jsonl")}); err != nil {
		t.Fatal(err)
	}
	if err := paths.WriteCompactionWitness(store.CompactionWitness{SessionID: "parent", At: future}); err != nil {
		t.Fatal(err)
	}
	fixture.cmd.detach = func(string, hookNotice) error {
		t.Error("context scheduled a tick")
		return nil
	}
	return fixture
}

const parentContextRecords = `{"timestamp":"2026-09-22T10:00:00Z","type":"turn_context","payload":{"model":"parent-model"}}
{"timestamp":"2026-09-22T10:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":150},"model_context_window":1000}}}
`

func TestContextVerifiedNativeReadLeavesStateUntouched(t *testing.T) {
	for _, binding := range []string{"child.jsonl", "rollout-test-parent.jsonl"} {
		for _, offset := range []int64{0, int64(len(`{"type":"session_meta","payload":{"id":"parent"}}` + "\n" + parentContextRecords)), 9999999} {
			t.Run(fmt.Sprintf("%s/cursor-%d", binding, offset), func(t *testing.T) {
				fixture := nativeContextFixture(t, binding, offset, parentContextRecords)
				root := fixture.env["GANG_STATE_ROOT"]
				before := contextStateSnapshot(t, root)
				for _, machine := range []bool{true, true, false} {
					fixture.out.Reset()
					args := []string{"worker"}
					if machine {
						args = append(args, "--json")
					}
					if err := fixture.cmd.context(args); err != nil {
						t.Errorf("verified read: %v", err)
					} else if machine {
						var got contextJSON
						decodeOutput(t, fixture, &got)
						if got.Status != "observed" || got.Model != "parent-model" || got.Used == nil || *got.Used != 150 || got.Limit == nil || *got.Limit != 1000 || got.Percent == nil || *got.Percent != 15 || got.Source != "telemetry" || got.Band == nil || *got.Band != "yellow" {
							t.Errorf("parent context = %+v", got)
						}
					} else if got := fixture.out.String(); got != "worker\t150/1k\t15%\tyellow\n" {
						t.Errorf("text context = %q", got)
					}
					if !reflect.DeepEqual(contextStateSnapshot(t, root), before) {
						t.Fatal("context changed saved state or witnesses")
					}
				}
			})
		}
	}
}

func TestNativeRefreshDoesNotRepairSavedState(t *testing.T) {
	for _, test := range []struct{ binding, reason string }{
		{"child.jsonl", "metadata does not match"},
		{"rollout-test-parent.jsonl", "cursor 9999999 exceeds size"},
	} {
		t.Run(test.binding, func(t *testing.T) {
			fixture := nativeContextFixture(t, test.binding, 9999999, parentContextRecords)
			paths, err := fixture.run.team.Agent("a")
			if err != nil {
				t.Fatal(err)
			}
			locked, err := paths.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer locked.Close()
			agent, err := paths.Read()
			if err != nil {
				t.Fatal(err)
			}
			collar, err := harness.EmbeddedCollar("codex")
			if err != nil {
				t.Fatal(err)
			}
			before := contextStateSnapshot(t, fixture.env["GANG_STATE_ROOT"])
			err = fixture.run.refreshNative(locked, &agent, collar)
			if err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Errorf("expected %q refusal, got %v", test.reason, err)
			}
			saved, err := paths.Read()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved, agent) || !reflect.DeepEqual(contextStateSnapshot(t, fixture.env["GANG_STATE_ROOT"]), before) {
				t.Fatal("native refresh repaired saved state")
			}
		})
	}
}

func TestContextVerifiedNativeReadRejectsUnverifiedMeasurement(t *testing.T) {
	for _, test := range []struct {
		name, records, replacement, reason string
	}{
		{name: "missing", replacement: "remove", reason: "no verified log"},
		{name: "wrong identity", replacement: `{"type":"session_meta","payload":{"id":"other"}}` + "\n", reason: "no verified log"},
		{name: "invalid reading", records: `{"timestamp":"2026-09-22T10:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{}}}` + "\n", reason: "token_count"},
		{name: "no reading", reason: "native context has not been observed"},
		{name: "earlier hitch", records: strings.ReplaceAll(parentContextRecords, "2026-09-22", "2026-09-21"), reason: "native context has not been observed"},
		{name: "native compaction", records: parentContextRecords + `{"timestamp":"2026-09-22T10:00:02Z","type":"event_msg","payload":{"type":"context_compacted"}}` + "\n", reason: "compaction completed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := nativeContextFixture(t, "child.jsonl", 9999999, test.records)
			root := fixture.env["GANG_STATE_ROOT"]
			parent := filepath.Join(root, "rollout-test-parent.jsonl")
			if test.replacement == "remove" {
				if err := os.Remove(parent); err != nil {
					t.Fatal(err)
				}
			} else if test.replacement != "" {
				if err := os.WriteFile(parent, []byte(test.replacement), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := contextStateSnapshot(t, root)
			err := fixture.cmd.context([]string{"worker", "--json"})
			if err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Errorf("expected %q refusal, got %v; output %s", test.reason, err, fixture.out.String())
			}
			if !reflect.DeepEqual(contextStateSnapshot(t, root), before) {
				t.Fatal("failed context read changed saved state or witnesses")
			}
		})
	}
}
