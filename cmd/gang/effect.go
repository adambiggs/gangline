package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

func (run *runtime) executeEffect(state core.State, effect core.Effect) (core.Event, error) {
	if record, ok := effect.(core.RecordEvent); ok {
		return record.Event, nil
	}
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return nil, err
	}
	switch effect := effect.(type) {
	case core.RecordEvent:
		return effect.Event, nil
	case core.SpawnHitch:
		return run.spawn(backend, effect)
	case core.AwaitBoot:
		return run.awaitBoot(state, backend, effect)
	case core.DeliverEnvelope:
		return run.deliver(state, backend, effect)
	case core.CompactHitch:
		return run.compactHitch(state, backend, effect)
	case core.InterruptHitch:
		return run.interruptHitch(state, backend, effect)
	case core.KillHitch:
		return run.killHitch(backend, effect)
	default:
		return nil, fmt.Errorf("unsupported effect %T", effect)
	}
}

func (run *runtime) spawn(backend *tmux.Backend, effect core.SpawnHitch) (core.Event, error) {
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	if existing, err := backend.PaneNamed(ctx, string(effect.Hitch.Name)); err == nil {
		return core.HitchSpawned{At: now, HitchID: effect.Hitch.ID, Pane: string(existing.ID)}, nil
	}
	collar, err := loadCollar(effect.Hitch.Collar, run.settings)
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: err.Error()}, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate gang executable: %w", err)
	}
	queued, _, err := run.readStartup(effect.Hitch.ID)
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: err.Error()}, nil
	}
	rolePrompt := ""
	if collar.Options.RolePrompt != nil {
		rolePrompt = queued.RolePrompt
	}
	launch, err := harness.RenderLaunch(collar, harness.LaunchOptions{
		ResumeSession: queued.Resume,
		HookCommand:   []string{executable, "hook"},
		Model:         queued.Model,
		Effort:        queued.Effort,
		RolePrompt:    rolePrompt,
	})
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: err.Error()}, nil
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
	exists, err := backend.SessionExists(ctx)
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.Hitch.ID, Reason: err.Error()}, nil
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
}

func (run *runtime) awaitBoot(state core.State, backend *tmux.Backend, effect core.AwaitBoot) (core.Event, error) {
	now := time.Now()
	if !now.Before(effect.Deadline) {
		return core.OperationTimedOut{At: now, Operation: core.TimeoutBoot, ID: string(effect.HitchID), Deadline: effect.Deadline, Evidence: "boot deadline elapsed without a readable composer"}, nil
	}
	hitch := state.Hitches[effect.HitchID]
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.HitchID, Reason: err.Error()}, nil
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(effect.Pane))
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.HitchID, Reason: err.Error()}, nil
	}
	startup, err := harness.InspectStartup(collar, screen)
	if err != nil {
		return core.HitchLaunchFailed{At: now, HitchID: effect.HitchID, Reason: err.Error()}, nil
	}
	if startup.State == harness.StartupReady {
		return core.HitchReady{At: time.Now(), HitchID: effect.HitchID}, nil
	}
	return nil, nil
}

func (run *runtime) interruptHitch(state core.State, backend *tmux.Backend, effect core.InterruptHitch) (core.Event, error) {
	now := time.Now()
	hitch := state.Hitches[effect.HitchID]
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return core.InterruptFailed{At: now, HitchID: effect.HitchID, Reason: err.Error()}, nil
	}
	if err := sendHarnessKeys(context.Background(), backend, substrate.PaneID(effect.Pane), collar, collar.Actions.Interrupt.Input()); err != nil {
		return core.InterruptFailed{At: now, HitchID: effect.HitchID, Reason: err.Error()}, nil
	}
	return core.InterruptSucceeded{At: time.Now(), HitchID: effect.HitchID}, nil
}

func (run *runtime) killHitch(backend *tmux.Backend, effect core.KillHitch) (core.Event, error) {
	now := time.Now()
	if !now.Before(effect.Deadline) {
		return core.OperationTimedOut{At: now, Operation: core.TimeoutDrop, ID: string(effect.HitchID), Deadline: effect.Deadline, Evidence: "drop deadline elapsed before pane termination"}, nil
	}
	windows, err := backend.Windows(context.Background())
	if err == nil {
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
	if err := backend.Kill(ctx, substrate.PaneID(effect.Pane)); err != nil {
		return core.DropFailed{At: now, HitchID: effect.HitchID, Reason: err.Error()}, nil
	}
	return core.DropSucceeded{At: time.Now(), HitchID: effect.HitchID}, nil
}

func (run *runtime) compactHitch(state core.State, backend *tmux.Backend, effect core.CompactHitch) (core.Event, error) {
	now := time.Now()
	fail := func(at time.Time, err error) core.Event {
		return core.CompactionFailedEvent{At: at, CompactionID: effect.Compaction.ID, Reason: err.Error()}
	}
	if !now.Before(effect.Compaction.Deadline) {
		return core.OperationTimedOut{At: now, Operation: core.TimeoutCompaction, ID: string(effect.Compaction.ID), Deadline: effect.Compaction.Deadline, Evidence: "compaction deadline elapsed"}, nil
	}
	hitch := state.Hitches[effect.Compaction.HitchID]
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return fail(now, err), nil
	}
	action, err := harness.RenderAction(collar.Actions.Compact, map[string]string{"instructions": effect.Compaction.Resume.Text})
	if err != nil {
		return fail(now, err), nil
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(effect.Pane))
	if err != nil {
		return fail(now, err), nil
	}
	composer, err := harness.ReadComposer(collar.Primitives.Composer, screen)
	if err != nil || composer.Text != "" {
		return nil, nil
	}
	settle, err := harness.SubmitSettle(collar.Primitives.Submit)
	if err != nil {
		return fail(now, err), nil
	}
	pane := substrate.PaneID(effect.Pane)
	input, err := harness.SubmitInput(collar.Primitives.Submit, action.Text)
	if err != nil {
		return fail(now, err), nil
	}
	if err := sendHarnessKeys(context.Background(), backend, pane, collar, input); err != nil {
		return fail(now, err), nil
	}
	ctx, cancel := context.WithDeadline(context.Background(), effect.Compaction.Deadline)
	defer cancel()
	if err := harness.AwaitComposerText(ctx, backend.Capture, pane, collar, action.Text, settle); err != nil {
		return fail(time.Now(), err), nil
	}
	if err := sendHarnessKeys(context.Background(), backend, pane, collar, substrate.Keys{Names: action.Keys, Submit: action.Submit}); err != nil {
		return fail(time.Now(), err), nil
	}
	return nil, nil
}
