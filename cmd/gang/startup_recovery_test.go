package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func TestStartupTrustSurvivesDeadlineAndKeepsContract(t *testing.T) {
	for _, prompt := range []struct {
		name    string
		lines   []string
		expired bool
	}{
		{name: "hook review", lines: []string{"Hooks need review", "› 1. Review hooks", "Press enter to confirm or esc to go back"}},
		{name: "permission", lines: []string{"Would you like to run this command?", "› 1. Yes, proceed"}},
		{name: "permission after boot deadline", lines: []string{"Would you like to run this command?", "› 1. Yes, proceed"}, expired: true},
	} {
		t.Run(prompt.name, func(t *testing.T) {
			testStartupPromptSurvivesDeadlineAndKeepsContract(t, prompt.lines, prompt.expired)
		})
	}
}

func testStartupPromptSurvivesDeadlineAndKeepsContract(t *testing.T, prompt []string, expired bool) {
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
	e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderAgent, Name: "lead", HitchID: "lead-id"}, Purpose: "assignment", Message: core.Message{Text: "Standing contract: report completion.\nAssignment: fix it."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText(prompt...)
	if expired {
		start := f.cmd.now()
		f.run.cmd.clock = func() time.Time { return start.Add(time.Hour) }
	}
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
	if err := f.run.recoverStartup("worker"); err != nil || !strings.Contains(f.out.String(), "delivered") || f.input.submits != 1 {
		t.Fatalf("delivered startup recovery: %v output=%q submits=%d", err, f.out, f.input.submits)
	}
	path, err := p.EnvelopePath("cur", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := f.run.recoverStartup("worker"); err == nil || !strings.Contains(err.Error(), "no retained") {
		t.Fatalf("cleaned startup receipt: %v", err)
	}
}

func TestBootTickDefersWhenTrustFollowsProvisionalComposer(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.Status = core.Booting
	a.BootDeadline = f.cmd.now().Add(-time.Second)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "startup-test", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: "send once"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.cmd.awaitStartup = func(context.Context, substrate.PaneID, harness.Collar) (harness.Startup, substrate.Screen, error) {
		return harness.Startup{State: harness.StartupTrustRequired, Prompt: "Codex folder trust is required"}, screenWithText("Trust this folder?", "› 1. Trust and continue"), nil
	}
	f.run.cmd = f.cmd
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil || got.Status != core.Booting || got.Activity != core.Blocked || len(pending) != 1 || f.input.submits != 0 {
		t.Fatalf("premature startup: agent=%+v pending=%+v submits=%d error=%v", got, pending, f.input.submits, err)
	}
}

func TestStartupPermissionMenuSurvivesBootDeadline(t *testing.T) {
	for _, afterDeadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("afterDeadline=%t", afterDeadline), func(t *testing.T) {
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
			e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderAgent, Name: "lead", HitchID: "lead-id"}, Purpose: "assignment", Message: core.Message{Text: "Standing contract and assignment"}, CreatedAt: f.cmd.now()}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.screen = screenWithText("Would you like to run this command?", "› 1. Yes, proceed", "  2. No")
			start := f.cmd.now()
			if afterDeadline {
				f.run.cmd.clock = func() time.Time { return start.Add(time.Hour) }
			}
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			blocked, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if blocked.Status != core.Booting || blocked.Activity != core.Blocked || !blocked.BootDeadline.IsZero() || f.input.submits != 0 {
				t.Fatalf("permission menu state: %+v submits=%d", blocked, f.input.submits)
			}
			f.run.cmd.clock = func() time.Time { return start.Add(2 * time.Hour) }
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			f.input.screen = screenWithText("READY", "› ")
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			got, err := p.ReadEnvelope("cur", e.ID)
			if err != nil || got.Message.Text != e.Message.Text || f.input.submits != 1 {
				t.Fatalf("retained startup: %+v err=%v submits=%d", got, err, f.input.submits)
			}
		})
	}
}

func TestUnknownCodexMenuDoesNotHoldBootDeadline(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.Status = core.Booting
	a.BootDeadline = f.cmd.now().Add(time.Second)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.screen = screenWithText("Choose a display mode", "› 1. Compact")
	start := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return start.Add(time.Hour) }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil || got.Status != core.Failed || f.input.submits != 0 {
		t.Fatalf("unknown menu state: %+v err=%v submits=%d", got, err, f.input.submits)
	}
}

// A hitch whose pane never shows a recognized startup screen has registered
// the agent and queued its startup, so its error names the pane and the route
// that resumes startup under the status for a native harness that needs
// attention.
func TestHitchNamesRecoverRouteForUnrecognizedStartup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, err := harness.EmbeddedCollar("claude")
		if err != nil {
			t.Fatal(err)
		}
		a := core.Agent{Name: "worker", Pane: "%7"}
		screen := screenWithText("Choose a display mode", "› 1. Compact")
		start := time.Now()
		_, err = awaitHitchStartup(context.Background(), func(context.Context, substrate.PaneID) (substrate.Screen, error) {
			return screen, nil
		}, a, c)
		var ce commandError
		if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(ce.text, "startup is queued in %7") || !strings.Contains(ce.text, "then run gang hitch worker --recover") {
			t.Fatalf("unrecognized startup err=%v", err)
		}
		if waited := time.Since(start); waited != 8*time.Second {
			t.Fatalf("startup wait = %v, want the 8s deadline", waited)
		}
	})
}

// failAtBootDeadline queues a startup and lets the boot deadline fail its
// agent while the pane shows a screen startup does not recognize.
func failAtBootDeadline(t *testing.T, f *stateFixture) (core.Agent, store.AgentPaths, core.Envelope) {
	t.Helper()
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.Status = core.Booting
	a.BootDeadline = f.cmd.now().Add(time.Second)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: "Standing contract: report completion.\nAssignment: fix it."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("Choose a display mode", "› 1. Compact")
	start := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return start.Add(time.Hour) }
	f.cmd.clock = f.run.cmd.clock
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if a, err = p.Read(); err != nil || a.Status != core.Failed || a.Evidence != core.BootDeadlineElapsed {
		t.Fatalf("boot deadline did not fail the agent: %+v err=%v", a, err)
	}
	return a, p, e
}

func TestRecoverResumesStartupFailedAtBootDeadline(t *testing.T) {
	t.Run("ready composer", func(t *testing.T) {
		f := newStateFixture(t)
		_, p, e := failAtBootDeadline(t, f)
		f.input.screen = screenWithText("READY", "› ")
		if err := f.cmd.hitch([]string{"worker", "--recover"}); err != nil {
			t.Fatal(err)
		}
		got, err := p.ReadEnvelope("cur", e.ID)
		if err != nil || got.Message.Text != e.Message.Text || f.input.submits != 1 {
			t.Fatalf("startup not delivered: %+v err=%v submits=%d", got, err, f.input.submits)
		}
		if a, err := p.Read(); err != nil || a.Status != core.Active {
			t.Fatalf("recovered agent: %+v err=%v", a, err)
		}
		file, err := os.Open(f.run.team.Log)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		var reopened []core.Event
		if err := store.ReadLog(file, func(e core.Event) error {
			if e.Type == "boot_reopened" {
				reopened = append(reopened, e)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if len(reopened) != 1 || !reopened[0].Deadline.Equal(f.cmd.now().Add(bootTimeout)) {
			t.Fatalf("reopened boot deadline: %+v", reopened)
		}
	})
	t.Run("recognized prompt", func(t *testing.T) {
		f := newStateFixture(t)
		_, p, e := failAtBootDeadline(t, f)
		f.input.screen = screenWithText("Hooks need review", "› 1. Review hooks", "Press enter to confirm or esc to go back")
		err := f.cmd.hitch([]string{"worker", "--recover"})
		var ce commandError
		if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(ce.text, "gang hitch worker --recover") || f.input.submits != 0 {
			t.Fatalf("prompt recovery err=%v submits=%d", err, f.input.submits)
		}
		a, err := p.Read()
		if err != nil || a.Status != core.Booting || a.Activity != core.Blocked || !a.BootDeadline.IsZero() {
			t.Fatalf("prompt left agent %+v err=%v", a, err)
		}
		// The route the refusal names delivers once the prompt is answered.
		f.input.screen = screenWithText("READY", "› ")
		if err := f.cmd.hitch([]string{"worker", "--recover"}); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReadEnvelope("cur", e.ID); err != nil || f.input.submits != 1 {
			t.Fatalf("tick after answer: err=%v submits=%d", err, f.input.submits)
		}
	})
	t.Run("unrecognized screen", func(t *testing.T) {
		f := newStateFixture(t)
		_, p, _ := failAtBootDeadline(t, f)
		err := f.cmd.hitch([]string{"worker", "--recover"})
		var ce commandError
		if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(ce.text, "gang hitch worker --recover") || f.input.submits != 0 {
			t.Fatalf("unknown screen recovery err=%v submits=%d", err, f.input.submits)
		}
		if a, err := p.Read(); err != nil || a.Status != core.Failed || a.Evidence != core.BootDeadlineElapsed {
			t.Fatalf("unknown screen reopened agent: %+v err=%v", a, err)
		}
	})
}

func TestRecoverDoesNotReopenOtherStartupFailures(t *testing.T) {
	f := newStateFixture(t)
	a, p, _ := failAtBootDeadline(t, f)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Evidence = "native harness exited"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.screen = screenWithText("READY", "› ")
	err = f.cmd.hitch([]string{"worker", "--recover"})
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitRefused || f.input.submits != 0 {
		t.Fatalf("other failure recovery err=%v submits=%d", err, f.input.submits)
	}
	if got, err := p.Read(); err != nil || got.Status != core.Failed {
		t.Fatalf("other failure reopened: %+v err=%v", got, err)
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
			lead := f.add(t, "b", "lead", "codex")
			p, _ := f.run.team.Agent(a.ID)
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
			if screen == "original" {
				// A later command found the startup unverified and told the
				// hitcher. Elsewhere the hitch command reported it itself.
				if err := f.run.notifySender(a, e, "unverified"); err != nil {
					t.Fatal(err)
				}
				outcomeNotice(t, f, lead, e.ID)
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
				requireNoNotices(t, f)
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
			// The hitcher was told the startup was unverified; it must also
			// learn that recovery delivered it, or it may hitch again.
			deliveredNotice(t, f, lead, e.ID)
		})
	}
}

func TestPlainSendCannotReplaceUnverifiedStartupContract(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.LastFailed = "original"
	if err := p.Publish(core.Envelope{ID: a.LastFailed, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: "original"}, CreatedAt: f.cmd.now()}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(p.Inbox, "new", "original.json"), filepath.Join(p.Inbox, "failed", "original.json")); err != nil {
		t.Fatal(err)
	}
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

func TestOrdinaryReceiptWithStartupLikeIDDoesNotBlockSend(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "startup-decoy", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "ordinary"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, e, "unverified", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.run.recoverStartup("worker"); err == nil || !strings.Contains(err.Error(), "no retained") {
		t.Fatalf("ordinary message treated as startup: %v", err)
	}
	f.input.submit = func(prompt string) error {
		return p.WriteWitness(store.Witness{ID: "replacement", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
	}
	f.cmd.stdin = strings.NewReader("follow-up")
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil || f.input.submits != 1 {
		t.Fatalf("ordinary send blocked: %v submits=%d", err, f.input.submits)
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
