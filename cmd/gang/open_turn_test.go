//go:build acceptance

package main

import (
	"os"
	"path/filepath"
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

// appendLines appends each line to the transcript at path.
func appendLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
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

// Claude Code answers a prompt typed mid-turn with the running turn's submit
// hook and starts it as its own turn, with no submit hook, once that turn's
// Stop has run. The transcript's queue records are the only sign of it.
func TestQueuedPromptKeepsTurnOpenPastItsPredecessor(t *testing.T) {
	f, a, start := openTurnFixture(t)
	stamp := func(d time.Duration) string { return start.Add(d).UTC().Format(time.RFC3339Nano) }
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	write := func(lines ...string) { appendLines(t, transcript, lines...) }
	write(`{"type":"user","promptId":"p1","timestamp":"`+stamp(0)+`"}`,
		`{"type":"queue-operation","operation":"enqueue","timestamp":"`+stamp(time.Second)+`","content":"next"}`)
	got := f.tickAt(t, a, start.Add(3*time.Second), hookNotice{Kind: "turn-finished", TurnID: "p1", Transcript: transcript})
	if got.Activity != core.Busy {
		t.Fatalf("queued turn read %s: %s", got.Activity, got.Evidence)
	}
	write(`{"type":"queue-operation","operation":"dequeue","timestamp":"`+stamp(3*time.Second)+`"}`,
		`{"type":"system","subtype":"stop_hook_summary","timestamp":"`+stamp(2900*time.Millisecond)+`"}`,
		`{"type":"user","promptId":"p2","timestamp":"`+stamp(4*time.Second)+`"}`)
	got = f.tickAt(t, a, start.Add(5*time.Second), hookNotice{Kind: "turn-finished", TurnID: "p2", Transcript: transcript})
	if got.Activity != core.Idle {
		t.Fatalf("finished queued turn read %s: %s", got.Activity, got.Evidence)
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

// transcriptTurnFixture is openTurnFixture whose submit witness names a
// Claude Code transcript holding prompt p1. stamp formats an offset from the
// fixture's start as a transcript timestamp.
func transcriptTurnFixture(t *testing.T) (f *stateFixture, a core.Agent, start time.Time, write func(...string), stamp func(time.Duration) string) {
	t.Helper()
	f, a, start = openTurnFixture(t)
	stamp = func(d time.Duration) string { return start.Add(d).UTC().Format(time.RFC3339Nano) }
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	write = func(lines ...string) { appendLines(t, transcript, lines...) }
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "w1", At: start, SessionID: "s", TurnID: "p1", Transcript: transcript}); err != nil {
		t.Fatal(err)
	}
	write(`{"type":"user","promptId":"p1","timestamp":"`+stamp(0)+`","message":{"role":"user","content":"go"}}`,
		`{"type":"assistant","timestamp":"`+stamp(time.Second)+`","message":{"role":"assistant","content":[]}}`)
	return f, a, start, write, stamp
}

// hookAt records a native status hook as the hook command does, then ticks
// the agent with it.
func (f *stateFixture) hookAt(t *testing.T, a core.Agent, at time.Time, kind string) core.Agent {
	t.Helper()
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteStatusHook(store.StatusHook{Kind: kind, At: at, SessionID: "s", TurnID: "p1"}); err != nil {
		t.Fatal(err)
	}
	return f.tickAt(t, a, at, hookNotice{Kind: kind, TurnID: "p1", At: at})
}

// underLock runs observe on agent id under its lock at time at, as a tick
// does, and returns the saved agent.
func (f *stateFixture) underLock(t *testing.T, id core.HitchID, at time.Time, observe func(*store.LockedAgent, *core.Agent, harness.Collar) error) core.Agent {
	t.Helper()
	f.cmd.clock = func() time.Time { return at }
	f.run.cmd = f.cmd
	l, current, err := f.run.acquire(id, false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadCollar(current.Collar, f.run.settings)
	if err == nil {
		err = observe(l, &current, c)
	}
	if closeErr := f.run.unlock(l); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return f.agent(t, id)
}

func eventLog(t *testing.T, f *stateFixture) string {
	t.Helper()
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The transcript's turn_duration closes a turn at once, before its Stop hook
// and well inside the quiet window, and the late Stop leaves it closed.
func TestTranscriptTurnEndClosesTurn(t *testing.T) {
	f, a, start, write, stamp := transcriptTurnFixture(t)
	if got := f.tickAt(t, a, start.Add(2*time.Second), hookNotice{}); got.Activity != core.Busy {
		t.Fatalf("running turn read %s: %s", got.Activity, got.Evidence)
	}
	write(`{"type":"system","subtype":"stop_hook_summary","timestamp":"`+stamp(3*time.Second)+`"}`,
		`{"type":"system","subtype":"turn_duration","timestamp":"`+stamp(3*time.Second+time.Millisecond)+`"}`)
	got := f.tickAt(t, a, start.Add(4*time.Second), hookNotice{})
	if got.Activity != core.Idle || !got.Native.FinishedAt.Equal(start.Add(3*time.Second+time.Millisecond)) {
		t.Fatalf("ended turn read %s (%s) finished %v", got.Activity, got.Evidence, got.Native.FinishedAt)
	}
	got = f.tickAt(t, a, start.Add(5*time.Second), hookNotice{Kind: "turn-finished", TurnID: "p1", At: start.Add(3 * time.Second)})
	if got.Activity != core.Idle {
		t.Fatalf("late Stop read %s: %s", got.Activity, got.Evidence)
	}
	if !strings.Contains(eventLog(t, f), "native transcript records the turn's end") {
		t.Fatal("transcript end not recorded")
	}
}

// Escape on a permission dialog fires no hook (Claude Code writes only the
// transcript's interrupt note and turn_duration). With an interrupt pending,
// the transcript completes it, so the status read needs no pane capture.
func TestTranscriptCompletesInterruptOnEscapedPermission(t *testing.T) {
	f, a, start, write, stamp := transcriptTurnFixture(t)
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WritePermissionWitness(store.PermissionWitness{At: start.Add(2 * time.Second), SessionID: "s"}); err != nil {
		t.Fatal(err)
	}
	if got := f.hookAt(t, a, start.Add(2*time.Second), "permission-requested"); got.Activity != core.Blocked {
		t.Fatalf("permission request read %s: %s", got.Activity, got.Evidence)
	}
	// gang sends the interrupt at 4s.
	a = f.setAgent(t, f.agent(t, a.ID), func(a *core.Agent) {
		a.Activity, a.InterruptAt, a.InterruptDeadline = core.Interrupting, start.Add(4*time.Second), start.Add(4*time.Second+operationTimeout)
	})
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	const gap = "interrupt completion has no native hook"
	if got := f.run.statusCaptureGap(a, c); got != gap {
		t.Fatalf("premise: gap %q", got)
	}
	write(`{"type":"user","promptId":"p1","timestamp":"`+stamp(5*time.Second)+`","message":{"role":"user","content":[{"type":"tool_result","content":"The user doesn't want to proceed with this tool use.","is_error":true}]}}`,
		`{"type":"user","promptId":"p1","timestamp":"`+stamp(5*time.Second+time.Millisecond)+`","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}`,
		`{"type":"system","subtype":"turn_duration","timestamp":"`+stamp(5*time.Second+2*time.Millisecond)+`"}`)
	captures := f.input.captures
	got := f.tickAt(t, a, start.Add(6*time.Second), hookNotice{})
	if got.Activity != core.Idle || !got.InterruptDeadline.IsZero() {
		t.Fatalf("interrupt: %s (%s) deadline=%v", got.Activity, got.Evidence, got.InterruptDeadline)
	}
	if f.input.captures != captures {
		t.Fatalf("captures=%d, want %d: the transcript ends the interrupt", f.input.captures, captures)
	}
	if g := f.run.statusCaptureGap(got, c); g == gap {
		t.Fatalf("gap still %q", g)
	}
	log := eventLog(t, f)
	if strings.Contains(log, "status pane capture: "+gap) || !strings.Contains(log, "native transcript records the turn interrupted by the user") {
		t.Fatalf("event log: %s", log)
	}
	if _, pending, err := p.ReadPermissionWitness(); err != nil || pending {
		t.Fatalf("dismissed permission request still pending: %v %v", pending, err)
	}
}

// An operator's Escape, with no interrupt from gang, ends the turn the same
// way: the status read turns idle without a capture or the quiet window.
func TestTranscriptEndsTurnAfterOperatorEscape(t *testing.T) {
	f, a, start, write, stamp := transcriptTurnFixture(t)
	if got := f.hookAt(t, a, start.Add(2*time.Second), "activity"); got.Activity != core.Busy {
		t.Fatalf("running turn read %s: %s", got.Activity, got.Evidence)
	}
	write(`{"type":"user","promptId":"p1","timestamp":"`+stamp(3*time.Second)+`","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user]"}]}}`,
		`{"type":"system","subtype":"turn_duration","timestamp":"`+stamp(3*time.Second+time.Millisecond)+`"}`)
	captures := f.input.captures
	got := f.tickAt(t, a, start.Add(4*time.Second), hookNotice{})
	if got.Activity != core.Idle || f.input.captures != captures {
		t.Fatalf("escaped turn read %s (%s), captures %d -> %d", got.Activity, got.Evidence, captures, f.input.captures)
	}
}

// Claude Code folds a prompt submitted at a tool boundary into the running
// turn and fires the submit hook again under the running turn's prompt id.
// The turn's one turn_duration closes it.
func TestTranscriptEndClosesTurnWithFoldedPrompt(t *testing.T) {
	f, a, start, write, stamp := transcriptTurnFixture(t)
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	write(`{"type":"queue-operation","operation":"enqueue","timestamp":"` + stamp(2*time.Second) + `","content":"second"}`)
	first, err := p.ReadWitness()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "w2", At: start.Add(3 * time.Second), SessionID: "s", TurnID: "p1", Transcript: first.Transcript}); err != nil {
		t.Fatal(err)
	}
	if got := f.hookAt(t, a, start.Add(3*time.Second), "turn-started"); got.Activity != core.Busy {
		t.Fatalf("folded prompt read %s: %s", got.Activity, got.Evidence)
	}
	write(`{"type":"user","promptId":"p1","timestamp":"`+stamp(4*time.Second)+`","message":{"role":"user","content":[{"type":"tool_result","content":"ok"}]}}`,
		`{"type":"queue-operation","operation":"remove","timestamp":"`+stamp(4*time.Second+time.Millisecond)+`","content":"second"}`,
		`{"type":"system","subtype":"stop_hook_summary","timestamp":"`+stamp(6*time.Second)+`"}`,
		`{"type":"system","subtype":"turn_duration","timestamp":"`+stamp(6*time.Second+time.Millisecond)+`"}`)
	if got := f.tickAt(t, a, start.Add(7*time.Second), hookNotice{}); got.Activity != core.Idle {
		t.Fatalf("ended turn read %s: %s", got.Activity, got.Evidence)
	}
}

// An end recorded before the interrupt was sent belongs to a turn already
// over: it does not show that the interrupt stopped whatever runs now, such
// as a compaction the operator typed, which fires no submit hook.
func TestTranscriptEndBeforeInterruptLeavesItPending(t *testing.T) {
	f, a, start, write, stamp := transcriptTurnFixture(t)
	write(`{"type":"system","subtype":"turn_duration","timestamp":"` + stamp(3*time.Second) + `"}`)
	if got := f.tickAt(t, a, start.Add(4*time.Second), hookNotice{}); got.Activity != core.Idle {
		t.Fatalf("ended turn read %s: %s", got.Activity, got.Evidence)
	}
	f.setAgent(t, f.agent(t, a.ID), func(a *core.Agent) {
		a.Activity, a.InterruptAt, a.InterruptDeadline = core.Interrupting, start.Add(10*time.Second), start.Add(10*time.Second+operationTimeout)
	})
	observe := func(at time.Time) core.Agent {
		t.Helper()
		return f.underLock(t, a.ID, at, func(l *store.LockedAgent, a *core.Agent, c harness.Collar) error {
			return f.run.observeTranscriptTurn(l, a, c)
		})
	}
	if got := observe(start.Add(11 * time.Second)); got.InterruptDeadline.IsZero() {
		t.Fatal("an end before the interrupt completed it")
	}
	write(`{"type":"system","subtype":"turn_duration","timestamp":"` + stamp(12*time.Second) + `"}`)
	if got := observe(start.Add(13 * time.Second)); !got.InterruptDeadline.IsZero() {
		t.Fatalf("an end after the interrupt left it pending: %s", got.Activity)
	}
}

// A turn failure recorded after the transcript closed the turn still reads,
// even though the closed turn already reads idle.
func TestTurnFailureReadsAfterTranscriptClose(t *testing.T) {
	f, a, start := openTurnFixture(t)
	a = f.setAgent(t, f.agent(t, a.ID), func(a *core.Agent) {
		a.Activity, a.Evidence = core.Idle, ""
		a.Native.HookAt, a.Native.HookKind = start.Add(time.Second), "activity"
		a.Native.FinishedAt, a.Native.TurnFailure = start.Add(2*time.Second), "server_error"
	})
	got := f.underLock(t, a.ID, start.Add(3*time.Second), func(l *store.LockedAgent, a *core.Agent, _ harness.Collar) error {
		return f.run.observeHookStatus(l, a)
	})
	if got.Activity == core.Idle {
		t.Fatalf("turn failure hidden: %s (%s)", got.Activity, got.Evidence)
	}
}
