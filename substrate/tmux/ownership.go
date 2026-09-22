package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/adambiggs/gangline/substrate"
)

// Identity is the native process identity recorded when a pane is registered.
// It is checked against a pinned kernel handle before any signal is sent.
type Identity struct {
	PID      int
	Started  string
	Version  uint32
	UniqueID uint64
	BootID   string
}

func (b *Backend) Identity(ctx context.Context, pane substrate.PaneID) (Identity, error) {
	pid, err := b.paneProcess(ctx, pane)
	if err != nil {
		return Identity{}, err
	}
	observation, err := observeProcess(pid)
	if err != nil {
		return Identity{}, err
	}
	defer observation.close()
	r := observation.record
	boot, err := bootIdentity()
	if err != nil {
		return Identity{}, err
	}
	return Identity{r.PID, r.started, r.version, r.uniqueID, boot}, nil
}

type Owned struct {
	processes  []processIdentity
	identities []Identity
}

func (o *Owned) Identities() []Identity         { return append([]Identity(nil), o.identities...) }
func (o *Owned) Close() error                   { return closeOwnedProcesses(o.processes) }
func (o *Owned) Stop(ctx context.Context) error { return reapOwnedProcesses(ctx, o.processes) }

func (b *Backend) AcquireTree(ctx context.Context, pane substrate.PaneID, expected Identity) (*Owned, error) {
	boot, err := bootIdentity()
	if err != nil {
		return nil, err
	}
	if expected.BootID != boot {
		return &Owned{}, nil
	}
	r, err := readCurrentProcess(expected.PID)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
		return &Owned{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !sameIdentity(expected, r) {
		return &Owned{}, nil
	}
	owned, err := b.ownedProcesses(ctx, pane)
	if err != nil {
		return nil, err
	}
	ok := false
	for _, p := range owned {
		if p.pid == expected.PID && p.started == r.started {
			ok = true
		}
	}
	if !ok {
		_ = closeOwnedProcesses(owned)
		return nil, fmt.Errorf("pane process differs from its registered identity")
	}
	out := &Owned{processes: owned}
	for _, p := range owned {
		r, err := readCurrentProcess(p.pid)
		if err != nil {
			_ = out.Close()
			return nil, err
		}
		if r.started != p.started {
			_ = out.Close()
			return nil, fmt.Errorf("process changed during teardown preparation")
		}
		out.identities = append(out.identities, Identity{p.pid, p.started, r.version, r.uniqueID, boot})
	}
	return out, nil
}

func sameIdentity(expected Identity, actual processRecord) bool {
	if expected.UniqueID != 0 {
		return expected.PID == actual.PID && expected.UniqueID == actual.uniqueID
	}
	return expected.PID == actual.PID && expected.Started == actual.started
}

// AcquireRecorded finishes a partial teardown without depending on a live pane.
// Missing or replaced processes have already exited; replacements are untouched.
func AcquireRecorded(ids []Identity) (*Owned, error) {
	out := &Owned{}
	boot, err := bootIdentity()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if id.BootID != boot {
			continue
		}
		observation, err := observeProcess(id.PID)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			_ = out.Close()
			return nil, err
		}
		if !sameIdentity(id, observation.record) {
			_ = observation.close()
			continue
		}
		pinned, err := pinObservedProcess(observation.record, observation.read, openProcessHandle)
		closeErr := observation.close()
		if err != nil || closeErr != nil {
			_ = out.Close()
			return nil, errors.Join(err, closeErr)
		}
		out.processes = append(out.processes, pinned)
		out.identities = append(out.identities, id)
	}
	return out, nil
}
func (b *Backend) RemovePane(ctx context.Context, pane substrate.PaneID, expected Identity) error {
	boot, err := bootIdentity()
	if err != nil {
		return err
	}
	if expected.BootID != boot {
		return nil
	}
	exists, err := b.SessionExists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	windows, err := b.Windows(ctx)
	if err != nil {
		return err
	}
	for _, w := range windows {
		if w.Pane.ID == pane {
			out, err := b.run(ctx, "display-message", "-p", "-t", string(pane), "#{pane_pid}")
			if err != nil {
				return tmuxError("read stopped pane", err, out)
			}
			if strings.TrimSpace(out) != strconv.Itoa(expected.PID) {
				return fmt.Errorf("refuse removal: pane process was replaced")
			}
			r, err := readCurrentProcess(expected.PID)
			if err == nil && !sameIdentity(expected, r) {
				return fmt.Errorf("refuse removal: pane process identity changed")
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ESRCH) {
				return err
			}
			out, err = b.run(ctx, "kill-pane", "-t", string(pane))
			if err != nil {
				return tmuxError("remove stopped pane", err, out)
			}
		}
	}
	return nil
}
