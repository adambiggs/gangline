package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

const (
	bootTimeout                = 30 * time.Second
	operationTimeout           = 30 * time.Second
	boundaryHookTimeoutSeconds = 5
)

type runtime struct {
	cmd              command
	settings         settings
	team             store.TeamPaths
	afterWitnessRead func()
}

func (cmd command) runtime() (*runtime, error) {
	s, err := cmd.settings()
	if err != nil {
		return nil, err
	}
	p, err := (store.Paths{Root: s.StateRoot}).Team(s.Session)
	if err != nil {
		return nil, err
	}
	return &runtime{cmd: cmd, settings: s, team: p}, nil
}
func (cmd command) now() time.Time {
	if cmd.clock != nil {
		return cmd.clock()
	}
	return time.Now()
}
func (cmd command) timeout(duration time.Duration) (context.Context, context.CancelFunc) {
	if cmd.newTimeout != nil {
		return cmd.newTimeout(context.Background(), duration)
	}
	return context.WithTimeout(context.Background(), duration)
}
func (run *runtime) record(a core.Agent, e core.Event) error {
	if e.At.IsZero() {
		e.At = run.cmd.now()
	}
	e.HitchID, e.Name = a.ID, a.Name
	return run.team.Append(e)
}
func (run *runtime) apply(l *store.LockedAgent, a *core.Agent, e core.Event) error {
	if e.At.IsZero() {
		e.At = run.cmd.now()
	}
	e.HitchID, e.Name = a.ID, a.Name
	next, effects := core.Step(*a, e)
	for _, effect := range effects {
		if effect.Kind == "reject" {
			return refuseError("%s", effect.Reason)
		}
	}
	if err := l.Save(next); err != nil {
		return err
	}
	*a = next
	return run.team.Append(e)
}
func (run *runtime) resolve(name string) (core.Agent, error) {
	if name == "" {
		agents, err := run.team.ListAgents()
		if err != nil {
			return core.Agent{}, err
		}
		for _, a := range agents {
			if a.Pane == run.cmd.environment("TMUX_PANE") && a.Pane != "" {
				return a, nil
			}
		}
		return core.Agent{}, refuseError("current pane is not a registered agent")
	}
	id, err := run.team.ResolveName(name)
	if errors.Is(err, os.ErrNotExist) {
		return core.Agent{}, refuseError("agent %q is not registered", name)
	}
	if err != nil {
		return core.Agent{}, err
	}
	p, err := run.team.Agent(id)
	if err != nil {
		return core.Agent{}, err
	}
	a, err := p.Read()
	if err != nil {
		return a, err
	}
	a, _ = core.Step(a, core.Event{Type: "deadline_checked", HitchID: a.ID, At: run.cmd.now()})
	return a, nil
}
func (run *runtime) acquire(id core.HitchID, wait bool) (*store.LockedAgent, core.Agent, error) {
	p, err := run.team.Agent(id)
	if err != nil {
		return nil, core.Agent{}, err
	}
	var l *store.LockedAgent
	if wait {
		l, err = p.LockAgent()
	} else {
		l, err = p.TryLock()
	}
	if err != nil {
		return nil, core.Agent{}, err
	}
	a, err := p.Read()
	if err == nil {
		err = run.team.FinishRename(l, &a)
		if errors.Is(err, store.ErrNameTaken) {
			err = nil
		}
	}
	if err == nil {
		err = l.CleanResult(&a)
	}
	if err == nil {
		err = run.recoverInput(l, &a)
	}
	if err == nil {
		err = run.reconcileDelivery(l, &a)
	}
	if err == nil {
		err = run.publishContextNotes(l, &a)
	}
	if err == nil {
		err = run.continueCompaction(l, &a)
	}
	if err == nil {
		err = run.checkDeadlines(l, &a)
	}
	if err != nil {
		_ = l.Close()
		return nil, a, err
	}
	return l, a, nil
}

// Short state readers release input work to a detached tick so they cannot
// wait for a harness. The tick owns the drain and its final unlock recheck.
func (run *runtime) release(l *store.LockedAgent) error {
	if !l.Held() {
		return nil
	}
	if err := l.Close(); err != nil {
		return err
	}
	if run.cmd.afterUnlock != nil {
		run.cmd.afterUnlock()
	}
	pending, err := l.Paths.ListNew()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range pending {
		if !e.NotBefore.After(run.cmd.now()) {
			if run.cmd.detach != nil {
				return run.cmd.detach(string(e.Recipient), hookNotice{})
			}
			return run.cmd.detachTick(string(e.Recipient), hookNotice{}, run.settings)
		}
	}
	return nil
}
func (run *runtime) checkDeadlines(l *store.LockedAgent, a *core.Agent) error {
	// Trust can appear after AwaitStartup returns. Observe it before an
	// expired boot budget converts an operator-owned prompt into failure.
	if a.Status == core.Booting && !a.BootDeadline.IsZero() && !run.cmd.now().Before(a.BootDeadline) {
		c, err := loadCollar(a.Collar, run.settings)
		if err != nil {
			return err
		}
		b, err := run.input()
		if err != nil {
			return err
		}
		screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
		if err != nil {
			return err
		}
		startup, err := harness.InspectStartup(c, screen)
		if err != nil {
			return err
		}
		if startup.State == harness.StartupTrustRequired {
			return run.apply(l, a, core.Event{Type: "hitch_blocked", Reason: startup.Prompt})
		}
		if startup.State == harness.StartupReady {
			return run.apply(l, a, core.Event{Type: "hitch_ready"})
		}
	}
	next, _ := core.Step(*a, core.Event{Type: "deadline_checked", At: run.cmd.now(), HitchID: a.ID})
	if next.Status == a.Status && next.Activity == a.Activity && next.Evidence == a.Evidence {
		return nil
	}
	return run.apply(l, a, core.Event{Type: "deadline_checked"})
}
func (run *runtime) recoverInput(l *store.LockedAgent, a *core.Agent) error {
	if a.Input == nil {
		return nil
	}
	id, kind := a.Input.ID, a.Input.Kind
	if kind == "compaction" {
		return run.apply(l, a, core.Event{Type: "compaction_unverified", ID: id, Reason: "input owner exited before recording the outcome"})
	}
	if kind != "envelope" {
		a.Input = nil
		a.Activity = core.Unknown
		a.Evidence = "input owner exited before recording the outcome"
		return l.Save(*a)
	}
	eid := core.EnvelopeID(id)
	e, err := l.Paths.ReadEnvelope("new", eid)
	if errors.Is(err, os.ErrNotExist) {
		for _, dir := range []string{"cur", "failed"} {
			e, err = l.Paths.ReadEnvelope(dir, eid)
			if err == nil {
				from, _ := l.Paths.EnvelopePath(dir, eid)
				to, _ := l.Paths.EnvelopePath("new", eid)
				if err = os.Rename(from, to); err != nil {
					return err
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	if err != nil {
		return fmt.Errorf("recover input %s: %w", id, err)
	}
	if a.LastAccepted == eid {
		a.LastAccepted = ""
	}
	if a.LastDelivered == eid {
		a.LastDelivered = ""
	}
	if a.LastFailed == eid {
		a.LastFailed = ""
	}
	if e.Outcome == "accepted" {
		return run.finishInput(l, a, e, "accepted", e.Reason)
	}
	return run.finishInput(l, a, e, "unverified", "input owner exited before recording the outcome")
}
func (run *runtime) finishInput(l *store.LockedAgent, a *core.Agent, e core.Envelope, outcome, reason string) error {
	if err := l.Settle(a, e, outcome, reason); err != nil {
		return err
	}
	if err := run.record(*a, core.Event{Type: map[string]string{"accepted": "delivery_accepted", "delivered": "delivery_succeeded", "failed": "delivery_failed", "unverified": "delivery_unverified"}[outcome], ID: string(e.ID), Reason: reason}); err != nil {
		return err
	}
	return run.apply(l, a, core.Event{Type: "input_finished", ID: string(e.ID), Status: outcome, Reason: reason})
}
func (run *runtime) input() (harnessInput, error) {
	if run.cmd.inputBackend != nil {
		return run.cmd.inputBackend, nil
	}
	return run.cmd.tmux(run.settings)
}
func (run *runtime) mark(a core.Agent) error {
	if a.Pane == "" {
		return nil
	}
	if run.cmd.inputBackend != nil {
		return nil
	}
	b, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	return b.Rename(context.Background(), substrate.PaneID(a.Pane), windowTitle(a))
}
