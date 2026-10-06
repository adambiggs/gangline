package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
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
	"github.com/adambiggs/gangline/substrate/tmux"
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
		"split": &options.Split, "vertical": &options.Vertical,
	})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("up: %v", err)
	}
	flagArguments, _ := partitionOptions(flags, args)
	taskSupplied := false
	flags.Visit(func(option *flag.Flag) {
		taskSupplied = taskSupplied || option.Name == "t" || option.Name == "task"
	})
	if options.Resume != "" && !taskSupplied && !options.Stdin {
		flagArguments = append(flagArguments, "--task", "This is a resumed session. Re-read your brief and durable state, then continue your work or report that you are waiting for an assignment.")
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
func (cmd command) down(args []string) (err error) {
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
	// Self-teardown can lose the output reader; retain returned failures in
	// the team's audit so they remain observable after the pane ends.
	defer func() {
		if err != nil {
			err = errors.Join(err, run.team.Append(core.Event{Type: "down_failed", At: cmd.now(), Reason: err.Error()}))
		}
	}()
	if err := cmd.eachDownAgent(agents, func(a core.Agent) error { return agentFailure(a.Name, run.dropAgent(a.ID, true, "")) }); err != nil {
		return err
	}
	if err := run.disarmEmptyWatchdog(); err != nil {
		return err
	}
	return os.RemoveAll(run.team.Directory)
}

// Ending the caller's terminal can interrupt its in-flight tmux clients.
// Finish other agents first, while still attempting every drop on failure.
func (cmd command) eachDownAgent(agents []core.Agent, action func(core.Agent) error) error {
	var peers, own []core.Agent
	for _, a := range agents {
		if cmd.dropEndsOwnPane(a) {
			own = append(own, a)
		} else {
			peers = append(peers, a)
		}
	}
	err := eachAgent(peers, action)
	for _, a := range own {
		err = errors.Join(err, action(a))
	}
	return err
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

// agentFailure names the agent on every line of its failure, so a command
// acting on several agents says which one each line is about.
func agentFailure(name core.AgentName, err error) error {
	if err == nil {
		return nil
	}
	lines := errorLines(err)
	for i, line := range lines {
		lines[i] = string(name) + ": " + line
	}
	text := strings.Join(lines, "\n")
	if status := errorStatus(err); status != exitError {
		return commandError{status: status, text: text}
	}
	return errors.New(text)
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

// The agent record retains the notified deadline after inbox receipts expire.
func (run *runtime) noteCurfew(l *store.LockedAgent, a *core.Agent) error {
	if a.Status != core.Active {
		return nil
	}
	team, err := run.team.ReadTeam()
	if err != nil {
		return err
	}
	now := run.cmd.now()
	if team.Curfew.IsZero() || now.Before(team.Curfew) || a.CurfewNoticeDeadline.Equal(team.Curfew) {
		return nil
	}
	id := core.EnvelopeID(fmt.Sprintf("curfew-%d", team.Curfew.UnixNano()))
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	if err := run.publishOnce(l, a, core.Envelope{
		ID: id, Token: token, Recipient: a.ID, To: a.Name,
		From: core.Sender{Kind: core.SenderGangline, Name: "curfew"}, CreatedAt: now,
		Message: core.Message{Text: fmt.Sprintf("Team %s curfew %s has passed. Observed %s.", run.settings.Session, team.Curfew.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))},
	}); err != nil {
		return err
	}
	a.CurfewNoticeDeadline = team.Curfew
	return l.Save(*a)
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
	listedPanes, err := b.Panes(context.Background())
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
	panes := map[string]tmux.PaneInfo{}
	for _, w := range listedPanes {
		panes[string(w.Pane.ID)] = w
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
		// A new server reuses pane ids, so a listed id is the record's pane
		// only under the record's registration.
		pane, present := panes[current.Pane]
		present = present && pane.Registration == paneIdentity(current)
		unlisted := false
		if current.Pane != "" && !present && current.Status != core.Dropping {
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
					err = run.apply(l, &current, core.Event{Type: "hitch_failed", Reason: exitedUnlistedReason(current)})
				}
			case current.Status != core.Failed:
				err = run.apply(l, &current, core.Event{Type: "hitch_failed", Reason: "registered pane is absent from tmux"})
			}
			if err != nil {
				_ = run.unlock(l)
				return nil, err
			}
		}
		if present && current.Status == core.Active {
			c, err := loadCollar(current.Collar, run.settings)
			if err != nil {
				_ = run.unlock(l)
				return nil, err
			}
			if err := run.observeRosterStatus(l, &current, c); err != nil {
				_ = run.unlock(l)
				return nil, err
			}
		}
		agents[i] = current
		if unlisted && current.Status != core.Failed {
			agents[i].Activity, agents[i].Evidence = core.Unknown, "tmux lists no team session"
		}
		if present && pane.Title != paneTitle(current) {
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
		line := fmt.Sprintf("%-16s %-10s %-14s %s%s", a.Name, a.Status, activityLabel(row.Activity, row.BackgroundTasks), a.Collar, marker)
		if _, err := fmt.Fprintln(cmd.stdout, line+cmd.operatorText(rosterReason(a, rosterReasonLimit(cmd.stdout, line)))); err != nil {
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
	paneName := ""
	if a.Pane != "" {
		paneName = string(a.Name)
	}
	row := agentJSON{InputOutage: a.InputOutage, Name: a.Name, HitchID: a.ID, Status: a.Status, Activity: a.Activity, Collar: a.Collar, Pane: paneName, ProcessAvailable: !limited, Evidence: run.cmd.operatorText(a.Evidence)}
	if a.Compaction != nil {
		row.Compaction = &compactionJSON{ID: a.Compaction.ID, Status: a.Compaction.Status, Reason: run.cmd.operatorText(a.Compaction.Reason)}
	}
	tasks, err := run.backgroundTasks(a)
	if err != nil {
		return agentJSON{}, err
	}
	row.BackgroundTasks = tasks
	return row, nil
}

// backgroundTasks is the count of native background tasks an idle agent's
// last turn left pending; a busy agent shows none.
func (run *runtime) backgroundTasks(a core.Agent) (int, error) {
	if a.Activity != core.Idle {
		return 0, nil
	}
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return 0, err
	}
	background, ok, err := p.ReadBackground()
	if err != nil || !ok {
		return 0, err
	}
	return background.Tasks, nil
}

// activityLabel is an activity with the background tasks the agent left
// pending, as in "idle (2 bg)".
func activityLabel(activity core.Activity, tasks int) string {
	if tasks > 0 {
		return fmt.Sprintf("%s (%d bg)", activity, tasks)
	}
	return string(activity)
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
	tasks, err := run.backgroundTasks(a)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", a.Name, a.Status, activityLabel(a.Activity, tasks)); err != nil {
		return err
	}
	if why && a.Evidence != "" {
		if _, err := fmt.Fprintln(cmd.stdout, cmd.operatorText(a.Evidence)); err != nil {
			return err
		}
	}
	if why && a.InputOutage != nil {
		if _, err := fmt.Fprintf(cmd.stdout, "pending input could not be observed since %s; inspect the native pane (persistent outage: %t)\n", a.InputOutage.Since.UTC().Format(time.RFC3339), a.InputOutage.Escalated); err != nil {
			return err
		}
	}
	if why && a.Compaction != nil {
		_, err = fmt.Fprintf(cmd.stdout, "compaction %s: %s\n", a.Compaction.Status, cmd.operatorText(a.Compaction.Reason))
	}
	return err
}
