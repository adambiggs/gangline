package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/adambiggs/gangline/store"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) deliveryWitnessPath(id core.HitchID) string {
	paths, _ := run.paths().Team(run.settings.Session)
	return filepath.Join(paths.Directory, "delivery-"+string(id)+".fifo")
}

func (run *runtime) deliver(state core.State, backend interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
	ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
	SendKeys(context.Context, substrate.PaneID, substrate.Keys) error
}, effect core.DeliverEnvelope) (core.Event, error) {
	now := time.Now()
	budget, err := capacitySubmissionBudget(state, effect.Envelope.ID, now)
	if err != nil {
		return core.DeliveryFailedEvent{At: now, EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}

	var hitch core.Hitch
	for _, candidate := range state.Hitches {
		if candidate.Name == effect.Envelope.To && candidate.Status == core.HitchActive {
			hitch = candidate
			break
		}
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return core.DeliveryFailedEvent{At: now, EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(effect.Pane))
	if err != nil {
		return core.DeliveryDeferred{At: now, EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	blocked, found, err := harness.InputBlocked(collar, screen)
	if err != nil {
		return core.DeliveryFailedEvent{At: now, EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	if found {
		if state.Deliveries[effect.Envelope.ID].DuringTurn {
			return core.DeliveryDeferred{At: now, EnvelopeID: effect.Envelope.ID, Reason: blocked.Evidence}, nil
		}
		return core.BlockedDetected{At: now, HitchID: hitch.ID, Evidence: blocked.Evidence}, nil
	}
	idle, err := harness.Idle(collar, screen)
	midTurn := collar.Primitives.MidTurn && state.Deliveries[effect.Envelope.ID].MidTurn
	if midTurn {
		composer, readErr := harness.ReadComposer(collar.Primitives.Composer, screen)
		idle, err = composer.Text == "", readErr
	}
	if err != nil || !idle {
		reason := "native turn is not idle with an empty composer"
		if err != nil {
			reason = err.Error()
		}
		return core.DeliveryDeferred{At: now, EnvelopeID: effect.Envelope.ID, Reason: reason}, nil
	}
	sender := string(effect.Envelope.From.Name)
	if effect.Envelope.From.Kind == core.SenderSelfDeclared {
		sender = "self-declared:" + sender
	}
	marker := ""
	if effect.Envelope.From.Kind == core.SenderSelfDeclared && effect.Envelope.From.Name == "hitch" {
		marker = "assignment"
	}
	wire, err := renderEnvelope(sender, string(effect.Envelope.ID), marker, effect.Envelope.Message.Text)
	if err != nil {
		return core.DeliveryFailedEvent{At: now, EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	input, err := harness.SubmitInput(collar.Primitives.Submit, wire)
	if err != nil {
		return core.DeliveryFailedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	if err := requireHarnessForeground(context.Background(), backend, substrate.PaneID(effect.Pane), collar); err != nil {
		return core.DeliveryDeferred{At: time.Now(), EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	state, err = run.drive(core.DeliveryInputStarted{At: time.Now(), EnvelopeID: effect.Envelope.ID})
	if err != nil {
		return nil, err
	}
	if delivery := state.Deliveries[effect.Envelope.ID]; delivery.Status != core.DeliveryDelivering || !delivery.InputStarted {
		return core.DeliveryDeferred{At: time.Now(), EnvelopeID: effect.Envelope.ID, Reason: "native input ownership changed before paste"}, nil
	}
	if err := backend.SendKeys(context.Background(), substrate.PaneID(effect.Pane), input); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "text input returned an error: " + err.Error()}, nil
	}
	settle, err := harness.SubmitSettle(collar.Primitives.Submit)
	if err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: err.Error()}, nil
	}
	settleCtx, stopSettling := context.WithTimeout(context.Background(), budget)
	defer stopSettling()
	captureForInput := func(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
		current, err := backend.Capture(ctx, pane)
		if err != nil {
			return substrate.Screen{}, err
		}
		blocked, found, err := harness.InputBlocked(collar, current)
		if err != nil {
			return substrate.Screen{}, err
		}
		if found {
			return substrate.Screen{}, nativeInputBlocked{evidence: blocked.Evidence}
		}
		return current, nil
	}
	unverified := func(err error) core.Event {
		result := core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: err.Error()}
		var blocked nativeInputBlocked
		if errors.As(err, &blocked) {
			result.BlockedEvidence = blocked.evidence
		}
		return result
	}
	var settleErr error
	if midTurn {
		settleErr = harness.AwaitComposerSettle(settleCtx, captureForInput, substrate.PaneID(effect.Pane), collar, settle)
	} else {
		settleErr = harness.AwaitScreenSettle(settleCtx, captureForInput, substrate.PaneID(effect.Pane), screen, settle)
	}
	if settleErr != nil {
		return unverified(settleErr), nil
	}
	// Native trust can take input after the composer appeared. This observation
	// is a submission guard; a raced surface after it remains unknown.
	if _, err := captureForInput(context.Background(), substrate.PaneID(effect.Pane)); err != nil {
		return unverified(err), nil
	}

	witness := run.deliveryWitnessPath(hitch.ID)
	reader, witnessed, err := openNativeWitness(witness)
	if err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "prepare native submit witness: " + err.Error()}, nil
	}
	defer os.Remove(witness)
	defer reader.Close()
	if err := sendHarnessKeys(context.Background(), backend, substrate.PaneID(effect.Pane), collar, substrate.Keys{Submit: true}); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "submit returned an error after verified text input: " + err.Error()}, nil
	}
	checkpoints := time.NewTicker(budget)
	defer checkpoints.Stop()
	result, waitErr := awaitNativeWitness(witnessed, checkpoints.C, func() error {
		if _, err := capacitySubmissionBudget(state, effect.Envelope.ID, time.Now()); err != nil {
			return err
		}

		paths, pathErr := run.paths().Team(run.settings.Session)
		if pathErr != nil {
			return pathErr
		}
		current, _, complete, readErr := store.ObserveLog(paths.Events, run.initial())
		if readErr != nil {
			return readErr
		}
		if !complete {
			return nil
		}
		delivery := current.Deliveries[effect.Envelope.ID]
		if delivery.Status == core.DeliveryUnverified {
			return fmt.Errorf("native submission is unverified: %s", delivery.Reason)
		}
		if delivery.Status == core.DeliveryFailed || delivery.Status == core.DeliveryCancelled {
			return refuseError("delivery failed: %s", delivery.Reason)
		}
		if current.Hitches[hitch.ID].Status == core.HitchDropping {
			return nil
		}
		if _, err := captureForInput(context.Background(), substrate.PaneID(effect.Pane)); err != nil {
			return err
		}
		if err := requireHarnessForeground(context.Background(), backend, substrate.PaneID(effect.Pane), collar); err != nil {
			return fmt.Errorf("native process no longer answers for the pending submission: %w", err)
		}
		return nil
	})
	if waitErr != nil {
		var commandErr commandError
		if errors.As(waitErr, &commandErr) {
			return nil, waitErr
		}
		return unverified(waitErr), nil
	}
	if result.err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "native submit witness failed: " + result.err.Error()}, nil
	}
	matched, matchErr := harness.SubmittedPromptMatches(collar.Primitives.SubmitWitness, wire, string(result.data))
	if matchErr != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: matchErr.Error()}, nil
	}
	if !matched {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "native submit witness did not match the attributed envelope"}, nil
	}
	return core.DeliverySucceeded{At: time.Now(), EnvelopeID: effect.Envelope.ID}, nil

}

type harnessInput interface {
	ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
	SendKeys(context.Context, substrate.PaneID, substrate.Keys) error
}

func sendHarnessKeys(ctx context.Context, backend harnessInput, pane substrate.PaneID, collar harness.Collar, keys substrate.Keys) error {
	if err := requireHarnessForeground(ctx, backend, pane, collar); err != nil {
		return err
	}
	return backend.SendKeys(ctx, pane, keys)
}

func requireHarnessForeground(ctx context.Context, backend harnessInput, pane substrate.PaneID, collar harness.Collar) error {
	processes, err := backend.ForegroundProcesses(ctx, pane)
	if err != nil {
		return fmt.Errorf("refuse input without foreground-process evidence: %w", err)
	}
	want := filepath.Base(collar.Launch.Command)
	for _, process := range processes {
		if filepath.Base(process.Command) == want {
			return nil
		}
	}
	commands := make([]string, len(processes))
	for index, process := range processes {
		commands[index] = process.Command
	}
	return fmt.Errorf("refuse input: pane foreground is %q, want harness %q", strings.Join(commands, ","), want)
}

type nativeWitnessResult struct {
	data []byte
	err  error
}

// Checkpoints detect recipient loss; they never expire a live submission.
func awaitNativeWitness(witnessed <-chan nativeWitnessResult, checkpoints <-chan time.Time, checkRecipient func() error) (nativeWitnessResult, error) {
	for {
		select {
		case result := <-witnessed:
			return result, nil
		case <-checkpoints:
			if err := checkRecipient(); err != nil {
				return nativeWitnessResult{}, err
			}
		}
	}
}

type nativeInputBlocked struct{ evidence string }

func (e nativeInputBlocked) Error() string {
	return "native input needs attention after paste: " + e.evidence
}
