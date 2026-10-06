package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

// An agent whose composer holds unsubmitted input for the notice threshold is
// reported to its hitcher once per reading, since messages to it wait behind
// that input and it cannot report it.
func TestHeldComposerInputNotifiesHitcher(t *testing.T) {
	f := newStateFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a := f.add(t, "a", "worker", "claude")
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
	f.run.cmd = f.cmd
	lp, _ := f.run.team.Agent(lead.ID)
	notices := func() []core.Envelope {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join(lp.Inbox, "new"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		var found []core.Envelope
		for _, entry := range entries {
			e, err := lp.ReadEnvelope("new", core.EnvelopeID(strings.TrimSuffix(entry.Name(), ".json")))
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(string(e.ID), "held-input-a-") {
				found = append(found, e)
			}
		}
		return found
	}
	held := screenWithText("────────", "❯ unsubmitted message", "────────")
	empty := screenWithText("────────", "❯ ", "────────")
	start := f.cmd.now()
	for _, step := range []struct {
		at     time.Duration
		screen []string
		want   int
	}{
		{0, nil, 0},
		{heldInputNoticeAfter - time.Millisecond, nil, 0},
		{heldInputNoticeAfter, nil, 1},
		{2 * heldInputNoticeAfter, nil, 1},
		{3 * heldInputNoticeAfter, []string{"empty"}, 1},
		{4 * heldInputNoticeAfter, nil, 1},
		{5 * heldInputNoticeAfter, nil, 2},
	} {
		f.input.screen = held
		if step.screen != nil {
			f.input.screen = empty
		}
		now := start.Add(step.at)
		f.run.cmd.clock = func() time.Time { return now }
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
		got := notices()
		if len(got) != step.want {
			t.Fatalf("at %s: notices = %+v", step.at, got)
		}
		if len(woken) != step.want {
			t.Fatalf("at %s: woken = %v", step.at, woken)
		}
	}
	for _, e := range notices() {
		if e.From.Kind != core.SenderGangline || e.Recipient != lead.ID || !strings.Contains(e.Message.Text, "worker, which you hitched, has held unsubmitted input in its composer since ") {
			t.Fatalf("notice = %+v", e)
		}
	}
}
