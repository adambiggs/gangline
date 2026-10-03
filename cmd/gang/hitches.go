package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

func (cmd command) hitch(args []string) error {
	return cmd.hitchWithStaleClaim(args, false)
}

func (cmd command) hitchWithStaleClaim(args []string, supersede bool) (result error) {
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	dir, err := cmd.getwd()
	if err != nil {
		return err
	}
	o, err := parseHitch(args, run.settings.Collar, dir)
	if err != nil {
		return err
	}
	if o.Recover {
		return run.recoverStartup(o.Name)
	}
	dir, err = filepath.Abs(o.Directory)
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return refuseError("hitch directory %q is not accessible", dir)
	}
	c, err := loadCollar(o.Collar, run.settings)
	if err != nil {
		return err
	}
	o.Collar = c.Name
	if o.Resume != "" {
		unverified, err := harness.ValidateResume(c, o.Resume)
		if err != nil {
			return refuseError("%s", err)
		}
		if unverified != "" {
			if _, err := fmt.Fprintf(cmd.stderr, "warning: hitch: resume session %q left to the native CLI: %s\n", o.Resume, unverified); err != nil {
				return err
			}
		}
	}
	if o.Effort != "" && o.Model == "" {
		return usageError("hitch: --effort requires --model")
	}
	if o.Effort != "" {
		// The native CLI judges model ids; only an effort the catalog omits
		// for a model it lists is refused here.
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		catalog, err := harness.DiscoverModels(ctx, c)
		cancel()
		if err != nil {
			if _, err := fmt.Fprintf(cmd.stderr, "warning: hitch: effort %q left to the native CLI: %v\n", o.Effort, err); err != nil {
				return err
			}
		} else if harness.ValidateEffort(catalog, o.Model, o.Effort) == harness.ModelUnrecognized {
			return refuseError("effort %q is not listed for model %q (%s)", o.Effort, o.Model, strings.Join(harness.ModelEfforts(catalog, o.Model), ", "))
		}
	}
	assignment := o.Task
	if o.Stdin {
		assignment, err = readBody(cmd.stdin)
		if err != nil {
			return err
		}
	}
	brief, err := cmd.startupProse(o.Role)
	if err != nil {
		return err
	}
	id, err := randomID("hitch")
	if err != nil {
		return err
	}
	run.hitching = core.HitchID(id)
	eid, err := randomID("startup")
	if err != nil {
		return err
	}
	token, err := randomEnvelopeToken()
	if err != nil {
		return err
	}
	sender, err := run.observedSender()
	if err != nil {
		return err
	}
	hitcher, err := run.hitcherID()
	if err != nil {
		return err
	}
	if sender.Kind == "" {
		sender = core.Sender{Kind: core.SenderGangline, Name: "hitch"}
	}
	rolePrompt, message := startupMessages(o.Name, brief, assignment, c.Options.RolePrompt != nil)
	purpose := "startup"
	if assignment != "" {
		purpose = "assignment"
	} else {
		sender = core.Sender{Kind: core.SenderGangline, Name: "startup"}
	}
	now := cmd.now()
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(o.Name), Collar: o.Collar, Role: o.Role, HitchedBy: hitcher, Directory: dir, Status: core.Starting, Activity: core.Unknown, CreatedAt: now, ChangedAt: now, BootDeadline: now.Add(bootTimeout)}
	a.Native.SessionID = o.Resume
	e := core.Envelope{ID: core.EnvelopeID(eid), Token: token, Recipient: a.ID, To: a.Name, From: sender, Purpose: purpose, Message: core.Message{Text: message}, CreatedAt: now}
	if c.Options.RolePrompt == nil {
		e.Startup = startupSections(o.Name, brief)
	}
	wire, err := envelopeText(e)
	if err != nil {
		return err
	}
	if err := refusePasteHazard(c, wire); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	launch, err := harness.RenderLaunch(c, harness.LaunchOptions{ResumeSession: o.Resume, HookCommand: []string{exe, "hook"}, HookTimeoutSeconds: boundaryHookTimeoutSeconds, Model: o.Model, Effort: o.Effort, RolePrompt: rolePrompt})
	if err != nil {
		return err
	}
	launch = applyLaunchPolicy(launch, o.Collar, run.settings)
	if filepath.Base(launch.Name) == "codex" {
		launch.Args, err = codexProfileLaunch(launch.Args, run.settings.CodexPermissionProfile, linkedWorktreeGitdir(dir))
		if err != nil {
			return refuseError("%v", err)
		}
	}
	if _, err := exec.LookPath(launch.Name); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return refuseError("%s: not found in PATH", launch.Name)
		}
		return fmt.Errorf("find %s: %w", launch.Name, err)
	}
	lock, err := run.lockTeam()
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := run.team.Create(); err != nil {
		return err
	}
	b, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	boot, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()
	exists, err := b.SessionExists(boot)
	if err != nil {
		return err
	}
	if exists {
		windows, err := b.Windows(boot)
		if err != nil {
			return err
		}
		agents, err := run.team.ListAgents()
		if err != nil {
			return err
		}
		owners := make(map[string][]core.Agent, len(agents))
		for _, agent := range agents {
			if agent.Pane != "" {
				owners[agent.Pane] = append(owners[agent.Pane], agent)
			}
		}
		for _, window := range windows {
			if !gangWindowTitle(window.Name) {
				continue
			}
			owned, err := ownedPane(boot, b, owners[string(window.Pane.ID)])
			if err != nil {
				return err
			}
			if !owned {
				return refuseError("unregistered pane %s (%s) in team %q; inspect it and close that exact pane before hitching", window.Pane.ID, window.Name, run.settings.Session)
			}
		}
	}
	// From the claim until startup is settled, SIGINT and SIGTERM sent to gang
	// cancel ctx, and the hitch fails the record and removes its pane. Calls
	// that create or replace a pane run on boot, so such a signal never loses a
	// pane's id; one sent to gang's whole process group also ends the tmux
	// client mid-command.
	ctx, stopInterrupts := interruptible(boot)
	defer stopInterrupts()
	l, err := run.team.CreateAgent(a)
	if errors.Is(err, store.ErrNameTaken) && supersede && !exists {
		l, err = run.supersedeStoppedClaim(boot, b, a)
	}
	if errors.Is(err, store.ErrNameTaken) {
		if !supersede && !exists {
			return refuseError("agent name %q is already claimed; restart the stopped team with 'gang up %s'", o.Name, o.Name)
		}
		return refuseError("agent name %q is already claimed", o.Name)
	}
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	if err := run.record(a, core.Event{Type: "hitch_claimed"}); err != nil {
		return err
	}
	if err := l.Paths.Publish(e); err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "send_queued", Envelope: &e}); err != nil {
		return err
	}
	agentToken, err := randomID("pane")
	if err != nil {
		return err
	}
	spec := launch.SpawnSpec(windowTitle(a), dir)
	spec.Env["GANG_AGENT_NONCE"] = agentToken
	// Hold the pane open until startup is observed, so a native CLI that
	// exits at boot leaves its status and final output for the hitch error.
	spec.KeepExited = true
	spec.HoldLog = filepath.Join(l.Paths.Directory, "hold")
	defer func() { _ = os.Remove(spec.HoldLog) }()
	// A pane that could not hold itself closes before the native CLI starts, so
	// the error that finds it gone carries what the hold printed.
	holdFailed := func(err error) (error, bool) {
		if why := holdFailure(spec.HoldLog); err != nil && why != "" {
			return fmt.Errorf("%w: pane hold failed: %s", err, why), true
		}
		return err, false
	}
	for k, v := range map[string]string{"GANG_SESSION": run.settings.Session, "GANG_STATE_ROOT": run.settings.StateRoot, "GANG_COLLAR": o.Collar, "GANG_CONFIG_DIR": run.settings.ConfigDir, "GANG_AGENT_ID": id, "GANGLINE_HITCH_ID": id} {
		spec.Env[k] = v
	}
	if run.settings.Socket != "" {
		spec.Env["GANG_TMUX_SOCKET"] = run.settings.Socket
	}
	if run.settings.CollarDir != "" {
		spec.Env["GANG_COLLARS"] = run.settings.CollarDir
	}
	registered := false
	fail := func(err error, reason string) error {
		registered = false
		return errors.Join(err, run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: reason}))
	}
	// reason names why the hitch stopped: the signal that interrupted it, the
	// native CLI's exit with its output, or both. It is empty for any other
	// error.
	reason := func(err error) (error, string) {
		var exited *substrate.ExitedError
		native := errors.As(err, &exited)
		var interrupt interruptError
		if errors.As(context.Cause(ctx), &interrupt) {
			if native {
				return errors.Join(interrupt, err), interrupt.Error() + ": " + exited.Error()
			}
			// The interrupt cancels the call in flight, so that call's error
			// (a cancelled context, or a tmux client killed with it) is its
			// consequence and adds nothing to it.
			return interrupt, interrupt.Error()
		}
		if native {
			return err, exited.Error()
		}
		if err, failed := holdFailed(err); failed {
			return err, err.Error()
		}
		return err, ""
	}
	if err, why := reason(nil); why != "" {
		return fail(err, why)
	}
	// Creation and registration ignore the interrupt: a tmux client killed
	// mid-command can leave a window whose pane id gang never learns.
	var pane substrate.Pane
	if exists {
		pane, err = b.Spawn(boot, spec)
	} else {
		pane, err = b.CreateSession(boot, spec)
	}
	if err != nil {
		_ = run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: err.Error()})
		return err
	}
	registration, err := b.RegisterPane(boot, pane.ID)
	if err != nil {
		err, _ = holdFailed(err)
		// Signals stay absorbed until this removal returns, so it is bounded.
		removal, cancel := context.WithTimeout(context.Background(), operationTimeout)
		cleanupErr := b.KillUnregisteredPane(removal, pane.ID)
		cancel()
		return errors.Join(err, cleanupErr, run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: err.Error()}))
	}
	a.Registration = core.PaneRegistration{Generation: registration.Generation, Session: registration.Session, TokenHash: tokenHash(agentToken), Held: true}
	exitedUnread := false
	defer func() {
		if registered {
			return
		}
		// Signals stay absorbed until this removal returns, so it is bounded.
		removal, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		if err := b.RemoveRegisteredPane(removal, registration); err != nil {
			result = errors.Join(result, err)
			return
		}
		// The record keeps a pane only while it exists, so tick and drop never
		// address a removed one. Its registration goes with the pane when the
		// native process was seen to exit before its identity was read: no
		// process is left whose cleanup drop could skip.
		if a.Pane != "" {
			a.Pane, a.Registration.Held = "", false
			if exitedUnread {
				a.Registration = core.PaneRegistration{}
			}
			result = errors.Join(result, l.Save(a))
		}
	}()
	// The record owns the pane from registration on, so a hitch killed before
	// startup leaves a pane that tick probes and drop removes.
	if err := run.apply(l, &a, core.Event{Type: "hitch_spawned", Pane: string(pane.ID)}); err != nil {
		return err
	}
	// A native exit fails the hitch with the native output as its reason and
	// removes the held pane.
	nativeExit := func(err error) error {
		var exited *substrate.ExitedError
		if !errors.As(err, &exited) {
			return err
		}
		return fail(err, exited.Error())
	}
	// A failure before the process identity is read leaves no pane worth
	// keeping.
	unspawned := func(err error) error {
		err, why := reason(err)
		if why == "" {
			why = err.Error()
		}
		exitedUnread = errors.As(err, new(*substrate.ExitedError))
		return fail(err, why)
	}
	// Hitch releases the hold on every path that keeps the pane, so a later
	// exit closes it, except a startup blocked on a prompt: an answer there
	// can end the native process, and tick reads that exit from the held pane
	// or releases it once startup is ready. The release outlives an expired
	// boot context, and an exit it finds still fails the hitch. The record
	// says the pane is held until the release, so tick ends a hold that a
	// hitch stopped short of.
	blocked := false
	var schedulerErr error
	defer func() { result = errors.Join(result, schedulerErr) }()
	defer func() {
		if !registered || blocked {
			return
		}
		// Startup delivery saves the record and closes its lock, so the
		// release reads the record again under the lock. A record dropped
		// since then took its pane with it.
		if !l.Held() {
			fresh, err := l.Paths.LockAgent()
			if err == nil {
				l = fresh
				a, err = l.Paths.Read()
			}
			if errors.Is(err, os.ErrNotExist) {
				return
			}
			if err != nil {
				result = errors.Join(result, err)
				return
			}
		}
		if !a.Registration.Held || a.Pane != string(pane.ID) {
			return
		}
		release, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		if err := nativeExit(b.ReleaseExit(release, pane.ID)); err != nil {
			// A native exit is the hitch's result: whatever the hitch said of
			// the pane before finding it describes a pane that is removed.
			if errors.As(err, new(*substrate.ExitedError)) {
				result = err
				return
			}
			result = errors.Join(result, err)
			return
		}
		a.Registration.Held = false
		result = errors.Join(result, l.Save(a))
	}()
	// The server's process is witnessed with the registration, so a later
	// reader can tell that server's exit from a socket it cannot reach.
	server, err := b.ServerIdentity(ctx, registration)
	if err != nil {
		return unspawned(err)
	}
	a.Registration.Server = storedIdentity(server)
	if err := l.Save(a); err != nil {
		return unspawned(err)
	}
	visible, err := b.ProcessVisibility(ctx, pane.ID)
	if err != nil {
		return unspawned(err)
	}
	if visible {
		identity, err := b.Identity(ctx, pane.ID)
		if err != nil {
			return unspawned(err)
		}
		a.Process = storedIdentity(identity)
		if err := l.Save(a); err != nil {
			return unspawned(err)
		}
	} else if err := run.noteProcessUnavailable(a); err != nil {
		return unspawned(err)
	}
	registered = true
	schedulerErr = run.ensureWatchdog()
	if err := run.mark(a); err != nil {
		return err
	}
	startup, _, err := harness.AwaitStartup(ctx, b.Capture, pane.ID, c)
	if err != nil {
		if err, why := reason(err); why != "" {
			return fail(err, why)
		}
		return err
	}
	stopInterrupts()
	if startup.State != harness.StartupReady {
		if err := run.apply(l, &a, core.Event{Type: "hitch_blocked", Reason: startup.Prompt}); err != nil {
			return err
		}
		blocked = true
		if err := run.mark(a); err != nil {
			return err
		}
		return startupAttention(a)
	}
	if err := run.apply(l, &a, core.Event{Type: "hitch_ready"}); err != nil {
		return err
	}
	if err := run.mark(a); err != nil {
		return err
	}
	outcome, err := run.drainFrom(l, a, e.ID)
	if err != nil {
		return err
	}
	if outcome == "queued" {
		return startupAttention(a)
	}
	if err := deliveryResult(outcome); err != nil {
		return commandError{status: exitUnknown, text: fmt.Sprintf("startup input is unverified; inspect %s, resolve native prompts, then run gang hitch %s --recover; do not replace the contract with plain send", a.Pane, a.Name)}
	}
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", a.Name, a.Pane); err != nil {
		return err
	}
	return nil
}

// holdFailureBytes bounds how much of a failed hold's output an error carries.
const holdFailureBytes = 512

// holdFailure reads what a pane's hold left in its log as one line. It is
// empty when the hold succeeded or left nothing.
func holdFailure(log string) string {
	data, err := os.ReadFile(log)
	if err != nil {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	text := strings.Join(lines, "; ")
	if len(text) > holdFailureBytes {
		text = text[len(text)-holdFailureBytes:]
		for text != "" && !utf8.RuneStart(text[0]) {
			text = text[1:]
		}
	}
	return text
}

// interruptError is the cause of a hitch cancelled by a signal.
type interruptError struct{ signal os.Signal }

func (e interruptError) Error() string {
	name := e.signal.String()
	switch e.signal {
	case os.Interrupt:
		name = "SIGINT"
	case syscall.SIGTERM:
		name = "SIGTERM"
	}
	return "hitch interrupted by " + name
}

// interruptible returns a context that the first SIGINT or SIGTERM cancels
// with an interruptError. Later signals are absorbed until stop, so the hitch
// records the interrupt and removes its pane; stop restores the inherited
// action. A SIGINT gang started with ignored stays ignored.
func interruptible(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	// The Go runtime keeps an inherited ignored SIGINT ignored, as a
	// non-interactive shell starts a background command; Notify would undo it.
	if !signal.Ignored(os.Interrupt) {
		signal.Notify(signals, os.Interrupt)
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case received := <-signals:
				// The first cause wins; a repeat only keeps gang alive.
				cancel(interruptError{received})
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			signal.Stop(signals)
			close(done)
		})
	}
}

// ownedPane reports whether any of the records naming a pane id owns that
// pane now. A record keeps its pane id across a tmux server restart, where
// the same id can name a pane no record created, so ownership is the
// record's registered server generation and session, not the id alone.
func ownedPane(ctx context.Context, b paneRegistry, records []core.Agent) (bool, error) {
	for _, a := range records {
		if a.Registration.Generation == "" || a.Registration.Session == "" {
			continue
		}
		present, err := b.CheckPane(ctx, paneIdentity(a))
		if errors.Is(err, tmux.ErrPaneReplaced) {
			continue
		}
		if err != nil {
			return false, err
		}
		if present {
			return true, nil
		}
	}
	return false, nil
}

func gangWindowTitle(name string) bool {
	if len(name) < 3 || name[0] != name[len(name)-1] {
		return false
	}
	switch name[0] {
	case '?', '~', '-', '!':
		return true
	}
	return false
}
func storedIdentity(i tmux.Identity) core.ProcessIdentity {
	return core.ProcessIdentity{PID: i.PID, Started: i.Started, Version: i.Version, UniqueID: i.UniqueID, BootID: i.BootID, Namespace: i.Namespace}
}
func nativeIdentity(i core.ProcessIdentity) tmux.Identity {
	return tmux.Identity{PID: i.PID, Started: i.Started, Version: i.Version, UniqueID: i.UniqueID, BootID: i.BootID, Namespace: i.Namespace}
}
func (cmd command) rename(args []string) (result error) {
	if err := exactly(args, 2, "rename"); err != nil {
		return err
	}
	if err := validateAgentName(args[1]); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(args[0])
	if err != nil {
		return err
	}
	l, a, err := run.acquire(a.ID, false)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	if a.Name == core.AgentName(args[1]) {
		return nil
	}
	a.RenameFrom, a.RenameTo = a.Name, core.AgentName(args[1])
	if err := l.Save(a); err != nil {
		return err
	}
	if err := run.team.FinishRename(l, &a); err != nil {
		if errors.Is(err, store.ErrNameTaken) {
			a.RenameFrom, a.RenameTo = "", ""
			if saveErr := l.Save(a); saveErr != nil {
				return saveErr
			}
			return refuseError("name %q is already claimed", args[1])
		}
		return err
	}
	if err := run.record(a, core.Event{Type: "renamed"}); err != nil {
		return err
	}
	return run.mark(a)
}
func (cmd command) drop(args []string) error {
	name, err := requiredName(args, "drop")
	if err != nil {
		return err
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
	id, err := run.team.ResolveName(name)
	if errors.Is(err, os.ErrNotExist) {
		return run.unregistered(name)
	}
	if err != nil {
		return err
	}
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	target, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		if err := run.team.RemoveName(core.AgentName(name), id); err != nil {
			return err
		}
		if err := run.disarmEmptyWatchdog(); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.stdout, "%s registration removed; native state missing; resume session: unknown\n", name)
		return err
	} else if err != nil {
		return err
	}
	if err := run.refuseDrop(target); err != nil {
		return err
	}
	return run.drop(id)
}
func (run *runtime) drop(id core.HitchID) error {
	return run.dropWithLock(id, true)
}
func (run *runtime) dropWithLock(id core.HitchID, wait bool) error {
	if err := run.dropAgent(id, wait); err != nil {
		return err
	}
	return run.disarmEmptyWatchdog()
}
func (run *runtime) dropAgent(id core.HitchID, wait bool) error {
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	var l *store.LockedAgent
	if wait {
		l, err = p.LockAgent()
	} else {
		l, err = p.TryLock()
	}
	if errors.Is(err, store.ErrLocked) && !wait {
		return nil
	}
	if err != nil {
		return err
	}
	defer run.unlock(l)
	a, err := p.Read()
	if err != nil {
		return err
	}
	registry, err := run.registry()
	if err != nil {
		return err
	}
	visible := false
	replaced := false
	exited := false
	if a.Pane != "" {
		if a.Registration.Generation == "" || a.Registration.Session == "" || a.Registration.TokenHash == "" {
			return refuseError("refuse teardown: incomplete pane registration; inspect retained state at %s", p.State)
		}
		present, checkErr := registry.CheckPane(context.Background(), paneIdentity(a))
		if errors.Is(checkErr, tmux.ErrPaneReplaced) {
			replaced = true
			present = false
		} else if checkErr != nil {
			return checkErr
		}
		if present {
			visible, err = registry.ProcessVisibility(context.Background(), substrate.PaneID(a.Pane))
		}
		// A pane held from boot outlives its native process, which leaves
		// nothing to tear down but the pane.
		if exit := (*substrate.ExitedError)(nil); errors.As(err, &exit) {
			exited, err = true, nil
		}
		if err != nil {
			return err
		}
	}
	// Cleanup is skipped when gang cannot see into a registered pane, open or
	// closed, or read its recorded process. A claim without a registration
	// either started no native process or saw it exit at boot, and leaves
	// nothing to clean up.
	if !visible && !exited && (a.Registration.Generation != "" || a.Process.PID != 0) && !tmux.CanReadIdentity(nativeIdentity(a.Process)) {
		if err := run.noteProcessUnavailable(a); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(run.cmd.stderr, "warning: host process visibility unavailable; detached-descendant cleanup skipped"); err != nil {
			return err
		}
	}
	if visible && a.Process.PID == 0 && a.Registration.Generation != "" {
		identity, err := registry.Identity(context.Background(), substrate.PaneID(a.Pane))
		// A native process that exited since its visibility was read records
		// no identity and leaves nothing to tear down but the pane. An unheld
		// pane closes with it, so tmux answers the read for a missing pane.
		if err == nil {
			a.Process = storedIdentity(identity)
		} else if !errors.As(err, new(*substrate.ExitedError)) {
			present, checkErr := registry.CheckPane(context.Background(), paneIdentity(a))
			if errors.Is(checkErr, tmux.ErrPaneReplaced) {
				replaced = true
			} else if checkErr != nil {
				return checkErr
			} else if present {
				return err
			}
		}
	}
	nativeVisible := visible || tmux.CanReadIdentity(nativeIdentity(a.Process))
	if nativeVisible && a.Process.PID != 0 {
		var owned *tmux.Owned
		if len(a.Teardown) > 0 {
			ids := make([]tmux.Identity, len(a.Teardown))
			for i, p := range a.Teardown {
				ids[i] = nativeIdentity(p)
			}
			owned, err = tmux.AcquireRecorded(ids)
		} else if !visible {
			owned, err = tmux.AcquireRecorded([]tmux.Identity{nativeIdentity(a.Process)})
		} else {
			owned, err = registry.AcquireTree(context.Background(), substrate.PaneID(a.Pane), nativeIdentity(a.Process))
			if err == nil {
				for _, p := range owned.Identities() {
					a.Teardown = append(a.Teardown, storedIdentity(p))
				}
				err = l.Save(a)
			}
		}
		if err != nil {
			if owned != nil {
				_ = owned.Close()
			}
			return err
		}
		defer owned.Close()
		// Teardown has its own fresh budget. Stored operation deadlines cannot stop it.
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		if err := owned.Stop(ctx); err != nil {
			return err
		}
	}
	if a.Pane != "" && !replaced {
		if nativeVisible && a.Process.PID != 0 {
			err = registry.RemoveRegisteredNativePane(context.Background(), paneIdentity(a), nativeIdentity(a.Process))
		} else {
			err = registry.RemoveRegisteredPane(context.Background(), paneIdentity(a))
		}
		if err != nil {
			return err
		}
	}
	if a.Status != core.Dropping {
		if err := l.CleanResult(&a); err != nil {
			return err
		}
		if err := run.recoverInput(l, &a); err != nil {
			return err
		}
	}
	if err := run.apply(l, &a, core.Event{Type: "drop_started"}); err != nil {
		return err
	}
	pending, err := l.SealInbox()
	if err != nil {
		return err
	}
	for _, e := range pending {
		from, _ := p.EnvelopePath("tmp/drop", e.ID)
		to, _ := p.EnvelopePath("failed", e.ID)
		if err := run.record(a, core.Event{Type: "delivery_failed", ID: string(e.ID), Reason: "recipient was dropped"}); err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
		if err := run.notifySender(a, e, "dropped"); err != nil {
			return err
		}
	}
	// A submit hook can have recorded identity after the last saved state.
	witness, err := p.ReadWitness()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	session := a.Native.SessionID
	if session == "" && witness.SessionID != "" {
		session = witness.SessionID
	}
	if session == "" {
		session = "unknown"
	}
	if err := run.record(a, core.Event{Type: "drop_finished"}); err != nil {
		return err
	}
	for _, name := range []core.AgentName{a.RenameFrom, a.RenameTo} {
		if name != "" && name != a.Name {
			if err := run.team.RemoveName(name, a.ID); err != nil {
				return err
			}
		}
	}
	if err := os.RemoveAll(p.Directory); err != nil {
		return err
	}
	if err := run.team.RemoveName(a.Name, a.ID); err != nil {
		return err
	}
	_, err = fmt.Fprintf(run.cmd.stdout, "%s dropped; resume session: %s\n", a.Name, session)
	return err
}
