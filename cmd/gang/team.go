package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) up(args []string) error {
	name := "lead"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	if err := cmd.hitch(append([]string{name, "--role", "lead"}, args...)); err != nil {
		return err
	}
	if f, ok := cmd.stdin.(*os.File); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return cmd.attach(nil)
		}
	}
	return nil
}
func (cmd command) down(args []string) error {
	if err := exactly(args, 1, "down"); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if args[0] != run.settings.Session {
		return refuseError("down requires configured session %q", run.settings.Session)
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	if err := eachAgent(agents, func(a core.Agent) error { return run.drop(a.ID) }); err != nil {
		return err
	}
	return os.RemoveAll(run.team.Directory)
}
func eachAgent(agents []core.Agent, action func(core.Agent) error) error {
	results := make([]error, len(agents))
	var group sync.WaitGroup
	for i, a := range agents {
		group.Go(func() { results[i] = action(a) })
	}
	group.Wait()
	return errors.Join(results...)
}
func (cmd command) curfew(args []string) error {
	if len(args) > 1 {
		return usageError("curfew: expected deadline, clear, or no argument")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	team, err := run.team.ReadTeam()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		if team.Curfew.IsZero() {
			_, err = fmt.Fprintln(cmd.stdout, "clear")
		} else {
			_, err = fmt.Fprintln(cmd.stdout, team.Curfew.Format(time.RFC3339))
		}
		return err
	}
	event := core.Event{Type: "curfew_set", At: cmd.now()}
	if args[0] == "clear" {
		team.Curfew = time.Time{}
		event.Type = "curfew_cleared"
	} else {
		team.Curfew, err = parseSchedule(args[0], cmd.now())
		if err != nil {
			return usageError("curfew: %v", err)
		}
		event.Deadline = team.Curfew
	}
	if err := run.team.WriteTeam(team); err != nil {
		return err
	}
	return run.team.Append(event)
}
func (cmd command) whoami(args []string) error {
	if err := noArguments(args, "whoami"); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve("")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.stdout, a.Name)
	return err
}
func (run *runtime) observeRoster(agents []core.Agent) ([]core.Agent, error) {
	b, err := run.cmd.tmux(run.settings)
	if err != nil {
		return nil, err
	}
	windows, err := b.Windows(context.Background())
	if err != nil {
		exists, checkErr := b.SessionExists(context.Background())
		if checkErr != nil {
			return nil, checkErr
		}
		if exists {
			return nil, err
		}
	}
	present := map[string]bool{}
	titles := map[string]string{}
	for _, w := range windows {
		present[string(w.Pane.ID)] = true
		titles[string(w.Pane.ID)] = w.Name
	}
	for i, a := range agents {
		l, current, err := run.acquire(a.ID, false)
		if errors.Is(err, store.ErrLocked) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := run.checkDeadlines(l, &current); err != nil {
			_ = l.Close()
			return nil, err
		}
		if current.Pane != "" && !present[current.Pane] && current.Status != core.Dropping && current.Status != core.Failed {
			if err := run.apply(l, &current, core.Event{Type: "hitch_failed", Reason: "registered pane is absent from tmux"}); err != nil {
				_ = l.Close()
				return nil, err
			}
		}
		if present[current.Pane] && current.Status == core.Active {
			c, err := loadCollar(current.Collar, run.settings)
			if err != nil {
				_ = l.Close()
				return nil, err
			}
			input, err := run.input()
			if err != nil {
				_ = l.Close()
				return nil, err
			}
			screen, err := input.Capture(context.Background(), substrate.PaneID(current.Pane))
			if err == nil {
				err = run.observeCompaction(l, &current, c, screen)
			}
			var refusal commandError
			if errors.As(err, &refusal) && refusal.status == exitNative {
				err = nil
			}
			if err == nil {
				err = run.observeActivity(l, &current, c, screen)
			}
			if err != nil {
				_ = l.Close()
				return nil, err
			}
		}
		agents[i] = current
		if present[current.Pane] && titles[current.Pane] != windowTitle(current) {
			if err := run.mark(current); err != nil {
				_ = l.Close()
				return nil, err
			}
		}
		if err := run.release(l); err != nil {
			return nil, err
		}
	}
	return agents, nil
}
func (cmd command) roster(args []string) error {
	machine := false
	flags := quietFlagSet("roster")
	flags.BoolVar(&machine, "porcelain", false, "machine format")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usageError("roster: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	agents, err = run.observeRoster(agents)
	if err != nil {
		return err
	}
	for _, a := range agents {
		if machine {
			_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\t%s\t%s\n", a.Name, a.Status, a.Activity, a.Collar, a.Pane)
		} else {
			_, err = fmt.Fprintf(cmd.stdout, "%-16s %-10s %-14s %s\n", a.Name, a.Status, a.Activity, a.Collar)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (cmd command) status(args []string) error {
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	why := false
	flags := quietFlagSet("status")
	flags.BoolVar(&why, "why", false, "evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usageError("status: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	agents, err := run.observeRoster([]core.Agent{a})
	if err != nil {
		return err
	}
	a = agents[0]
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", a.Name, a.Status, a.Activity); err != nil {
		return err
	}
	if why && a.Evidence != "" {
		if _, err := fmt.Fprintln(cmd.stdout, a.Evidence); err != nil {
			return err
		}
	}
	if why && a.Compaction != nil {
		_, err = fmt.Fprintf(cmd.stdout, "compaction %s: %s\n", a.Compaction.Status, a.Compaction.Reason)
	}
	return err
}
