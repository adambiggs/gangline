package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// A delivery receipt proves submission, not successful execution. Only the
// exact native prompt of that startup can attribute a failure to it.
func startupTurnFailed(l *store.LockedAgent, a core.Agent, c harness.Collar) (bool, error) {
	if a.Native.TurnFailure == "" || a.Native.FailedTurn == "" {
		return false, nil
	}
	w, err := l.Paths.ReadWitness()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if w.TurnID != a.Native.FailedTurn || w.SessionID != a.Native.SessionID {
		return false, nil
	}
	if a.Native.StartupTurnID != "" {
		return a.Native.StartupSessionID == w.SessionID && a.Native.StartupTurnID == w.TurnID, nil
	}
	// Earlier records carry no startup binding; the retained exact receipt
	// can still establish it while that startup's witness remains current.
	e, startup, err := retainedStartup(l.Paths, "cur", a.LastDelivered)
	if err != nil || !startup {
		return false, err
	}
	wire, err := envelopeText(e)
	if err != nil {
		return false, err
	}
	return harness.SubmittedPromptStartsWith(c.Primitives.SubmitWitness, wire, w.Prompt)
}

func (run *runtime) notifyStartupFailure(l *store.LockedAgent, a *core.Agent, c harness.Collar) error {
	startup, err := startupTurnFailed(l, *a, c)
	if err != nil || !startup || a.Native.StartupFailureNoticed == a.Native.FailedTurn {
		return err
	}
	id := core.EnvelopeID(fmt.Sprintf("startup-turn-failed-%s-%s", a.ID, a.Native.FailedTurn))
	published, err := run.noticeHitcher(*a, id, fmt.Sprintf("%s, which you hitched, failed its native startup turn: %s. The startup was submitted but its execution failed; gang status %s --why shows its record.", a.Name, a.Native.TurnFailure, a.Name))
	if err != nil || !published {
		return err
	}
	a.Native.StartupFailureNoticed = a.Native.FailedTurn
	return l.Save(*a)
}
