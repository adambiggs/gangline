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

func (run *runtime) admitCompactionResume(id core.HitchID, c harness.Collar, prompt, session string) (string, error) {
	l, a, err := run.acquire(id, true)
	if err != nil {
		return "", err
	}
	defer l.Close()
	if reason := run.resumePromptBlock(a, c, prompt, session); reason != "" {
		return reason, nil
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
	return l.Close()
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
	screen, err := awaitCompactionComposer(ctx, b, pane, c, compactText)
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

func awaitCompactionComposer(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, compactText string) (substrate.Screen, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
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
		if composer.Text == "" {
			return screen, nil
		}
		if !harness.SameComposerText(composer.Text, compactText) {
			return substrate.Screen{}, fmt.Errorf("native composer occupied before continuation input; resume retained")
		}
		select {
		case <-ctx.Done():
			return substrate.Screen{}, ctx.Err()
		case <-deadline.C:
			return substrate.Screen{}, fmt.Errorf("native compact command remained in the composer; resume retained")
		case <-ticker.C:
		}
	}
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
		reason := strings.TrimSpace(refusals[len(refusals)-1])
		if err := run.apply(l, a, core.Event{Type: "compaction_failed", ID: pending.ID, Reason: reason}); err != nil {
			return err
		}
		if err := run.cancelPendingCompactionResume(l, a, reason); err != nil {
			return err
		}
		return commandError{status: exitNative, text: fmt.Sprintf("native compaction refused: %s; resume withheld", reason)}
	}
	return nil
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
		return refuseError("recipient is not active")
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	screen, err := b.Capture(ctx, pane)
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
