package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type failedStartupPasteInput struct{ *inputFixture }

func (b failedStartupPasteInput) SendKeys(context.Context, substrate.PaneID, substrate.Keys) error {
	return errors.New("paste result unknown")
}

func TestStartupRetryProofRequiresBracketedPaste(t *testing.T) {
	for _, mode := range []string{"", "bracketed"} {
		t.Run(mode, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "startup"}, Purpose: "startup", Message: core.Message{Text: "contract\nassignment"}, CreatedAt: f.cmd.now()}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			l, err := p.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			c, err := harness.EmbeddedCollar("codex")
			if err != nil {
				t.Fatal(err)
			}
			c.Primitives.Submit.Params["paste"] = mode
			f.input.registeredSender = failedStartupPasteInput{f.input}
			outcome, err := f.run.deliver(l, &a, e, f.input, c)
			if err != nil || outcome != "unverified" {
				t.Fatalf("delivery %q: %v", outcome, err)
			}
			got, err := p.ReadEnvelope("failed", e.ID)
			if err != nil || (got.PasteOnly != nil) != (mode == "bracketed") || f.input.submits != 0 {
				t.Fatalf("unsafe retry proof for mode %q: %+v err=%v submits=%d", mode, got, err, f.input.submits)
			}
		})
	}
}

func TestResumedStartupRecoversEmptyComposerOnlyBeforeSubmit(t *testing.T) {
	for _, test := range []struct {
		name                                   string
		proof, changed, busy, foreign, missing bool
		want                                   bool
	}{
		{name: "paste failed before submit", proof: true, want: true},
		{name: "submission unknown"},
		{name: "another prompt submitted", proof: true, changed: true},
		{name: "native work running", proof: true, busy: true},
		{name: "foreign session witness", proof: true, foreign: true},
		{name: "missing session witness", proof: true, missing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, p, e, b, wire := newCollapsedRecoveryFixture(t)
			a, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			a.Native.SessionID = "resumed-session"
			l, err := p.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			// Encode the receipt directly so this regression also runs against
			// the implementation that did not retain pre-submit proof.
			path, err := p.EnvelopePath("failed", e.ID)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var receipt map[string]any
			if err := json.Unmarshal(data, &receipt); err != nil {
				t.Fatal(err)
			}
			if test.proof {
				receipt["paste_only"] = map[string]string{"witness_id": ""}
			}
			data, err = json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			b.pasted = ""
			b.screen = screenWithText("READY", "› ")
			if test.busy {
				b.screen = screenWithText("esc to interrupt", "› ")
			}
			if test.changed {
				if err := p.WriteWitness(store.Witness{ID: "operator", At: f.cmd.now(), Prompt: "operator prompt", SessionID: "resumed-session"}); err != nil {
					t.Fatal(err)
				}
			}
			b.submit = func(prompt string) error {
				pending, err := p.ReadEnvelope("new", e.ID)
				if err != nil {
					return err
				}
				data, err := json.Marshal(pending)
				if err != nil {
					return err
				}
				if strings.Contains(string(data), "paste_only") {
					t.Fatal("Enter attempted before clearing pre-submit proof")
				}
				session := "resumed-session"
				if test.foreign {
					session = "another-session"
				}
				if test.missing {
					session = ""
				}
				return p.WriteWitness(store.Witness{ID: "recovered", At: f.cmd.now(), Prompt: prompt, SessionID: session})
			}
			err = f.cmd.hitch([]string{"worker", "--recover"})
			if test.want {
				got, readErr := p.ReadEnvelope("cur", e.ID)
				if err != nil || readErr != nil || got.Outcome != "delivered" || b.submits != 1 || b.repastes != 1 || b.clears != 0 || b.pasted != wire {
					t.Fatalf("recovery err=%v receipt=%+v read=%v submits=%d pastes=%d clears=%d", err, got, readErr, b.submits, b.repastes, b.clears)
				}
			} else {
				got, readErr := p.ReadEnvelope("failed", e.ID)
				if err == nil || readErr != nil || got.Outcome != "unverified" || !test.foreign && !test.missing && (b.submits != 0 || b.repastes != 0) {
					t.Fatalf("unsafe recovery err=%v receipt=%+v read=%v submits=%d pastes=%d", err, got, readErr, b.submits, b.repastes)
				}
			}
		})
	}
}

func TestStartupPasteProofSurvivesInputOwnerExit(t *testing.T) {
	f, p, e, _, _ := newCollapsedRecoveryFixture(t)
	e.PasteOnly = &core.StartupPaste{WitnessID: "before"}
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.reopenUnverified(l, &a, e); err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.recoverInput(l, &a); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("failed", e.ID)
	if err != nil || got.PasteOnly == nil || got.PasteOnly.WitnessID != "before" {
		t.Fatalf("lost proof: %+v %v", got, err)
	}
	if err := f.run.reopenUnverified(l, &a, got); err != nil {
		t.Fatal(err)
	}
	if err := startupSubmitPossible(p, &got); err != nil {
		t.Fatal(err)
	}
	if err := f.run.recoverInput(l, &a); err != nil {
		t.Fatal(err)
	}
	got, err = p.ReadEnvelope("failed", e.ID)
	if err != nil || got.PasteOnly != nil {
		t.Fatalf("submit intent regained retry authority: %+v %v", got, err)
	}
}

func TestStartupEnterErrorDoesNotAllowEmptyRetry(t *testing.T) {
	f, p, e, b, _ := newCollapsedRecoveryFixture(t)
	e.PasteOnly = &core.StartupPaste{}
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.reopenUnverified(l, &a, e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.finishInput(l, &a, e, "unverified", "paste failed"); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	b.screen = screenWithText("READY", "› ")
	b.submit = func(string) error { return errors.New("Enter result unknown") }
	if err := f.cmd.hitch([]string{"worker", "--recover"}); err == nil || !strings.Contains(err.Error(), "Enter result unknown") || !strings.Contains(err.Error(), "retained startup:") || b.submits != 1 {
		t.Fatalf("Enter uncertainty: %v, submits=%d", err, b.submits)
	}
	b.screen = screenWithText("READY", "› ")
	if err := f.cmd.hitch([]string{"worker", "--recover"}); err == nil || b.submits != 1 || b.repastes != 1 {
		t.Fatalf("uncertain Enter retried: %v, submits=%d pastes=%d", err, b.submits, b.repastes)
	}
}
