package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type harnessInput interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
	ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
	SendKeys(context.Context, substrate.PaneID, substrate.Keys) error
}

func requireHarnessForeground(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar) error {
	processes, err := b.ForegroundProcesses(ctx, pane)
	if err != nil {
		return err
	}
	for _, p := range processes {
		if filepath.Base(p.Command) == filepath.Base(c.Launch.Command) {
			return nil
		}
	}
	return fmt.Errorf("refuse input: harness %q is not in the pane foreground", c.Launch.Command)
}
func sendHarnessKeys(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, keys substrate.Keys) error {
	if err := requireHarnessForeground(ctx, b, pane, c); err != nil {
		return err
	}
	return b.SendKeys(ctx, pane, keys)
}
func envelopeText(e core.Envelope) (string, error) {
	sender := string(e.From.Name)
	if e.From.Kind == core.SenderSelfDeclared {
		sender = "self-declared:" + sender
	} else if e.From.Kind == core.SenderGangline {
		sender = "gangline:" + sender
	}
	return renderEnvelope(sender, string(e.ID), e.Purpose, e.Message.Text)
}

// available observes the composer even during a running turn. A permission
// prompt or foreign foreground process never qualifies as a free composer.
func (run *runtime) available(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar) (bool, string, error) {
	if a.Status != core.Active || a.Activity == core.Interrupting || a.Compaction != nil && a.Compaction.Status == "submitted" {
		return false, "", nil
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return false, "", err
	}
	blocker, blocked, err := harness.InputBlocked(c, screen)
	if err != nil {
		return false, "", err
	}
	if blocked {
		if a.Activity != core.Blocked || a.Evidence != blocker.Evidence {
			if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: core.Blocked, Reason: blocker.Evidence}); err != nil {
				return false, "", err
			}
		}
		return false, blocker.Evidence, nil
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil {
		return false, "", err
	}
	if composer.Text != "" {
		return false, "", nil
	}
	if !c.Primitives.MidTurn {
		idle, err := harness.Idle(c, screen)
		if err != nil || !idle {
			return false, "", err
		}
	}
	if err := requireHarnessForeground(context.Background(), b, substrate.PaneID(a.Pane), c); err != nil {
		return false, "", err
	}
	return true, "", nil
}
func (run *runtime) deliver(l *store.LockedAgent, a *core.Agent, e core.Envelope, b harnessInput, c harness.Collar) (string, error) {
	wire, err := envelopeText(e)
	if err != nil {
		return "", err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, wire)
	if err != nil {
		return "", err
	}
	var queue func(context.Context) (bool, error)
	// Startup keeps its exact contract witness; queue previews cannot show it.
	if c.Primitives.QueueWitness != nil && c.Primitives.MidTurn && e.Purpose == "" {
		opener := wire[:strings.Index(wire, "]")+1]
		queue = func(ctx context.Context) (bool, error) {
			if err := requireHarnessForeground(ctx, b, substrate.PaneID(a.Pane), c); err != nil {
				return false, err
			}
			screen, err := b.Capture(ctx, substrate.PaneID(a.Pane))
			if err != nil {
				return false, err
			}
			return harness.NativeQueueAccepted(c, screen, opener)
		}
		// A fresh one-time ID must not already appear before this submission.
		seen, err := queue(context.Background())
		if err != nil {
			return "", err
		}
		if seen {
			return "", fmt.Errorf("message already appears in native queue before input; inspect the recipient; do not resend")
		}
	}
	old, err := l.Paths.ReadWitness()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return "", err
	}
	outcome, reason := "delivered", ""
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	pane := substrate.PaneID(a.Pane)
	if err = sendHarnessKeys(ctx, b, pane, c, input); err == nil {
		settle, settleErr := harness.SubmitSettle(c.Primitives.Submit)
		if settleErr != nil {
			err = settleErr
		} else if run.cmd.settleInput != nil {
			err = run.cmd.settleInput(ctx, b, pane, c, settle)
		} else {
			err = harness.AwaitComposerSettle(ctx, b.Capture, pane, c, settle)
		}
	}
	if err == nil {
		screen, captureErr := b.Capture(ctx, pane)
		err = captureErr
		if err == nil {
			var blocked bool
			var blocker harness.Blocked
			blocker, blocked, err = harness.InputBlocked(c, screen)
			if blocked && err == nil {
				err = fmt.Errorf("native prompt needs attention after paste: %s", blocker.Evidence)
			}
		}
	}
	if err == nil {
		err = sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Submit: true})
	}
	var witness store.Witness
	var accepted bool
	if err == nil {
		witness, accepted, err = run.cmd.awaitReceipt(ctx, l.Paths, old.ID, queue)
	}
	if err == nil && !accepted {
		var matched bool
		matched, err = harness.SubmittedPromptMatches(c.Primitives.SubmitWitness, wire, witness.Prompt)
		if err == nil && !matched {
			if queue != nil && (a.Native.SessionID == "" || witness.SessionID == a.Native.SessionID) {
				accepted, err = queue(ctx)
			}
			if err == nil && !accepted {
				err = fmt.Errorf("submit witness does not match the message text and one-time ID")
			}
		}
	}
	if err != nil {
		outcome, reason = "unverified", err.Error()
	} else if accepted {
		outcome, reason = "accepted", "native queue shows sender and one-time ID; do not resend"
	} else {
		if a.Native.SessionID != "" && witness.SessionID != "" && a.Native.SessionID != witness.SessionID {
			outcome, reason = "unverified", "submit witness belongs to another native session"
		} else {
			a.Native.SessionID = witness.SessionID
			a.Native.TurnID = witness.TurnID
			a.Native.Transcript = witness.Transcript
		}
	}
	if err := run.finishInput(l, a, e, outcome, reason); err != nil {
		return outcome, err
	}
	if outcome == "unverified" {
		if err := run.reconcileDelivery(l, a); err != nil {
			return outcome, err
		}
		if a.LastDelivered == e.ID {
			outcome = "delivered"
		}
	}
	if err := run.mark(*a); err != nil {
		return outcome, err
	}
	return outcome, nil
}

// completedResume finds a completed compaction's resume note. It goes first:
// it carries the state the agent needs before reading anything queued behind it.
func completedResume(a *core.Agent, pending []core.Envelope) *core.Envelope {
	if c := a.Compaction; c != nil && c.Status == "completed" {
		for i := range pending {
			if pending[i].ID == core.EnvelopeID("resume-"+c.ID) {
				return &pending[i]
			}
		}
	}
	return nil
}

// drainLocked returns the queue it could not deliver. The owner uses those
// identities to distinguish existing blocked work from arrivals during unlock.
func (run *runtime) drainLocked(l *store.LockedAgent, a *core.Agent, target core.EnvelopeID) (string, []core.Envelope, error) {
	result := "queued"
	if err := run.checkDeadlines(l, a); err != nil {
		return result, nil, err
	}
	if a.Status != core.Active {
		pending, err := l.Paths.ListNew()
		return result, pending, err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return result, nil, err
	}
	b, err := run.input()
	if err != nil {
		return result, nil, err
	}
	for {
		pending, err := l.Paths.ListNew()
		if err != nil {
			return result, nil, err
		}
		next := completedResume(a, pending)
		for i := 0; next == nil && i < len(pending); i++ {
			if !pending[i].NotBefore.After(run.cmd.now()) {
				next = &pending[i]
			}
		}
		if next == nil {
			return result, pending, nil
		}
		if next.From.Name == "compact" && strings.HasPrefix(string(next.ID), "resume-") {
			c := a.Compaction
			if c == nil || next.ID != core.EnvelopeID("resume-"+c.ID) || c.Status != "completed" {
				return result, pending, nil
			}
		}
		if strings.HasPrefix(string(a.LastFailed), "startup-") {
			return result, pending, nil
		}
		free, reason, err := run.available(l, a, b, c)
		if err != nil {
			return result, pending, err
		}
		if !free {
			if reason != "" && target != "" && run.cmd.stderr != nil {
				if _, err := fmt.Fprintf(run.cmd.stderr, "%s input blocked: %s; message remains queued\n", a.Name, reason); err != nil {
					return result, pending, err
				}
			}
			return result, pending, nil
		}
		outcome, err := run.deliver(l, a, *next, b, c)
		if next.ID == target {
			result = outcome
		}
		if err != nil {
			return result, pending, err
		}
	}
}
func (run *runtime) drain(id core.HitchID, target core.EnvelopeID) (string, error) {
	l, a, err := run.acquire(id, false)
	if errors.Is(err, store.ErrLocked) {
		return "queued", nil
	}
	if err != nil {
		return "queued", err
	}
	return run.drainFrom(l, a, target)
}
func (run *runtime) drainFrom(l *store.LockedAgent, a core.Agent, target core.EnvelopeID) (string, error) {
	result := "queued"
	for {
		got, waiting, err := run.drainLocked(l, &a, target)
		if got != "queued" {
			result = got
		}
		closeErr := l.Close()
		if err != nil {
			return result, err
		}
		if closeErr != nil {
			return result, closeErr
		}
		if run.cmd.afterUnlock != nil {
			run.cmd.afterUnlock()
		}
		pending, err := l.Paths.ListNew()
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		if err != nil {
			return result, err
		}
		seen := make(map[core.EnvelopeID]bool, len(waiting))
		for _, e := range waiting {
			seen[e.ID] = true
		}
		arrived := false
		for _, e := range pending {
			if !seen[e.ID] && !e.NotBefore.After(run.cmd.now()) {
				arrived = true
				break
			}
		}
		if !arrived {
			return result, nil
		}
		l, a, err = run.acquire(a.ID, false)
		if errors.Is(err, store.ErrLocked) || errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		if err != nil {
			return result, err
		}
	}
}
func deliveryResult(outcome string) error {
	if outcome == "unverified" {
		return commandError{status: exitUnknown, text: "message input is unverified; inspect the recipient before sending again"}
	}
	return nil
}
