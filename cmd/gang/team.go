package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
	"golang.org/x/term"
)

func (cmd command) up(args []string) error {
	options := hitchOptions{}
	flags := boundFlagSet("up", map[string]any{
		"c": &options.Collar, "collar": &options.Collar,
		"d": &options.Directory, "dir": &options.Directory,
		"m": &options.Model, "model": &options.Model,
		"e": &options.Effort, "effort": &options.Effort,
		"t": &options.Task, "task": &options.Task,
		"r": &options.Role, "role": &options.Role,
		"resume": &options.Resume, "recover": &options.Recover, "stdin": &options.Stdin,
	})
	flagArguments, positionals := partitionOptions(flags, args)
	if err := flags.Parse(flagArguments); err != nil {
		return usageError("up: %v", err)
	}
	name := "lead"
	if len(positionals) > 1 {
		return usageError("up: unexpected argument %q", positionals[1])
	}
	if len(positionals) == 1 {
		name = positionals[0]
	}
	if err := cmd.hitchWithStaleClaim(upHitchArguments(name, flagArguments), true); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if err := run.flushUsageWork(); err != nil {
		return err
	}
	if cmd.upAttaches() {
		return cmd.attach(nil)
	}
	return nil
}

// upAttaches reports whether up should attach: only a terminal can drive tmux.
// /dev/null is a character device, so the file mode alone is not enough.
func (cmd command) upAttaches() bool {
	return cmd.stdinIsTerminal()
}

func upHitchArguments(name string, args []string) []string {
	if len(args) == 1 {
		if parsed, err := parseHitch([]string{name, args[0]}, "default", "default"); err == nil && parsed.Recover {
			return []string{name, args[0]}
		}
	}
	return append([]string{name, "--role", "lead"}, args...)
}
func (cmd command) down(args []string) error {
	var yes bool
	flags := boundFlagSet("down", map[string]any{"y": &yes, "yes": &yes})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("down: %v", err)
	}
	if len(positionals) != 0 {
		return usageError("down: unexpected argument %q", positionals[0])
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	lock, err := run.lockTeam()
	if err != nil {
		return err
	}
	defer lock.Close()
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	if !yes {
		if !cmd.stdinIsTerminal() {
			return refuseError("down requires a terminal for confirmation; use --yes to stop team %q non-interactively", run.settings.Session)
		}
		if err := confirmDown(cmd.stdin, cmd.stderr, run.settings.Session, len(agents)); err != nil {
			return err
		}
	}
	if err := eachAgent(agents, func(a core.Agent) error { return run.dropAgent(a.ID, true) }); err != nil {
		return err
	}
	if err := run.disarmEmptyWatchdog(); err != nil {
		return err
	}
	return os.RemoveAll(run.team.Directory)
}

func (cmd command) stdinIsTerminal() bool {
	if cmd.terminalInput != nil {
		return cmd.terminalInput()
	}
	stdin, ok := cmd.stdin.(*os.File)
	return ok && term.IsTerminal(int(stdin.Fd()))
}

func confirmDown(input io.Reader, output io.Writer, session string, agentCount int) error {
	if _, err := fmt.Fprintf(output, "Stop team %q and its %d agents? [y/N] ", session, agentCount); err != nil {
		return err
	}
	response, err := bufio.NewReader(io.LimitReader(input, 64)).ReadString('\n')
	response = strings.TrimSpace(response)
	if err != nil || !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
		return refuseError("down cancelled; team %q was not stopped", session)
	}
	return nil
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
			agents[i].Activity, agents[i].Evidence = core.Unknown, "native activity probe unavailable: agent state is locked"
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
			if err != nil {
				cause := err
				err = run.observeProbeFailure(l, &current, cause)
				if err == nil && run.cmd.stderr != nil {
					_, err = fmt.Fprintf(run.cmd.stderr, "%s: %s\n", current.Name, current.Evidence)
				}
			} else {
				err = run.observeCompaction(l, &current, c, screen)
				var refusal commandError
				if errors.As(err, &refusal) && refusal.status == exitNative {
					err = nil
				}
				if err == nil {
					err = run.observeActivity(l, &current, c, screen)
				}
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
	flags := boundFlagSet("roster", map[string]any{"porcelain": &machine})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("roster: %v", err)
	}
	if len(positionals) != 0 {
		return usageError("roster: unexpected argument %q", positionals[0])
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
	watchdogLimited := false
	if _, err := os.Stat(filepath.Join(run.team.Directory, "watchdog-unavailable")); err == nil {
		watchdogLimited = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, a := range agents {
		limited, err := run.processLimited(a)
		if err != nil {
			return err
		}
		marker := ""
		if limited {
			marker = " [process-unavailable]"
		}
		if watchdogLimited {
			marker += " [watchdog-unavailable]"
		}
		if machine {
			if marker != "" {
				marker = "\t" + strings.TrimSpace(marker)
			}
			_, err = fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\t%s\t%s%s\n", a.Name, a.Status, a.Activity, a.Collar, a.Pane, marker)
		} else {
			_, err = fmt.Fprintf(cmd.stdout, "%-16s %-10s %-14s %s%s\n", a.Name, a.Status, a.Activity, a.Collar, marker)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (cmd command) status(args []string) error {
	why := false
	flags := boundFlagSet("status", map[string]any{"why": &why})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("status: %v", err)
	}
	if len(positionals) > 1 {
		return usageError("status: unexpected argument %q", positionals[1])
	}
	name := ""
	if len(positionals) == 1 {
		name = positionals[0]
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
