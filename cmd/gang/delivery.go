package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) deliver(state core.State, backend interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
	ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
	SendKeys(context.Context, substrate.PaneID, substrate.Keys) error
}, effect core.DeliverEnvelope) (core.Event, error) {
	now := time.Now()
	if !now.Before(effect.Deadline) {
		return core.OperationTimedOut{At: now, Operation: core.TimeoutDelivery, ID: string(effect.Envelope.ID), Deadline: effect.Deadline, Evidence: "delivery deadline elapsed before verified submit"}, nil
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
	blocked, found, err := harness.DetectBlocked(collar.Primitives.Blocked, screen)
	if err != nil {
		return core.DeliveryFailedEvent{At: now, EnvelopeID: effect.Envelope.ID, Reason: err.Error()}, nil
	}
	if found {
		return core.BlockedDetected{At: now, HitchID: hitch.ID, Evidence: blocked.Evidence}, nil
	}
	composer, err := harness.ReadComposer(collar.Primitives.Composer, screen)
	if err != nil || composer.Text != "" {
		reason := "composer is not empty"
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
	ctx, cancel := context.WithDeadline(context.Background(), effect.Deadline)
	defer cancel()
	if err := awaitScreenSettle(ctx, backend, substrate.PaneID(effect.Pane), screen, settle); err != nil {
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
	reader := exec.CommandContext(ctx, "cat", witness)
	type witnessResult struct {
		data []byte
		err  error
	}
	witnessed := make(chan witnessResult, 1)
	go func() {
		data, readErr := reader.Output()
		witnessed <- witnessResult{data: data, err: readErr}
	}()
	if err := sendHarnessKeys(context.Background(), backend, substrate.PaneID(effect.Pane), collar, substrate.Keys{Submit: true}); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "submit returned an error after verified text input: " + err.Error()}, nil
	}
	select {
	case result := <-witnessed:
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
	case <-ctx.Done():
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "native submit witness timed out after input was sent"}, nil
	}
}

func awaitScreenSettle(ctx context.Context, backend interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
}, pane substrate.PaneID, before substrate.Screen, settle time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last substrate.Screen
	var stableSince time.Time
	for {
		screen, err := backend.Capture(ctx, pane)
		if err == nil && !reflect.DeepEqual(screen, before) {
			if stableSince.IsZero() || !reflect.DeepEqual(screen, last) {
				stableSince = time.Now()
				last = screen
			}
			if time.Since(stableSince) >= settle {
				return nil
			}
		} else {
			stableSince = time.Time{}
			last = substrate.Screen{}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not settle before submission: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
