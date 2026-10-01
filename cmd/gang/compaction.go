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
