package main

import (
	"errors"
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

// appendEvent records exactly one fact while holding the team lock. Effects
// are returned to the caller and run only after the lock closes.
func (run *runtime) appendEvent(event core.Event) (next core.State, effects []core.Effect, err error) {
	locked, err := run.paths().Lock(run.settings.Session)
	if errors.Is(err, store.ErrLocked) {
		return core.State{}, nil, refuseError("team %q is busy; retry the command", run.settings.Session)
	}
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
