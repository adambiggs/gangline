package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

var resumePromptPattern = regexp.MustCompile(`(?m)^\[gang:[^]\n]+ resume\]`)

func (run *runtime) resumePromptBlock(a core.Agent, c harness.Collar, prompt, session string) string {
	if !resumePromptPattern.MatchString(prompt) {
		return ""
	}
	pending := a.Compaction
	if pending == nil || pending.ResumeToken == "" {
		return "stale or unknown compaction continuation"
	}
	if pending.ResumeAdmitted {
		return "compaction continuation already admitted"
	}
	e := core.Envelope{ID: core.EnvelopeID("resume-" + pending.ID), Token: pending.ResumeToken, From: pending.ResumeFrom, Message: pending.Resume, Purpose: "resume"}
	wire, err := envelopeText(e)
	if err != nil {
		return "compaction continuation could not be verified"
	}
	matched, err := harness.SubmittedPromptStartsWith(c.Primitives.SubmitWitness, wire, prompt)
	if err != nil || !matched {
		return "stale or altered compaction continuation"
	}
	if (pending.Status != "completed" || !pending.CompletedAt.After(pending.StartedAt)) &&
		((pending.Status != "submitted" && pending.Status != "unverified") || !pending.Continuation) {
		return "compaction completion is unconfirmed; continuation withheld"
	}
	if session == "" || a.Native.SessionID != "" && session != a.Native.SessionID {
		return "compaction continuation belongs to another native session"
	}
	return ""
}

// pendingResumeNote reports a prompt headed by the resume token of a
// compaction whose queued note has not been admitted.
func pendingResumeNote(c *core.Compaction, prompt string) bool {
	if c == nil || c.ResumeToken == "" || c.ResumeAdmitted || c.Status == "failed" {
		return false
	}
	return regexp.MustCompile(`(?m)^\[gang:[^]\n]*#` + regexp.QuoteMeta(c.ResumeToken) + ` resume\]`).MatchString(prompt)
}

func (run *runtime) admitCompactionResume(id core.HitchID, c harness.Collar, prompt, session string) (string, error) {
	l, a, err := run.acquire(id, true)
	if err != nil {
		return "", err
	}
	defer run.unlock(l)
	if reason := run.resumePromptBlock(a, c, prompt, session); reason != "" {
		if !pendingResumeNote(a.Compaction, prompt) {
			return reason, nil
		}
		// The blocked note was the one input waits behind, and it will not
		// reach the agent, so the compaction fails and says so.
		phase := compactionMayHaveRun
		if a.Compaction.Status == "completed" {
			phase = compactionRan
		}
		return reason, run.failCompaction(l, &a, phase, "resume note blocked: "+reason)
	}
	a.Compaction.ResumeAdmitted = true
	if a.Compaction.Status == "submitted" && !a.Compaction.CompletedAt.After(a.Compaction.StartedAt) {
		a.Compaction.Status = "unverified"
		a.Compaction.Reason = "native compaction completion unconfirmed; queued continuation admitted"
	}
	return "", l.Save(a)
}

func (run *runtime) confirmCompactionHook(id core.HitchID, notice hookNotice) error {
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	a, err := p.Read()
	if err != nil {
		return err
	}
	if notice.SessionID == "" || a.Native.SessionID != "" && notice.SessionID != a.Native.SessionID {
		return fmt.Errorf("compaction completion belongs to another native session")
	}
	if err := p.WriteCompactionWitness(store.CompactionWitness{At: notice.At, SessionID: notice.SessionID}); err != nil {
		return err
	}
	l, _, err := run.acquire(id, false)
	if errors.Is(err, store.ErrLocked) {
		return nil
	}
	if err != nil {
		return err
	}
	return run.unlock(l)
}

func (run *runtime) reconcileCompactionWitness(l *store.LockedAgent, a *core.Agent) error {
	w, err := l.Paths.ReadCompactionWitness()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if w.SessionID == "" || a.Native.SessionID != "" && w.SessionID != a.Native.SessionID || !w.At.After(a.Native.ConfirmedCompactedAt) {
		return nil
	}
	if pending := a.Compaction; pending != nil && (pending.Status == "submitted" || pending.Status == "unverified") && w.At.After(pending.StartedAt) {
		pending.CompletedAt = w.At
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	if err := run.acceptContextReadings(a, harness.Collar{}, []core.Reading{{Kind: "compaction-finished", Source: "native-hook", At: &w.At}}); err != nil {
		return err
	}
	if err := run.publishContextNotes(l, a); err != nil {
		return err
	}
	return run.continueCompaction(l, a)
}

func (run *runtime) cancelPendingCompactionResume(l *store.LockedAgent, a *core.Agent, reason string) error {
	id := core.EnvelopeID("resume-" + a.Compaction.ID)
	var e core.Envelope
	var dir string
	for _, candidate := range []string{"new", "cur", "failed"} {
		found, err := l.Paths.ReadEnvelope(candidate, id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		e, dir = found, candidate
		break
	}
	if dir == "" {
		return nil
	}
	if e.Outcome == "delivered" {
		return fmt.Errorf("delivered continuation cannot be cancelled")
	}
	if dir != "new" {
		from, err := l.Paths.EnvelopePath(dir, id)
		if err != nil {
			return err
		}
		to, err := l.Paths.EnvelopePath("new", id)
		if err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
		if a.LastAccepted == id {
			a.LastAccepted = ""
		}
		if a.LastFailed == id {
			a.LastFailed = ""
		}
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	var err error
	if a.LastFailed != "" {
		// Another retained failure, such as an unconfirmed startup, outranks it.
		err = l.FileFailure(e, "cancelled", reason)
	} else {
		err = l.Settle(a, e, "cancelled", reason)
	}
	if err != nil {
		return err
	}
	return run.record(*a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: reason})
}

// queueCompactionResume submits the continuation while native compaction is
// running, so later native input follows it. The submit hook admits that exact
// queued prompt once, even if completion evidence has not arrived yet.
func (run *runtime) queueCompactionResume(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar, compactText string) error {
	e, err := run.publishCompactionResume(l, a)
	if err != nil {
		return err
	}
	pane := substrate.PaneID(a.Pane)
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	window, cancelWindow := run.cmd.timeout(compactStartWindow)
	defer cancelWindow()
	screen, err := awaitCompactionComposer(ctx, window, b, pane, c, compactText)
	if err != nil {
		return err
	}
	wire, err := envelopeText(e)
	if err != nil {
		return err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, wire)
	if err != nil {
		return err
	}
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return err
	}
	if err = sendHarnessKeys(ctx, b, pane, c, input); err == nil {
		var settle time.Duration
		settle, err = harness.SubmitSettle(c.Primitives.Submit)
		if err == nil {
			if run.cmd.settleInput != nil {
				err = run.cmd.settleInput(ctx, b, pane, c, settle)
			} else {
				err = harness.AwaitComposerSettle(ctx, b.Capture, pane, c, settle)
			}
		}
	}
	if err == nil {
		err = sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Submit: true})
	}
	if err != nil {
		if finishErr := run.finishInput(l, a, e, "unverified", err.Error()); finishErr != nil {
			return errors.Join(err, finishErr)
		}
		return err
	}
	accepted := false
	if c.Primitives.QueueWitness != nil {
		screen, err = b.Capture(ctx, pane)
		if err == nil {
			accepted, err = harness.NativeQueueAccepted(c, screen, wire[:strings.Index(wire, "]")+1])
		}
		if err != nil {
			return errors.Join(err, run.finishInput(l, a, e, "unverified", err.Error()))
		}
	}
	outcome, reason := "unverified", "native continuation submitted; awaiting exact hook proof"
	if accepted {
		outcome, reason = "accepted", "native queue shows continuation token; awaiting confirmed completion"
	}
	if err := run.finishInput(l, a, e, outcome, reason); err != nil {
		return err
	}
	return run.reconcileDelivery(l, a)
}

// compactStartWindow bounds the wait, after the compact submit key, for the
// command to leave the composer and the compaction to show on screen.
const compactStartWindow = 2 * time.Second

// errCompactionNotStarted reports a compact command that left the composer
// without the pane showing a compaction within the start window. The harness
// may have consumed it as nothing, or queued it behind a turn whose screen
// looks idle, or still be running its pre-compaction hooks.
var errCompactionNotStarted = errors.New("compact command left the composer but no compaction showed")

// awaitCompactionComposer waits until the compact command has left the
// composer and, for a collar that can show a compaction running, the pane
// shows one or a native task. An empty composer alone is not evidence that the
// command started anything. Captures run under ctx; window ends the wait.
func awaitCompactionComposer(ctx, window context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, compactText string) (substrate.Screen, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return substrate.Screen{}, err
		}
		if blocker, blocked, err := harness.InputBlocked(c, screen); err != nil {
			return substrate.Screen{}, err
		} else if blocked {
			return substrate.Screen{}, fmt.Errorf("native continuation input blocked: %s", blocker.Evidence)
		}
		composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
		if err != nil {
			return substrate.Screen{}, err
		}
		left := composer.Text == ""
		if !left && !harness.SameComposerText(composer.Text, compactText) {
			return substrate.Screen{}, fmt.Errorf("native composer occupied before continuation input; resume retained")
		}
		if left {
			started, err := compactionShown(c, screen)
			if err != nil {
				return substrate.Screen{}, err
			}
			if started || c.Actions.Compact.Active == "" {
				return screen, nil
			}
		}
		if window.Err() != nil {
			if left {
				return substrate.Screen{}, errCompactionNotStarted
			}
			return substrate.Screen{}, fmt.Errorf("native compact command remained in the composer; resume retained")
		}
		select {
		case <-ctx.Done():
		case <-window.Done():
		case <-ticker.C:
		}
	}
}

// compactionShown reports whether the screen shows the harness compacting or
// running a native task.
func compactionShown(c harness.Collar, screen substrate.Screen) (bool, error) {
	if active, err := harness.CompactionActive(c, screen); err != nil || active {
		return active, err
	}
	return harness.Busy(c, screen)
}

func (run *runtime) observeCompaction(l *store.LockedAgent, a *core.Agent, c harness.Collar, screen substrate.Screen) error {
	pending := a.Compaction
	if pending == nil || (pending.Status != "submitted" && pending.Status != "unverified") {
		return nil
	}
	if pending.CompletedAt.After(pending.StartedAt) {
		return run.continueCompaction(l, a)
	}
	refusals, err := harness.ActionRefusals(c.Actions.Compact, screen)
	if err != nil {
		return err
	}
	if len(refusals) > pending.RefusalBefore {
		return run.failCompaction(l, a, compactionNotRun, fmt.Sprintf("native compaction refused: %s; resume withheld", strings.TrimSpace(refusals[len(refusals)-1])))
	}
	return nil
}

// compactionPhase says how far a failed compaction got, which decides what
// its failure notice can tell the agent about its own context.
type compactionPhase int

const (
	compactionNotRun compactionPhase = iota
	compactionMayHaveRun
	compactionRan
)

// failCompaction records a failed compaction, withholds its continuation, and
// tells the agent, whose resume note will not arrive. The notice waits in the
// agent's queue like any message, so a caller that has already returned still
// learns of the failure.
func (run *runtime) failCompaction(l *store.LockedAgent, a *core.Agent, phase compactionPhase, reason string) error {
	id := a.Compaction.ID
	if err := run.apply(l, a, core.Event{Type: "compaction_failed", ID: id, Reason: reason}); err != nil {
		return err
	}
	if err := run.cancelPendingCompactionResume(l, a, reason); err != nil {
		return err
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Compaction %s failed: %s. ", id, reason)
	switch phase {
	case compactionNotRun:
		text += "Your context was not compacted, and the resume note was not delivered."
	case compactionMayHaveRun:
		text += "The compaction may have run, and the resume note was withheld. If your context was compacted, re-read your brief and durable state."
	default:
		text += "Your context was compacted, but the resume note was withheld. Re-read your brief and durable state."
	}
	if err := run.publishOnce(l, a, core.Envelope{ID: core.EnvelopeID("failed-" + id), Token: token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "compact"}, Message: core.Message{Text: text}, CreatedAt: run.cmd.now()}); err != nil {
		return err
	}
	return run.notifyRequester(*a, phase, reason)
}

// notifyRequester tells the agent that requested a compaction of another agent
// that it failed. The requesting command reports a failure it saw itself, so
// only a failure in a later operation needs the message.
func (run *runtime) notifyRequester(a core.Agent, phase compactionPhase, reason string) error {
	c, r := a.Compaction, a.Compaction.Requester
	if r.Kind != core.SenderAgent || r.HitchID == a.ID || c.ID == run.reported {
		return nil
	}
	p, err := run.team.Agent(r.HitchID)
	if err != nil {
		return err
	}
	requester, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if requester.Status != core.Active {
		return nil
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Compaction %s failed: %s. ", c.ID, reason)
	switch phase {
	case compactionNotRun:
		text += fmt.Sprintf("%s's context was not compacted, and its resume note was not delivered.", a.Name)
	case compactionMayHaveRun:
		text += fmt.Sprintf("%s's compaction may have run, and its resume note was withheld.", a.Name)
	default:
		text += fmt.Sprintf("%s's context was compacted, but its resume note was withheld.", a.Name)
	}
	if err := run.publishOnceTo(p, requester, core.Envelope{ID: core.EnvelopeID("failed-" + c.ID), Token: token, Recipient: requester.ID, To: requester.Name, From: core.Sender{Kind: core.SenderGangline, Name: "compact"}, Message: core.Message{Text: text}, CreatedAt: run.cmd.now()}); err != nil {
		return err
	}
	run.wake = append(run.wake, requester.ID)
	return nil
}

// abandonCompactDraft clears compact input that must not be submitted and
// fails the compaction.
func (run *runtime) abandonCompactDraft(ctx context.Context, l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar, draft string, phase compactionPhase, reason string) error {
	if draft != "" {
		if err := run.clearComposerDraft(ctx, b, substrate.PaneID(a.Pane), c, draft); err != nil {
			reason += "; " + err.Error()
		} else {
			reason += "; composer cleared"
		}
	}
	return run.failCompaction(l, a, phase, reason)
}

// abortCompactInput fails a compaction whose input was abandoned before its
// submit key, clearing any draft it left. It works under a fresh deadline
// because the cause may be the operation's own. Behind a blocking prompt the
// clear keys would answer the prompt, so the draft stays and the reason says so.
//
// A bracketed paste cannot submit itself, so while the command is still in the
// composer, or gone with the pane idle, the compaction never ran. Gone with the
// pane running something, someone else's Enter may have started it, and the
// composer is left alone.
func (run *runtime) abortCompactInput(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar, compactText string, cause error) error {
	ctx, cancel := run.cmd.timeout(compactAbortTimeout)
	defer cancel()
	reason := "compaction not submitted: " + cause.Error()
	notRun := compactionNotRun
	if !harness.BracketedPaste(c.Primitives.Submit) {
		notRun = compactionMayHaveRun
	}
	pane := substrate.PaneID(a.Pane)
	screen, err := b.Capture(ctx, pane)
	if err != nil {
		return run.failCompaction(l, a, notRun, reason+"; composer unread: "+err.Error())
	}
	if _, blocked, err := harness.InputBlocked(c, screen); err != nil || blocked {
		if err != nil {
			reason += "; " + err.Error()
		}
		return run.failCompaction(l, a, notRun, reason+"; the compact input may remain in the composer behind the prompt")
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return run.failCompaction(l, a, notRun, reason+"; composer unread: "+err.Error())
	}
	if compactText == "" || !harness.SameComposerText(composer.Text, compactText) {
		if running, err := compactionShown(c, screen); err != nil {
			return run.failCompaction(l, a, compactionMayHaveRun, reason+"; "+err.Error())
		} else if running {
			return run.failCompaction(l, a, compactionMayHaveRun, reason+"; the native harness is running a compaction or task")
		}
	}
	return run.abandonCompactDraft(ctx, l, a, b, c, composer.Text, notRun, reason)
}

// compactAbortTimeout bounds the clear after an abandoned compact or message
// paste. When it runs out the draft stays, and the failure reason says the
// composer still holds it.
const compactAbortTimeout = 10 * time.Second

// clearComposerDraft removes unsubmitted input with the collar's compact clear
// keys, one press per attempt, until the composer reads empty. A press can
// clear as little as one composer line.
func (run *runtime) clearComposerDraft(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, draft string) error {
	if c.Actions.CompactClear == nil {
		return fmt.Errorf("the collar declares no compact clear keys; the composer still holds the unsubmitted input")
	}
	shown := draft
	for attempt := 0; attempt < 2*(strings.Count(draft, "\n")+1); attempt++ {
		if err := sendHarnessKeys(ctx, b, pane, c, c.Actions.CompactClear.Input()); err != nil {
			return err
		}
		var err error
		if run.cmd.settleInput != nil {
			err = run.cmd.settleInput(ctx, b, pane, c, compactClearSettle)
		} else {
			err = awaitComposerChange(ctx, b, pane, c, shown, compactClearSettle)
		}
		if err != nil {
			return err
		}
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return err
		}
		composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
		if err != nil {
			return err
		}
		if composer.Text == "" {
			return nil
		}
		shown = composer.Text
	}
	return fmt.Errorf("the composer still holds the unsubmitted input after clearing")
}

// compactClearSettle bounds how long one clear press may take to repaint the
// composer before the next press or the final verdict.
const compactClearSettle = time.Second

// awaitComposerChange waits until the composer no longer reads as shown, or
// until limit passes; the caller judges the composer it then reads.
func awaitComposerChange(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, shown string, limit time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	for {
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return err
		}
		if composer, err := harness.ReadComposer(c.Primitives.Composer, screen); err == nil && composer.Text != shown {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return nil
		case <-ticker.C:
		}
	}
}

// compactRecoverSettle bounds how long recovery watches the harness after a
// key before it reports the surface it last saw.
const compactRecoverSettle = 2 * time.Second

type recoverSurface struct {
	kind, evidence string
}

// classifyRecoverSurface names the native surface a recovery key would land
// on. Only "busy" is a surface recovery may send to: an approval choice, a
// draft, or an unrecognized screen can each consume a key as an answer.
func classifyRecoverSurface(c harness.Collar, screen substrate.Screen) (recoverSurface, error) {
	if blocker, blocked, err := harness.InputBlocked(c, screen); err != nil {
		return recoverSurface{}, err
	} else if blocked {
		return recoverSurface{"blocked", "native input blocked: " + blocker.Evidence}, nil
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return recoverSurface{"unrecognized", "unrecognized native surface: " + err.Error()}, nil
	}
	if composer.Text != "" {
		return recoverSurface{"draft", "native composer holds unsubmitted input"}, nil
	}
	busy, err := harness.Busy(c, screen)
	if err != nil {
		return recoverSurface{}, err
	}
	if compacting, err := harness.CompactionActive(c, screen); err != nil {
		return recoverSurface{}, err
	} else if compacting {
		busy = true
	}
	if !busy {
		return recoverSurface{"idle", "native harness is idle"}, nil
	}
	return recoverSurface{"busy", "native task still active"}, nil
}

// recoverCompaction interrupts a submitted or unverified compaction that the
// native harness still shows as running. It sends nothing unless the screen
// is a recognized busy surface with an empty composer, and records every key
// it sends as an unverified compaction before returning.
func (run *runtime) recoverCompaction(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar) (result error) {
	pending := a.Compaction
	switch {
	case pending == nil:
		return refuseError("no compaction to recover")
	case pending.Status == "queued":
		return refuseError("compaction %s is queued and not submitted; nothing to recover", pending.ID)
	case pending.Status == "failed":
		return refuseError("compaction %s already failed: %s; nothing to recover", pending.ID, pending.Reason)
	case pending.Status == "completed":
		return refuseError("compaction %s already completed; nothing to recover", pending.ID)
	case pending.Status != "submitted" && pending.Status != "unverified":
		return refuseError("compaction %s is %s; nothing to recover", pending.ID, pending.Status)
	case !pending.Continuation:
		return refuseError("compaction %s never queued its continuation in the harness; nothing to recover", pending.ID)
	case pending.ResumeAdmitted:
		// The harness has taken the continuation, so a busy pane is the
		// resumed work rather than the compaction.
		return refuseError("compaction %s continuation was already admitted; a busy pane is later work; use gang interrupt to stop it", pending.ID)
	case pending.Recovered:
		return refuseError("compaction %s was already recovered; a busy pane is later work; use gang interrupt to stop it", pending.ID)
	}
	if a.Status != core.Active {
		return inactiveRecipient(*a)
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	screen, err := run.captureAgentPane(ctx, b, *a)
	if err != nil {
		return err
	}
	surface, err := classifyRecoverSurface(c, screen)
	if err != nil {
		return err
	}
	if surface.kind != "busy" {
		return refuseError("%s; recovery interrupts only an active native task; no keys sent", surface.evidence)
	}
	if err := requirePaneRegistration(*a); err != nil {
		return err
	}
	if err := requireHarnessForeground(ctx, b, pane, c); err != nil {
		var ce commandError
		if errors.As(err, &ce) {
			return err
		}
		return refuseError("%v; no keys sent", err)
	}
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: pending.ID, Status: "compaction-recovery"}); err != nil {
		return err
	}
	var sent []string
	var cause error
	observed := false
	defer func() {
		if cause == nil {
			cause = result
		}
		reason := fmt.Sprintf("operator recovery sent %s; outcome unknown: %v", strings.Join(sent, " then "), cause)
		if observed {
			reason = fmt.Sprintf("operator recovery sent %s; %s; inspect the harness before retrying", strings.Join(sent, " then "), surface.evidence)
		}
		if err := run.apply(l, a, core.Event{Type: "compaction_unverified", ID: pending.ID, Status: "recovery", Reason: reason}); err != nil {
			result = errors.Join(result, err)
			return
		}
		if observed {
			result = errors.Join(result, run.observeActivity(l, a, c, screen))
		}
	}()
	for i, action := range c.Actions.CompactRecover {
		keys := action.Input()
		if i > 0 {
			if err := requireHarnessForeground(ctx, b, pane, c); err != nil {
				return commandError{status: exitNative, text: fmt.Sprintf("sent %s; stopped before the next key: %v; compaction %s recorded unverified", strings.Join(sent, " then "), err, pending.ID)}
			}
		}
		sent, observed = append(sent, keyLabel(keys)), false
		if err := b.SendKeys(ctx, pane, keys); err != nil {
			cause = err
			return commandError{status: exitUnknown, text: fmt.Sprintf("sent %s; outcome unknown: %v; compaction %s recorded unverified", strings.Join(sent, " then "), err, pending.ID)}
		}
		screen, surface, err = awaitRecoverSurface(ctx, run.cmd, b, pane, c)
		if err != nil {
			cause = err
			return commandError{status: exitUnknown, text: fmt.Sprintf("sent %s; outcome unknown: %v; compaction %s recorded unverified", strings.Join(sent, " then "), err, pending.ID)}
		}
		observed = true
		if surface.kind != "busy" {
			break
		}
	}
	if surface.kind == "idle" {
		_, err := fmt.Fprintf(run.cmd.stdout, "%s\tunverified; sent %s; native harness idle\n", pending.ID, strings.Join(sent, " then "))
		return err
	}
	return commandError{status: exitNative, text: fmt.Sprintf("sent %s; %s; compaction %s recorded unverified", strings.Join(sent, " then "), surface.evidence, pending.ID)}
}

func keyLabel(keys substrate.Keys) string {
	label := strings.Join(keys.Names, "+")
	if keys.Text != "" {
		label = strings.TrimSpace(label + " text")
	}
	if keys.Submit {
		label = strings.TrimSpace(label + " submit")
	}
	return label
}

// awaitRecoverSurface watches until the native task leaves its busy surface
// or the settle window ends, and returns the last surface it saw.
func awaitRecoverSurface(ctx context.Context, cmd command, b harnessInput, pane substrate.PaneID, c harness.Collar) (substrate.Screen, recoverSurface, error) {
	settle, cancel := cmd.timeout(compactRecoverSettle)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return substrate.Screen{}, recoverSurface{}, err
		}
		surface, err := classifyRecoverSurface(c, screen)
		if err != nil {
			return substrate.Screen{}, recoverSurface{}, err
		}
		if surface.kind != "busy" && surface.kind != "unrecognized" {
			return screen, surface, nil
		}
		select {
		case <-ctx.Done():
			return substrate.Screen{}, recoverSurface{}, ctx.Err()
		case <-settle.Done():
			return screen, surface, nil
		case <-ticker.C:
		}
	}
}
