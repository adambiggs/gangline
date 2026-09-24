package main

import (
	"fmt"
	"os"
	"strings"
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

func TestSendReportsReceiptReconciledAfterUnlock(t *testing.T) {
	for _, initial := range []string{"unverified", "accepted"} {
		for _, exact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/exact=%t", initial, exact), func(t *testing.T) {
				f, a, p := queueSendFixture(t)
				f.input.submit = func(wire string) error {
					if initial == "accepted" {
						f.input.screen = nativeQueueScreen(wire)
					}
					return nil
				}
				f.cmd.afterUnlock = func() {
					prompt := f.input.pasted
					if !exact {
						prompt = "unrelated message"
					}
					if err := p.WriteWitness(store.Witness{ID: "late-hook", At: f.cmd.now(), SessionID: "s", Prompt: prompt}); err != nil {
						t.Fatal(err)
					}
					l, _, err := f.run.acquire(a.ID, false)
					if err != nil {
						t.Fatal(err)
					}
					if err := l.Close(); err != nil {
						t.Fatal(err)
					}
				}
				err := f.cmd.send([]string{"worker", "--from", "operator"})
				fields := strings.Fields(f.out.String())
				if len(fields) != 2 {
					t.Fatalf("output=%q error=%v", f.out.String(), err)
				}
				want := initial
				if exact {
					want = "delivered"
				}
				if fields[1] != want || (err != nil) != (want == "unverified") {
					t.Fatalf("output=%q error=%v; want %s", f.out.String(), err, want)
				}
				log, err := os.Open(f.run.team.Log)
				if err != nil {
					t.Fatal(err)
				}
				defer log.Close()
				final, succeeded := "", false
				if err := store.ReadLog(log, func(e core.Event) error {
					if e.ID == fields[0] {
						if e.Type == "delivery_succeeded" {
							succeeded = true
						}
						if e.Type == "input_finished" {
							final = e.Status
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if final != fields[1] || succeeded != exact || f.input.submits != 1 {
					t.Fatalf("logged=%s succeeded=%t printed=%s submits=%d", final, succeeded, fields[1], f.input.submits)
				}
			})
		}
	}
}
