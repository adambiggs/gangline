package main

import (
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
func writeFilteredLog(out io.Writer, in io.Reader, filter logFilter) error {
	return store.ReadLog(in, func(e core.Event) error {
		if filter.Agent != "" && filter.Agent != string(e.HitchID) && filter.Agent != string(e.Name) {
			return nil
		}
		if filter.Type != "" && filter.Type != e.Type {
			var readings []core.Reading
			for _, r := range e.Readings {
				if r.Kind == filter.Type {
					readings = append(readings, r)
				}
			}
			if len(readings) == 0 {
				return nil
			}
			e.Readings = readings
		}
		data, err := core.EncodeEvent(e)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	})
}
