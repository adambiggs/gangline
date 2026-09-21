package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type wedgeObservationFile struct {
	Screen substrate.Screen `json:"screen"`
}

const paneLossGrace = 500 * time.Millisecond

type paneLossObservation struct {
	MissingSince time.Time `json:"missing_since"`
	Evidence     string    `json:"evidence"`
}

func paneLossEvent(hitch core.Hitch, observation paneLossObservation, now time.Time) *core.PaneVanished {
	if observation.MissingSince.IsZero() || now.Sub(observation.MissingSince) < paneLossGrace {
		return nil
	}
	return &core.PaneVanished{At: now, HitchID: hitch.ID, Evidence: observation.Evidence}
}

func (run *runtime) reconcilePanes(state core.State, now time.Time) (core.State, error) {
	active := false
	for _, hitch := range state.Hitches {
		active = active || hitch.Status == core.HitchActive
	}
	if !active {
		return state, nil
	}

	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return core.State{}, err
	}
	present := make(map[string]bool)
	exists, err := backend.SessionExists(context.Background())
	if err != nil {
		return core.State{}, err
	}
	if exists {
		windows, listErr := backend.Windows(context.Background())
		if listErr != nil {
			return core.State{}, listErr
		}
		for _, window := range windows {
			present[string(window.Pane.ID)] = true
		}
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return core.State{}, err
	}

	for id, hitch := range state.Hitches {
		if hitch.Status != core.HitchActive {
			continue
		}
		file := filepath.Join(paths.Directory, "pane-loss-"+string(id)+".json")
		if present[hitch.Pane] {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				return core.State{}, fmt.Errorf("clear pane-loss observation: %w", err)
			}
			continue
		}

		var observation paneLossObservation
		data, readErr := os.ReadFile(file)
		switch {
		case os.IsNotExist(readErr):
			observation = paneLossObservation{
				MissingSince: now,
				Evidence: fmt.Sprintf("pane %q has been absent from tmux session %q since %s",
					hitch.Pane, run.settings.Session, now.UTC().Format(time.RFC3339Nano)),
			}
			encoded, encodeErr := json.Marshal(observation)
			if encodeErr != nil {
				return core.State{}, encodeErr
			}
			if err := os.WriteFile(file, append(encoded, '\n'), 0o600); err != nil {
				return core.State{}, fmt.Errorf("record pane-loss observation: %w", err)
			}
			continue
		case readErr != nil:
			return core.State{}, fmt.Errorf("read pane-loss observation: %w", readErr)
		case json.Unmarshal(data, &observation) != nil:
			return core.State{}, fmt.Errorf("read pane-loss observation %q: invalid JSON", file)
		}

		event := paneLossEvent(hitch, observation, now)
		if event == nil {
			continue
		}
		state, err = run.drive(*event)
		if err != nil {
			return core.State{}, err
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return core.State{}, fmt.Errorf("remove pane-loss observation: %w", err)
		}
	}
	return state, nil
}

func (run *runtime) observeWedges(state core.State) error {
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	entries, err := run.teamLog()
	if err != nil {
		return err
	}
	for _, hitch := range state.Hitches {
		if hitch.Status != core.HitchActive || hitch.Activity != core.ActivityBusy {
			continue
		}
		screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
		if err != nil {
			continue
		}
		file := filepath.Join(paths.Directory, "observe-"+string(hitch.ID)+".json")
		var prior wedgeObservationFile
		data, readErr := os.ReadFile(file)
		hasPrior := readErr == nil && json.Unmarshal(data, &prior) == nil
		encoded, _ := json.Marshal(wedgeObservationFile{Screen: screen})
		if err := os.WriteFile(file, append(encoded, '\n'), 0o600); err != nil {
			return err
		}
		if !hasPrior {
			continue
		}
		collar, err := loadCollar(hitch.Collar, run.settings)
		if err != nil {
			return err
		}
		wedge, err := harness.DetectWedge(collar.Primitives.Wedge, harness.WedgeObservation{
			Previous: prior.Screen, Current: screen, BusySince: busySince(entries, hitch.ID), ObservedAt: time.Now(), TurnActive: true,
		})
		if err != nil {
			return err
		}
		if wedge.Detected {
			state, err = run.drive(core.WedgeDetected{At: time.Now(), HitchID: hitch.ID, Evidence: wedge.Evidence})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (run *runtime) teamLog() ([]store.LogEntry, error) {
	locked, err := run.paths().Lock(run.settings.Session)
	if err != nil {
		return nil, err
	}
	defer locked.Close()
	return locked.Log()
}

func busySince(entries []store.LogEntry, id core.HitchID) time.Time {
	var since time.Time
	for _, entry := range entries {
		if event, ok := entry.Event.(core.TurnStarted); ok && event.HitchID == id {
			since = event.At
		}
		if event, ok := entry.Event.(core.TurnBoundaryReached); ok && event.HitchID == id {
			since = time.Time{}
		}
	}
	return since
}
