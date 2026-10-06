package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func TestStatusDraftIsIdleAndStillHoldsDelivery(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			f.input.screen = screenWithText("────────", "❯ operator draft", "────────")
			if collar == "codex" {
				f.input.screen = screenWithText("› operator draft")
			}
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			got := f.agent(t, a.ID)
			if paneTitle(got) != "~worker~" {
				t.Fatalf("operator draft title=%q, want idle", paneTitle(got))
			}
			if got.Activity != core.Blocked || got.Evidence != heldInputEvidence || f.input.submits != 0 {
				t.Fatalf("draft lost its delivery hold: %+v", got)
			}
		})
	}
}

func TestStatusNewTurnDoesNotInheritOldIdleAge(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			f.input.screen = screenWithText("────────", "❯ ", "────────")
			if collar == "codex" {
				f.input.screen = screenWithText("› ")
			}
			start := f.cmd.now()
			a = f.setAgent(t, a, func(a *core.Agent) {
				a.ScreenFingerprint = harness.ScreenFingerprint(f.input.screen)
				a.ScreenSince = start.Add(-time.Hour)
			})
			p, _ := f.run.team.Agent(a.ID)
			if err := p.WriteWitness(store.Witness{ID: "start", At: start, SessionID: "s", TurnID: "p1"}); err != nil {
				t.Fatal(err)
			}
			if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-started", SessionID: "s", At: start}, false); err != nil {
				t.Fatal(err)
			}
			if got := f.agent(t, a.ID); paneTitle(got) != "-worker-" {
				t.Fatalf("new turn title=%q native=%+v", paneTitle(got), got.Native)
			}
		})
	}
}

func TestStatusActivityRefreshesUnknownWithoutWaitingForWatchdog(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			a = f.setAgent(t, a, func(a *core.Agent) { a.Activity = core.Unknown; a.Evidence = "no composer on screen" })
			f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
			if collar == "claude" {
				f.input.screen = screenWithText("✻ Working… (running Bash)", "────────", "❯ ", "────────")
			}
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			var notices []hookNotice
			f.cmd.detach = func(_ string, n hookNotice) error { notices = append(notices, n); return nil }
			payload, _ := json.Marshal(map[string]string{"hook_event_name": "PostToolUse", "session_id": "s"})
			hook := f.cmd
			hook.stdin = bytes.NewReader(payload)
			if err := hook.handleHook(nil); err != nil {
				t.Fatal(err)
			}
			if len(notices) != 1 {
				t.Fatalf("activity scheduled %d refreshes, want one", len(notices))
			}
			if err := f.run.tickAgent(a.ID, notices[0], false); err != nil {
				t.Fatal(err)
			}
			if got := f.agent(t, a.ID); paneTitle(got) != "-worker-" {
				t.Fatalf("active title=%q", paneTitle(got))
			}
		})
	}
}

func TestStatusWatchdogRepairsBoundaryRepaint(t *testing.T) {
	f, s := watchdogFixture(t)
	now := f.cmd.now()
	f.cmd.clock = func() time.Time { return now }
	s.now = f.cmd.now
	f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("› ") // Native UI finishes painting after the boundary's refresh.
	budget := 5 * time.Second
	if s.due.After(now.Add(budget)) {
		t.Fatalf("idle repaint cannot refresh within %s: watchdog due in %s", budget, s.due.Sub(now))
	}
	now = s.due
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", s.armed}); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, "a"); paneTitle(got) != "~worker~" {
		t.Fatalf("idle title=%q", paneTitle(got))
	}
}

func TestStatusStreamingDraftKeepsWorkSymbolAndHoldsInput(t *testing.T) {
	f, a, start := openTurnFixture(t)
	f.input.screen = screenWithText("Reply is streaming", "────────", "❯ operator draft", "────────")
	got := f.tickAt(t, a, start.Add(time.Second), hookNotice{})
	if paneTitle(got) != "-worker-" || got.Evidence != heldInputEvidence {
		t.Fatalf("streaming draft: title=%q reason=%q", paneTitle(got), got.Evidence)
	}
	f.cmd.stdin = bytes.NewBufferString("queued behind the draft")
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || f.input.pasted != "" {
		t.Fatalf("draft overwritten: submits=%d pasted=%q", f.input.submits, f.input.pasted)
	}
	got = f.tickAt(t, a, start.Add(2*time.Second), hookNotice{Kind: "turn-finished", TurnID: "p1"})
	if paneTitle(got) != "~worker~" || f.input.submits != 0 {
		t.Fatalf("finished draft: title=%q submits=%d", paneTitle(got), f.input.submits)
	}
}

func TestStatusQuietWindowStartsAtCurrentSubmit(t *testing.T) {
	f, a, start := openTurnFixture(t)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.ScreenFingerprint = harness.ScreenFingerprint(f.input.screen)
		a.ScreenSince = start.Add(-time.Hour)
	})
	c, err := loadCollar(a.Collar, f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	quiet, err := harness.OpenTurnQuiet(c.Primitives.TurnBoundary)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		at   time.Time
		want core.Activity
	}{{start.Add(quiet - time.Millisecond), core.Busy}, {start.Add(quiet), core.Idle}} {
		got := f.tickAt(t, a, step.at, hookNotice{})
		if got.Activity != step.want {
			t.Fatalf("at %s activity=%s, want %s", step.at.Sub(start), got.Activity, step.want)
		}
	}
}

func TestStatusSubmitPaintGraceDoesNotRecordFinish(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	start := f.cmd.now()
	p, _ := f.run.team.Agent(a.ID)
	if err := p.WriteWitness(store.Witness{ID: "w", At: start, SessionID: "s", TurnID: "p1"}); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("› ")
	for _, step := range []struct {
		at   time.Duration
		want core.Activity
	}{{submitPaintWindow - time.Millisecond, core.Busy}, {submitPaintWindow, core.Idle}} {
		got := f.tickAt(t, a, start.Add(step.at), hookNotice{})
		if got.Activity != step.want || !got.Native.FinishedAt.IsZero() {
			t.Fatalf("at %s activity=%s native=%+v", step.at, got.Activity, got.Native)
		}
	}
}

func TestStatusPromptAndCompactionHooksScheduleRefreshWithoutAgentLock(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		for _, native := range []string{"PermissionRequest", "PreCompact"} {
			t.Run(collar+"/"+native, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", collar)
				p, _ := f.run.team.Agent(a.ID)
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				var notices []hookNotice
				f.cmd.detach = func(_ string, n hookNotice) error { notices = append(notices, n); return nil }
				payload, _ := json.Marshal(map[string]string{"hook_event_name": native, "session_id": "s"})
				hook := f.cmd
				hook.stdin = bytes.NewReader(payload)
				if err := hook.handleHook(nil); err != nil {
					t.Fatal(err)
				}
				if len(notices) != 1 || notices[0].SessionID != "s" || f.input.captures != 0 {
					t.Fatalf("hook did not detach without capture: notices=%+v captures=%d", notices, f.input.captures)
				}
			})
		}
	}
}

func TestStatusActivityTickSkipsOccupiedAgent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "worker", "codex")
		p, _ := f.run.team.Agent(a.ID)
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		f.env["GANGLINE_BOUNDARY"] = `{"kind":"activity"}`
		done := make(chan error, 1)
		go func() { done <- f.cmd.tick([]string{"--agent", "a"}) }()
		synctest.Wait()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatal("activity refresh waited behind the agent lock")
		}
		if f.input.captures != 0 {
			t.Fatalf("occupied activity captured pane %d times", f.input.captures)
		}
	})
}
