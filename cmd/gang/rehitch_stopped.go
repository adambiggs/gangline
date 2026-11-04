package main

import (
	"context"
	"errors"
	"os"
	"reflect"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// The operator stops the previous execution location before copying its state.
// A foreign process namespace cannot establish that fact from this machine.
func (cmd command) rehitchStopped(name, note string) error {
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	id, err := run.team.ResolveName(name)
	if err != nil {
		return err
	}
	if isOutsideID(id) || store.IsSenderID(id) {
		return refuseError("rehitch: %s is not a native harness", name)
	}
	p, err := run.team.Agent(id)
	if err != nil {
		return err
	}
	a, err := p.Read()
	if err != nil {
		return err
	}
	if err := stoppedRehitchState(a); err != nil {
		return err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if len(c.Launch.ResumeArgs) == 0 {
		return refuseError("rehitch: collar %q declares no native resume arguments", c.Name)
	}
	args := []string{string(a.Name), "-c", a.Collar, "-d", a.Directory, "--resume", a.Native.SessionID, "-t", note}
	if a.Role != "" {
		args = append(args, "-r", a.Role)
	}
	if a.ConfiguredModel != "" {
		args = append(args, "-m", a.ConfiguredModel)
		if a.ConfiguredEffort != "" {
			args = append(args, "-e", a.ConfiguredEffort)
		}
	}
	return cmd.hitchReplacing(args, false, &a, true)
}

func stoppedRehitchState(a core.Agent) error {
	switch {
	case a.Native.SessionID == "":
		return refuseError("rehitch: native resume identity is unknown for %s", a.Name)
	case a.Status == core.Dropping || len(a.Teardown) != 0:
		return refuseError("rehitch: %s has unfinished teardown", a.Name)
	case a.RenameFrom != "" || a.RenameTo != "":
		return refuseError("rehitch: %s has an unfinished rename", a.Name)
	case a.Input != nil || a.Cleanup != nil || clearPending(a) || a.Rehitch != nil:
		return refuseError("rehitch: %s has an unresolved operation; retained state needs reconciliation", a.Name)
	case a.Compaction != nil && a.Compaction.Status != "failed" && !a.Compaction.ResumeAdmitted:
		return refuseError("rehitch: %s has an unresolved compaction; retained state needs reconciliation", a.Name)
	}
	return nil
}

func (run *runtime) prepareStoppedRehitch(ctx context.Context, b *tmux.Backend, previous core.Agent, next *core.Agent) (*store.LockedAgent, error) {
	p, err := run.team.Agent(previous.ID)
	if err != nil {
		return nil, err
	}
	l, err := p.TryLock()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*store.LockedAgent, error) { return nil, errors.Join(err, l.Close()) }
	a, err := p.Read()
	if err != nil {
		return fail(err)
	}
	id, err := run.team.ResolveName(string(previous.Name))
	if err != nil {
		return fail(err)
	}
	if id != previous.ID || !reflect.DeepEqual(a, previous) {
		return fail(refuseError("rehitch: %s changed during recovery preparation", previous.Name))
	}
	if err := stoppedRehitchState(a); err != nil {
		return fail(err)
	}
	if a.Pane != "" {
		if err := requirePaneRegistration(a); err != nil {
			return fail(err)
		}
		present, err := b.CheckPane(ctx, paneIdentity(a))
		if err != nil && !errors.Is(err, tmux.ErrPaneReplaced) {
			return fail(err)
		}
		if present {
			return fail(refuseError("rehitch: %s still has its registered pane", a.Name))
		}
	}
	if a.Process.PID != 0 {
		identity := nativeIdentity(a.Process)
		if identity.BootID == "" || identity.Namespace == "" || identity.Started == "" && identity.UniqueID == 0 {
			return fail(refuseError("rehitch: %s has an incomplete process witness", a.Name))
		}
		earlier, err := tmux.EarlierBoot(identity)
		if err != nil {
			return fail(err)
		}
		if !earlier {
			visible, err := tmux.IdentityNamespaceMatches(identity)
			if err != nil {
				return fail(err)
			}
			if visible {
				owned, err := tmux.AcquireRecorded([]tmux.Identity{identity})
				if err != nil {
					return fail(err)
				}
				alive := len(owned.Identities()) != 0
				if err := owned.Close(); err != nil {
					return fail(err)
				}
				if alive {
					return fail(refuseError("rehitch: %s still has its recorded process", a.Name))
				}
			}
		}
	} else if a.Process != (core.ProcessIdentity{}) {
		return fail(refuseError("rehitch: %s has an incomplete process witness", a.Name))
	}
	// A harness can run where its native PID is not visible. A missing process
	// witness cannot prove exit; --stopped explicitly attests the old execution
	// location is stopped. A matching pane above still refuses recovery.
	// Preserve durable conversation state and inbox ownership. The normal
	// startup path reacquires observations tied to the old native process.
	// Retain the old witness and its launch metadata until a new pane is
	// registered. A failed spawn can then retry without losing those facts.
	next.Registration, next.Process = a.Registration, a.Process
	next.Registration.Held = false
	next.Native = a.Native
	next.Native.HookAt, next.Native.HookKind = run.cmd.now(), ""
	next.NativeTitle = a.NativeTitle
	next.ContextBands = a.ContextBands
	next.Compaction, next.Clear, next.Capacity = a.Compaction, a.Clear, a.Capacity
	next.BackgroundNoticed = a.BackgroundNoticed
	next.CurfewNoticeDeadline = a.CurfewNoticeDeadline
	for _, path := range []string{p.Witness, p.CompactionWitness, p.Background, p.Permission} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		}
	}
	if err := l.Save(*next); err != nil {
		return fail(err)
	}
	reason := "operator confirmed previous execution location stopped"
	if a.Process.PID == 0 {
		reason += "; native process witness unavailable, relying on stop attestation"
	}
	if a.ConfiguredModel == "" {
		reason += "; configured model/effort unknown, using collar/native defaults"
	}
	if err := run.record(*next, core.Event{Type: "rehitch_started", Reason: reason}); err != nil {
		return fail(err)
	}
	return l, nil
}
