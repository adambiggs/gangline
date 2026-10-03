package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// movedSessionFixture is a Claude agent hitched by lead l, tracking session s
// through its transcript s.jsonl, whose submit hook then witnessed prompt p2 in
// session next. It returns the agent and next's transcript path.
func movedSessionFixture(t *testing.T, next string, oldLines ...string) (*stateFixture, core.Agent, string) {
	t.Helper()
	f, a, start := openTurnFixture(t)
	f.addHitched(t, "l", "lead", "lead", "")
	dir := t.TempDir()
	old := filepath.Join(dir, "s.jsonl")
	appendLines(t, old, oldLines...)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.HitchedBy, a.Role = "l", "builder"
		a.Native.SessionID, a.Native.TurnID, a.Native.Transcript, a.Native.SubmittedAt = "s", "p1", old, start
	})
	transcript := filepath.Join(dir, next+".jsonl")
	appendLines(t, transcript, `{"type":"user","sessionId":"`+next+`","promptId":"p2"}`)
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "w2", At: start.Add(time.Second), SessionID: next, TurnID: "p2", Transcript: transcript}); err != nil {
		t.Fatal(err)
	}
	return f, a, transcript
}

// failureNotices are the failure notices lead l holds about agent a.
func failureNotices(t *testing.T, f *stateFixture) []core.Envelope {
	t.Helper()
	lp, err := f.run.team.Agent("l")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(lp.Inbox, "new"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var found []core.Envelope
	for _, entry := range entries {
		e, err := lp.ReadEnvelope("new", core.EnvelopeID(strings.TrimSuffix(entry.Name(), ".json")))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(e.ID), "hitch-failed-a-") {
			found = append(found, e)
		}
	}
	return found
}

// Claude Code moves a conversation to a background session under a new id
// and ends the old transcript with a record naming it; the pane's composer
// then submits into the new session.
func TestTickFollowsRecordedSessionMove(t *testing.T) {
	f, a, transcript := movedSessionFixture(t, "n",
		`{"type":"user","sessionId":"s","promptId":"p1"}`,
		`{"type":"continued-in","sessionId":"s","continuedInSessionId":"n"}`)
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatalf("tick after a recorded move: %v", err)
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Active || got.Native.SessionID != "n" || got.Native.Transcript != transcript || got.Native.TurnID != "p2" {
		t.Fatalf("agent = %s session %q transcript %q turn %q", got.Status, got.Native.SessionID, got.Native.Transcript, got.Native.TurnID)
	}
	if got.Activity != core.Busy {
		t.Fatalf("moved turn read %s: %s", got.Activity, got.Evidence)
	}
	at := f.cmd.now().Add(2 * time.Second)
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-finished", SessionID: "n", TurnID: "p2", Transcript: transcript, At: at}, false); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Activity != core.Idle {
		t.Fatalf("finished moved turn read %s: %s", got.Activity, got.Evidence)
	}
}

// A session no harness record ties to the agent's conversation may be another
// conversation entirely, so the agent fails once and its hitcher learns both
// sessions, instead of every tick failing unseen.
func TestTickFailsAgentOnUnrecordedSessionChange(t *testing.T) {
	f, a, _ := movedSessionFixture(t, "x",
		`{"type":"user","sessionId":"s","promptId":"p1"}`,
		`{"type":"continued-in","sessionId":"s","continuedInSessionId":"n"}`)
	for range 2 {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatalf("tick after an unrecorded session change: %v", err)
		}
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Failed || got.Native.SessionID != "s" {
		t.Fatalf("agent = %s session %q: %s", got.Status, got.Native.SessionID, got.Evidence)
	}
	notices := failureNotices(t, f)
	if len(notices) != 1 {
		t.Fatalf("notices = %+v", notices)
	}
	route := fmt.Sprintf("gang drop worker, then gang hitch worker -c claude -d %q -r builder --resume SESSION", got.Directory)
	for _, want := range []string{"native session changed from s to x", route} {
		if !strings.Contains(notices[0].Message.Text, want) {
			t.Fatalf("notice %q lacks %q", notices[0].Message.Text, want)
		}
	}
}

// A message the composer submits into the session the agent's conversation
// moved to is delivered; one submitted into an unrecorded session stays
// unverified.
func TestDeliveryFollowsRecordedSessionMove(t *testing.T) {
	for _, test := range []struct {
		session, want string
	}{{"n", "delivered"}, {"x", "unverified"}} {
		t.Run(test.session, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.command = "claude"
			f.input.screen = screenWithText("────────────────", "❯ ", "────────────────")
			dir := t.TempDir()
			old := filepath.Join(dir, "s.jsonl")
			appendLines(t, old, `{"type":"user","sessionId":"s","promptId":"p1"}`, `{"type":"continued-in","sessionId":"s","continuedInSessionId":"n"}`)
			f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID, a.Native.Transcript = "s", old })
			transcript := filepath.Join(dir, test.session+".jsonl")
			f.input.submit = func(prompt string) error {
				payload, _ := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt, "session_id": test.session, "transcript_path": transcript})
				hook := f.cmd
				hook.stdin = bytes.NewReader(payload)
				return hook.hook(nil)
			}
			f.cmd.stdin = strings.NewReader("check the build")
			err := f.cmd.send([]string{"worker", "--from", "operator"})
			if !strings.Contains(f.out.String(), test.want) {
				t.Fatalf("send: %v output=%s errors=%s", err, f.out, f.errOut)
			}
			if got := f.agent(t, a.ID); test.want == "delivered" && (got.Native.SessionID != "n" || got.Native.Transcript != transcript) {
				t.Fatalf("delivered into session %q transcript %q", got.Native.SessionID, got.Native.Transcript)
			}
		})
	}
}

// A receipt left unverified is delivered when the submit witness that matches
// it names the session the agent's conversation moved to, and stays
// unverified when it names an unrecorded session.
func TestLateWitnessFollowsRecordedSessionMove(t *testing.T) {
	for _, session := range []string{"n", "x"} {
		t.Run(session, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			lead := f.add(t, "b", "lead", "claude")
			p, _ := f.run.team.Agent(a.ID)
			dir := t.TempDir()
			old := filepath.Join(dir, "s.jsonl")
			appendLines(t, old, `{"type":"user","sessionId":"s","promptId":"p1"}`, `{"type":"continued-in","sessionId":"s","continuedInSessionId":"n"}`)
			a.Native.SessionID, a.Native.Transcript = "s", old
			e := core.Envelope{ID: "late", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: agentSender(lead), Message: core.Message{Text: "typed before the move settled"}, CreatedAt: f.cmd.now()}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
			if err := f.run.finishInput(l, &a, e, "unverified", "context deadline exceeded"); err != nil {
				t.Fatal(err)
			}
			l.Close()
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(dir, session+".jsonl")
			if err := p.WriteWitness(store.Witness{ID: "late-hook", SessionID: session, Transcript: transcript, Prompt: wire, At: f.cmd.now().Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
			l, a, err = f.run.acquire(a.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			l.Close()
			if session == "x" {
				if a.LastFailed != e.ID || a.Native.SessionID != "s" {
					t.Fatalf("unrecorded session cleared uncertainty: %+v", a)
				}
				return
			}
			got, err := p.ReadEnvelope("cur", e.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != "delivered" || a.LastFailed != "" || a.Native.SessionID != "n" || a.Native.Transcript != transcript {
				t.Fatalf("late receipt: %+v %+v", got, a)
			}
		})
	}
}

// Startup recovery delivers the original startup message when its submit
// witness names the session the agent's conversation moved to, and leaves it
// unverified when it names an unrecorded session.
func TestRecoverStartupFollowsRecordedSessionMove(t *testing.T) {
	for _, test := range []struct {
		session, want string
	}{{"n", "delivered"}, {"x", "unverified"}} {
		t.Run(test.session, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			lead := f.add(t, "b", "lead", "claude")
			p, _ := f.run.team.Agent(a.ID)
			dir := t.TempDir()
			old := filepath.Join(dir, "s.jsonl")
			appendLines(t, old, `{"type":"user","sessionId":"s","promptId":"p1"}`, `{"type":"continued-in","sessionId":"s","continuedInSessionId":"n"}`)
			a.Native.SessionID, a.Native.Transcript = "s", old
			e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: agentSender(lead), Purpose: "assignment", Message: core.Message{Text: "Standing contract: report completion. Assignment: fix it."}, CreatedAt: f.cmd.now()}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
			if err := f.run.finishInput(l, &a, e, "unverified", "another surface owns input"); err != nil {
				t.Fatal(err)
			}
			l.Close()
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			f.input.command = "claude"
			f.input.pasted = wire
			f.input.screen = screenWithText("────────────────", "❯ "+wire, "────────────────")
			transcript := filepath.Join(dir, test.session+".jsonl")
			f.input.submit = func(prompt string) error {
				return p.WriteWitness(store.Witness{ID: "recovered", At: f.cmd.now(), Prompt: prompt, SessionID: test.session, Transcript: transcript})
			}
			f.cmd.inputBackend = submitOnlyFixture{f.input}
			err = f.cmd.hitch([]string{"worker", "--recover"})
			if !strings.Contains(f.out.String(), test.want) && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("recover: %v output=%s", err, f.out)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "delivered" && (got.Native.SessionID != "n" || got.Native.Transcript != transcript) {
				t.Fatalf("delivered into session %q transcript %q", got.Native.SessionID, got.Native.Transcript)
			}
			if test.want == "unverified" && got.Native.SessionID != "s" {
				t.Fatalf("unverified recovery adopted session %q", got.Native.SessionID)
			}
		})
	}
}
