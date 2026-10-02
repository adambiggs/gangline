package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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

// closingPaneRegistry reports a visible pane that closes with its native
// process before the identity read, as an unheld pane does: tmux answers the
// read with no pane process, and the pane is gone when checked again, or
// checking it again answers recheck.
type closingPaneRegistry struct {
	*inputFixture
	recheck          error
	closed           bool
	removed, checked int
}

func (r *closingPaneRegistry) Identity(context.Context, substrate.PaneID) (tmux.Identity, error) {
	r.closed = true
	return tmux.Identity{}, errors.New(`read pane process: no live pane process in ""`)
}

func (r *closingPaneRegistry) CheckPane(context.Context, tmux.PaneIdentity) (bool, error) {
	r.checked++
	if r.closed && r.recheck != nil {
		return false, r.recheck
	}
	return !r.closed, nil
}

func (r *closingPaneRegistry) RemoveRegisteredPane(context.Context, tmux.PaneIdentity) error {
	r.removed++
	return nil
}

func TestDropFinishesWhenUnheldPaneClosesBeforeIdentityRead(t *testing.T) {
	f := newStateFixture(t)
	registry := &closingPaneRegistry{inputFixture: f.input}
	f.cmd.paneBackend = registry
	a := f.add(t, "a", "worker", "codex")
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
		t.Fatalf("drop of a native whose unheld pane closed: %v", err)
	}
	if registry.checked != 2 || registry.removed != 1 {
		t.Fatalf("pane checks = %d, pane removals = %d, want 2 and 1", registry.checked, registry.removed)
	}
	if _, err := p.Read(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("agent state after drop: %v, want removed", err)
	}
}

// survivingPaneRegistry fails the identity read of a pane that is still there.
type survivingPaneRegistry struct{ *inputFixture }

func (r *survivingPaneRegistry) Identity(context.Context, substrate.PaneID) (tmux.Identity, error) {
	return tmux.Identity{}, errors.New("read pane process: permission denied")
}

func TestDropAfterIdentityReadOnAClosedPaneTrustsTheRecheck(t *testing.T) {
	for _, c := range []struct {
		name    string
		recheck error
		removed int
		fails   bool
	}{
		{"replaced pane is left alone", fmt.Errorf("%w: pane differs", tmux.ErrPaneReplaced), 0, false},
		{"unreadable pane stops the drop", errors.New("read pane registration: server unreachable"), 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStateFixture(t)
			registry := &closingPaneRegistry{inputFixture: f.input, recheck: c.recheck}
			f.cmd.paneBackend = registry
			a := f.add(t, "a", "worker", "codex")
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
			err = f.cmd.drop([]string{"worker"})
			if c.fails != (err != nil) || c.fails && !strings.Contains(err.Error(), "server unreachable") {
				t.Fatalf("drop error = %v, want failure %v", err, c.fails)
			}
			if registry.removed != c.removed {
				t.Fatalf("pane removals = %d, want %d", registry.removed, c.removed)
			}
		})
	}
}

func TestDropStopsWhenIdentityReadFailsOnAPresentPane(t *testing.T) {
	f := newStateFixture(t)
	f.cmd.paneBackend = &survivingPaneRegistry{inputFixture: f.input}
	a := f.add(t, "a", "worker", "codex")
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
	if err := f.cmd.drop([]string{"worker"}); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("drop error = %v, want the identity read failure", err)
	}
}
