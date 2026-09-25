package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// Supersede only the name registration. The immutable hitch directory keeps
// native session identity, receipts, and transcript pointers for inspection.
func (run *runtime) supersedeStoppedClaim(ctx context.Context, b *tmux.Backend, replacement core.Agent) (created *store.LockedAgent, result error) {
	id, err := run.team.ResolveName(string(replacement.Name))
	if err != nil {
		return nil, err
	}
	p, err := run.team.Agent(id)
	if err != nil {
		return nil, err
	}
	l, err := p.TryLock()
	if errors.Is(err, store.ErrLocked) {
		return nil, refuseError("agent name %q is locked; retry when its current operation finishes", replacement.Name)
	}
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, l.Close()) }()
	current, err := run.team.ResolveName(string(replacement.Name))
	if err != nil {
		return nil, err
	}
	if current != id {
		return nil, store.ErrNameTaken
	}
	exists, err := b.SessionExists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, store.ErrNameTaken
	}
	old, err := p.Read()
	if err != nil {
		return nil, err
	}
	if old.Status == core.Dropping || len(old.Teardown) != 0 {
		return nil, refuseError("agent %q has unfinished teardown; run 'gang drop %s' first", old.Name, old.Name)
	}
	if old.Process.PID != 0 {
		if old.Process.BootID == "" {
			return nil, refuseError("agent %q has an unverified process identity", old.Name)
		}
		owned, err := tmux.AcquireRecorded([]tmux.Identity{nativeIdentity(old.Process)})
		if err != nil {
			return nil, err
		}
		alive := len(owned.Identities()) != 0
		if err := owned.Close(); err != nil {
			return nil, err
		}
		if alive {
			return nil, refuseError("agent %q still has a live recorded process; check the tmux socket", old.Name)
		}
	}
	// Never let a retained record target a pane ID reused by the new server.
	old.Pane = ""
	if err := run.apply(l, &old, core.Event{Type: "hitch_failed", Reason: "retiring stale name claim: tmux team and recorded process are absent"}); err != nil {
		return nil, err
	}
	if err := run.team.RemoveName(replacement.Name, id); err != nil {
		return nil, err
	}
	created, err = run.team.CreateAgent(replacement)
	if err != nil {
		return nil, err
	}
	if run.cmd.stderr != nil {
		if _, err := fmt.Fprintf(run.cmd.stderr, "superseded %s (%s); retained record: %s\n", old.Name, old.ID, p.Directory); err != nil {
			return nil, errors.Join(err, created.Close())
		}
	}
	return created, nil
}
