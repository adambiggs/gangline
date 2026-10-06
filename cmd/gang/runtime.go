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
	"github.com/adambiggs/gangline/substrate/tmux"
)

const (
	bootTimeout                = 30 * time.Second
	operationTimeout           = 30 * time.Second
	schedulerLockTimeout       = operationTimeout
	boundaryHookTimeoutSeconds = 5
)

type runtime struct {
	cmd              command
	settings         settings
	team             store.TeamPaths
	afterWitnessRead func()
	// reported names the compaction whose outcome this command returns to
	// its requester directly.
	reported string
	// hitching names the agent this hitch command is starting; the command
	// returns that agent's boot failure to its caller directly.
	hitching core.HitchID
	// wake holds agents given a message while another agent's lock was held;
	// unlock starts their ticks once that lock is gone.
	wake []core.HitchID
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
func (run *runtime) lockTeam() (*os.File, error) {
	lock, err := (store.Paths{Root: run.settings.StateRoot}).LockTeam(run.settings.Session)
	if errors.Is(err, store.ErrLocked) {
		return nil, refuseError("team %q is changing; retry after the current operation finishes", run.settings.Session)
	}
	return lock, err
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
	wasFailed := a.Status == core.Failed
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
	if err := run.team.Append(e); err != nil {
		return err
	}
	if !wasFailed && a.Status == core.Failed {
		reason := e.Reason
		if reason == "" {
			reason = a.Evidence
		}
		return run.notifyHitcherFailed(*a, reason)
	}
	return nil
}

// inactiveRecipient refuses input to an agent that is not active and names
// the command that shows why.
func inactiveRecipient(a core.Agent) error {
	return refuseError("recipient %s is %s, not active; gang status %s shows why", a.Name, a.Status, a.Name)
}

// unregistered refuses a name that no agent in the selected team holds.
func (run *runtime) unregistered(name string) error {
	return refuseError("agent %q is not registered in team %s; gang roster --team %s lists the registered agents", name, run.settings.Session, run.settings.Session)
}

func (run *runtime) resolve(name string) (core.Agent, error) {
	if name == "" {
		a, err := run.observedAgent()
		if err != nil {
			return core.Agent{}, err
		}
		if a != nil {
			return *a, nil
		}
		return core.Agent{}, refuseError("current pane is not a registered agent")
	}
	id, err := run.team.ResolveName(name)
	if errors.Is(err, os.ErrNotExist) {
		return core.Agent{}, run.unregistered(name)
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
	return run.acquireAgent(id, func(p store.AgentPaths) (*store.LockedAgent, error) {
		if wait {
			return p.LockAgent()
		}
		return p.TryLock()
	})
}

const agentLockBudget = 500 * time.Millisecond

// acquireBounded retries only flock. Reconciliation and command preconditions
// run once, after the same hitch's lock is held.
func (run *runtime) acquireBounded(id core.HitchID) (*store.LockedAgent, core.Agent, error) {
	clock, pause := run.cmd.lockClock, run.cmd.lockWait
	if clock == nil {
		clock = time.Now
	}
	if pause == nil {
		pause = time.Sleep
	}
	return run.acquireAgent(id, func(p store.AgentPaths) (*store.LockedAgent, error) {
		deadline := clock().Add(agentLockBudget)
		delay := 10 * time.Millisecond
		for {
			l, err := p.TryLock()
			if !errors.Is(err, store.ErrLocked) {
				return l, err
			}
			remaining := deadline.Sub(clock())
			if remaining <= 0 {
				return nil, err
			}
			pause(min(delay, remaining))
			delay = min(delay*2, 100*time.Millisecond)
		}
	})
}

func (run *runtime) acquireAgent(id core.HitchID, lock func(store.AgentPaths) (*store.LockedAgent, error)) (*store.LockedAgent, core.Agent, error) {
	p, err := run.team.Agent(id)
	if err != nil {
		return nil, core.Agent{}, err
	}
	l, err := lock(p)
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
		err = run.reconcileUsageSubmission(l, &a)
	}
	if err == nil {
		err = run.reconcileCompactionWitness(l, &a)
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
		_ = run.unlock(l)
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
	if err := run.unlock(l); err != nil {
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
			return run.detachTick(e.Recipient)
		}
	}
	return nil
}

// unlock releases an agent lock, then starts the ticks of agents given a
// message while it was held.
func (run *runtime) unlock(l *store.LockedAgent) error {
	err := l.Close()
	wake := run.wake
	run.wake = nil
	for _, id := range wake {
		err = errors.Join(err, run.detachTick(id))
	}
	return err
}
func (run *runtime) detachTick(id core.HitchID) error {
	if run.cmd.detach != nil {
		return run.cmd.detach(string(id), hookNotice{})
	}
	return run.cmd.detachTick(string(id), hookNotice{}, run.settings)
}
func (run *runtime) checkDeadlines(l *store.LockedAgent, a *core.Agent) error {
	// Trust can appear after AwaitStartup returns. Observe it before an
	// expired boot budget converts an operator-owned prompt into failure. A
	// blocked startup keeps its pane held, so an exit at the prompt is read
	// here too.
	expired := !a.BootDeadline.IsZero() && !run.cmd.now().Before(a.BootDeadline)
	if a.Status == core.Booting && a.Pane != "" && (expired || a.Activity == core.Blocked) {
		c, err := loadCollar(a.Collar, run.settings)
		if err != nil {
			return err
		}
		b, err := run.input()
		if err != nil {
			return err
		}
		screen, err := run.captureRegistered(context.Background(), b, *a)
		// A held pane shows no prompt, only its native exit.
		var exited *substrate.ExitedError
		if errors.As(err, &exited) {
			return run.apply(l, a, core.Event{Type: "hitch_failed", Reason: exited.Error()})
		}
		if err != nil {
			// A registered pane that is gone shows nothing either. A closed pane
			// fails the record, which no longer owns it. A pane that is only
			// unobservable stays recorded, and an expired deadline fails it.
			gone, closed, checkErr := run.paneGone(*a)
			switch {
			case checkErr == nil && closed:
				return run.forgetPane(l, a)
			case !expired:
				// The startup is not due; tick reports its own capture error.
				return nil
			case checkErr != nil || !gone:
				return errors.Join(err, checkErr)
			}
		} else if expired {
			startup, err := harness.InspectStartup(c, screen)
			if err != nil {
				return err
			}
			if startup.State == harness.StartupTrustRequired {
				return run.apply(l, a, core.Event{Type: "hitch_blocked", Reason: startup.Prompt})
			}
			if startup.State == harness.StartupReady {
				// A composer can precede Codex trust. Leave readiness to the
				// stabilized startup observation in hitch or tick.
				return nil
			}
		}
	}
	// Name the compaction that outlived its deadline, so the log records why
	// the agent's reading changed.
	if c := a.Compaction; c != nil && c.Status == "submitted" && !run.cmd.now().Before(c.Deadline) {
		if err := run.apply(l, a, core.Event{Type: "compaction_unverified", ID: c.ID, Reason: core.CompactionUnconfirmed}); err != nil {
			return err
		}
	}
	next, _ := core.Step(*a, core.Event{Type: "deadline_checked", At: run.cmd.now(), HitchID: a.ID})
	if next.Status == a.Status && next.Activity == a.Activity && next.Evidence == a.Evidence {
		return nil
	}
	return run.apply(l, a, core.Event{Type: "deadline_checked"})
}

// forgetClosedPane fails a record whose registered pane is closed and removes
// the pane from it, so no later observation addresses a pane gang does not
// own. It reports whether the pane was closed.
func (run *runtime) forgetClosedPane(l *store.LockedAgent, a *core.Agent) (bool, error) {
	_, closed, err := run.paneGone(*a)
	if err != nil || !closed {
		return false, err
	}
	return true, run.forgetPane(l, a)
}

func (run *runtime) forgetPane(l *store.LockedAgent, a *core.Agent) error {
	a.Pane = ""
	if a.Status == core.Failed {
		return l.Save(*a)
	}
	return run.apply(l, a, core.Event{Type: "hitch_failed", Reason: "registered pane is absent from tmux"})
}

// paneGone reports whether the record's registered pane is absent or now
// names another server's or session's pane, and whether it is closed: the
// server that registered it confirms that, or that server's process has
// exited. Only a closed pane may leave the record: an unreachable server or a
// renamed session hides a pane that still runs.
func (run *runtime) paneGone(a core.Agent) (gone, closed bool, err error) {
	registry, err := run.registry()
	if err != nil {
		return false, false, err
	}
	id := paneIdentity(a)
	present, err := registry.CheckPane(context.Background(), id)
	if errors.Is(err, tmux.ErrPaneReplaced) {
		present, err = false, nil
	}
	if err != nil || present {
		return false, false, err
	}
	closed, err = registry.PaneClosed(context.Background(), id, nativeIdentity(a.Registration.Server))
	return true, closed, err
}

// captureRegistered captures the record's pane unless its id names a pane of
// another server or session. A new server reuses pane ids, so a stale record
// would otherwise read another agent's screen as its own. An absent pane or
// server is left to the capture, whose error says which.
func (run *runtime) captureRegistered(ctx context.Context, b harnessInput, a core.Agent) (substrate.Screen, error) {
	registry, err := run.registry()
	if err != nil {
		return substrate.Screen{}, err
	}
	if _, err := registry.CheckPane(ctx, paneIdentity(a)); err != nil {
		return substrate.Screen{}, err
	}
	return b.Capture(ctx, substrate.PaneID(a.Pane))
}

// captureAgentPane captures the record's pane for a command run on the
// agent. A pane id that names another server's or session's pane refuses with
// the step that frees the agent's name: the pane it registered ended with its
// server, unless the team session was only renamed. A record with no complete
// registration is refused as such before any read.
func (run *runtime) captureAgentPane(ctx context.Context, b harnessInput, a core.Agent) (substrate.Screen, error) {
	if err := requirePaneRegistration(a); err != nil {
		return substrate.Screen{}, err
	}
	screen, err := run.captureRegistered(ctx, b, a)
	if errors.Is(err, tmux.ErrPaneReplaced) {
		return screen, refuseError("%s cannot be read: %v; if the team session was renamed, restore its name; otherwise drop %s and hitch it again", a.Name, err, a.Name)
	}
	return screen, err
}

// recordedProcessExited reports whether the record's native process is
// witnessed gone: recorded under an earlier boot, or absent or replaced now.
// An identity gang cannot check is not a witness.
func recordedProcessExited(a core.Agent) bool {
	if a.Process.PID == 0 || a.Process.BootID == "" {
		return false
	}
	owned, err := tmux.AcquireRecorded([]tmux.Identity{nativeIdentity(a.Process)})
	if err != nil {
		return false
	}
	exited := len(owned.Identities()) == 0
	return owned.Close() == nil && exited
}

// exitedUnlistedReason says why an agent with no listed team session and an
// exited process failed. A process recorded under an earlier boot ended with
// it, so the team did not survive the restart and only its records remain.
func exitedUnlistedReason(a core.Agent) string {
	if earlier, err := tmux.EarlierBoot(nativeIdentity(a.Process)); err == nil && earlier {
		return "the team did not survive a host reboot; gang down clears its records"
	}
	return "tmux lists no team session and the recorded process has exited"
}

func (run *runtime) recoverInput(l *store.LockedAgent, a *core.Agent) error {
	if a.Input == nil {
		return nil
	}
	id, kind := a.Input.ID, a.Input.Kind
	if kind == "compaction" {
		return run.apply(l, a, core.Event{Type: "compaction_unverified", ID: id, Reason: "input owner exited before recording the outcome"})
	}
	if kind == "compaction-recovery" {
		return run.apply(l, a, core.Event{Type: "compaction_unverified", ID: id, Status: "recovery", Reason: "recovery owner exited before recording the outcome"})
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
	if err := run.finishInput(l, a, e, "unverified", "input owner exited before recording the outcome"); err != nil {
		return err
	}
	return run.notifySender(*a, e, "unverified")
}
func (run *runtime) finishInput(l *store.LockedAgent, a *core.Agent, e core.Envelope, outcome, reason string, witnessed ...store.Witness) error {
	if outcome == "delivered" && len(witnessed) > 0 && witnessed[0].Prompt != "" {
		c, err := loadCollar(a.Collar, run.settings)
		if err != nil {
			return err
		}
		wire, err := envelopeText(e)
		if err != nil {
			return err
		}
		matched, outside, err := harness.SubmittedEnvelopeMatches(c.Primitives.SubmitWitness, wire, witnessed[0].Prompt)
		if err != nil {
			return err
		}
		if matched && outside != "" {
			if reason != "" {
				reason += "; "
			}
			reason += outsideEnvelopeObservation(outside)
		}
	}
	if err := run.acknowledgeUsageDelivery(l, *a, e, outcome, witnessed...); err != nil {
		return err
	}
	if err := l.Settle(a, e, outcome, reason); err != nil {
		return err
	}
	if err := run.record(*a, core.Event{Type: map[string]string{"accepted": "delivery_accepted", "delivered": "delivery_succeeded", "failed": "delivery_failed", "unverified": "delivery_unverified"}[outcome], ID: string(e.ID), Reason: reason}); err != nil {
		return err
	}
	return run.apply(l, a, core.Event{Type: "input_finished", ID: string(e.ID), Status: outcome, Reason: reason})
}

func outsideEnvelopeObservation(outside string) string {
	return fmt.Sprintf("session keyboard input outside gang envelope: %q", outside)
}
func (run *runtime) input() (harnessInput, error) {
	if run.cmd.inputBackend != nil {
		return run.cmd.inputBackend, nil
	}
	return run.cmd.tmux(run.settings)
}

// mark titles the agent's pane through its registration, so a pane id that
// now names another pane keeps that pane's title.
func (run *runtime) mark(a core.Agent) error {
	if requirePaneRegistration(a) != nil {
		return nil
	}
	if run.cmd.inputBackend != nil {
		return nil
	}
	b, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	return b.TitleRegisteredPane(context.Background(), paneIdentity(a), paneTitle(a))
}
