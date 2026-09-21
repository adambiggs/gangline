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
	if err := cmd.hitch(append([]string{name}, arguments...)); err != nil {
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
		if hitch.Status == core.HitchActive {
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
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
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
