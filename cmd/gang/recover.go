package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) tickAgent(id core.HitchID, notice hookNotice, wait bool) error {
	team, err := run.team.ReadTeam()
	if err != nil {
		return err
	}
	if !team.Curfew.IsZero() && !run.cmd.now().Before(team.Curfew) {
		return run.dropWithLock(id, wait)
	}
	l, a, err := run.acquire(id, wait)
	if errors.Is(err, store.ErrLocked) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer l.Close()
	if err := run.checkDeadlines(l, &a); err != nil {
		return err
	}
	if a.Status == core.Dropping || a.Status == core.Failed {
		return run.mark(a)
	}
	if err := run.reconcileNativeBoundary(l, &a, notice); err != nil {
		return err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	screen, err := b.Capture(ctx, substrate.PaneID(a.Pane))
	if err != nil {
		return errors.Join(err, run.observeProbeFailure(l, &a, err))
	}
	if a.Status == core.Booting {
		startup, err := harness.InspectStartup(c, screen)
		if err != nil {
			return err
		}
		if startup.State != harness.StartupReady {
			if startup.State == harness.StartupTrustRequired {
				if err := run.apply(l, &a, core.Event{Type: "hitch_blocked", Reason: startup.Prompt}); err != nil {
					return err
				}
				return run.mark(a)
			}
			return nil
		}
		if err := run.apply(l, &a, core.Event{Type: "hitch_ready"}); err != nil {
			return err
		}
	}
	if err := run.refreshNative(l, &a, c); err != nil {
		return err
	}
	if notice.Kind == "compaction-finished" && !notice.At.IsZero() {
		if err := run.acceptContextReadings(&a, c, []core.Reading{{Kind: "compaction-finished", Source: "native-hook", At: &notice.At}}); err != nil {
			return err
		}
		if err := l.Save(a); err != nil {
			return err
		}
		if err := run.publishContext(a); err != nil {
			return err
		}
	}
	if err := run.acceptContextReadings(&a, c, notice.Readings); err != nil {
		return err
	}
	if err := run.observeContextBands(l, &a, c, screen); err != nil {
		return err
	}
	if err := run.observeCompaction(l, &a, c, screen); err != nil {
		return err
	}
	if err := run.observeActivity(l, &a, c, screen); err != nil {
		return err
	}
	if a.Activity == core.Idle && !a.InterruptDeadline.IsZero() {
		if err := run.apply(l, &a, core.Event{Type: "interrupt_completed"}); err != nil {
			return err
		}
	}
	if err := run.continueCompaction(l, &a); err != nil {
		return err
	}
	if err := run.startCompaction(l, &a); err != nil {
		return err
	}
	capacity, found, err := harness.DetectCapacity(c, screen)
	if err != nil {
		return err
	}
	if found {
		if err := run.apply(l, &a, core.Event{Type: "capacity_detected", Fingerprint: capacity.Fingerprint, Reason: capacity.Evidence, Deadline: run.cmd.now().Add(run.settings.CapacityTimeout)}); err != nil {
			return err
		}
		if !a.Capacity.Submitted && !run.cmd.now().Before(a.Capacity.NextAt) && run.cmd.now().Before(a.Capacity.Deadline) {
			pending, err := l.Paths.ListNew()
			if err != nil {
				return err
			}
			due := false
			for _, queued := range pending {
				if !queued.NotBefore.After(run.cmd.now()) {
					due = true
					break
				}
			}
			if !due {
				token, err := randomEnvelopeToken()
				if err != nil {
					return err
				}
				e := core.Envelope{ID: core.EnvelopeID("capacity-" + a.Capacity.Fingerprint), Token: token, Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "capacity-recovery"}, Message: core.Message{Text: "Continue the interrupted work after the provider capacity error."}, CreatedAt: run.cmd.now()}
				if err := run.publishOnce(l, &a, e); err != nil {
					return err
				}
				if err := run.apply(l, &a, core.Event{Type: "capacity_submitted"}); err != nil {
					return err
				}
			}
		}
	} else if a.Capacity.Fingerprint != "" && a.Activity == core.Busy {
		if err := run.apply(l, &a, core.Event{Type: "capacity_cleared"}); err != nil {
			return err
		}
	}
	if err := run.mark(a); err != nil {
		return err
	}
	_, err = run.drainFrom(l, a, "")
	return err
}
func (run *runtime) reconcileNativeBoundary(l *store.LockedAgent, a *core.Agent, notice hookNotice) error {
	witness, witnessErr := l.Paths.ReadWitness()
	if run.afterWitnessRead != nil {
		run.afterWitnessRead()
	}
	if witnessErr == nil {
		if a.Native.SessionID != "" && witness.SessionID != a.Native.SessionID {
			return fmt.Errorf("native witness changed session identity")
		}
		if witness.At.After(a.Native.SubmittedAt) {
			a.Native.SessionID, a.Native.TurnID, a.Native.Transcript, a.Native.SubmittedAt = witness.SessionID, witness.TurnID, witness.Transcript, witness.At
			if err := l.Save(*a); err != nil {
				return err
			}
		}
		// UserPromptSubmit is synchronous: a distinct native prompt ID proves
		// the prior failure no longer describes the current turn.
		if a.Native.TurnFailure != "" && a.Native.FailedTurn != "" && witness.TurnID != "" && witness.TurnID != a.Native.FailedTurn {
			a.Native.TurnFailure, a.Native.FailedTurn = "", ""
			if err := l.Save(*a); err != nil {
				return err
			}
		}
	} else if !errors.Is(witnessErr, os.ErrNotExist) {
		return witnessErr
	}
	if notice.SessionID != "" && a.Native.SessionID != "" && notice.SessionID != a.Native.SessionID {
		return fmt.Errorf("hook boundary belongs to another native session")
	}
	if a.Native.SessionID == "" {
		a.Native.SessionID = notice.SessionID
	}
	if a.Native.Transcript == "" {
		a.Native.Transcript = notice.Transcript
	}
	if notice.Kind == "turn-failed" {
		// A delayed async hook may start after the next synchronous submit.
		// Only native prompt identity can attribute its reason to this turn.
		reason := notice.Failure
		if reason == "" {
			reason = "native turn failed"
		}
		if notice.TurnID != "" && witnessErr == nil && witness.TurnID != "" && notice.TurnID != witness.TurnID {
			// The old failure is still present in the raw native_hook audit event.
		} else if notice.TurnID != "" && witnessErr == nil && witness.TurnID == notice.TurnID {
			a.Native.TurnFailure, a.Native.FailedTurn = reason, notice.TurnID
		} else {
			a.Native.TurnFailure, a.Native.FailedTurn = "native failure without turn identity: "+reason, ""
		}
		if err := l.Save(*a); err != nil {
			return err
		}
	} else if notice.Kind == "turn-finished" && a.Native.TurnFailure != "" && a.Native.FailedTurn == "" && notice.TurnID != "" && witnessErr == nil && notice.TurnID == witness.TurnID {
		a.Native.TurnFailure = ""
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	return nil
}

func (run *runtime) publishOnce(l *store.LockedAgent, a *core.Agent, e core.Envelope) error {
	for _, dir := range []string{"new", "cur", "failed"} {
		_, err := l.Paths.ReadEnvelope(dir, e.ID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := l.Paths.Publish(e); err != nil {
		return err
	}
	return run.record(*a, core.Event{Type: "send_queued", Envelope: &e})
}
func (run *runtime) continueCompaction(l *store.LockedAgent, a *core.Agent) error {
	c := a.Compaction
	if c == nil {
		return nil
	}
	if (c.Status == "submitted" || c.Status == "unverified") && c.CompletedAt.After(c.StartedAt) {
		if err := run.apply(l, a, core.Event{Type: "compaction_completed", ID: c.ID, At: c.CompletedAt}); err != nil {
			return err
		}
		c = a.Compaction
	}
	if c.Status != "completed" || c.Continuation {
		return nil
	}
	sender := c.ResumeFrom
	if sender.Kind == "" {
		sender = core.Sender{Kind: core.SenderGangline, Name: "compact"}
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	e := core.Envelope{ID: core.EnvelopeID("resume-" + c.ID), Token: token, Recipient: a.ID, To: a.Name, From: sender, Message: c.Resume, CreatedAt: run.cmd.now()}
	if err := run.publishOnce(l, a, e); err != nil {
		return err
	}
	c.Continuation = true
	return l.Save(*a)
}
