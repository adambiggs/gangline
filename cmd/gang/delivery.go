package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/adambiggs/gangline/store"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	blocked, found, err := harness.DetectBlocked(collar.Primitives.Blocked, screen)
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
	if err := backend.SendKeys(context.Background(), substrate.PaneID(effect.Pane), input); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "text input returned an error: " + err.Error()}, nil
	}
	settle, err := harness.SubmitSettle(collar.Primitives.Submit)
	if err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: err.Error()}, nil
	}
	settleCtx, stopSettling := context.WithTimeout(context.Background(), deliveryTimeout)
	defer stopSettling()
	var settleErr error
	if midTurn {
		settleErr = harness.AwaitComposerSettle(settleCtx, backend.Capture, substrate.PaneID(effect.Pane), collar, settle)
	} else {
		settleErr = harness.AwaitScreenSettle(settleCtx, backend.Capture, substrate.PaneID(effect.Pane), screen, settle)
	}
	if err := settleErr; err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: err.Error()}, nil
	}
	// Recapture supplies diagnostic evidence when the native renderer is fast
	// enough. The exact UserPromptSubmit payload below is authoritative: long
	// composers can be clipped and a TUI may still be painting immediately
	// after tmux accepted the literal bytes.
	if readback, captureErr := backend.Capture(context.Background(), substrate.PaneID(effect.Pane)); captureErr == nil {
		_, _ = harness.ReadComposer(collar.Primitives.Composer, readback)
	}
	witness := run.deliveryWitnessPath(hitch.ID)
	_ = os.Remove(witness)
	if err := syscall.Mkfifo(witness, 0o600); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "prepare native submit witness: " + err.Error()}, nil
	}
	defer os.Remove(witness)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := exec.CommandContext(ctx, "cat", witness)
	witnessed := make(chan nativeWitnessResult, 1)
	go func() {
		data, readErr := reader.Output()
		witnessed <- nativeWitnessResult{data: data, err: readErr}
	}()
	if err := sendHarnessKeys(context.Background(), backend, substrate.PaneID(effect.Pane), collar, substrate.Keys{Submit: true}); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "submit returned an error after verified text input: " + err.Error()}, nil
	}
	checkpoints := time.NewTicker(deliveryTimeout)
	defer checkpoints.Stop()
	result, waitErr := awaitNativeWitness(witnessed, checkpoints.C, func() error {
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
		if delivery.Status == core.DeliveryFailed || delivery.Status == core.DeliveryCancelled {
			return refuseError("delivery failed: %s", delivery.Reason)
		}
		if current.Hitches[hitch.ID].Status == core.HitchDropping {
			return nil
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
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: waitErr.Error()}, nil
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
