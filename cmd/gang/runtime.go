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
	bootTimeout                = 30 * time.Second
	deliveryTimeout            = 30 * time.Second
	boundaryHookTimeoutSeconds = 360
	startupRetryInterval       = 100 * time.Millisecond
	operationTimeout           = 30 * time.Second
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
	return run.paths().LockWait(run.settings.Session)
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

// appendEvent records intent and claims native input under the same team lock.
// A contended input claim is deferred before the writer releases that lock.
func (run *runtime) appendEvent(event core.Event, held map[core.HitchID]*os.File) (previous, next core.State, effects []core.Effect, err error) {
	locked, err := run.lock()
	if err != nil {
		return core.State{}, core.State{}, nil, err
	}
	defer func() {
		if closeErr := locked.Close(); err == nil {
			err = closeErr
		}
	}()
	state, _, err := locked.Load(run.initial())
	if err != nil {
		return core.State{}, core.State{}, nil, err
	}
	next, effects = core.Step(state, event)
	var ready []core.Effect
	var deferred []core.Event
	for _, effect := range effects {
		if delivery, ok := effect.(core.DeliverEnvelope); ok {
			hitch, found := activeByName(next, string(delivery.Envelope.To))
			if !found {
				return core.State{}, core.State{}, nil, fmt.Errorf("delivery recipient disappeared before input ownership")
			}
			if held[hitch.ID] == nil {
				owner, lockErr := run.paths().LockInput(run.settings.Session, string(hitch.ID))
				if errors.Is(lockErr, store.ErrLocked) {
					deferred = append(deferred, core.DeliveryDeferred{At: time.Now(), EnvelopeID: delivery.Envelope.ID, Reason: "another command still owns native input"})
					continue
				}
				if lockErr != nil {
					return core.State{}, core.State{}, nil, lockErr
				}
				held[hitch.ID] = owner
			}
		}
		ready = append(ready, effect)
	}
	if err := locked.Append(event); err != nil {
		return core.State{}, core.State{}, nil, err
	}
	for _, refusal := range deferred {
		if err := locked.Append(refusal); err != nil {
			return core.State{}, core.State{}, nil, err
		}
		next, _ = core.Step(next, refusal)
	}
	effects = ready
	if err := locked.SaveSnapshot(next); err != nil {
		return core.State{}, core.State{}, nil, err
	}
	return state, next, effects, nil
}

func (run *runtime) drive(events ...core.Event) (core.State, error) {
	held := make(map[core.HitchID]*os.File)
	defer func() {
		for _, file := range held {
			_ = file.Close()
		}
	}()
	var latest core.State
	queue := append([]core.Event(nil), events...)
	for len(queue) != 0 {
		event := queue[0]
		queue = queue[1:]
		previous, state, effects, err := run.appendEvent(event, held)
		if err != nil {
			return core.State{}, err
		}
		latest = state
		if err := run.markChangedWindows(previous, state); err != nil {
			return core.State{}, err
		}
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
	var selected core.Hitch
	selectedRank := 100
	for _, hitch := range state.Hitches {
		if hitch.Name != core.AgentName(name) || hitch.Status == core.HitchDropped {
			continue
		}
		rank := hitchStatusRank(hitch.Status)
		if rank < selectedRank || (rank == selectedRank && hitch.ID < selected.ID) {
			selected, selectedRank = hitch, rank
		}
	}
	return selected, selectedRank != 100
}

func dropCandidateByName(state core.State, name string) (core.Hitch, bool) {
	var failed core.Hitch
	for _, hitch := range state.Hitches {
		if hitch.Name == core.AgentName(name) && hitch.Status == core.HitchFailed && (failed.ID == "" || hitch.ID < failed.ID) {
			failed = hitch
		}
	}
	if failed.ID != "" {
		return failed, true
	}
	return hitchByName(state, name)
}

func hitchStatusRank(status core.HitchStatus) int {
	switch status {
	case core.HitchActive:
		return 0
	case core.HitchStarting, core.HitchBooting:
		return 1
	case core.HitchDropping:
		return 2
	case core.HitchFailed:
		return 3
	default:
		return 100
	}
}

func nameAtPane(state core.State, pane string) string {
	for _, hitch := range state.Hitches {
		if hitch.Pane == pane {
			return string(hitch.Name)
		}
	}
	return ""
}
