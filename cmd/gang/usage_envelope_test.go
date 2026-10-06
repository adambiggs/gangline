package main

import (
	"os"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestRetainedUsageEnvelopeRecordsKeyboardSuffix(t *testing.T) {
	for _, kind := range []string{"usage-band", "snooze"} {
		t.Run(kind, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			e := core.Envelope{ID: "retained", Token: "aaaaaaaaaaaaaaaa", From: core.Sender{Kind: core.SenderGangline, Name: core.AgentName(kind)}, Message: core.Message{Text: "Resume saved work."}}
			if err := f.run.withUsageState(func(state *usageState) error {
				if kind == "usage-band" {
					state.Notices = []usageNotice{{ID: e.ID, Token: e.Token, RecipientID: a.ID, Submission: "unverified", Text: e.Message.Text, CreatedAt: f.cmd.now()}}
				} else {
					state.Snoozes[string(a.ID)] = usageSnooze{ID: e.ID, Token: e.Token, CallerID: a.ID, RecipientID: a.ID, Submission: "accepted", InputText: e.Message.Text, At: f.cmd.now()}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			wire := mustEnvelopeText(t, e)
			if err := p.WriteWitness(store.Witness{ID: "hook", At: f.cmd.now(), Prompt: wire + "s", SessionID: "s", TurnID: "turn"}); err != nil {
				t.Fatal(err)
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			// There is no inbox receipt: later input has displaced it.
			for range 2 {
				if err := f.run.reconcileUsageSubmission(l, &a); err != nil {
					t.Fatal(err)
				}
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			state := usageSnapshot(t, f.run)
			if len(state.Notices) != 0 || len(state.Snoozes) != 0 || kind == "snooze" && state.Recent[string(a.ID)].TurnID != "turn" {
				t.Fatalf("retained submission did not match: %+v", state)
			}
			log, err := os.Open(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			observations := 0
			if err := store.ReadLog(log, func(event core.Event) error {
				if event.Type == "native_hook" && event.Status == "usage-submission" && event.ID == string(e.ID) && event.Reason == `session keyboard input outside gang envelope: "s"` {
					observations++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if observations != 1 {
				t.Fatalf("outside-byte observations=%d; want one", observations)
			}
		})
	}
}
