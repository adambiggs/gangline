package main

import (
	"context"
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
	for _, id := range core.DueTimedDeliveries(state, time.Now()) {
		state, err = run.drive(core.TimedDeliveryReleased{At: time.Now(), EnvelopeID: id})
		if err != nil {
			return core.State{}, err
		}
	}
	state, err = run.expireQueuedDeliveries(state, time.Now())
	if err != nil {
		return core.State{}, err
	}
	state, err = run.retryQueuedDeliveries(state)
	if err != nil {
		return core.State{}, err
	}
	effects := core.PendingEffects(state)
	for _, effect := range effects {
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

func (run *runtime) expireQueuedDeliveries(state core.State, now time.Time) (core.State, error) {
	var err error
	for _, id := range state.DeliveryOrder {
		delivery := state.Deliveries[id]
		if delivery.Status != core.DeliveryQueued || !delivery.NotBefore.IsZero() || now.Before(delivery.Deadline) {
			continue
		}
		state, err = run.drive(core.OperationTimedOut{
			At: now, Operation: core.TimeoutDelivery, ID: string(id), Deadline: delivery.Deadline,
			Evidence: "delivery deadline elapsed before input",
		})
		if err != nil {
			return core.State{}, err
		}
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
		blocked, found, detectErr := harness.DetectBlocked(collar.Primitives.Blocked, screen)
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

// retryQueuedDeliveries turns a directly observed empty composer into the
// boundary fact that releases parked delivery. A hook normally records that
// boundary, but a refused delivery or compaction can leave no later hook for a
// durable queue to ride. One delivery per recipient keeps the pass bounded and
// preserves the rule that a submitted turn must finish before the next send.
func (run *runtime) retryQueuedDeliveries(state core.State) (core.State, error) {
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return core.State{}, err
	}
	seen := make(map[core.HitchID]bool)
	for _, envelopeID := range state.DeliveryOrder {
		delivery := state.Deliveries[envelopeID]
		if delivery.Status != core.DeliveryQueued || !delivery.NotBefore.IsZero() {
			continue
		}
		hitch, ok := activeByName(state, string(delivery.Envelope.To))
		if !ok || seen[hitch.ID] {
			continue
		}
		seen[hitch.ID] = true
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
		if readErr != nil || !idle {
			continue
		}
		now := time.Now()
		if hitch.Activity == core.ActivityIdle {
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
