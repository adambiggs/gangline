package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adambiggs/gangline/core"
)

func (cmd command) whoami(arguments []string) error {
	if err := noArguments(arguments, "whoami"); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	pane := cmd.environment("TMUX_PANE")
	for _, hitch := range state.Hitches {
		if hitch.Pane == pane && hitch.Status == core.HitchActive {
			_, err = fmt.Fprintln(cmd.stdout, hitch.Name)
			return err
		}
	}
	return refuseError("current pane is not a registered active agent")
}
func (cmd command) roster(arguments []string) error {
	porcelain := false
	flags := quietFlagSet("roster")
	flags.BoolVar(&porcelain, "porcelain", false, "machine format")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("roster: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	hitches := make([]core.Hitch, 0, len(state.Hitches))
	for _, hitch := range state.Hitches {
		hitches = append(hitches, hitch)
	}
	sort.Slice(hitches, func(i, j int) bool { return hitches[i].Name < hitches[j].Name })
	for _, hitch := range hitches {
		if hitch.Status == core.HitchDropped {
			continue
		}
		if porcelain {
			_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\t%s\t%s\n", hitch.Name, hitch.Status, hitch.Activity, hitch.Collar, hitch.Pane)
		} else {
			_, err = fmt.Fprintf(cmd.stdout, "%-16s %-10s %-14s %s\n", hitch.Name, hitch.Status, hitch.Activity, hitch.Collar)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (cmd command) status(arguments []string) error {
	why := false
	name := ""
	flags := quietFlagSet("status")
	flags.BoolVar(&why, "why", false, "include evidence")
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name, arguments = arguments[0], arguments[1:]
	}
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("status: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	if name == "" {
		pane := cmd.environment("TMUX_PANE")
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane {
				name = string(hitch.Name)
				break
			}
		}
	}
	hitch, ok := hitchByName(state, name)
	if !ok {
		return refuseError("agent %q is not registered", name)
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", hitch.Name, hitch.Status, hitch.Activity)
	if err == nil && why {
		evidence := hitch.WedgeEvidence
		if hitch.Activity == core.ActivityBlocked {
			evidence = hitch.BlockedEvidence
		}
		if evidence != "" {
			_, err = fmt.Fprintln(cmd.stdout, evidence)
		}
	}
	return err
}
