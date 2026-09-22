package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type logFilter struct{ Agent, Type string }

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
