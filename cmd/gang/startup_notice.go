package main

import (
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// A blocked startup is polled every few seconds. Remind its hitcher at most
// once per fifteen minutes while the recognized prompt remains unresolved.
const startupPromptReminder = 15 * time.Minute

func (run *runtime) notifyStartupPrompt(l *store.LockedAgent, a *core.Agent) error {
	if a.Status != core.Booting || a.Activity != core.Blocked {
		return nil
	}
	now := run.cmd.now()
	if !a.StartupPromptNoticedAt.IsZero() && now.Before(a.StartupPromptNoticedAt.Add(startupPromptReminder)) {
		return nil
	}
	// The interval identifies an interrupted publication; the saved time also
	// prevents replay after another delivery retires the hitcher's receipt.
	interval := now.Sub(a.ChangedAt) / startupPromptReminder
	id := core.EnvelopeID(fmt.Sprintf("startup-blocked-%s-%d-%d", a.ID, a.ChangedAt.UnixNano(), interval))
	published, err := run.noticeHitcher(*a, id, fmt.Sprintf("%s, which you hitched, is blocked at a native startup prompt: %s. Resolve the prompt, then run gang tick; gang status %s --why shows its record.", a.Name, a.Evidence, a.Name))
	if err != nil || !published {
		return err
	}
	a.StartupPromptNoticedAt = now
	if err := l.Save(*a); err != nil {
		return err
	}
	return run.record(*a, core.Event{Type: "startup_prompt_notice", ID: string(id), Reason: a.Evidence})
}
