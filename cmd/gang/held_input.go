package main

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// observeHeldInput measures only readable, unchanged draft text. Output above
// the composer and unrelated agent events do not identify a new draft.
func (run *runtime) observeHeldInput(l *store.LockedAgent, a core.Agent, c harness.Collar, screen substrate.Screen) error {
	composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
	if err != nil || composer.CollapsedChars != 0 || composer.TailOccupied {
		return resetHeldInputWindow(l)
	}
	if composer.Text == "" || a.Status != core.Active || a.Input != nil {
		return l.SaveHeldInput(nil)
	}
	draft, err := l.ReadHeldInput()
	if err != nil {
		return err
	}
	now := run.cmd.now()
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(composer.Text)))
	if draft.Fingerprint != fingerprint || draft.Generation != a.Registration.Generation || draft.Session != a.Native.SessionID || !draft.SubmittedAt.Equal(a.Native.SubmittedAt) {
		return l.SaveHeldInput(&store.HeldInput{Fingerprint: fingerprint, Generation: a.Registration.Generation, Session: a.Native.SessionID, SubmittedAt: a.Native.SubmittedAt, Since: now})
	}
	if draft.Since.IsZero() || now.Before(draft.Since) {
		draft.Since = now
		return l.SaveHeldInput(&draft)
	}
	return nil
}

// Missing observation breaks the timer but cannot prove an announced draft
// changed. Keep its fingerprint and notice latch until readable evidence does.
func resetHeldInputWindow(l *store.LockedAgent) error {
	draft, err := l.ReadHeldInput()
	if err != nil || draft.Since.IsZero() {
		return err
	}
	draft.Since = time.Time{}
	return l.SaveHeldInput(&draft)
}

// notifyHeldInput retains publication separately from prunable mail receipts.
func (run *runtime) notifyHeldInput(l *store.LockedAgent, a core.Agent) error {
	draft, err := l.ReadHeldInput()
	if err != nil || draft.Since.IsZero() || draft.Noticed || a.Status != core.Active || a.Activity != core.Blocked || a.Evidence != heldInputEvidence || run.cmd.now().Before(draft.Since.Add(run.settings.TeamIdleMinDuration)) {
		return err
	}
	id := core.EnvelopeID(fmt.Sprintf("held-input-%s-%d", a.ID, draft.Since.UnixNano()))
	published, err := run.noticeHitcher(a, id, fmt.Sprintf("%s, which you hitched, has held unsubmitted input in its composer since %s with the same text across observations; messages to %s wait behind it. gang capture --composer %s shows it.", a.Name, draft.Since.UTC().Format(time.RFC3339), a.Name, a.Name))
	if err != nil {
		return err
	}
	if !published && a.HitchedBy != "" {
		p, err := run.team.Agent(a.HitchedBy)
		if err != nil {
			return err
		}
		published, err = envelopePublished(p, id)
		if err != nil {
			return err
		}
	}
	if published {
		draft.Noticed = true
		return l.SaveHeldInput(&draft)
	}
	return nil
}
