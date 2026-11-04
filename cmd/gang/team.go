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
		"context-bands": &options.ContextBands,
		"t":             &options.Task, "task": &options.Task,
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
	// The team lock also covers absent state: a repeated down must not create
	// a new log identity after the previous teardown retained the original.
	if _, err := os.Lstat(run.team.Directory); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	retainedLog, err := run.team.RetainLog()
	if err != nil {
		return fmt.Errorf("retain team log: %w", err)
	}
	if _, err := fmt.Fprintf(cmd.stdout, "retained team log: %s\n", retainedLog); err != nil {
		return err
	}
	if err := run.recordDown(len(agents), retainedLog); err != nil {
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
	observed, _, err := run.observeRosterPartial(agents, false)
	return observed, err
}

func (run *runtime) observeRosterPartial(agents []core.Agent, allowPartial bool) ([]core.Agent, []rosterError, error) {
	b, err := run.cmd.tmux(run.settings)
	if err != nil {
		return nil, nil, err
	}
	listedPanes, err := b.Panes(context.Background())
	listed := err == nil
	if err != nil {
		exists, checkErr := b.SessionExists(context.Background())
		if checkErr != nil {
			return nil, nil, checkErr
		}
		if exists {
			return nil, nil, err
		}
	}
	panes := map[string]tmux.PaneInfo{}
	for _, w := range listedPanes {
		panes[string(w.Pane.ID)] = w
	}
	observed := make([]core.Agent, 0, len(agents))
	diagnostics := []rosterError{}
	for _, a := range agents {
		l, current, err := run.acquire(a.ID, false)
		if errors.Is(err, store.ErrLocked) {
			// The operation holding the lock maintains the record, so the row
			// is the record as saved, with its deadlines read against now.
			saved, _ := core.Step(a, core.Event{Type: "deadline_checked", At: run.cmd.now(), HitchID: a.ID})
			observed = append(observed, saved)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if err := run.checkDeadlines(l, &current); err != nil {
			_ = run.unlock(l)
			return nil, nil, err
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
				return nil, nil, err
			}
		}
		if present && current.Status == core.Active {
			c, err := loadCollar(current.Collar, run.settings)
			if err != nil {
				if closeErr := run.unlock(l); closeErr != nil {
					return nil, nil, errors.Join(err, closeErr)
				}
				if !allowPartial {
					return nil, nil, err
				}
				diagnostics = append(diagnostics, rosterError{
					Kind: "native", HitchID: &current.ID, Name: &current.Name, Stage: "collar_load",
				})
				continue
			}
			if err := run.observeRosterStatus(l, &current, c); err != nil {
				_ = run.unlock(l)
				return nil, nil, err
			}
		}
		if unlisted && current.Status != core.Failed {
			current.Activity, current.Evidence = core.Unknown, "tmux lists no team session"
		}
		if present && pane.Title != paneTitle(current) {
			if err := run.mark(current); err != nil {
				_ = run.unlock(l)
				return nil, nil, err
			}
		}
		if err := run.release(l); err != nil {
			return nil, nil, err
		}
		observed = append(observed, current)
	}
	return observed, diagnostics, nil
}
func (cmd command) roster(args []string) error {
	machine, allowPartial := false, false
	page, positionals, err := parseListOptions("roster", args, map[string]any{"json": &machine, "allow-partial": &allowPartial})
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
	var agents []core.Agent
	failedReads := 0
	if allowPartial {
		agents, failedReads, err = run.team.ListAgentsForRoster()
	} else {
		agents, err = run.team.ListAgents()
	}
	if err != nil {
		return err
	}
	agents, diagnostics, err := run.observeRosterPartial(agents, allowPartial)
	if err != nil {
		return err
	}
	for range failedReads {
		diagnostics = append(diagnostics, rosterError{Kind: "native", Stage: "state_read"})
	}
	watchdogLimited := false
	if _, err := os.Stat(filepath.Join(run.team.Directory, "watchdog-unavailable")); err == nil {
		watchdogLimited = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	out := rosterJSON{WatchdogAvailable: !watchdogLimited, Agents: []agentJSON{}}
	var lines []string
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
		line := fmt.Sprintf("%-16s %-10s %-18s %s%s", a.Name, a.Status, activityLabel(row.Activity, row.BackgroundTasks, row.BackgroundSince, run.cmd.now()), a.Collar, marker)
		lines = append(lines, line+cmd.operatorText(rosterReason(a, rosterReasonLimit(cmd.stdout, line))))
	}
	outside, err := run.team.ListOutside()
	if err != nil {
		return err
	}
	for _, registration := range outside {
		if machine {
			out.Agents = append(out.Agents, agentJSON{Kind: "outside", Name: registration.Name, HitchID: registration.ID, Status: core.Active})
			continue
		}
		lines = append(lines, fmt.Sprintf("%-16s %-10s mailbox            outside", registration.Name, "registered"))
	}
	if machine {
		out.Total = len(out.Agents)
		out.Agents = out.Agents[:page.count(out.Total)]
		out.Truncated = len(out.Agents) < out.Total
		var result any = out
		if allowPartial {
			result = partialRosterJSON{rosterJSON: out, Partial: len(diagnostics) > 0, Errors: diagnostics}
		}
		if err := writeJSON(cmd.stdout, result); err != nil {
			return err
		}
		return page.notice(cmd.stderr, out.Total)
	}
	if err := page.writeLines(cmd.stdout, cmd.stderr, lines); err != nil {
		return err
	}
	if len(diagnostics) > 0 {
		if _, err := fmt.Fprintln(cmd.stdout, "Agent inventory incomplete; some native entries could not be read:"); err != nil {
			return err
		}
		for _, diagnostic := range diagnostics {
			if _, err := fmt.Fprintln(cmd.stdout, "  native: "+diagnostic.Stage); err != nil {
				return err
			}
		}
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
	available, err := run.processAvailable(a)
	if err != nil {
		return agentJSON{}, err
	}
	paneName := ""
	if a.Pane != "" {
		paneName = string(a.Name)
	}
	row := agentJSON{Kind: "native", InputOutage: a.InputOutage, Name: a.Name, HitchID: a.ID, Status: a.Status, Activity: a.Activity, Collar: a.Collar, Pane: paneName, ProcessAvailable: available, Evidence: run.cmd.operatorText(a.Evidence)}
	row.ConfiguredModel, row.ConfiguredEffort = optionalModelValue(a.ConfiguredModel), optionalModelValue(a.ConfiguredEffort)
	row.ObservedModel, row.ObservedEffort = optionalModelValue(a.Native.Model), optionalModelValue(a.Native.Effort)
	if a.Native.Context.Source == "status-line" {
		// Claude reports its model in the existing status-line payload but
		// does not report effort there. Screen labels are not native IDs.
		row.ObservedModel, row.ObservedEffort = optionalModelValue(a.Native.Context.Model), nil
	}
	if a.Compaction != nil {
		row.Compaction = &compactionJSON{ID: a.Compaction.ID, Status: a.Compaction.Status, Reason: run.cmd.operatorText(a.Compaction.Reason)}
	}
	background, err := run.background(a)
	if err != nil {
		return agentJSON{}, err
	}
	if background.Tasks > 0 {
		row.BackgroundTasks, row.BackgroundSince = background.Tasks, &background.At
	}
	return row, nil
}

func optionalModelValue(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// background returns an idle agent's stored background report and its receipt
// time, without certifying current task state; a busy agent shows none.
func (run *runtime) background(a core.Agent) (store.Background, error) {
	if a.Activity != core.Idle {
		return store.Background{}, nil
	}
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return store.Background{}, err
	}
	background, _, err := p.ReadBackground()
	return background, err
}

// activityLabel adds the background report count and age to an activity, as in
// "idle (2 bg 1h05m)". The age is not the duration of verified idle or pending work.
func activityLabel(activity core.Activity, tasks int, since *time.Time, now time.Time) string {
	if tasks > 0 && since != nil {
		return fmt.Sprintf("%s (%d bg %s)", activity, tasks, idleDuration(now.Sub(*since)))
	}
	return string(activity)
}

// idleDuration is a duration in whole minutes, as in "1h05m" or "12m".
func idleDuration(d time.Duration) string {
	minutes := int(d / time.Minute)
	if minutes < 1 {
		return "<1m"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
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
	background, err := run.background(a)
	if err != nil {
		return err
	}
	now := cmd.now()
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%s\n", a.Name, a.Status, activityLabel(a.Activity, background.Tasks, &background.At, now)); err != nil {
		return err
	}
	if why && a.Evidence != "" {
		if _, err := fmt.Fprintln(cmd.stdout, cmd.operatorText(a.Evidence)); err != nil {
			return err
		}
	}
	if why && background.Tasks > 0 {
		if _, err := fmt.Fprintf(cmd.stdout, "background report received at %s (%s ago): %s; current task state is unknown\n", background.At.UTC().Format(time.RFC3339), idleDuration(now.Sub(background.At)), backgroundTaskCount(background.Tasks)); err != nil {
			return err
		}
	}
	if why && a.InputOutage != nil {
		if _, err := fmt.Fprintf(cmd.stdout, "pending input could not be observed since %s; inspect the native pane (persistent outage: %t)\n", a.InputOutage.Since.UTC().Format(time.RFC3339), a.InputOutage.Escalated); err != nil {
			return err
		}
	}
	if a.Clear != nil {
		if _, err := fmt.Fprintf(cmd.stdout, "clear %s (previous native %s): %s\n", a.Clear.Status, a.Clear.PreviousSession, cmd.operatorText(a.Clear.Reason)); err != nil {
			return err
		}
	}
	if why && a.Compaction != nil {
		if _, err := fmt.Fprintf(cmd.stdout, "compaction %s: %s\n", a.Compaction.Status, cmd.operatorText(a.Compaction.Reason)); err != nil {
			return err
		}
	}
	if why {
		path, err := run.team.ForeignSubmitPath(a.ID)
		if err != nil {
			return err
		}
		foreign, err := run.team.ReadForeignSubmit(a.ID)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			_, err = fmt.Fprintf(cmd.stdout, "ignored submit record at %s is unreadable: %s\n", path, cmd.operatorText(err.Error()))
			return err
		}
		_, err = fmt.Fprintf(cmd.stdout, "ignored at %s: %s; raw payload: %s\n", foreign.At.UTC().Format(time.RFC3339), cmd.operatorText(foreign.Reason), path)
		return err
	}
	return nil
}
