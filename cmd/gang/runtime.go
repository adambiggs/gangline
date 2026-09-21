package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

const (
	bootTimeout      = 30 * time.Second
	deliveryTimeout  = 30 * time.Second
	operationTimeout = 30 * time.Second
)

type runtime struct {
	cmd      command
	settings settings
}

func (cmd command) runtime() (*runtime, error) {
	settings, err := cmd.settings()
	if err != nil {
		return nil, err
	}
	return &runtime{cmd: cmd, settings: settings}, nil
}

func (run *runtime) initial() core.State {
	return core.NewState(core.Team{ID: core.TeamID(run.settings.Session), Name: run.settings.Session})
}

func (run *runtime) paths() store.Paths { return store.Paths{Root: run.settings.StateRoot} }

func (run *runtime) load() (core.State, error) {
	locked, err := run.paths().Lock(run.settings.Session)
	if errors.Is(err, store.ErrLocked) {
		return core.State{}, refuseError("team %q is busy; retry the command", run.settings.Session)
	}
	if err != nil {
		return core.State{}, err
	}
	state, _, loadErr := locked.Load(run.initial())
	closeErr := locked.Close()
	if loadErr != nil {
		return core.State{}, loadErr
	}
	return state, closeErr
}

// append applies exactly one fact while holding the team lock. Effects are
// returned to the caller and are deliberately executed after the lock closes.
func (run *runtime) append(event core.Event) (core.State, []core.Effect, error) {
	locked, err := run.paths().Lock(run.settings.Session)
	if errors.Is(err, store.ErrLocked) {
		return core.State{}, nil, refuseError("team %q is busy; retry the command", run.settings.Session)
	}
	if err != nil {
		return core.State{}, nil, err
	}
	defer locked.Close()
	state, _, err := locked.Load(run.initial())
	if err != nil {
		return core.State{}, nil, err
	}
	if err := locked.Append(event); err != nil {
		return core.State{}, nil, err
	}
	next, effects := core.Step(state, event)
	if err := locked.SaveSnapshot(next); err != nil {
		return core.State{}, nil, err
	}
	if err := locked.Close(); err != nil {
		return core.State{}, nil, err
	}
	return next, effects, nil
}

func (run *runtime) drive(events ...core.Event) (core.State, error) {
	var latest core.State
	queue := append([]core.Event(nil), events...)
	for len(queue) != 0 {
		event := queue[0]
		queue = queue[1:]
		state, effects, err := run.append(event)
		if err != nil {
			return core.State{}, err
		}
		latest = state
		for _, effect := range effects {
			outcome, err := run.executeEffect(state, effect)
			if err != nil {
				return core.State{}, err
			}
			if outcome != nil {
				queue = append(queue, outcome)
			}
		}
		if ready, ok := event.(core.HitchReady); ok {
			startup, found, err := run.readStartup(ready.HitchID)
			if err != nil {
				return core.State{}, err
			}
			if found {
				queue = append(queue, startup.Event)
				if err := os.Remove(run.startupPath(ready.HitchID)); err != nil && !os.IsNotExist(err) {
					return core.State{}, fmt.Errorf("remove queued startup: %w", err)
				}
			}
		}
	}
	return latest, nil
}

func (run *runtime) recover() (core.State, error) {
	state, err := run.load()
	if err != nil {
		return core.State{}, err
	}
	for _, id := range core.DueTimedDeliveries(state, time.Now()) {
		state, err = run.drive(core.TimedDeliveryReleased{At: time.Now(), EnvelopeID: id})
		if err != nil {
			return core.State{}, err
		}
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
		spec.Env["GANGLINE_HITCH_ID"] = string(effect.Hitch.ID)
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
		if sendErr := backend.SendKeys(context.Background(), substrate.PaneID(effect.Pane), action.Input()); sendErr != nil {
			return core.CompactionFailedEvent{At: now, CompactionID: effect.Compaction.ID, Reason: sendErr.Error()}, nil
		}
		return nil, nil
	case core.InterruptHitch:
		hitch := state.Hitches[effect.HitchID]
		collar, loadErr := loadCollar(hitch.Collar, run.settings)
		if loadErr != nil {
			return core.InterruptFailed{At: now, HitchID: effect.HitchID, Reason: loadErr.Error()}, nil
		}
		if sendErr := backend.SendKeys(context.Background(), substrate.PaneID(effect.Pane), collar.Actions.Interrupt.Input()); sendErr != nil {
			return core.InterruptFailed{At: now, HitchID: effect.HitchID, Reason: sendErr.Error()}, nil
		}
		return core.InterruptSucceeded{At: time.Now(), HitchID: effect.HitchID}, nil
	case core.KillHitch:
		if killErr := backend.Kill(context.Background(), substrate.PaneID(effect.Pane)); killErr != nil {
			return core.DropFailed{At: now, HitchID: effect.HitchID, Reason: killErr.Error()}, nil
		}
		return core.DropSucceeded{At: time.Now(), HitchID: effect.HitchID}, nil
	default:
		return nil, fmt.Errorf("unsupported effect %T", effect)
	}
}

func (run *runtime) deliver(state core.State, backend interface {
	Capture(context.Context, substrate.PaneID) (substrate.Screen, error)
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
	if err := backend.SendKeys(context.Background(), substrate.PaneID(effect.Pane), substrate.Keys{Text: wire}); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "text input returned an error: " + err.Error()}, nil
	}
	readback, err := backend.Capture(context.Background(), substrate.PaneID(effect.Pane))
	if err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "cannot verify composer after input: " + err.Error()}, nil
	}
	observed, err := harness.ReadComposer(collar.Primitives.Composer, readback)
	if err != nil || observed.Text != wire {
		evidence := "composer readback did not match the attributed envelope"
		if err != nil {
			evidence = "composer readback failed: " + err.Error()
		}
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: evidence}, nil
	}
	if err := backend.SendKeys(context.Background(), substrate.PaneID(effect.Pane), substrate.Keys{Submit: true}); err != nil {
		return core.DeliveryUnverifiedEvent{At: time.Now(), EnvelopeID: effect.Envelope.ID, Evidence: "submit returned an error after verified text input: " + err.Error()}, nil
	}
	return core.DeliverySucceeded{At: time.Now(), EnvelopeID: effect.Envelope.ID}, nil
}

type startupRecord struct {
	Event      core.SendRequested `json:"event"`
	Model      string             `json:"model,omitempty"`
	Effort     string             `json:"effort,omitempty"`
	Resume     string             `json:"resume,omitempty"`
	RolePrompt string             `json:"role_prompt,omitempty"`
}

func (run *runtime) startupPath(id core.HitchID) string {
	paths, _ := run.paths().Team(run.settings.Session)
	return filepath.Join(paths.Directory, "startup-"+string(id)+".json")
}

func (run *runtime) writeStartup(id core.HitchID, record startupRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.Directory, 0o700); err != nil {
		return err
	}
	return os.WriteFile(run.startupPath(id), append(data, '\n'), 0o600)
}

func (run *runtime) readStartup(id core.HitchID) (startupRecord, bool, error) {
	data, err := os.ReadFile(run.startupPath(id))
	if os.IsNotExist(err) {
		return startupRecord{}, false, nil
	}
	if err != nil {
		return startupRecord{}, false, err
	}
	var record startupRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return startupRecord{}, false, fmt.Errorf("decode queued startup: %w", err)
	}
	return record, true, nil
}

func activeByName(state core.State, name string) (core.Hitch, bool) {
	for _, hitch := range state.Hitches {
		if hitch.Name == core.AgentName(name) && hitch.Status == core.HitchActive {
			return hitch, true
		}
	}
	return core.Hitch{}, false
}

func hitchByName(state core.State, name string) (core.Hitch, bool) {
	for _, hitch := range state.Hitches {
		if hitch.Name == core.AgentName(name) && hitch.Status != core.HitchDropped {
			return hitch, true
		}
	}
	return core.Hitch{}, false
}
