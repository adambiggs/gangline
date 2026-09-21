package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
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

func (cmd command) loaded() (*runtime, core.State, error) {
	run, err := cmd.runtime()
	if err != nil {
		return nil, core.State{}, err
	}
	state, err := run.load()
	return run, state, err
}

func (run *runtime) initial() core.State {
	return core.NewState(core.Team{ID: core.TeamID(run.settings.Session), Name: run.settings.Session})
}

func (run *runtime) paths() store.Paths { return store.Paths{Root: run.settings.StateRoot} }

func (run *runtime) lock() (*store.LockedTeam, error) {
	locked, err := run.paths().Lock(run.settings.Session)
	if errors.Is(err, store.ErrLocked) {
		return nil, refuseError("team %q is busy; retry the command", run.settings.Session)
	}
	return locked, err
}

func (run *runtime) load() (core.State, error) {
	locked, err := run.lock()
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

// appendEvent records exactly one fact while holding the team lock. Effects
// are returned to the caller and run only after the lock closes.
func (run *runtime) appendEvent(event core.Event) (next core.State, effects []core.Effect, err error) {
	locked, err := run.lock()
	if err != nil {
		return core.State{}, nil, err
	}
	defer func() {
		if closeErr := locked.Close(); err == nil {
			err = closeErr
		}
	}()
	state, _, err := locked.Load(run.initial())
	if err != nil {
		return core.State{}, nil, err
	}
	if err := locked.Append(event); err != nil {
		return core.State{}, nil, err
	}
	next, effects = core.Step(state, event)
	if err := locked.SaveSnapshot(next); err != nil {
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
		state, effects, err := run.appendEvent(event)
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
		startup, err := run.releaseStartup(event)
		if err != nil {
			return core.State{}, err
		}
		if startup != nil {
			queue = append(queue, startup)
		}
	}
	return latest, nil
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

func (run *runtime) releaseStartup(event core.Event) (core.Event, error) {
	ready, ok := event.(core.HitchReady)
	if !ok {
		return nil, nil
	}
	record, found, err := run.readStartup(ready.HitchID)
	if err != nil || !found {
		return nil, err
	}
	if err := os.Remove(run.startupPath(ready.HitchID)); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove queued startup: %w", err)
	}
	return record.Event, nil
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

func nameAtPane(state core.State, pane string) string {
	for _, hitch := range state.Hitches {
		if hitch.Pane == pane {
			return string(hitch.Name)
		}
	}
	return ""
}
