package main

import (
	"context"
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

const statusHookFreshness = 30 * time.Second

func statusHook(n hookNotice) store.StatusHook {
	return store.StatusHook{Kind: n.Kind, NativeEvent: n.NativeEvent, At: n.At, SessionID: n.SessionID, TurnID: n.TurnID, Transcript: n.Transcript, Failure: n.Failure}
}
func hookFromStatus(n store.StatusHook) hookNotice {
	return hookNotice{Kind: n.Kind, NativeEvent: n.NativeEvent, At: n.At, SessionID: n.SessionID, TurnID: n.TurnID, Transcript: n.Transcript, Failure: n.Failure}
}

// statusHookBatch is acknowledged only after the sweep handles native outcomes.
// A roster may display the same records but cannot consume recovery work.
type statusHookBatch struct {
	sequence uint64
	notices  []hookNotice
}

// Reconcile retained hooks even when their detached refresh could not run.
// Terminal boundaries remain pending until their recovery effects complete.
func (run *runtime) reconcileStatusHooks(l *store.LockedAgent, a *core.Agent, c harness.Collar, notice hookNotice) (statusHookBatch, error) {
	batch := statusHookBatch{sequence: a.Native.HookSequence}
	records, err := l.Paths.ReadStatusHooks()
	if err != nil {
		return batch, err
	}
	if len(records) == 0 {
		batch.notices = []hookNotice{notice}
		return batch, run.reconcileNativeBoundary(l, a, c, notice)
	}
	if err := run.reconcileNativeBoundary(l, a, c, hookNotice{}); err != nil {
		return batch, err
	}
	directSeen := notice.Kind == ""
	for _, n := range records {
		if n.Kind == notice.Kind && n.At.Equal(notice.At) && n.TurnID == notice.TurnID {
			directSeen = true
		}
		if n.Sequence <= a.Native.HookSequence {
			continue
		}
		batch.sequence = n.Sequence
		// The submission witness has established the current native session.
		if n.SessionID != "" && a.Native.SessionID != "" && n.SessionID != a.Native.SessionID {
			if err := run.record(*a, core.Event{Type: "observation", Reason: "status hook ignored: session differs from current submission witness"}); err != nil {
				return batch, err
			}
			continue
		}
		nnotice := hookFromStatus(n)
		if err := run.reconcileNativeBoundary(l, a, c, nnotice); err != nil {
			return batch, err
		}
		if a.Status == core.Failed {
			return batch, nil
		}
		batch.notices = append(batch.notices, nnotice)
		if n.Kind == "compaction-finished" {
			if err := run.acceptContextReadings(a, c, []core.Reading{{Kind: "compaction-finished", Source: "native-hook", At: &n.At}}); err != nil {
				return batch, err
			}
		}
		current := n.TurnID == "" || a.Native.TurnID == "" || n.TurnID == a.Native.TurnID
		if !current {
			current, _ = harness.TurnRanAfter(c.Primitives.TurnBoundary, a.Native.Transcript, a.Native.TurnID, n.TurnID)
		}
		if current && !n.At.Before(a.Native.HookAt) {
			a.Native.HookAt, a.Native.HookKind = n.At, n.Kind
		}
		if err := l.Save(*a); err != nil {
			return batch, err
		}
	}
	// A detached native boundary may arrive after its display record was
	// acknowledged. Its outcome handlers are idempotent and still see it.
	if !directSeen || len(batch.notices) == 0 {
		batch.notices = append(batch.notices, notice)
	}
	return batch, nil
}

func (run *runtime) observeStatusOutcomes(a core.Agent, c harness.Collar, batch statusHookBatch, blocked bool) error {
	for _, notice := range batch.notices {
		if err := run.observeSnoozeTurn(a, c.Primitives.TurnBoundary, notice, blocked); err != nil {
			return err
		}
		if err := run.observeAutoCap(a, notice); err != nil {
			return err
		}
	}
	return nil
}
func acknowledgeStatusHooks(l *store.LockedAgent, a *core.Agent, batch statusHookBatch) error {
	if batch.sequence <= a.Native.HookSequence {
		return nil
	}
	a.Native.HookSequence = batch.sequence
	if err := l.Save(*a); err != nil {
		return err
	}
	return l.Paths.PruneStatusHooks(batch.sequence)
}

// Each screen retained here fills a gap in hooks or validates a pending
// operation. A fresh status reading alone never authorizes native input.
func (run *runtime) statusCaptureGap(a core.Agent, c harness.Collar) string {
	switch {
	case a.Status != core.Active || a.Registration.Held:
		return "startup readiness and native exit validation"
	case a.Input != nil || a.InputOutage != nil:
		return "pending input receipt or composer recovery validation"
	case !a.InterruptDeadline.IsZero():
		return "interrupt completion has no native hook"
	case a.Compaction != nil && a.Compaction.Status != "completed" && a.Compaction.Status != "failed":
		return "compaction refusal and continuation validation"
	case a.Capacity.Fingerprint != "":
		return "capacity retry screen validation"
	case c.Primitives.Telemetry == nil:
		return "collar telemetry requires the screen"
	case a.Native.HookAt.IsZero():
		return "hook evidence absent; inspect native prompt, exit, capacity and activity"
	case run.cmd.now().Before(a.Native.HookAt):
		return "hook timestamp is in the future"
	case !run.cmd.now().Before(a.Native.HookAt.Add(statusHookFreshness)) && !run.cmd.now().Before(a.StatusProbeAt.Add(statusHookFreshness)):
		return "hook evidence expired; inspect missed boundary, permission dismissal, native prompt, exit, capacity and wedge"
	}
	return ""
}

func (run *runtime) observeHookStatus(l *store.LockedAgent, a *core.Agent) error {
	// A stale hook must not overwrite the later fallback's observed state.
	if !a.Native.HookAt.After(a.StatusProbeAt) {
		return nil
	}
	activity, evidence := core.Busy, "native hook: "+a.Native.HookKind
	switch a.Native.HookKind {
	case "turn-finished", "compaction-finished":
		activity = core.Idle
		if a.Native.SubmittedAt.After(a.Native.FinishedAt) {
			activity = core.Busy
		}
	case "permission-requested":
		activity, evidence = core.Blocked, "native permission request awaits an answer"
	case "compaction-started":
		activity = core.Compacting
	}
	if a.Native.TurnFailure != "" {
		activity, evidence = core.Unknown, "native turn failed: "+a.Native.TurnFailure
	}
	if a.Activity == activity && a.Evidence == evidence {
		return nil
	}
	basis := activityBasis(*a, "unread", "native-hook")
	return run.apply(l, a, core.Event{Type: "activity_observed", Activity: activity, Reason: evidence, Basis: &basis})
}

func (run *runtime) tickHookStatus(l *store.LockedAgent, a *core.Agent, c harness.Collar, notice hookNotice, batch statusHookBatch) error {
	if err := run.refreshNative(l, a, c); err != nil {
		return err
	}
	if err := run.acceptContextReadings(a, c, notice.Readings); err != nil {
		return err
	}
	if err := run.observeContextBands(l, a, c, substrate.Screen{}); err != nil {
		return err
	}
	if err := run.observeUsageBands(*a, c); err != nil {
		return err
	}
	if err := run.observeHookStatus(l, a); err != nil {
		return err
	}
	if err := run.observeStatusOutcomes(*a, c, batch, false); err != nil {
		return err
	}
	if err := run.notifyHeldInput(*a); err != nil {
		return err
	}
	if err := acknowledgeStatusHooks(l, a, batch); err != nil {
		return err
	}
	if err := run.mark(*a); err != nil {
		return err
	}
	// Delivery performs its own composer, foreground and receipt validation.
	_, err := run.drainFrom(l, *a, "")
	return err
}

// A success hook does not report terminal-only provider capacity failures.
// Validate those before settling a wake that depends on this turn's success.
func (run *runtime) wakeNeedsScreen(a core.Agent, batch statusHookBatch) (bool, error) {
	finished := false
	for _, notice := range batch.notices {
		finished = finished || notice.Kind == "turn-finished"
	}
	if !finished {
		return false, nil
	}
	needed := false
	err := run.withUsageState(func(state *usageState) error {
		for _, wake := range state.Recent {
			if wake.RecipientID == a.ID && wake.TurnID != "" {
				needed = true
				break
			}
		}
		return nil
	})
	return needed, err
}

func (run *runtime) observeRosterStatus(l *store.LockedAgent, a *core.Agent, c harness.Collar) error {
	if _, err := run.reconcileStatusHooks(l, a, c, hookNotice{}); err != nil {
		return err
	}
	if a.Status == core.Failed {
		return nil
	}
	gap := run.statusCaptureGap(*a, c)
	if gap == "" {
		return run.observeHookStatus(l, a)
	}
	if err := run.record(*a, core.Event{Type: "observation", Reason: "status pane capture: " + gap}); err != nil {
		return err
	}
	input, err := run.input()
	if err != nil {
		return err
	}
	// The roster already matched the pane's registration in its team listing.
	screen, err := input.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		if err := run.observeProbeFailure(l, a, err); err != nil {
			return err
		}
		if run.cmd.stderr != nil {
			_, err := fmt.Fprintf(run.cmd.stderr, "%s: %s\n", a.Name, a.Evidence)
			return err
		}
		return nil
	}
	// A roster reads display state only. It must not postpone the sweep's
	// capacity and recovery checks by advancing their probe deadline.
	if err := run.observeCompaction(l, a, c, screen); err != nil {
		return err
	}
	return run.observeActivity(l, a, c, screen)
}
