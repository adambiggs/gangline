package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// A submit hook from another native session in the same harness process,
// one the harness never persists or one it spawned as an agent, leaves the
// agent's registration and witness alone and is retained with the team. A
// different session with a transcript still fails the hitch.
func TestForeignSubmitKeepsRegistration(t *testing.T) {
	for _, test := range []struct {
		name, payload, reason string
		foreign               bool
	}{
		{"missing transcript", `{"hook_event_name":"UserPromptSubmit","session_id":"x","turn_id":"t2","prompt":"Generate a title"}`, "native session x submitted without a transcript", true},
		{"null transcript", `{"hook_event_name":"UserPromptSubmit","session_id":"x","turn_id":"t2","transcript_path":null,"prompt":"Generate a title"}`, "native session x submitted without a transcript", true},
		{"spawned agent", `{"hook_event_name":"UserPromptSubmit","session_id":"x","turn_id":"t2","transcript_path":"/sessions/x.jsonl","agent_id":"x","prompt":"review"}`, "native session x submitted as spawned agent x", true},
		{"replacement", `{"hook_event_name":"UserPromptSubmit","session_id":"x","turn_id":"t2","transcript_path":"/sessions/x.jsonl","prompt":"carry on"}`, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.command = "codex"
			f.input.screen = screenWithText("› ")
			transcript := filepath.Join(t.TempDir(), "s.jsonl")
			a = f.setAgent(t, a, func(a *core.Agent) {
				a.Native.SessionID, a.Native.TurnID, a.Native.Transcript, a.Native.SubmittedAt = "s", "t1", transcript, f.cmd.now()
			})
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(transcript, []byte(`{"type":"session_meta","payload":{"id":"s"}}`+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			registered := store.Witness{ID: "w1", At: f.cmd.now(), SessionID: "s", TurnID: "t1", Transcript: transcript}
			if err := p.WriteWitness(registered); err != nil {
				t.Fatal(err)
			}
			f.cmd.stdin = strings.NewReader(test.payload)
			if err := f.cmd.hook(nil); err != nil || f.errOut.Len() != 0 {
				t.Fatalf("hook: %v %s", err, f.errOut)
			}
			for range 2 {
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatalf("tick: %v", err)
				}
			}
			got := f.agent(t, a.ID)
			retained, retainErr := f.run.team.ReadForeignSubmit(a.ID)
			if !test.foreign {
				if got.Status != core.Failed || !strings.Contains(got.Evidence, "native session changed from s to x") {
					t.Fatalf("replacement left agent %s: %s", got.Status, got.Evidence)
				}
				if !errors.Is(retainErr, os.ErrNotExist) {
					t.Fatalf("replacement retained as foreign: %+v %v", retained, retainErr)
				}
				return
			}
			if got.Status != core.Active || got.Native.SessionID != "s" || got.Native.Transcript != transcript {
				t.Fatalf("foreign submit changed agent: %s session %q: %s", got.Status, got.Native.SessionID, got.Evidence)
			}
			if w, err := p.ReadWitness(); err != nil || w != registered {
				t.Fatalf("witness = %+v %v", w, err)
			}
			if retainErr != nil || retained.Payload != test.payload || retained.SessionID != "x" || retained.Registered != "s" || retained.Reason != test.reason {
				t.Fatalf("retained = %+v %v", retained, retainErr)
			}
			path, err := f.run.team.ForeignSubmitPath(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Dir(filepath.Dir(path)) != f.run.team.Directory {
				t.Fatalf("retained at %s, outside team %s", path, f.run.team.Directory)
			}
			log, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(log), `"status":"foreign-session"`) || !strings.Contains(string(log), test.reason) {
				t.Fatalf("log lacks the foreign submit:\n%s", log)
			}
			f.out.Reset()
			if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{test.reason, path} {
				if !strings.Contains(f.out.String(), want) {
					t.Fatalf("status --why lacks %q:\n%s", want, f.out)
				}
			}
		})
	}
}

// The registered session itself is never foreign, whatever its transcript.
func TestRegisteredSessionWithoutTranscriptIsNotForeign(t *testing.T) {
	a := core.Agent{Native: core.NativeState{SessionID: "s"}}
	event := harness.HookEvent{Kind: string(harness.TurnStarted), Payload: map[string]string{"session_id": "s"}, Absent: map[string]bool{"transcript_path": true}}
	if reason := foreignSubmit(a, event); reason != "" {
		t.Fatalf("registered session classified foreign: %s", reason)
	}
}

// A corrupt retained record is reported by status --why, not a command failure.
func TestStatusWhyReportsUnreadableForeignSubmit(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	path, err := f.run.team.ForeignSubmitPath(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatalf("status --why: %v", err)
	}
	if want := "ignored submit record at " + path + " is unreadable"; !strings.Contains(f.out.String(), want) {
		t.Fatalf("status --why lacks %q:\n%s", want, f.out)
	}
}
