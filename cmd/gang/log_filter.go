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
	flags := boundFlagSet("log", map[string]any{"agent": &filter.Agent, "type": &filter.Type})
	for i := 0; i < len(args); i++ {
		argument := args[i]
		if strings.HasPrefix(argument, "-") {
			name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(argument, "-"), "-"), "=")
			option := flags.Lookup(name)
			if option == nil {
				return filter, nil, usageError("unexpected log argument %q", argument)
			}
			if boolean, ok := option.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
				if !hasValue {
					value = "true"
				}
			} else if !hasValue {
				i++
				if i == len(args) {
					return filter, nil, usageError("%s requires a value", argument)
				}
				value = args[i]
			}
			if value == "" {
				return filter, nil, usageError("%s requires a value", argument)
			}
			if err := flags.Set(name, value); err != nil {
				return filter, nil, usageError("log: %v", err)
			}
		} else {
			if !allowFile || len(files) > 0 {
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
