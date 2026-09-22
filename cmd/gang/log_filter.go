package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type logFilter struct {
	Agent string
	Type  string
}

func parseLogFilter(args []string, allowFile bool) (logFilter, []string, error) {
	var filter logFilter
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent", "--type":
			flag := args[i]
			i++
			if i == len(args) || args[i] == "" {
				return filter, nil, usageError("%s requires a value", flag)
			}
			if flag == "--agent" {
				filter.Agent = args[i]
			} else {
				filter.Type = args[i]
			}
		default:
			if !allowFile || strings.HasPrefix(args[i], "-") || len(files) > 0 {
				return filter, nil, usageError("unexpected log argument %q", args[i])
			}
			files = append(files, args[i])
		}
	}
	return filter, files, nil
}

// Filtering walks complete history so outcome-only records retain their target.
// Agent names mean the name at that event; immutable hitch IDs also match.
func writeFilteredLog(out io.Writer, in io.Reader, filter logFilter) error {
	entries, err := store.ReadLog(in)
	if err != nil {
		return err
	}
	state := core.NewState(core.Team{ID: "log", Name: "log"})
	targets := map[core.EnvelopeID]core.HitchID{}
	for _, entry := range entries {
		e := entry.Event
		data, err := core.EncodeEvent(e)
		if err != nil {
			return err
		}
		var r struct {
			ID           string                `json:"id"`
			Operation    core.TimeoutOperation `json:"operation"`
			Recipient    core.AgentName        `json:"recipient"`
			HitchID      core.HitchID          `json:"hitch_id"`
			Hitch        *core.Hitch           `json:"hitch"`
			EnvelopeID   core.EnvelopeID       `json:"envelope_id"`
			Envelope     *core.Envelope        `json:"envelope"`
			Compaction   *core.Compaction      `json:"compaction"`
			CompactionID core.CompactionID     `json:"compaction_id"`
		}
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		if core.EventName(e) == "operation_timed_out" {
			switch r.Operation {
			case core.TimeoutDelivery:
				r.EnvelopeID = core.EnvelopeID(r.ID)
			case core.TimeoutCompaction:
				r.CompactionID = core.CompactionID(r.ID)
			default:
				r.HitchID = core.HitchID(r.ID)
			}
		}
		id := r.HitchID
		if r.Recipient != "" {
			if h, ok := activeByName(state, string(r.Recipient)); ok {
				id = h.ID
			}
		}
		if r.Hitch != nil {
			id = r.Hitch.ID
		}
		if r.Compaction != nil {
			id = r.Compaction.HitchID
		}
		if r.CompactionID != "" {
			id = state.Compactions[r.CompactionID].HitchID
		}
		if r.Envelope != nil {
			if h, ok := activeByName(state, string(r.Envelope.To)); ok {
				id = h.ID
				targets[r.Envelope.ID] = id
			}
		}
		if r.EnvelopeID != "" {
			id = targets[r.EnvelopeID]
			if id == "" {
				if d, ok := state.Deliveries[r.EnvelopeID]; ok {
					if h, ok := activeByName(state, string(d.Envelope.To)); ok {
						id = h.ID
						targets[r.EnvelopeID] = id
					}
				}
			}
		}
		name := string(state.Hitches[id].Name)
		if r.Hitch != nil {
			name = string(r.Hitch.Name)
		}
		matchAgent := filter.Agent == "" || filter.Agent == string(id) || filter.Agent == name
		matchType := filter.Type == "" || filter.Type == core.EventName(e)
		if observation, ok := e.(core.Observation); ok && !matchType {
			var readings []core.Reading
			for _, reading := range observation.Readings {
				if reading.Kind == filter.Type {
					readings = append(readings, reading)
				}
			}
			if len(readings) > 0 {
				observation.Readings = readings
				data, err = core.EncodeEvent(observation)
				if err != nil {
					return err
				}
				matchType = true
			}
		}
		if matchAgent && matchType {
			if _, err := fmt.Fprintln(out, string(data)); err != nil {
				return err
			}
		}
		state, _ = core.Step(state, e)
	}
	return nil
}
