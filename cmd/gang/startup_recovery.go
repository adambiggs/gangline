package main

import (
	"errors"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func startupAttention(a core.Agent, route string) error {
	return commandError{status: exitNative, text: fmt.Sprintf("%s startup is queued in %s; resolve native prompts, then run %s; the original contract and assignment are retained", a.Name, a.Pane, route)}
}

func isStartupEnvelope(e core.Envelope) bool {
	return e.Purpose == "startup" || e.Purpose == "assignment"
}

func retainedStartup(p store.AgentPaths, dir string, id core.EnvelopeID) (core.Envelope, bool, error) {
	if id == "" {
		return core.Envelope{}, false, nil
	}
	e, err := p.ReadEnvelope(dir, id)
	if errors.Is(err, os.ErrNotExist) {
		return core.Envelope{}, false, nil
	}
	return e, err == nil && isStartupEnvelope(e), err
}

// reopenBoot resumes the startup of an agent that its boot deadline failed
// while the pane showed a screen startup did not recognize, typically a native
// prompt. Once the pane shows a ready composer or a recognized prompt, the
// ordinary startup path takes over in the same pane. Any other failure, or a
// screen still unrecognized, leaves the agent failed.
func (run *runtime) reopenBoot(l *store.LockedAgent, a *core.Agent) error {
	if a.Evidence != core.BootDeadlineElapsed || a.Pane == "" {
		return inactiveRecipient(*a)
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	screen, err := b.Capture(ctx, substrate.PaneID(a.Pane))
	if err != nil {
		return refuseError("%s failed when its boot deadline elapsed and %s cannot be read: %v; drop it and hitch it again", a.Name, a.Pane, err)
	}
	startup, err := harness.InspectStartup(c, screen)
	if err != nil {
		return err
	}
	if startup.State == harness.StartupOccupied {
		return commandError{status: exitNative, text: fmt.Sprintf("%s failed when its boot deadline elapsed and %s still shows no recognized startup screen (%s); answer any native prompt there, then run gang hitch %s --recover; the original contract and assignment are retained", a.Name, a.Pane, startup.Prompt, a.Name)}
	}
	return run.apply(l, a, core.Event{Type: "boot_reopened", Deadline: run.cmd.now().Add(bootTimeout)})
}

// Recovery never reconstructs startup from today's prose or from an ordinary send.
func (run *runtime) recoverStartup(name string) (result error) {
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	l, a, err := run.acquire(a.ID, false)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	pending, err := l.Paths.ListNew()
	if err != nil {
		return err
	}
	for _, e := range pending {
		if isStartupEnvelope(e) {
			if a.Status == core.Failed {
				if err := run.reopenBoot(l, &a); err != nil {
					return err
				}
			}
			if err := run.unlock(l); err != nil {
				return err
			}
			if err := run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				return err
			}
			got, err := l.Paths.ReadEnvelope("cur", e.ID)
			if errors.Is(err, os.ErrNotExist) {
				// A boot deadline can still fail the agent, which only recovery resumes.
				return startupAttention(a, "gang hitch "+string(a.Name)+" --recover")
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(run.cmd.stdout, "%s\t%s\n", got.ID, got.Outcome)
			return err
		}
	}
	_, deliveredStartup, err := retainedStartup(l.Paths, "cur", a.LastDelivered)
	if err != nil {
		return err
	}
	e, failedStartup, err := retainedStartup(l.Paths, "failed", a.LastFailed)
	if err != nil {
		return err
	}
	if deliveredStartup && !failedStartup {
		_, err := fmt.Fprintf(run.cmd.stdout, "%s\tdelivered\n", a.LastDelivered)
		return err
	}
	if a.Status != core.Active {
		return inactiveRecipient(a)
	}
	if !failedStartup {
		return refuseError("no retained unverified startup message for %s", name)
	}
	if e.Outcome != "unverified" {
		return refuseError("startup result is %s, not unverified", e.Outcome)
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	b = run.registeredInput(a, b)
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	screen, err := b.Capture(ctx, substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	if blocked, found, err := harness.InputBlocked(c, screen); err != nil {
		return err
	} else if found {
		return commandError{status: exitNative, text: blocked.Evidence + "; resolve it before recovering startup"}
	}
	wire, err := envelopeText(e)
	if err != nil {
		return err
	}
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	replaceCollapsed := err == nil && c.Actions.StartupReplace != nil && composer.CollapsedChars == utf8.RuneCountInString(wire)
	idle, idleErr := harness.Idle(c, screen)
	repasteEmpty := err == nil && idleErr == nil && idle && composer.Text == "" && !composer.TailOccupied && composer.CollapsedChars == 0 && e.PasteOnly != nil
	if err != nil || !harness.SameComposerText(composer.Text, wire) && !replaceCollapsed && !repasteEmpty {
		path, _ := l.Paths.EnvelopePath("failed", e.ID)
		return commandError{status: exitUnknown, text: fmt.Sprintf("original startup text is not identifiable in the composer; input remains unverified; if the composer is empty, re-hitch with the retained contract and assignment: %s", path)}
	}
	if err := requireHarnessForeground(ctx, b, substrate.PaneID(a.Pane), c); err != nil {
		return err
	}
	old, err := l.Paths.ReadWitness()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if repasteEmpty && old.ID != e.PasteOnly.WitnessID {
		path, _ := l.Paths.EnvelopePath("failed", e.ID)
		return commandError{status: exitUnknown, text: fmt.Sprintf("startup submit witness changed after paste; input remains unverified; retained startup: %s", path)}
	}
	// Record the new submit intent before moving the prior receipt. A crash at
	// either boundary stays unverified through normal input-owner recovery.
	if err := run.reopenUnverified(l, &a, e); err != nil {
		return err
	}
	beforeSubmit := func() error { return startupSubmitPossible(l.Paths, &e) }
	if replaceCollapsed || repasteEmpty {
		err = run.replaceCollapsedStartup(ctx, b, substrate.PaneID(a.Pane), c, wire, l.Paths, old.ID, replaceCollapsed, beforeSubmit)
	} else if err = beforeSubmit(); err == nil {
		err = sendHarnessKeys(ctx, b, substrate.PaneID(a.Pane), c, substrate.Keys{Submit: true})
	}
	var witness store.Witness
	if err == nil {
		witness, err = run.cmd.awaitWitness(ctx, l.Paths, old.ID)
	}
	if err == nil {
		var matched bool
		matched, err = harness.SubmittedPromptMatches(c.Primitives.SubmitWitness, wire, witness.Prompt)
		if err == nil && !matched {
			err = fmt.Errorf("submit witness does not match the original startup message")
		}
	}
	outcome, reason := "delivered", ""
	if err != nil {
		outcome, reason = "unverified", err.Error()
		if replaceCollapsed {
			reason = "startup draft replacement incomplete; if the composer is empty, re-hitch with the retained assignment: " + reason
		}
	} else if a.Native.SessionID != "" && a.Native.SessionID != witness.SessionID {
		outcome, reason = "unverified", "submit witness belongs to another native session"
	} else {
		a.Native.SessionID = witness.SessionID
		a.Native.TurnID = witness.TurnID
		a.Native.Transcript = witness.Transcript
	}
	if err := run.finishInput(l, &a, e, outcome, reason); err != nil {
		return err
	}
	if outcome == "delivered" {
		if err := run.notifySender(a, e, outcome); err != nil {
			return err
		}
	}
	if err := run.mark(a); err != nil {
		return err
	}
	if err := deliveryResult(outcome); err != nil {
		path, _ := l.Paths.EnvelopePath("failed", e.ID)
		return commandError{status: exitUnknown, text: fmt.Sprintf("startup recovery is unverified: %s; inspect %s before retrying gang hitch %s --recover; retained startup: %s", reason, a.Name, a.Name, path)}
	}
	_, err = fmt.Fprintf(run.cmd.stdout, "%s\t%s\n", e.ID, outcome)
	return err
}
