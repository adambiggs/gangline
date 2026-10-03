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
	"unicode/utf8"

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
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("up: %v", err)
	}
	flagArguments, _ := partitionOptions(flags, args)
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
	if err := run.refuseTeamWide("down"); err != nil {
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
	if err := run.recordDown(len(agents)); err != nil {
		return err
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
	if err := run.refuseTeamWide("curfew"); err != nil {
		return err
	}
	event := core.Event{Type: "curfew_set", At: cmd.now()}
	if args[0] == "clear" {
		team.Curfew = time.Time{}
		event.Type = "curfew_cleared"
	} else {
		team.Curfew, err = parseSchedule(args[0], cmd.now())
		if err != nil {
			return usageError("curfew: invalid deadline %q (%v)", args[0], err)
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
	listed := err == nil
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
			// The operation holding the lock maintains the record, so the row
			// is the record as saved, with its deadlines read against now.
			agents[i], _ = core.Step(a, core.Event{Type: "deadline_checked", At: run.cmd.now(), HitchID: a.ID})
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := run.checkDeadlines(l, &current); err != nil {
			_ = run.unlock(l)
			return nil, err
		}
		unlisted := false
		if current.Pane != "" && !present[current.Pane] && current.Status != core.Dropping {
			// A listing is enough to fail the agent. With no session to list,
			// an unreachable socket hides a pane that may still run, so the
			// record stands unless its recorded process has exited. Only a
			// check that confirms the pane closed removes it from the record.
			var err error
			_, closed, checkErr := run.paneGone(current)
			switch {
			case checkErr == nil && closed:
				err = run.forgetPane(l, &current)
			case !listed:
				if !recordedProcessExited(current) {
					unlisted = true
				} else if current.Status != core.Failed {
					err = run.apply(l, &current, core.Event{Type: "hitch_failed", Reason: "tmux lists no team session and the recorded process has exited"})
				}
			case current.Status != core.Failed:
				err = run.apply(l, &current, core.Event{Type: "hitch_failed", Reason: "registered pane is absent from tmux"})
			}
			if err != nil {
				_ = run.unlock(l)
				return nil, err
			}
		}
		if present[current.Pane] && current.Status == core.Active {
			c, err := loadCollar(current.Collar, run.settings)
			if err != nil {
				_ = run.unlock(l)
				return nil, err
			}
			input, err := run.input()
			if err != nil {
				_ = run.unlock(l)
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
				if err == nil {
					err = run.observeActivity(l, &current, c, screen)
				}
			}
			if err != nil {
				_ = run.unlock(l)
				return nil, err
			}
		}
		agents[i] = current
		if unlisted && current.Status != core.Failed {
			agents[i].Activity, agents[i].Evidence = core.Unknown, "tmux lists no team session"
		}
		if present[current.Pane] && titles[current.Pane] != windowTitle(current) {
			if err := run.mark(current); err != nil {
				_ = run.unlock(l)
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
	flags := boundFlagSet("roster", map[string]any{"json": &machine})
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
	out := rosterJSON{WatchdogAvailable: !watchdogLimited, Agents: []agentJSON{}}
	for _, a := range agents {
		row, err := run.agentRow(a)
		if err != nil {
			return err
		}
		if machine {
			out.Agents = append(out.Agents, row)
			continue
		}
		marker := ""
		if !row.ProcessAvailable {
			marker = " [process-unavailable]"
		}
		if watchdogLimited {
			marker += " [watchdog-unavailable]"
		}
		line := fmt.Sprintf("%-16s %-10s %-14s %s%s", a.Name, a.Status, a.Activity, a.Collar, marker)
		if _, err := fmt.Fprintln(cmd.stdout, line+rosterReason(a, rosterReasonLimit(cmd.stdout, line))); err != nil {
			return err
		}
	}
	if machine {
		return writeJSON(cmd.stdout, out)
	}
	return nil
}

// A long reason keeps its first rosterReasonHead characters, where a native
// exit carries its status, and spends the rest of its room on its end, where
// the native's last output is. The room is rosterReasonWidth where the output
// has no width.
const (
	rosterReasonHead  = 40
	rosterReasonWidth = 100
	rosterReasonGap   = "  "
)

// rosterReasonLimit is the room a row leaves for its reason: on a terminal,
// the rest of the terminal's row.
func rosterReasonLimit(output io.Writer, row string) int {
	file, ok := outputFile(output)
	if !ok {
		return rosterReasonWidth
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return rosterReasonWidth
	}
	return rosterReasonRoom(width, row)
}

// rosterReasonRoom counts characters, so a row with characters wider than one
// cell wraps. The room always holds the reason's head and the mark of its cut;
// a terminal too narrow for those wraps.
func rosterReasonRoom(width int, row string) int {
	return max(width-utf8.RuneCountInString(row)-len(rosterReasonGap), rosterReasonHead+1)
}

// rosterReason is the evidence of an agent that is failed or whose activity
// is not known to be healthy, on one line of at most limit characters. A
// longer one keeps its head and its end; `status --why` and `--json` carry
// the whole text.
func rosterReason(a core.Agent, limit int) string {
	if a.Evidence == "" || a.Status != core.Failed && a.Activity != core.Blocked && a.Activity != core.Unknown && a.Activity != core.Wedged {
		return ""
	}
	reason := []rune(strings.Join(strings.Fields(a.Evidence), " "))
	if len(reason) > limit {
		head := min(rosterReasonHead, limit-1)
		reason = append(append(reason[:head:head], '…'), reason[len(reason)-(limit-head-1):]...)
	}
	return rosterReasonGap + string(reason)
}

func (run *runtime) agentRow(a core.Agent) (agentJSON, error) {
	limited, err := run.processLimited(a)
	if err != nil {
		return agentJSON{}, err
	}
	row := agentJSON{Name: a.Name, HitchID: a.ID, Status: a.Status, Activity: a.Activity, Collar: a.Collar, Pane: a.Pane, ProcessAvailable: !limited, Evidence: a.Evidence}
	if a.Compaction != nil {
		row.Compaction = &compactionJSON{ID: a.Compaction.ID, Status: a.Compaction.Status, Reason: a.Compaction.Reason}
	}
	return row, nil
}
func (cmd command) status(args []string) error {
	why, machine := false, false
	flags := boundFlagSet("status", map[string]any{"why": &why, "json": &machine})
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
	if machine {
		row, err := run.agentRow(a)
		if err != nil {
			return err
		}
		return writeJSON(cmd.stdout, row)
	}
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
