package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	if o.Resume != "" {
		if err := harness.ValidateResume(c, o.Resume); err != nil {
			return refuseError("%s", err)
		}
	}
	if o.Effort != "" && o.Model == "" {
		return usageError("hitch: --effort requires --model")
	}
	if o.Model != "" {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		catalog, err := harness.DiscoverModels(ctx, c)
		cancel()
		if err != nil {
			return err
		}
		if harness.ValidateModel(catalog, o.Model) == harness.ModelUnrecognized {
			return refuseError("model %q is not recognized", o.Model)
		}
		if o.Effort != "" && harness.ValidateEffort(catalog, o.Model, o.Effort) == harness.ModelUnrecognized {
			return refuseError("effort %q is not recognized", o.Effort)
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
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(o.Name), Collar: o.Collar, Role: o.Role, Directory: dir, Status: core.Starting, Activity: core.Unknown, CreatedAt: now, ChangedAt: now, BootDeadline: now.Add(bootTimeout)}
	e := core.Envelope{ID: core.EnvelopeID(eid), Token: token, Recipient: a.ID, To: a.Name, From: sender, Purpose: purpose, Message: core.Message{Text: message}, CreatedAt: now}
	if c.Options.RolePrompt == nil {
		e.Startup = startupSections(o.Name, brief)
	}
	if _, err := envelopeText(e); err != nil {
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
	if err := run.team.Create(); err != nil {
		return err
	}
	b, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()
	exists, err := b.SessionExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		windows, err := b.Windows(ctx)
		if err != nil {
			return err
		}
		agents, err := run.team.ListAgents()
		if err != nil {
			return err
		}
		registered := make(map[string]bool, len(agents))
		for _, agent := range agents {
			registered[agent.Pane] = true
		}
		for _, window := range windows {
			if !registered[string(window.Pane.ID)] && gangWindowTitle(window.Name) {
				return refuseError("unregistered pane %s (%s) in team %q; inspect it, adopt it with 'gang adopt NAME -c COLLAR', or close that exact pane before hitching", window.Pane.ID, window.Name, run.settings.Session)
			}
		}
	}
	l, err := run.team.CreateAgent(a)
	if errors.Is(err, store.ErrNameTaken) && supersede && !exists {
		l, err = run.supersedeStoppedClaim(ctx, b, a)
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
	_, schedulerErr := run.updateWatchdog("", false, false)
	defer func() { result = errors.Join(result, schedulerErr) }()
	if err := run.record(a, core.Event{Type: "hitch_claimed"}); err != nil {
		return err
	}
	if err := l.Paths.Publish(e); err != nil {
		return err
	}
	if err := run.record(a, core.Event{Type: "send_queued", Envelope: &e}); err != nil {
		return err
	}
	spec := launch.SpawnSpec(windowTitle(a), dir)
	for k, v := range map[string]string{"GANG_SESSION": run.settings.Session, "GANG_STATE_ROOT": run.settings.StateRoot, "GANG_COLLAR": o.Collar, "GANG_CONFIG_DIR": run.settings.ConfigDir, "GANGLINE_HITCH_ID": id} {
		spec.Env[k] = v
	}
	if run.settings.Socket != "" {
		spec.Env["GANG_TMUX_SOCKET"] = run.settings.Socket
	}
	if run.settings.CollarDir != "" {
		spec.Env["GANG_COLLARS"] = run.settings.CollarDir
	}
	var pane substrate.Pane
	if exists {
		pane, err = b.Spawn(ctx, spec)
	} else {
		pane, err = b.CreateSession(ctx, spec)
	}
	if err != nil {
		_ = run.apply(l, &a, core.Event{Type: "hitch_failed", Reason: err.Error()})
		return err
	}
	identity, err := b.Identity(ctx, pane.ID)
	if err != nil {
		return err
	}
	a.Process = storedIdentity(identity)
	if err := run.apply(l, &a, core.Event{Type: "hitch_spawned", Pane: string(pane.ID)}); err != nil {
		return err
	}
	if err := run.mark(a); err != nil {
		return err
	}
	startup, _, err := harness.AwaitStartup(ctx, b.Capture, pane.ID, c)
	if err != nil {
		return err
	}
	if startup.State != harness.StartupReady {
		if err := run.apply(l, &a, core.Event{Type: "hitch_blocked", Reason: startup.Prompt}); err != nil {
			return err
		}
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
	return core.ProcessIdentity{PID: i.PID, Started: i.Started, Version: i.Version, UniqueID: i.UniqueID, BootID: i.BootID}
}
func nativeIdentity(i core.ProcessIdentity) tmux.Identity {
	return tmux.Identity{PID: i.PID, Started: i.Started, Version: i.Version, UniqueID: i.UniqueID, BootID: i.BootID}
}
func (cmd command) adopt(args []string) (result error) {
	name, collar, err := parseAdopt(args)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if collar == "" {
		collar = run.settings.Collar
	}
	if _, err := loadCollar(collar, run.settings); err != nil {
		return err
	}
	pane := cmd.environment("TMUX_PANE")
	if pane == "" {
		return refuseError("adopt requires the current tmux pane")
	}
	dir, err := cmd.getwd()
	if err != nil {
		return err
	}
	id, err := randomID("hitch")
	if err != nil {
		return err
	}
	b, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	identity, err := b.Identity(context.Background(), substrate.PaneID(pane))
	if err != nil {
		return err
	}
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(name), Collar: collar, Directory: dir, Pane: pane, Status: core.Active, Activity: core.Idle, Process: storedIdentity(identity), CreatedAt: cmd.now(), ChangedAt: cmd.now()}
	if err := run.team.Create(); err != nil {
		return err
	}
	l, err := run.team.CreateAgent(a)
	if errors.Is(err, store.ErrNameTaken) {
		return refuseError("name %q is already claimed", a.Name)
	}
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, run.release(l)) }()
	_, schedulerErr := run.updateWatchdog("", false, false)
	defer func() { result = errors.Join(result, schedulerErr) }()
	if err := run.record(a, core.Event{Type: "adopted"}); err != nil {
		return err
	}
	return run.mark(a)
}

func parseAdopt(args []string) (string, string, error) {
	collar := ""
	flags := boundFlagSet("adopt", map[string]any{"c": &collar, "collar": &collar})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return "", "", usageError("adopt: %v", err)
	}
	if len(positionals) == 0 {
		return "", "", usageError("adopt: name required")
	}
	if len(positionals) > 1 {
		return "", "", usageError("adopt: unexpected argument %q", positionals[1])
	}
	if err := validateAgentName(positionals[0]); err != nil {
		return "", "", err
	}
	if collar == "" && (flagWasSet(flags, "c") || flagWasSet(flags, "collar")) {
		return "", "", usageError("adopt: collar must not be empty")
	}
	return positionals[0], collar, nil
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
	id, err := run.team.ResolveName(name)
	if err != nil {
		return err
	}
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	if _, err := p.Read(); errors.Is(err, os.ErrNotExist) {
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
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		return err
	}
	b, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	if a.Process.PID != 0 {
		var owned *tmux.Owned
		if len(a.Teardown) > 0 {
			ids := make([]tmux.Identity, len(a.Teardown))
			for i, p := range a.Teardown {
				ids[i] = nativeIdentity(p)
			}
			owned, err = tmux.AcquireRecorded(ids)
		} else {
			owned, err = b.AcquireTree(context.Background(), substrate.PaneID(a.Pane), nativeIdentity(a.Process))
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
		if err := b.RemovePane(ctx, substrate.PaneID(a.Pane), nativeIdentity(a.Process)); err != nil {
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
