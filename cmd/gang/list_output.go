package main

import (
	"fmt"
	"io"
	"strconv"
)

type listOptions struct {
	limit int // zero means --all
	set   bool
}

func parseListOptions(name string, args []string, bindings map[string]any) (listOptions, []string, error) {
	limit, all := "50", false
	if bindings == nil {
		bindings = make(map[string]any)
	}
	bindings["limit"], bindings["all"] = &limit, &all
	flags := boundFlagSet(name, bindings)
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return listOptions{}, nil, usageError("%s: %v", name, err)
	}
	n, err := strconv.Atoi(limit)
	if err != nil || n <= 0 {
		return listOptions{}, nil, usageError("%s: --limit must be a positive integer; use --all for the full list", name)
	}
	if all {
		if flagWasSet(flags, "limit") {
			return listOptions{}, nil, usageError("%s: --all and --limit cannot be combined", name)
		}
		n = 0
	}
	return listOptions{limit: n, set: flagWasSet(flags, "limit") || flagWasSet(flags, "all")}, positionals, nil
}

func (o listOptions) count(total int) int {
	if o.limit > 0 && total > o.limit {
		return o.limit
	}
	return total
}

func (o listOptions) notice(w io.Writer, total int) error {
	if count := o.count(total); count < total {
		_, err := fmt.Fprintf(w, "Showing %d of %d entries; use --limit N or --all for more.\n", count, total)
		return err
	}
	return nil
}

func (o listOptions) writeLines(out, warn io.Writer, rows []string) error {
	for _, row := range rows[:o.count(len(rows))] {
		if _, err := fmt.Fprintln(out, row); err != nil {
			return err
		}
	}
	return o.notice(warn, len(rows))
}
