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
	"github.com/adambiggs/gangline/substrate/tmux"
)

func (run *runtime) tickAgent(id core.HitchID, notice hookNotice, wait bool) error {
	team, err := run.team.ReadTeam()
	if err != nil {
		return err
	}
	if !team.Curfew.IsZero() && !run.cmd.now().Before(team.Curfew) {
		lock, err := (store.Paths{Root: run.settings.StateRoot}).LockTeam(run.settings.Session)
		if errors.Is(err, store.ErrLocked) {
			return nil
		}
		if err != nil {
			return err
		}
		defer lock.Close()
		team, err = run.team.ReadTeam()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if team.Curfew.IsZero() || run.cmd.now().Before(team.Curfew) {
			return nil
		}
		return run.dropWithLock(id, wait)
	}
	l, a, err := run.acquire(id, wait)
	if errors.Is(err, store.ErrLocked) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer run.unlock(l)
	if err := run.checkDeadlines(l, &a); err != nil {
		return err
	}
	if a.Status == core.Failed && a.Pane != "" {
		if _, err := run.forgetClosedPane(l, &a); err != nil {
			return err
		}
	}
	if a.Status == core.Dropping || a.Status == core.Failed {
		return run.mark(a)
	}
	// A claim whose hitch ended before creating a pane has nothing to probe;
	// its boot deadline fails it.
	if a.Pane == "" {
		return nil
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if err := run.reconcileNativeBoundary(l, &a, c, notice); err != nil {
		return err
	}
	if a.Status == core.Failed {
		return run.mark(a)
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	screen, err := run.captureRegistered(ctx, b, a)
	// A held pane reports its native exit instead of a screen, and that exit
	// ends the agent.
	var exited *substrate.ExitedError
	if errors.As(err, &exited) {
		if err := run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: exited.Error()}); err != nil {
			return err
		}
		return run.mark(a)
	}
	if err != nil {
		// A pane closed outside gang ends the agent; nothing is left to probe.
		closed, checkErr := run.forgetClosedPane(l, &a)
		if checkErr != nil || !closed {
			return errors.Join(err, checkErr, run.observeProbeFailure(l, &a, err))
		}
		return nil
	}
	if a.Status == core.Booting {
		startup, err := harness.InspectStartup(c, screen)
		if err != nil {
			return err
		}
		if startup.State == harness.StartupReady {
			if run.cmd.awaitStartup != nil {
				startup, screen, err = run.cmd.awaitStartup(ctx, substrate.PaneID(a.Pane), c)
			} else {
				startup, screen, err = harness.AwaitStartup(ctx, b.Capture, substrate.PaneID(a.Pane), c)
			}
			if err != nil {
				return err
			}
		}
		if startup.State != harness.StartupReady {
			if startup.State == harness.StartupTrustRequired {
				// Every tick and roster read sees the prompt until the
				// operator answers it, so it is recorded once.
				if a.Activity != core.Blocked || a.Evidence != startup.Prompt {
					if err := run.apply(l, &a, core.Event{Type: "hitch_blocked", Reason: startup.Prompt}); err != nil {
						return err
					}
				}
				return run.mark(a)
			}
			return nil
		}
	}
	if a.Status == core.Booting || a.Registration.Held {
		// Startup is observed, so the pane stops holding a native exit. An
		// exit it already holds ends the agent with that output. A ready agent
		// whose hitch stopped before its release is still held.
		registry, err := run.registry()
		if err != nil {
			return err
		}
		if err := registry.ReleaseRegisteredExit(ctx, paneIdentity(a)); err != nil {
			// A pane id that names another pane shows nothing of this agent's
			// startup, so the boot deadline fails the agent. A blocked startup
			// has no deadline and waits only on its own pane.
			if a.Status == core.Booting && errors.Is(err, tmux.ErrPaneReplaced) && (a.Activity == core.Blocked || !a.BootDeadline.IsZero() && !run.cmd.now().Before(a.BootDeadline)) {
				return run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: tmux.ErrPaneReplaced.Error()})
			}
			if !errors.As(err, &exited) {
				return err
			}
			if err := run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: exited.Error()}); err != nil {
				return err
			}
			return run.mark(a)
		}
		a.Registration.Held = false
		if a.Status != core.Booting {
			if err := l.Save(a); err != nil {
				return err
			}
		} else if err := run.apply(l, &a, core.Event{Type: "hitch_ready"}); err != nil {
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
	if err := run.observeUsageBands(a, c); err != nil {
		return err
	}
	if err := run.observeCompaction(l, &a, c, screen); err != nil {
		return err
	}
	if err := run.observeActivity(l, &a, c, screen); err != nil {
		return err
	}
	if err := run.notifyHeldInput(a); err != nil {
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
	if err := run.observeSnoozeTurn(a, c.Primitives.TurnBoundary, notice, found); err != nil {
		return err
	}
	if err := run.observeAutoCap(a, notice); err != nil {
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

// reconcileNativeBoundary records a hook boundary in the agent's native turn
// state.
func (run *runtime) reconcileNativeBoundary(l *store.LockedAgent, a *core.Agent, c harness.Collar, notice hookNotice) error {
	witness, witnessErr := l.Paths.ReadWitness()
	if run.afterWitnessRead != nil {
		run.afterWitnessRead()
	}
	if witnessErr == nil {
		session := a.Native.SessionID
		same, err := followNativeSession(c, a, witness.SessionID, witness.Transcript)
		if err != nil || !same {
			// The witness persists, so the agent fails once and its hitcher
			// learns both sessions instead of every tick failing unseen.
			reason := fmt.Sprintf("its native session changed from %s to %s, and no harness record shows that %s continues the conversation", session, witness.SessionID, witness.SessionID)
			if err != nil {
				reason += fmt.Sprintf(" (%v)", err)
			}
			// Hitch takes its directory from the caller and its role from
			// flags, so the route names the agent's own.
			role := ""
			if a.Role != "" {
				role = " -r " + a.Role
			}
			reason += fmt.Sprintf("; to continue either session, gang drop %s, then gang hitch %s -c %s -d %q%s --resume SESSION", a.Name, a.Name, a.Collar, a.Directory, role)
			return run.apply(l, a, core.Event{Type: "hitch_failed", Reason: reason})
		}
		if witness.At.After(a.Native.SubmittedAt) {
			a.Native.SessionID, a.Native.TurnID, a.Native.Transcript, a.Native.SubmittedAt = witness.SessionID, witness.TurnID, witness.Transcript, witness.At
			if err := l.Save(*a); err != nil {
				return err
			}
		}
		// UserPromptSubmit is synchronous: a distinct native prompt ID proves
		// the prior failure no longer describes the current turn, unless the
		// failed turn ran from the queue after the witnessed prompt.
		if a.Native.TurnFailure != "" && a.Native.FailedTurn != "" && witness.TurnID != "" && witness.TurnID != a.Native.FailedTurn {
			if queuedAfter, _ := harness.TurnRanAfter(c.Primitives.TurnBoundary, a.Native.Transcript, witness.TurnID, a.Native.FailedTurn); !queuedAfter {
				a.Native.TurnFailure, a.Native.FailedTurn = "", ""
				if err := l.Save(*a); err != nil {
					return err
				}
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
	// A prompt queued behind the finished turn runs next with no submit hook,
	// so the turn stays open for it. An unreadable queue closes the turn.
	transcript := notice.Transcript
	if transcript == "" {
		transcript = a.Native.Transcript
	}
	queued := false
	if notice.Kind == "turn-finished" {
		queued, _ = harness.QueuedTurnPending(c.Primitives.TurnBoundary, transcript, notice.TurnID)
	}
	if (notice.Kind == "turn-finished" || notice.Kind == "turn-failed") && !queued {
		// Gang records its own delivery after the submit hook ran, so the
		// witnessed prompt's boundary closes the turn even when it ran first.
		finished := notice.At
		if notice.TurnID != "" && witnessErr == nil && notice.TurnID == witness.TurnID && a.Native.SubmittedAt.After(finished) {
			finished = a.Native.SubmittedAt
		}
		if finished.After(a.Native.FinishedAt) {
			a.Native.FinishedAt = finished
			if err := l.Save(*a); err != nil {
				return err
			}
		}
	}
	if notice.Kind == "turn-failed" {
		// A delayed async hook may start after the next synchronous submit.
		// Only native prompt identity can attribute its reason to this turn.
		// A turn that ran from the queue carries a prompt id no submit hook
		// announced, so the transcript must order it after the witnessed one.
		reason := notice.Failure
		if reason == "" {
			reason = "native turn failed"
		}
		if queuedAfter, _ := harness.TurnRanAfter(c.Primitives.TurnBoundary, transcript, witness.TurnID, notice.TurnID); notice.TurnID != "" && witnessErr == nil && witness.TurnID != "" && notice.TurnID != witness.TurnID && queuedAfter {
			a.Native.TurnFailure, a.Native.FailedTurn = reason, notice.TurnID
		} else if notice.TurnID != "" && witnessErr == nil && witness.TurnID != "" && notice.TurnID != witness.TurnID {
			// The old failure is still present in the raw native_hook audit event.
		} else if notice.TurnID != "" && witnessErr == nil && witness.TurnID == notice.TurnID {
			a.Native.TurnFailure, a.Native.FailedTurn = reason, notice.TurnID
		} else {
			a.Native.TurnFailure, a.Native.FailedTurn = "native failure without turn identity: "+reason, ""
		}
		if err := l.Save(*a); err != nil {
			return err
		}
	} else if notice.Kind == "turn-finished" && a.Native.TurnFailure != "" && a.Native.FailedTurn == "" && notice.TurnID != "" && witnessErr == nil {
		// The witnessed turn's finish, or that of a turn the transcript
		// shows ran from the queue after it, is a success after the failure.
		if queuedAfter, _ := harness.TurnRanAfter(c.Primitives.TurnBoundary, transcript, witness.TurnID, notice.TurnID); notice.TurnID == witness.TurnID || queuedAfter {
			a.Native.TurnFailure = ""
			if err := l.Save(*a); err != nil {
				return err
			}
		}
	} else if notice.Kind == "turn-finished" && a.Native.FailedTurn != "" {
		// A turn that ran from the queue fires no submit hook, so only the
		// transcript shows it started after the failed turn. A finish that
		// the transcript cannot order may be a stale hook from an older turn.
		if after, _ := harness.TurnRanAfter(c.Primitives.TurnBoundary, transcript, a.Native.FailedTurn, notice.TurnID); after {
			a.Native.TurnFailure, a.Native.FailedTurn = "", ""
			if err := l.Save(*a); err != nil {
				return err
			}
		}
	}
	return nil
}

func (run *runtime) publishOnce(l *store.LockedAgent, a *core.Agent, e core.Envelope) error {
	return run.publishOnceTo(l.Paths, *a, e)
}
func (run *runtime) publishOnceTo(p store.AgentPaths, a core.Agent, e core.Envelope) error {
	if published, err := envelopePublished(p, e.ID); published || err != nil {
		return err
	}
	if err := p.Publish(e); err != nil {
		return err
	}
	return run.record(a, core.Event{Type: "send_queued", Envelope: &e})
}

// envelopePublished reports whether an envelope with id was ever published to
// p: queued, delivered, or failed.
func envelopePublished(p store.AgentPaths, id core.EnvelopeID) (bool, error) {
	for _, dir := range []string{"new", "cur", "failed"} {
		_, err := p.ReadEnvelope(dir, id)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
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
	if c.Status != "completed" {
		return nil
	}
	id := core.EnvelopeID("resume-" + c.ID)
	_, err := l.Paths.ReadEnvelope("new", id)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if c.Continuation && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return run.failCompaction(l, a, compactionRan, "resume was not entered when compaction started; continuation withheld")
}

func (run *runtime) publishCompactionResume(l *store.LockedAgent, a *core.Agent) (core.Envelope, error) {
	c := a.Compaction
	if c.ResumeToken == "" {
		token, err := randomEnvelopeToken()
		if err != nil {
			return core.Envelope{}, err
		}
		c.ResumeToken = token
		if err := l.Save(*a); err != nil {
			return core.Envelope{}, err
		}
	}
	e := core.Envelope{ID: core.EnvelopeID("resume-" + c.ID), Token: c.ResumeToken, Recipient: a.ID, To: a.Name, From: c.ResumeFrom, Message: c.Resume, Purpose: "resume", CreatedAt: run.cmd.now()}
	if err := run.publishOnce(l, a, e); err != nil {
		return core.Envelope{}, err
	}
	c.Continuation = true
	if err := l.Save(*a); err != nil {
		return core.Envelope{}, err
	}
	return e, nil
}
