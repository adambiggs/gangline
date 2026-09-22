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

func (cmd command) hitch(args []string) (result error) {
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
	}
	now := cmd.now()
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(o.Name), Collar: o.Collar, Role: o.Role, Directory: dir, Status: core.Starting, Activity: core.Unknown, CreatedAt: now, ChangedAt: now, BootDeadline: now.Add(bootTimeout)}
	e := core.Envelope{ID: core.EnvelopeID(eid), Recipient: a.ID, To: a.Name, From: sender, Purpose: purpose, Message: core.Message{Text: message}, CreatedAt: now}
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
	l, err := run.team.CreateAgent(a)
	if errors.Is(err, store.ErrNameTaken) {
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
	b, err := cmd.tmux(run.settings)
	if err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()
	exists, err := b.SessionExists(ctx)
	if err != nil {
		return err
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
		return commandError{status: exitNative, text: fmt.Sprintf("%s needs attention in %s: %s", a.Name, a.Pane, startup.Prompt)}
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
	if err := deliveryResult(outcome); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", a.Name, a.Pane); err != nil {
		return err
	}
	return nil
}
func storedIdentity(i tmux.Identity) core.ProcessIdentity {
	return core.ProcessIdentity{PID: i.PID, Started: i.Started, Version: i.Version, UniqueID: i.UniqueID, BootID: i.BootID}
}
func nativeIdentity(i core.ProcessIdentity) tmux.Identity {
	return tmux.Identity{PID: i.PID, Started: i.Started, Version: i.Version, UniqueID: i.UniqueID, BootID: i.BootID}
}
func (cmd command) adopt(args []string) (result error) {
	if len(args) == 0 {
		return usageError("adopt: name required")
	}
	if err := validateAgentName(args[0]); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	collar := run.settings.Collar
	flags := quietFlagSet("adopt")
	flags.StringVar(&collar, "c", collar, "collar")
	flags.StringVar(&collar, "collar", collar, "collar")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return usageError("adopt: expected NAME -c COLLAR")
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
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(args[0]), Collar: collar, Directory: dir, Pane: pane, Status: core.Active, Activity: core.Idle, Process: storedIdentity(identity), CreatedAt: cmd.now(), ChangedAt: cmd.now()}
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
	if err := run.record(a, core.Event{Type: "adopted"}); err != nil {
		return err
	}
	return run.mark(a)
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
		return run.team.RemoveName(core.AgentName(name), id)
	} else if err != nil {
		return err
	}
	return run.drop(id)
}
func (run *runtime) drop(id core.HitchID) error {
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	l, err := p.LockAgent()
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
	return run.team.RemoveName(a.Name, a.ID)
}
