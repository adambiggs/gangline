//go:build acceptance

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
		{30*time.Minute - time.Millisecond, nil, 0},
		{30 * time.Minute, nil, 1},
		{2 * 30 * time.Minute, nil, 1},
		{3 * 30 * time.Minute, []string{"empty"}, 1},
		{4 * 30 * time.Minute, nil, 1},
		{5 * 30 * time.Minute, nil, 2},
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

// Fake-clock ticks exercise the actual composer reader and notice publication.
func TestHeldComposerDraftContinuity(t *testing.T) {
	for _, mode := range []string{"changing", "unchanged", "changed-after-notice", "submitted", "unreadable", "unreadable-after-notice", "native-submit"} {
		t.Run(mode, func(t *testing.T) {
			f := newStateFixture(t)
			lead := f.addHitched(t, "l", "lead", "lead", "")
			a := f.addHitched(t, "a", "worker", "", lead.ID)
			lp, _ := f.run.team.Agent(lead.ID)
			start := f.cmd.now()
			tick := func(at time.Duration, draft string, want int) {
				t.Helper()
				f.input.screen = screenWithText("READY", "› "+draft)
				if draft == "<unreadable>" {
					f.input.screen = screenWithText("no composer")
				}
				now := start.Add(at)
				f.run.cmd.clock = func() time.Time { return now }
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil && !(draft == "<unreadable>" && strings.Contains(err.Error(), "no composer")) {
					t.Fatal(err)
				}
				envelopes, err := lp.ListNew()
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, e := range envelopes {
					if strings.HasPrefix(string(e.ID), "held-input-a-") {
						count++
					}
				}
				if count != want {
					t.Fatalf("at %s: got %d notices, want %d", at, count, want)
				}
			}
			tick(0, "draft", 0)
			switch mode {
			case "changing":
				for i := 1; i <= 8; i++ {
					tick(time.Duration(i)*10*time.Minute, strings.Repeat("x", i), 0)
				}
			case "unchanged":
				tick(30*time.Minute-time.Nanosecond, "draft", 0)
				tick(30*time.Minute, "draft", 1)
				tick(60*time.Minute, "draft", 1)
				var err error
				f.run, err = f.cmd.runtime()
				if err != nil {
					t.Fatal(err)
				}
				// Receipt pruning cannot permit another notice for the same draft.
				envelopes, _ := lp.ListNew()
				for _, e := range envelopes {
					if err := os.Remove(filepath.Join(lp.Inbox, "new", string(e.ID)+".json")); err != nil {
						t.Fatal(err)
					}
				}
				tick(90*time.Minute, "draft", 0)
			case "unreadable-after-notice":
				tick(30*time.Minute, "draft", 1)
				tick(31*time.Minute, "<unreadable>", 1)
				tick(32*time.Minute, "draft", 1)
				tick(62*time.Minute, "draft", 1)
			case "native-submit":
				tick(29*time.Minute, "draft", 0)
				p, _ := f.run.team.Agent(a.ID)
				l, err := p.LockAgent()
				if err != nil {
					t.Fatal(err)
				}
				current, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				current.Native.SubmittedAt = start.Add(29 * time.Minute)
				current.Native.FinishedAt = current.Native.SubmittedAt
				if err := l.Save(current); err != nil {
					t.Fatal(err)
				}
				l.Close()
				tick(30*time.Minute, "draft", 0)
				tick(60*time.Minute-time.Nanosecond, "draft", 0)
				tick(60*time.Minute, "draft", 1)
			case "changed-after-notice":
				tick(30*time.Minute, "draft", 1)
				tick(31*time.Minute, "edited draft", 1)
				tick(61*time.Minute-time.Nanosecond, "edited draft", 1)
				tick(61*time.Minute, "edited draft", 2)
				tick(91*time.Minute, "edited draft", 2)
			case "submitted", "unreadable":
				text := ""
				if mode == "unreadable" {
					text = "<unreadable>"
				}
				tick(29*time.Minute, text, 0)
				tick(30*time.Minute, "draft", 0)
				tick(60*time.Minute-time.Nanosecond, "draft", 0)
				tick(60*time.Minute, "draft", 1)
			}
		})
	}
}
