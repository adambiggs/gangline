package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// A message paste abandoned before its submit key is withdrawn from a
// composer that reads clearly, and the message fails rather than staying
// unverified in front of the recipient. A paste behind a native prompt, a
// composer the paste already left, and a paste whose submit key was sent are
// left alone, as is a composer something else emptied, since what it holds
// then may not be the paste. The recipient's hitcher is told of a withdrawn
// message, which is lost, and of a paste that may remain, since later messages
// wait behind it.
func TestDeliveryWithdrawsPasteAbandonedBeforeSubmit(t *testing.T) {
	empty := screenWithText("────────", "❯ ", "────────")
	draft := screenWithText("────────", "❯ operator draft", "────────")
	clipped := screenWithText("────────", "reply", "────────", "❯ pasted")
	trust := screenWithText("Quick safety check: Is this a project you created or one you trust?", "❯ 1. Yes, I trust this folder", "  2. No, exit")
	neverSettled := fmt.Errorf("native composer did not settle before submission: %w", context.DeadlineExceeded)
	for _, tc := range []struct {
		name        string
		collar      string
		settle      error
		after       func(pasted string) substrate.Screen
		submitErr   error
		wantStatus  int
		wantOutcome string
		wantReason  string
		wantClear   bool
		wantNotice  string
	}{
		{name: "composer never settled", settle: neverSettled, after: pasteShown, wantOutcome: "failed", wantReason: "the pasted input was withdrawn from the composer", wantClear: true, wantNotice: "withdrawn-"},
		{name: "submit key failed", after: pasteShown, submitErr: errors.New("send-keys failed"), wantStatus: exitUnknown, wantOutcome: "unverified", wantReason: "send-keys failed"},
		{name: "input blocked", after: func(string) substrate.Screen { return trust }, wantStatus: exitUnknown, wantOutcome: "unverified", wantReason: "the pasted input may remain in the composer behind a native prompt", wantNotice: "held-input-"},
		{name: "composer emptied then refilled", settle: harness.ErrComposerEmptied, after: func(string) substrate.Screen { return draft }, wantStatus: exitUnknown, wantOutcome: "unverified", wantReason: "emptied before submission"},
		{name: "collar without clear keys", collar: "codex", settle: neverSettled, after: func(pasted string) substrate.Screen {
			return screenWithText("READY", "› "+strings.ReplaceAll(pasted, "\n", " "))
		}, wantStatus: exitUnknown, wantOutcome: "unverified", wantReason: "the pasted input remains in the composer and the collar declares no clear keys", wantNotice: "held-input-"},
		{name: "composer clipped", settle: neverSettled, after: func(string) substrate.Screen { return clipped }, wantStatus: exitUnknown, wantOutcome: "unverified", wantReason: "the pasted input may remain in the composer; composer unread: ", wantNotice: "held-input-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			lead := f.addHitched(t, "l", "lead", "lead", "")
			collar := tc.collar
			if collar == "" {
				collar = "claude"
			}
			a := f.add(t, "a", "worker", collar)
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			a.HitchedBy = lead.ID
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			var woken []string
			f.cmd.detach = func(id string, _ hookNotice) error { woken = append(woken, id); return nil }
			f.input.command = collar
			f.input.screen = empty
			if collar == "codex" {
				f.input.screen = screenWithText("READY", "› ")
			}
			f.input.onKeys = func(k substrate.Keys) error {
				if k.Text != "" {
					f.input.screen = tc.after(strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~"))
				}
				if k.Submit && tc.submitErr != nil {
					return tc.submitErr
				}
				if slices.Contains(k.Names, "C-u") {
					f.input.screen = empty
				}
				return nil
			}
			// The paste's settle fails; the clear's own wait succeeds.
			settle := tc.settle
			f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
				err := settle
				settle = nil
				return err
			}
			f.run.cmd = f.cmd
			f.cmd.stdin = strings.NewReader("hello")
			err = f.cmd.send([]string{"worker", "--from", "operator"})
			if ce := (commandError{}); tc.wantStatus == 0 && err != nil || tc.wantStatus != 0 && (!errors.As(err, &ce) || ce.status != tc.wantStatus) {
				t.Fatalf("send result: %v out=%q keys=%v", err, f.out, f.input.keys)
			}
			id := core.EnvelopeID(strings.SplitN(f.out.String(), "\t", 2)[0])
			e, err := p.ReadEnvelope("failed", id)
			if err != nil {
				t.Fatalf("receipt: %v (stdout %q)", err, f.out)
			}
			if e.Outcome != tc.wantOutcome || !strings.Contains(e.Reason, tc.wantReason) {
				t.Fatalf("receipt %s: %q", e.Outcome, e.Reason)
			}
			if cleared := slices.Contains(f.input.keys, "C-u"); cleared != tc.wantClear {
				t.Fatalf("clear keys sent=%v, want %v", cleared, tc.wantClear)
			}
			if tc.submitErr == nil && f.input.submits != 0 {
				t.Fatalf("Enter reached the pane %d times", f.input.submits)
			}
			lp, _ := f.run.team.Agent(lead.ID)
			notice, err := lp.ReadEnvelope("new", core.EnvelopeID(tc.wantNotice+string(id)))
			if tc.wantNotice == "" {
				if !errors.Is(err, os.ErrNotExist) || len(woken) != 0 {
					t.Fatalf("unexpected hitcher notice: %+v %v woken=%v", notice, err, woken)
				}
				return
			}
			if err != nil {
				t.Fatalf("hitcher notice: %v", err)
			}
			if notice.From.Kind != core.SenderGangline || !strings.Contains(notice.Message.Text, map[string]string{"held-input-": "A message to worker, which you hitched, is unverified: ", "withdrawn-": "to worker, which you hitched, was withdrawn from its composer and not delivered: "}[tc.wantNotice]) || !strings.Contains(notice.Message.Text, tc.wantReason) {
				t.Fatalf("hitcher notice = %+v", notice)
			}
			if len(woken) != 1 || woken[0] != "l" {
				t.Fatalf("woken = %v", woken)
			}
		})
	}
}

func pasteShown(pasted string) substrate.Screen {
	return screenWithText("────────", "❯ "+strings.ReplaceAll(pasted, "\n", " "), "────────")
}

// Gangline's own input is never withdrawn, since a usage wake cannot
// republish a failed receipt under its ID, and neither is an assignment, which
// startup recovery resubmits from the composer. After a withdrawn message the
// drain stops, so a composer that keeps refusing pastes does not fail the
// whole queue in one pass.
func TestWithdrawalSparesGanglineInputAndStopsDrain(t *testing.T) {
	agent := core.Sender{Kind: core.SenderAgent, Name: "sender", HitchID: "s"}
	gangline := core.Sender{Kind: core.SenderGangline, Name: "usage-band"}
	for _, tc := range []struct {
		name       string
		from       core.Sender
		purpose    string
		wantPastes int
		wantQueued int
		wantClear  bool
	}{
		{name: "messages", from: agent, wantPastes: 1, wantQueued: 1, wantClear: true},
		{name: "gangline notices", from: gangline, wantPastes: 1, wantQueued: 1},
		{name: "assignments from an agent", from: agent, purpose: "assignment", wantPastes: 1, wantQueued: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			f.add(t, "s", "sender", "codex")
			a := f.add(t, "a", "worker", "claude")
			p, _ := f.run.team.Agent(a.ID)
			empty := screenWithText("────────", "❯ ", "────────")
			f.input.command = "claude"
			f.input.screen = empty
			pastes := 0
			f.input.onKeys = func(k substrate.Keys) error {
				if k.Text != "" {
					pastes++
					f.input.screen = pasteShown(strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~"))
				}
				if slices.Contains(k.Names, "C-u") {
					f.input.screen = empty
				}
				return nil
			}
			// Every paste fails to settle; the clear's own wait succeeds.
			f.cmd.settleInput = func(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, _ time.Duration) error {
				screen, err := b.Capture(ctx, pane)
				if err != nil {
					return err
				}
				if composer, err := harness.ReadComposer(c.Primitives.Composer, screen); err == nil && composer.Text != "" {
					return fmt.Errorf("native composer did not settle before submission: %w", context.DeadlineExceeded)
				}
				return nil
			}
			f.run.cmd = f.cmd
			for i := range 2 {
				e := core.Envelope{ID: core.EnvelopeID(fmt.Sprintf("msg-%d", i)), Token: fmt.Sprintf("%016d", i), Recipient: a.ID, To: a.Name, From: tc.from, Purpose: tc.purpose, Message: core.Message{Text: "hello"}, CreatedAt: f.cmd.now()}
				if err := p.Publish(e); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.run.drain(a.ID, ""); err != nil {
				t.Fatal(err)
			}
			queued, err := p.ListNew()
			if err != nil {
				t.Fatal(err)
			}
			if pastes != tc.wantPastes || len(queued) != tc.wantQueued {
				t.Fatalf("pastes=%d queued=%d", pastes, len(queued))
			}
			if cleared := slices.Contains(f.input.keys, "C-u"); cleared != tc.wantClear {
				t.Fatalf("clear keys sent=%v, want %v", cleared, tc.wantClear)
			}
		})
	}
}
