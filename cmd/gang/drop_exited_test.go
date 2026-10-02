package main

import (
	"context"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// exitedIdentityRegistry reports a visible pane whose native process exits
// before its identity is read.
type exitedIdentityRegistry struct {
	*inputFixture
	removed, identities int
}

func (r *exitedIdentityRegistry) Identity(context.Context, substrate.PaneID) (tmux.Identity, error) {
	r.identities++
	return tmux.Identity{}, &substrate.ExitedError{Status: "3", Output: "boot failure"}
}

func (r *exitedIdentityRegistry) RemoveRegisteredPane(context.Context, tmux.PaneIdentity) error {
	r.removed++
	return nil
}

func TestDropFinishesWhenNativeExitsBeforeIdentityRead(t *testing.T) {
	f := newStateFixture(t)
	registry := &exitedIdentityRegistry{inputFixture: f.input}
	f.cmd.paneBackend = registry
	a := f.add(t, "a", "worker", "codex")
	a.Status = core.Failed
	a.Process = core.ProcessIdentity{}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatalf("drop of an exited native: %v", err)
	}
	if registry.identities != 1 || registry.removed != 1 {
		t.Fatalf("identity reads = %d, pane removals = %d, want 1 each", registry.identities, registry.removed)
	}
}
