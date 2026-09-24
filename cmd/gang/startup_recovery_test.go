package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func TestStartupTrustSurvivesDeadlineAndKeepsContract(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.Status = core.Booting
	a.Activity = core.Unknown
	a.BootDeadline = f.cmd.now().Add(time.Second)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	e := core.Envelope{ID: "startup-original", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderAgent, Name: "lead", HitchID: "lead-id"}, Message: core.Message{Text: "Standing contract: report completion.\nAssignment: fix it."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("Hooks need review", "› 1. Review hooks", "Press enter to confirm or esc to go back")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	blocked, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != core.Booting || blocked.Activity != core.Blocked || !blocked.BootDeadline.IsZero() || f.input.submits != 0 {
		t.Fatalf("trust state: %+v submits=%d", blocked, f.input.submits)
	}
	start := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return start.Add(time.Hour) }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("READY", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Message.Text != e.Message.Text || f.input.submits != 1 {
		t.Fatalf("startup changed: %+v submits=%d", got, f.input.submits)
	}
}

type submitOnlyFixture struct{ *inputFixture }

func (b submitOnlyFixture) SendKeys(ctx context.Context, pane substrate.PaneID, k substrate.Keys) error {
	if k.Text != "" {
		return fmt.Errorf("recovery attempted to paste")
	}
	return b.inputFixture.SendKeys(ctx, pane, k)
}

func TestRecoverStartupSubmitsOriginalComposerWithoutRepaste(t *testing.T) {
	for _, screen := range []string{"original", "empty", "prompt", "collapsed"} {
		t.Run(screen, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			e := core.Envelope{ID: "startup-original", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "hitch"}, Message: core.Message{Text: "Standing contract: report completion. Assignment: fix it."}, CreatedAt: f.cmd.now()}
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
			f.input.pasted = wire // Input painted before trust took over. Recovery must not paste it again.
			f.input.screen = screenWithText("› " + wire)
			if screen == "empty" {
				f.input.screen = screenWithText("READY", "› ")
			}
			if screen == "collapsed" {
				f.input.screen = screenWithText(fmt.Sprintf("› [Pasted Content %d chars]", len(wire)))
			}
			if screen == "prompt" {
				f.input.screen = screenWithText("Hooks need review", "› 1. Review hooks", "Press enter to confirm or esc to go back")
			}
			f.input.submit = func(prompt string) error {
				return p.WriteWitness(store.Witness{ID: "recovered", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
			}
			f.cmd.inputBackend = submitOnlyFixture{f.input}
			err = f.cmd.hitch([]string{"worker", "--recover"})
			if screen != "original" {
				var ce commandError
				if !errors.As(err, &ce) || (ce.status != exitNative && ce.status != exitUnknown) || f.input.submits != 0 {
					t.Fatalf("unsafe recovery err=%v submits=%d", err, f.input.submits)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.ReadEnvelope("cur", e.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Message.Text != e.Message.Text || f.input.submits != 1 || !strings.Contains(f.out.String(), "delivered") {
				t.Fatalf("recovery: %+v submits=%d output=%s", got, f.input.submits, f.out)
			}
		})
	}
}

func TestPlainSendCannotReplaceUnverifiedStartupContract(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.LastFailed = "startup-original"
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.submit = func(prompt string) error {
		return p.WriteWitness(store.Witness{ID: "replacement", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
	}
	f.cmd.stdin = strings.NewReader("replacement assignment without contract")
	err = f.cmd.send([]string{"worker", "--from", "operator"})
	if err == nil || !strings.Contains(err.Error(), "--recover") || f.input.submits != 0 {
		t.Fatalf("plain replacement err=%v submits=%d", err, f.input.submits)
	}
}

func TestLateStartupTrustCheckedBeforeDeadline(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.Status = core.Booting
	a.BootDeadline = f.cmd.now().Add(-time.Second)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.screen = screenWithText("Hooks need review", "› 1. Review hooks", "Press enter to confirm or esc to go back")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.Booting || got.Activity != core.Blocked || !got.BootDeadline.IsZero() {
		t.Fatalf("late prompt expired: %+v", got)
	}
}
