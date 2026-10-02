package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// claudeStreamingScreen is a Claude Code pane captured while a turn streamed
// its reply. Claude paints no spinner while text streams, so the screen alone
// reads as an idle harness.
func claudeStreamingScreen(t *testing.T, c harness.Collar) substrate.Screen {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/claude-code-2.1.287-streaming.txt")
	if err != nil {
		t.Fatal(err)
	}
	screen := screenWithText(strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")...)
	if idle, err := harness.Idle(c, screen); err != nil || !idle {
		t.Fatalf("streaming screen premise: idle=%v err=%v", idle, err)
	}
	return screen
}

// openTurnFixture is a Claude agent whose submit hook witnessed prompt p1 at
// the fixture's start, with its pane mid-reply.
func openTurnFixture(t *testing.T) (*stateFixture, core.Agent, time.Time) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	f.input.command = "claude"
	f.input.screen = claudeStreamingScreen(t, c)
	start := f.cmd.now()
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "w1", At: start, SessionID: "s", TurnID: "p1"}); err != nil {
		t.Fatal(err)
	}
	return f, a, start
}

func (f *stateFixture) tickAt(t *testing.T, a core.Agent, at time.Time, notice hookNotice) core.Agent {
	t.Helper()
	f.cmd.clock = func() time.Time { return at }
	f.run.cmd = f.cmd
	notice.SessionID = "s"
	if notice.At.IsZero() {
		notice.At = at
	}
	if err := f.run.tickAgent(a.ID, notice, false); err != nil {
		t.Fatal(err)
	}
	return f.agent(t, a.ID)
}

// A turn the submit hook witnessed stays busy on an idle-looking screen until
// its finish boundary arrives.
func TestWitnessedTurnReadsBusyUntilItsBoundary(t *testing.T) {
	f, a, start := openTurnFixture(t)
	got := f.tickAt(t, a, start.Add(2*time.Second), hookNotice{})
	if got.Activity != core.Busy {
		t.Fatalf("open turn read %s: %s", got.Activity, got.Evidence)
	}
	got = f.tickAt(t, a, start.Add(3*time.Second), hookNotice{Kind: "turn-finished", TurnID: "p1"})
	if got.Activity != core.Idle {
		t.Fatalf("finished turn read %s: %s", got.Activity, got.Evidence)
	}
}

// A Stop hook that ran before the next submit belongs to the earlier turn and
// leaves the next one open.
func TestEarlierTurnBoundaryLeavesNextTurnOpen(t *testing.T) {
	f, a, start := openTurnFixture(t)
	got := f.tickAt(t, a, start.Add(time.Second), hookNotice{Kind: "turn-finished", TurnID: "p0", At: start.Add(-time.Second)})
	if got.Activity != core.Busy {
		t.Fatalf("open turn read %s: %s", got.Activity, got.Evidence)
	}
}

// Gang records its own delivery after the submit hook ran, so a fast turn's
// Stop can carry an earlier time than the delivery record. The prompt id ties
// the Stop to the witnessed turn.
func TestBoundaryForWitnessedPromptClosesTurnRecordedLater(t *testing.T) {
	f, a, start := openTurnFixture(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SubmittedAt = start.Add(100 * time.Millisecond) })
	got := f.tickAt(t, a, start.Add(time.Second), hookNotice{Kind: "turn-finished", TurnID: "p1", At: start.Add(50 * time.Millisecond)})
	if got.Activity != core.Idle {
		t.Fatalf("finished turn read %s: %s", got.Activity, got.Evidence)
	}
}

// A turn that ends with no boundary, as after Escape or a dead turn, closes
// once its idle-looking screen stays unchanged for the collar's quiet window.
func TestOpenTurnClosesAfterQuietIdleScreen(t *testing.T) {
	f, a, start := openTurnFixture(t)
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	quiet, err := harness.OpenTurnQuiet(c.Primitives.TurnBoundary)
	if err != nil || quiet <= 0 {
		t.Fatalf("claude open-turn quiet: %v %v", quiet, err)
	}
	f.tickAt(t, a, start.Add(time.Second), hookNotice{})
	if got := f.tickAt(t, a, start.Add(time.Second+quiet-time.Millisecond), hookNotice{}); got.Activity != core.Busy {
		t.Fatalf("turn closed before its quiet window: %s", got.Activity)
	}
	if got := f.tickAt(t, a, start.Add(time.Second+quiet), hookNotice{}); got.Activity != core.Idle {
		t.Fatalf("quiet turn read %s: %s", got.Activity, got.Evidence)
	}
	if got := f.tickAt(t, a, start.Add(2*time.Second+quiet), hookNotice{}); got.Activity != core.Idle {
		t.Fatalf("closed turn reopened: %s", got.Activity)
	}
}

// Escape fires no Stop. An interrupt that brings the composer back ends the
// turn it interrupted.
func TestCompletedInterruptClosesOpenTurn(t *testing.T) {
	f, a, start := openTurnFixture(t)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Activity, a.InterruptDeadline = core.Interrupting, start.Add(time.Minute)
	})
	if got := f.tickAt(t, a, start.Add(time.Second), hookNotice{}); got.Activity != core.Idle || !got.InterruptDeadline.IsZero() {
		t.Fatalf("interrupt: %s deadline=%v", got.Activity, got.InterruptDeadline)
	}
	if got := f.tickAt(t, a, start.Add(2*time.Second), hookNotice{}); got.Activity != core.Idle {
		t.Fatalf("interrupted turn read %s: %s", got.Activity, got.Evidence)
	}
}
