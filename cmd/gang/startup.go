package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adambiggs/gangline/core"
)

func (run *runtime) deliveryWitnessPath(id core.HitchID) string {
	paths, _ := run.paths().Team(run.settings.Session)
	return filepath.Join(paths.Directory, "delivery-"+string(id)+".fifo")
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
