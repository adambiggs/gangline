package main

import (
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
// a filtered read costs schema validation only for the events it prints.
func writeFilteredLog(out io.Writer, in io.Reader, filter logFilter) error {
	return store.ScanLog(in, func(line int, data []byte) error {
		var peek core.Event
		if json.Unmarshal(data, &peek) == nil {
			if _, ok := filter.apply(peek); !ok {
				return nil
			}
		}
		e, err := core.DecodeEvent(data)
		if err != nil {
			return fmt.Errorf("audit line %d: %w", line, err)
		}
		e, ok := filter.apply(e)
		if !ok {
			return nil
		}
		data, err = core.EncodeEvent(e)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	})
}
