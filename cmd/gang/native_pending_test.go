//go:build acceptance

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

func TestNativePendingBatchConfirmsEverySenderAfterReceiptCleanup(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
	sender := f.add(t, "sender", "lead", "codex")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	var attempts []store.NativePending
	for i := range 3 {
		e := core.Envelope{ID: core.EnvelopeID(fmt.Sprintf("native-%d", i)), Token: fmt.Sprintf("0123456789abcde%d", i), Recipient: a.ID, To: a.Name, From: agentSender(sender), Message: core.Message{Text: fmt.Sprint(i)}, CreatedAt: f.cmd.now()}
		wire, err := envelopeText(e)
		if err != nil {
			t.Fatal(err)
		}
		n := store.NativePending{Envelope: e, SessionID: "s", Prompt: claudePeerBody(wire)}
		if err := p.Publish(e); err != nil {
			t.Fatal(err)
		}
		if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
			t.Fatal(err)
		}
		if err := p.BeginNative(n); err != nil {
			t.Fatal(err)
		}
		attempts = append(attempts, n)
		// Last attempt crashes before settling, as after a partial socket write.
		if i < 2 {
			if err := f.run.finishInput(l, &a, e, "unverified", "write uncertain"); err != nil {
				t.Fatal(err)
			}
		}
	}
	l.Close()
	// All hooks arrive before any tick, in reverse order. Each must survive
	// replacement of the ordinary witness, including the still-settling sender.
	for i := len(attempts) - 1; i >= 0; i-- {
		n := attempts[i]
		if err := p.WriteNativeWitness(store.Witness{ID: fmt.Sprint(i), At: f.cmd.now(), SessionID: "s", Prompt: n.Prompt}, "s"); err != nil {
			t.Fatal(err)
		}
	}
	// A later unrelated hook must not replace the per-message evidence.
	if err := p.WriteWitness(store.Witness{ID: "later", At: f.cmd.now(), SessionID: "s", Prompt: "other input"}); err != nil {
		t.Fatal(err)
	}
	f.run.cmd = f.cmd
	l, a, err = f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	for _, n := range attempts {
		deliveredNotice(t, f, sender, n.Envelope.ID)
	}
	if pending, err := p.ListNativePending(); err != nil || len(pending) != 0 {
		t.Fatalf("pending=%d error=%v", len(pending), err)
	}
	audit, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"type":"delivery_succeeded"`) != 3 {
		t.Fatalf("batch confirmations: %s", audit)
	}
	// Repeated acquisition neither sends bytes nor republishes outcomes.
	l, _, err = f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	after, _ := os.ReadFile(f.run.team.Log)
	if strings.Count(string(after), `"type":"delivery_succeeded"`) != 3 || f.input.submits != 0 || f.input.pasted != "" {
		t.Fatal("confirmation replayed input or outcomes")
	}
}

func TestNativePendingRejectsWrongSessionAndPartialBody(t *testing.T) {
	for _, kind := range []string{"session", "body", "suffix"} {
		t.Run(kind, func(t *testing.T) {
			f, a, p := claudeRecipient(t)
			a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
			e := core.Envelope{ID: "pending", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, Message: core.Message{Text: "full message"}, CreatedAt: f.cmd.now()}
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			n := store.NativePending{Envelope: e, SessionID: "s", Prompt: claudePeerBody(wire)}
			if err := p.BeginNative(n); err != nil {
				t.Fatal(err)
			}
			w := store.Witness{ID: "hook", At: f.cmd.now(), SessionID: "s", Prompt: n.Prompt}
			switch kind {
			case "session":
				w.SessionID = "other"
			case "body":
				w.Prompt = wire
			case "suffix":
				w.Prompt += "extra"
			}
			if err := p.WriteNativeWitness(w, "s"); err != nil {
				t.Fatal(err)
			}
			saved, err := p.ReadNativePending(e.ID)
			if err != nil || saved.Witness != nil {
				t.Fatalf("unrelated hook confirmed: %+v %v", saved, err)
			}
		})
	}
}

func TestNativePendingRetiresOnlyUnwitnessedOldSessions(t *testing.T) {
	for _, witnessed := range []bool{false, true} {
		t.Run(fmt.Sprint(witnessed), func(t *testing.T) {
			f, a, p := claudeRecipient(t)
			a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
			e := core.Envelope{ID: "old", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, Message: core.Message{Text: "old message"}, CreatedAt: f.cmd.now()}
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			n := store.NativePending{Envelope: e, SessionID: "s", Prompt: claudePeerBody(wire)}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := f.run.apply(l, &a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
				t.Fatal(err)
			}
			if err := p.BeginNative(n); err != nil {
				t.Fatal(err)
			}
			if err := f.run.finishInput(l, &a, e, "unverified", "native write uncertain"); err != nil {
				t.Fatal(err)
			}
			if witnessed {
				if err := p.WriteNativeWitness(store.Witness{ID: "old-hook", At: f.cmd.now(), SessionID: "s", Prompt: n.Prompt}, "s"); err != nil {
					t.Fatal(err)
				}
			}
			a.Native.SessionID = "replacement"
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			f.run.cmd = f.cmd
			if err := f.run.reconcileNativePending(l, &a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			pending, err := p.ListNativePending()
			if err != nil || len(pending) != 0 {
				t.Fatalf("retired pending=%d err=%v", len(pending), err)
			}
			dir, want := "failed", "unverified"
			if witnessed {
				dir, want = "cur", "delivered"
			}
			receipt, err := p.ReadEnvelope(dir, e.ID)
			if err != nil || receipt.Outcome != want {
				t.Fatalf("receipt=%+v err=%v", receipt, err)
			}
			if a.Native.SessionID != "replacement" {
				t.Fatal("old proof regressed current session")
			}
		})
	}
}

func TestNativePendingConfirmationKeepsLatestWitnessedTurn(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Native.SessionID = "s"
		a.Native.TurnID = "previous"
		a.Native.SubmittedAt = f.cmd.now().Add(-time.Minute)
	})
	e := core.Envelope{ID: "turn", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, Message: core.Message{Text: "new turn"}, CreatedAt: f.cmd.now().Add(-time.Second)}
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	n := store.NativePending{Envelope: e, SessionID: "s", Prompt: claudePeerBody(wire)}
	if err := p.BeginNative(n); err != nil {
		t.Fatal(err)
	}
	w := store.Witness{ID: "new-hook", At: f.cmd.now().Add(-time.Millisecond), SessionID: "s", TurnID: "current", Prompt: n.Prompt}
	if err := p.WriteNativeWitness(w, "s"); err != nil {
		t.Fatal(err)
	}
	f.run.cmd = f.cmd
	l, a, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	if a.Native.TurnID != "current" || !a.Native.SubmittedAt.Equal(w.At) || a.Activity != core.Busy {
		t.Fatalf("confirmation lost current turn: %+v", a.Native)
	}
}

func TestNativePendingRetriesAuditAfterReceiptWasSaved(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
	e := core.Envelope{ID: "partial-settlement", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, Message: core.Message{Text: "confirmed"}, CreatedAt: f.cmd.now()}
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	n := store.NativePending{Envelope: e, SessionID: "s", Prompt: claudePeerBody(wire)}
	if err := p.BeginNative(n); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteNativeWitness(store.Witness{ID: "hook", At: f.cmd.now(), SessionID: "s", Prompt: n.Prompt}, "s"); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	// Exit boundary: the delivered receipt is durable, but the audit append
	// and sender notification have not run.
	if err := l.Settle(&a, e, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.run.cmd = f.cmd
	l, _, err = f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	audit, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"type":"delivery_succeeded"`) != 1 {
		t.Fatalf("missing confirmation audit: %s", audit)
	}
	if pending, err := p.ListNativePending(); err != nil || len(pending) != 0 {
		t.Fatalf("pending=%d err=%v", len(pending), err)
	}
}

func TestNativePendingResetsOnlyCurrentTakeoverCapacityEpisode(t *testing.T) {
	for _, kind := range []string{"takeover", "historical", "new failure"} {
		t.Run(kind, func(t *testing.T) {
			f, a, p := claudeRecipient(t)
			a = f.setAgent(t, a, func(a *core.Agent) {
				a.Native.SessionID = "s"
				a.Native.TurnID = "previous"
				a.Native.SubmittedAt = f.cmd.now().Add(-time.Minute)
				a.Capacity = core.Capacity{Fingerprint: "old-episode", Deadline: f.cmd.now().Add(-time.Second), Attempts: 3}
				if kind == "new failure" {
					a.Capacity.Fingerprint = "new-episode"
				}
			})
			e := core.Envelope{ID: "takeover", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, Message: core.Message{Text: "new input"}, CreatedAt: f.cmd.now().Add(-time.Second)}
			wire, err := envelopeText(e)
			if err != nil {
				t.Fatal(err)
			}
			n := store.NativePending{Envelope: e, SessionID: "s", Prompt: claudePeerBody(wire), CapacityFingerprint: "old-episode"}
			if err := p.BeginNative(n); err != nil {
				t.Fatal(err)
			}
			w := store.Witness{ID: "takeover-hook", At: f.cmd.now().Add(-time.Millisecond), SessionID: "s", TurnID: "takeover-turn", Prompt: n.Prompt}
			if err := p.WriteNativeWitness(w, "s"); err != nil {
				t.Fatal(err)
			}
			if kind == "historical" {
				if err := p.WriteNativeWitness(store.Witness{ID: "later", At: f.cmd.now(), SessionID: "s", TurnID: "later-turn", Prompt: "later input"}, "s"); err != nil {
					t.Fatal(err)
				}
			}
			f.run.cmd = f.cmd
			l, a, err := f.run.acquire(a.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			if kind == "takeover" {
				if a.Capacity.Fingerprint != "" || !a.Capacity.Deadline.IsZero() || a.Capacity.Attempts != 0 {
					t.Fatalf("old budget retained: %+v", a.Capacity)
				}
				if err := f.run.apply(l, &a, core.Event{Type: "capacity_detected", Fingerprint: "next", Deadline: f.cmd.now().Add(time.Minute)}); err != nil {
					t.Fatal(err)
				}
				if a.Activity == core.Wedged || a.Capacity.Deadline.Before(f.cmd.now()) {
					t.Fatalf("fresh failure inherited expired budget: %+v", a.Capacity)
				}
			} else if a.Capacity.Fingerprint == "" || a.Capacity.Attempts != 3 {
				t.Fatalf("historical confirmation reset recovery: %+v", a.Capacity)
			}
		})
	}
}
