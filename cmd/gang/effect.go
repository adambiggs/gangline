package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) executeEffect(state core.State, effect core.Effect) (core.Event, error) {
	now := time.Now()
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return nil, err
	}
	switch effect := effect.(type) {
	case core.RecordEvent:
		return effect.Event, nil
	case core.SpawnHitch:
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		if existing, findErr := backend.PaneNamed(ctx, string(effect.Hitch.Name)); findErr == nil {
			return core.HitchSpawned{At: now, HitchID: effect.Hitch.ID, Pane: string(existing.ID)}, nil
		}
		collar, loadErr := loadCollar(effect.Hitch.Collar, run.settings)
		if loadErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: loadErr.Error()}, nil
		}
		executable, execErr := os.Executable()
		if execErr != nil {
			return nil, fmt.Errorf("locate gang executable: %w", execErr)
		}
		queued, _, queueErr := run.readStartup(effect.Hitch.ID)
		if queueErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: queueErr.Error()}, nil
		}
		rolePrompt := ""
		if collar.Options.RolePrompt != nil {
			rolePrompt = queued.RolePrompt
		}
		launch, launchErr := harness.RenderLaunch(collar, harness.LaunchOptions{
			ResumeSession: queued.Resume,
			HookCommand:   []string{executable, "hook"},
			Model:         queued.Model,
			Effort:        queued.Effort,
			RolePrompt:    rolePrompt,
		})
		if launchErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: launchErr.Error()}, nil
		}
		launch = applyLaunchPolicy(launch, effect.Hitch.Collar, run.settings)
		spec := launch.SpawnSpec(string(effect.Hitch.Name), effect.Hitch.Directory)
		spec.Env["GANG_SESSION"] = run.settings.Session
		spec.Env["GANG_STATE_ROOT"] = run.settings.StateRoot
		spec.Env["GANG_COLLAR"] = effect.Hitch.Collar
		spec.Env["GANG_CONFIG_DIR"] = run.settings.ConfigDir
		spec.Env["GANGLINE_HITCH_ID"] = string(effect.Hitch.ID)
		if run.settings.CollarDir != "" {
			spec.Env["GANG_COLLARS"] = run.settings.CollarDir
		}
		if run.settings.Socket != "" {
			spec.Env["GANG_TMUX_SOCKET"] = run.settings.Socket
		}
		exists, existsErr := backend.SessionExists(ctx)
		if existsErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: existsErr.Error()}, nil
		}
		var pane substrate.Pane
		if exists {
			pane, err = backend.Spawn(ctx, spec)
		} else {
			pane, err = backend.CreateSession(ctx, spec)
		}
		if err != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: err.Error()}, nil
		}
		return core.HitchSpawned{At: time.Now(), HitchID: effect.Hitch.ID, Pane: string(pane.ID)}, nil
	case core.AwaitBoot:
		if !now.Before(effect.Deadline) {
			return core.OperationTimedOut{At: now, Operation: core.TimeoutBoot, ID: string(effect.HitchID), Deadline: effect.Deadline, Evidence: "boot deadline elapsed without a readable composer"}, nil
		}
		hitch := state.Hitches[effect.HitchID]
		collar, loadErr := loadCollar(hitch.Collar, run.settings)
		if loadErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.HitchID, Reason: loadErr.Error()}, nil
		}
		screen, captureErr := backend.Capture(context.Background(), substrate.PaneID(effect.Pane))
		if captureErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.HitchID, Reason: captureErr.Error()}, nil
		}
		startup, inspectErr := harness.InspectStartup(collar, screen)
		if inspectErr != nil {
			return core.HitchLaunchFailed{At: now, HitchID: effect.HitchID, Reason: inspectErr.Error()}, nil
		}
		if startup.State == harness.StartupReady {
			return core.HitchReady{At: time.Now(), HitchID: effect.HitchID}, nil
		}
		return nil, nil
	case core.DeliverEnvelope:
		return run.deliver(state, backend, effect)
	case core.CompactHitch:
		if !now.Before(effect.Compaction.Deadline) {
			return core.OperationTimedOut{At: now, Operation: core.TimeoutCompaction, ID: string(effect.Compaction.ID), Deadline: effect.Compaction.Deadline, Evidence: "compaction deadline elapsed"}, nil
		}
		hitch := state.Hitches[effect.Compaction.HitchID]
		collar, loadErr := loadCollar(hitch.Collar, run.settings)
		if loadErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: loadErr.Error()}, nil
		}
		action, renderErr := harness.RenderAction(collar.Actions.Compact, map[string]string{"instructions": effect.Compaction.Resume.Text})
		if renderErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: renderErr.Error()}, nil
		}
		screen, captureErr := backend.Capture(context.Background(), substrate.PaneID(effect.Pane))
		if captureErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: captureErr.Error()}, nil
		}
		composer, composerErr := harness.ReadComposer(collar.Primitives.Composer, screen)
		if composerErr != nil || composer.Text != "" {
			return nil, nil
		}
		settle, settleErr := harness.SubmitSettle(collar.Primitives.Submit)
		if settleErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: settleErr.Error()}, nil
		}
		pane := substrate.PaneID(effect.Pane)
		input, inputErr := harness.SubmitInput(collar.Primitives.Submit, action.Text)
		if inputErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: inputErr.Error()}, nil
		}
		if sendErr := sendHarnessKeys(context.Background(), backend, pane, collar, input); sendErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: sendErr.Error()}, nil
		}
		ctx, cancel := context.WithDeadline(context.Background(), effect.Compaction.Deadline)
		defer cancel()
		if waitErr := awaitComposerText(ctx, backend, pane, collar, action.Text, settle); waitErr != nil {
			return core.CompactionFailedEvent{At: time.Now(), CompactionID: effect.Compaction.ID, Reason: waitErr.Error()}, nil
		}
		if sendErr := sendHarnessKeys(context.Background(), backend, pane, collar, substrate.Keys{Names: action.Keys, Submit: action.Submit}); sendErr != nil {
			return core.CompactionFailedEvent{At: time.Now(), CompactionID: effect.Compaction.ID, Reason: sendErr.Error()}, nil
		}
		return nil, nil
	case core.InterruptHitch:
		hitch := state.Hitches[effect.HitchID]
		collar, loadErr := loadCollar(hitch.Collar, run.settings)
		if loadErr != nil {
			return core.InterruptFailed{At: now, HitchID: effect.HitchID, Reason: loadErr.Error()}, nil
		}
		if sendErr := sendHarnessKeys(context.Background(), backend, substrate.PaneID(effect.Pane), collar, collar.Actions.Interrupt.Input()); sendErr != nil {
			return core.InterruptFailed{At: now, HitchID: effect.HitchID, Reason: sendErr.Error()}, nil
		}
		return core.InterruptSucceeded{At: time.Now(), HitchID: effect.HitchID}, nil
	case core.KillHitch:
		if !now.Before(effect.Deadline) {
			return core.OperationTimedOut{At: now, Operation: core.TimeoutDrop, ID: string(effect.HitchID), Deadline: effect.Deadline, Evidence: "drop deadline elapsed before pane termination"}, nil
		}
		windows, listErr := backend.Windows(context.Background())
		if listErr == nil {
			found := false
			for _, window := range windows {
				found = found || string(window.Pane.ID) == effect.Pane
			}
			if !found {
				return core.DropSucceeded{At: now, HitchID: effect.HitchID}, nil
			}
		}
		ctx, cancel := context.WithDeadline(context.Background(), effect.Deadline)
		defer cancel()
		if killErr := backend.Kill(ctx, substrate.PaneID(effect.Pane)); killErr != nil {
			return core.DropFailed{At: now, HitchID: effect.HitchID, Reason: killErr.Error()}, nil
		}
		return core.DropSucceeded{At: time.Now(), HitchID: effect.HitchID}, nil
	default:
		return nil, fmt.Errorf("unsupported effect %T", effect)
	}
}
