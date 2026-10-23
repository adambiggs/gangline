package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type logFilter struct{ Agent, Type string }

func parseLogFilter(args []string, allowFile bool) (logFilter, []string, error) {
	var filter logFilter
	flags := boundFlagSet("log", map[string]any{"agent": &filter.Agent, "type": &filter.Type})
	files, err := parseOptions(flags, args)
	if err != nil {
		return filter, nil, usageError("log: %v", err)
	}
	if len(files) > 0 && !allowFile {
		return filter, nil, usageError("unexpected log argument %q", files[0])
	}
	if len(files) > 1 {
		return filter, nil, usageError("unexpected log argument %q", files[1])
	}
	return filter, files, nil
}
func (filter logFilter) apply(e core.Event) (core.Event, bool) {
	if filter.Agent != "" && filter.Agent != string(e.HitchID) && filter.Agent != string(e.Name) {
		return e, false
	}
	if filter.Type != "" && filter.Type != e.Type {
		var readings []core.Reading
		for _, r := range e.Readings {
			if r.Kind == filter.Type {
				readings = append(readings, r)
			}
		}
		if len(readings) == 0 {
			return e, false
		}
		e.Readings = readings
	}
	return e, true
}

// writeFilteredLog validates each event it prints, as stored and as printed.
// A line the filter rejects on a plain JSON decode is skipped unvalidated, so
// a filtered read costs schema validation only for the events it prints. A
// line that is not a valid event is a warning on warn, the intact event
// appended after a torn record on that line prints, and the export continues:
// the log keeps the damage, so failing on it would fail every later export.
func writeFilteredLog(out, warn io.Writer, in io.Reader, filter logFilter, present func(string) string) error {
	names := make(map[core.HitchID]core.AgentName)
	var write func(line int, data []byte) error
	write = func(line int, data []byte) error {
		var peek core.Event
		if json.Unmarshal(data, &peek) == nil {
			if peek.Name != "" {
				names[peek.HitchID] = peek.Name
			}
			if _, ok := filter.apply(peek); !ok {
				return nil
			}
		}
		e, err := core.DecodeEvent(data)
		if err != nil {
			if _, err := fmt.Fprintf(warn, "warning: audit line %d: %v\n", line, err); err != nil {
				return err
			}
			if appended := appendedEvent(data); appended != nil {
				return write(line, appended)
			}
			return nil
		}
		e, ok := filter.apply(e)
		if !ok {
			return nil
		}
		if e.Pane != "" {
			name := e.Name
			if name == "" {
				name = names[e.HitchID]
			}
			e.Pane = string(name)
		}
		e.Reason = present(e.Reason)
		if e.Envelope != nil {
			e.Envelope.Reason = present(e.Envelope.Reason)
		}
		if e.Compaction != nil {
			e.Compaction.Reason = present(e.Compaction.Reason)
		}
		for i := range e.Readings {
			e.Readings[i].Reason = present(e.Readings[i].Reason)
		}
		data, err = core.EncodeEvent(e)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}
	return store.ScanLog(in, write)
}

// appendedEvent returns the valid event that ends a damaged line. A write cut
// short, as by a full disk, leaves a record without its newline, and the next
// append continues on the same line. A JSON string escapes its quotes, so an
// event can begin only where the line holds an unescaped {".
func appendedEvent(data []byte) []byte {
	for i := 1; i < len(data); i++ {
		next := bytes.Index(data[i:], []byte(`{"`))
		if next < 0 {
			return nil
		}
		i += next
		if _, err := core.DecodeEvent(data[i:]); err == nil {
			return data[i:]
		}
	}
	return nil
}
