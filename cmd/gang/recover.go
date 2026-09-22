package main

import (
	"context"
	"errors"
	"github.com/adambiggs/gangline/store"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) recover() (core.State, error) {
	state, err := run.load()
	if err != nil {
		return core.State{}, err
	}
	state, err = run.reconcilePanes(state, time.Now())
	if err != nil {
		return core.State{}, err
	}
	if err := run.reconcileWindowMarks(state); err != nil {
		return core.State{}, err
	}
	state, err = run.refreshBlocked(state)
	if err != nil {
		return core.State{}, err
	}
	state, err = run.refreshCapacity(state, time.Now())
	if err != nil {
		return core.State{}, err
	}
	for _, id := range core.DueTimedDeliveries(state, time.Now()) {
		state, err = run.drive(core.TimedDeliveryReleased{At: time.Now(), EnvelopeID: id})
		if err != nil {
			return core.State{}, err
		}
	}
	state, err = run.retryQueuedDeliveries(state)
	if err != nil {
		return core.State{}, err
	}
	effects := core.PendingEffects(state)
	for _, effect := range effects {
		if compact, ok := effect.(core.CompactHitch); ok {
			state, err = run.recoverCompaction(compact)
			if err != nil {
				return core.State{}, err
			}
			continue
		}
		if delivery, ok := effect.(core.DeliverEnvelope); ok {
			state, err = run.recoverDelivery(delivery)
			if err != nil {
				return core.State{}, err
			}
			continue
		}
		outcome, err := run.executeEffect(state, effect)
		if err != nil {
			return core.State{}, err
		}
		if outcome != nil {
			state, err = run.drive(outcome)
			if err != nil {
				return core.State{}, err
			}
		}
	}
	return state, nil
}

// A recorded compact intent never authorizes recovery to type the command
// again. A live owner finishes input; submitted commands await PostCompact.
func (run *runtime) recoverCompaction(effect core.CompactHitch) (core.State, error) {
	state, err := run.load()
	if err != nil {
		return core.State{}, err
	}
	owner, err := run.paths().LockInput(run.settings.Session, string(effect.Compaction.HitchID))
	if errors.Is(err, store.ErrLocked) {
		return state, nil
	}
	if err != nil {
		return core.State{}, err
	}
	defer owner.Close()
	state, err = run.load()
	if err != nil {
		return core.State{}, err
	}
	compact := state.Compactions[effect.Compaction.ID]
	if compact.Status != core.CompactionRunning {
		return state, nil
	}
	if !compact.Submitted {
		return run.drive(core.CompactionUnverifiedEvent{At: time.Now(), CompactionID: compact.ID, Evidence: "compaction owner exited before recording its native input outcome"})
	}
	if !time.Now().Before(compact.Deadline) {
		return run.drive(core.OperationTimedOut{At: time.Now(), Operation: core.TimeoutCompaction, ID: string(compact.ID), Deadline: compact.Deadline, Evidence: "compaction deadline elapsed without a native completion witness"})
	}
	return state, nil
}

func (run *runtime) refreshBlocked(state core.State) (core.State, error) {
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return core.State{}, err
	}
	for id, hitch := range state.Hitches {
		if hitch.Status != core.HitchActive {
			continue
		}
		collar, loadErr := loadCollar(hitch.Collar, run.settings)
		if loadErr != nil {
			return core.State{}, loadErr
		}
		screen, captureErr := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
		if captureErr != nil {
			continue
		}
		blocked, found, detectErr := harness.InputBlocked(collar, screen)
		if detectErr != nil {
			return core.State{}, detectErr
		}
		switch {
		case found && hitch.Activity != core.ActivityBlocked:
			state, err = run.drive(core.BlockedDetected{At: time.Now(), HitchID: id, Evidence: blocked.Evidence})
		case !found && hitch.Activity == core.ActivityBlocked:
			if _, composerErr := harness.ReadComposer(collar.Primitives.Composer, screen); composerErr == nil {
				state, err = run.drive(core.BlockedCleared{At: time.Now(), HitchID: id})
			}
		}
		if err != nil {
			return core.State{}, err
		}
	}
	return state, nil
}

// retryQueuedDeliveries uses direct composer evidence to retry known pre-input
// refusals. Capable collars can submit while busy without inventing a boundary;
// other collars require a directly observed native idle composer.
func (run *runtime) retryQueuedDeliveries(state core.State) (core.State, error) {
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return core.State{}, err
	}
	seen := make(map[core.HitchID]bool)
	for _, envelopeID := range state.DeliveryOrder {
		delivery := state.Deliveries[envelopeID]
		if delivery.Status != core.DeliveryQueued || !delivery.NotBefore.IsZero() || delivery.CapacityRecovery {
			continue
		}
		hitch, ok := activeByName(state, string(delivery.Envelope.To))
		if !ok || seen[hitch.ID] {
			continue
		}
		seen[hitch.ID] = true
		inFlight := false
		for _, other := range state.Deliveries {
			if other.Envelope.To == hitch.Name && other.Status == core.DeliveryDelivering {
				inFlight = true
				break
			}
		}
		if inFlight {
			continue
		}
		if (hitch.Activity != core.ActivityBusy && hitch.Activity != core.ActivityIdle) || hitch.PendingCompactID != "" {
			continue
		}
		collar, loadErr := loadCollar(hitch.Collar, run.settings)
		if loadErr != nil {
			return core.State{}, loadErr
		}
		screen, captureErr := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
		if captureErr != nil {
			continue
		}
		idle, readErr := harness.Idle(collar, screen)
		if delivery.MidTurn && collar.Primitives.MidTurn {
			composer, composerErr := harness.ReadComposer(collar.Primitives.Composer, screen)
			idle, readErr = composer.Text == "", composerErr
		}
		if readErr != nil || !idle {
			continue
		}
		now := time.Now()
		if hitch.Activity == core.ActivityIdle || delivery.MidTurn {
			state, err = run.drive(core.DeliveryRetryRequested{At: now, EnvelopeID: envelopeID})
		} else {
			state, err = run.drive(core.TurnBoundaryReached{At: now, HitchID: hitch.ID})
		}
		if err != nil {
			return core.State{}, err
		}
	}
	return state, nil
}

func (run *runtime) recoverDelivery(effect core.DeliverEnvelope) (core.State, error) {
	state, err := run.load()
	if err != nil {
		return core.State{}, err
	}
	hitch, ok := activeByName(state, string(effect.Envelope.To))
	if !ok {
		return state, nil
	}
	owner, err := run.paths().LockInput(run.settings.Session, string(hitch.ID))
	if errors.Is(err, store.ErrLocked) {
		return state, nil
	}
	if err != nil {
		return core.State{}, err
	}
	defer owner.Close()
	state, err = run.load()
	if err != nil {
		return core.State{}, err
	}
	if state.Deliveries[effect.Envelope.ID].Status != core.DeliveryDelivering {
		return state, nil
	}
	return run.drive(core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "delivery owner exited before recording its native input outcome"})
}

// A deferred delivery's owner retries known pre-input refusals only. It does
// not replay other commands' effects or recover unrelated hitches.
func (run *runtime) refreshDeliveries() (core.State, error) {
	state, err := run.load()
	if err != nil {
		return core.State{}, err
	}
	state, err = run.reconcilePanes(state, time.Now())
	if err != nil {
		return core.State{}, err
	}
	state, err = run.refreshBlocked(state)
	if err != nil {
		return core.State{}, err
	}
	return run.retryQueuedDeliveries(state)
}
