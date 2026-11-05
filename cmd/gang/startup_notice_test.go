//go:build acceptance

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestStartupPromptNotifiesAndRemindsHitcher(t *testing.T) {
	f := newStateFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a := f.addHitched(t, "a", "worker", "", lead.ID)
	f.setAgent(t, a, func(a *core.Agent) { a.Status, a.Activity = core.Booting, core.Unknown })
	f.input.screen = screenWithText("Trust this folder?", "› 1. Trust and continue")
	now := f.cmd.now()
	f.cmd.clock = func() time.Time { return now }
	lp, _ := f.run.team.Agent(lead.ID)
	tick := func(want int) {
		t.Helper()
		if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
			t.Fatal(err)
		}
		notices, err := lp.ListNew()
		if err != nil || len(notices) != want {
			t.Fatalf("notices=%+v err=%v, want %d", notices, err, want)
		}
		if want > 0 && (notices[0].From.Kind != core.SenderGangline || notices[0].From.Name != "hitch" || !strings.Contains(notices[0].Message.Text, "Codex folder trust is required")) {
			t.Fatalf("notice=%+v", notices[0])
		}
	}
	tick(1)
	tick(1)
	// The saved throttle must survive retiring the hitcher's notice receipt.
	notices, _ := lp.ListNew()
	l, err := lp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Withdraw(notices[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(startupPromptReminder - time.Nanosecond)
	tick(0)
	now = now.Add(time.Nanosecond)
	tick(1)
	f.out.Reset()
	if err := f.cmd.status([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	if out := f.out.String(); !strings.Contains(out, "booting\tblocked") || !strings.Contains(out, "Codex folder trust is required") {
		t.Fatalf("plain status=%q", out)
	}
	// A resolved or terminal startup no longer sends reminders.
	f.setAgent(t, f.agent(t, a.ID), func(a *core.Agent) { a.Status, a.Activity, a.Evidence = core.Active, core.Idle, "" })
	f.input.screen = screenWithText("› ")
	now = now.Add(2 * startupPromptReminder)
	tick(1)
	f.setAgent(t, f.agent(t, a.ID), func(a *core.Agent) { a.Status = core.Failed })
	now = now.Add(startupPromptReminder)
	tick(1)
}

func TestStartupWithoutPromptSendsNoNotice(t *testing.T) {
	f := newStateFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a := f.addHitched(t, "a", "worker", "", lead.ID)
	if err := f.cmd.tick([]string{"--agent", string(a.Name)}); err != nil {
		t.Fatal(err)
	}
	lp, _ := f.run.team.Agent(lead.ID)
	if notices, err := lp.ListNew(); err != nil || len(notices) != 0 {
		t.Fatalf("notices=%+v err=%v", notices, err)
	}
}
