package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
)

func (cmd command) up(arguments []string) error {
	name := "lead"
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name = arguments[0]
		arguments = arguments[1:]
	}
	hitchArguments := append([]string{name, "--role", "lead"}, arguments...)
	if err := cmd.hitch(hitchArguments); err != nil {
		return err
	}
	if file, ok := cmd.stdin.(*os.File); ok {
		if info, err := file.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return cmd.attach(nil)
		}
	}
	return nil
}

func (cmd command) down(arguments []string) error {
	if err := exactly(arguments, 1, "down"); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	if arguments[0] != settings.Session {
		return refuseError("down requires the configured session name %q", settings.Session)
	}
	run := &runtime{cmd: cmd, settings: settings}
	state, err := run.load()
	if err != nil {
		return err
	}
	names := make([]string, 0)
	for _, hitch := range state.Hitches {
		if hitch.Status == core.HitchActive || hitch.Status == core.HitchFailed {
			names = append(names, string(hitch.Name))
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := cmd.drop([]string{name}); err != nil {
			return err
		}
	}
	paths, err := run.paths().Team(settings.Session)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(paths.Directory); err != nil {
		return fmt.Errorf("remove stopped team state: %w", err)
	}
	return nil
}

func (cmd command) curfew(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("curfew: expected duration, HH:MM, clear, or no argument")
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	if len(arguments) == 0 {
		if state.Team.Curfew.IsZero() {
			_, err = fmt.Fprintln(cmd.stdout, "clear")
		} else {
			_, err = fmt.Fprintln(cmd.stdout, state.Team.Curfew.Format(time.RFC3339))
		}
		return err
	}
	if arguments[0] == "clear" {
		if state.Team.Curfew.IsZero() {
			return refuseError("team curfew is already clear")
		}
		_, err = run.drive(core.CurfewCleared{At: time.Now()})
		return err
	}
	now := time.Now()
	deadline, err := parseSchedule(arguments[0], now)
	if err != nil {
		return usageError("curfew: %v", err)
	}
	_, err = run.drive(core.CurfewSet{At: now, Deadline: deadline})
	return err
}

func (cmd command) whoami(arguments []string) error {
	if err := noArguments(arguments, "whoami"); err != nil {
		return err
	}
	_, state, err := cmd.loaded()
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
	_, state, err := cmd.loaded()
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
	_, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	if name == "" {
		name = nameAtPane(state, cmd.environment("TMUX_PANE"))
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
