package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func startupAttention(a core.Agent) error {
	return commandError{status: exitNative, text: fmt.Sprintf("%s startup is queued in %s; resolve native prompts, then run gang tick; the original contract and assignment are retained", a.Name, a.Pane)}
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
			if err := l.Close(); err != nil {
				return err
			}
			if err := run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				return err
			}
			got, err := l.Paths.ReadEnvelope("cur", e.ID)
			if errors.Is(err, os.ErrNotExist) {
				return startupAttention(a)
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
		return refuseError("recipient is not active")
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
	if err != nil || composer.Text != wire {
		path, _ := l.Paths.EnvelopePath("failed", e.ID)
		return commandError{status: exitUnknown, text: fmt.Sprintf("original startup text is not identifiable in the composer; input remains unverified; retained contract and assignment: %s", path)}
	}
	if err := requireHarnessForeground(ctx, b, substrate.PaneID(a.Pane), c); err != nil {
		return err
	}
	old, err := l.Paths.ReadWitness()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Record the new submit intent before moving the prior receipt. A crash at
	// either boundary stays unverified through normal input-owner recovery.
	if err := run.reopenUnverified(l, &a, e); err != nil {
		return err
	}
	err = sendHarnessKeys(ctx, b, substrate.PaneID(a.Pane), c, substrate.Keys{Submit: true})
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
	} else if a.Native.SessionID != "" && witness.SessionID != "" && a.Native.SessionID != witness.SessionID {
		outcome, reason = "unverified", "submit witness belongs to another native session"
	} else {
		a.Native.SessionID = witness.SessionID
		a.Native.TurnID = witness.TurnID
		a.Native.Transcript = witness.Transcript
	}
	if err := run.finishInput(l, &a, e, outcome, reason); err != nil {
		return err
	}
	if err := run.mark(a); err != nil {
		return err
	}
	if err := deliveryResult(outcome); err != nil {
		return err
	}
	_, err = fmt.Fprintf(run.cmd.stdout, "%s\t%s\n", e.ID, outcome)
	return err
}
