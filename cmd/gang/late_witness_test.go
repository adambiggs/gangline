package main

import (
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestLateSubmitWitnessReconcilesWithoutRetyping(t *testing.T) {
	for _, kind := range []string{"exact", "different text", "different session"} {
		t.Run(kind, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			a.Native.SessionID = "s"
			e := core.Envelope{ID: "late", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "queued during native work"}, CreatedAt: f.cmd.now()}
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
			w := store.Witness{ID: "late-hook", SessionID: "s", Prompt: wire, At: f.cmd.now().Add(time.Hour)}
			if kind == "different text" {
				w.Prompt = "some other message"
			}
			if kind == "different session" {
				w.SessionID = "foreign"
			}
			if err := p.WriteWitness(w); err != nil {
				t.Fatal(err)
			}
			// Acquisition is the recovery boundary; no native input is required.
			l, a, err = f.run.acquire(a.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			l.Close()
			if kind == "exact" {
				got, err := p.ReadEnvelope("cur", e.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.Outcome != "delivered" || a.LastFailed != "" {
					t.Fatalf("late receipt: %+v %+v", got, a)
				}
			} else if a.LastFailed != e.ID {
				t.Fatalf("unrelated witness cleared uncertainty: %+v", a)
			}
			if f.input.pasted != "" || f.input.submits != 0 {
				t.Fatal("late witness retyped input")
			}
		})
	}
}
