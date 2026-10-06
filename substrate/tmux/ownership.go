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
	PID       int
	Started   string
	Version   uint32
	UniqueID  uint64
	BootID    string
	Namespace string
}

func (b *Backend) Identity(ctx context.Context, pane substrate.PaneID) (Identity, error) {
	return b.identity(ctx, pane, observeProcess)
}

func (b *Backend) identity(ctx context.Context, pane substrate.PaneID, observe func(int) (processObservation, error)) (Identity, error) {
	pid, err := b.paneProcess(ctx, pane)
	if err != nil {
		return Identity{}, err
	}
	observation, err := observe(pid)
	if processGone(err) {
		return Identity{}, b.paneExited(ctx, pane, err)
	}
	if err != nil {
		return Identity{}, err
	}
	defer observation.close()
	r := observation.record
	boot, err := bootIdentity()
	if err != nil {
		return Identity{}, err
	}
	namespace, err := nativeProcessNamespace()
	if err != nil {
		return Identity{}, err
	}
	return Identity{PID: r.PID, Started: r.started, Version: r.version, UniqueID: r.uniqueID, BootID: boot, Namespace: namespace}, nil
}

// CanReadIdentity requires a saved witness that the numeric PID belongs to the
// caller's process namespace. Older identities without a witness are unknown.
func CanReadIdentity(expected Identity) bool {
	namespace, err := nativeProcessNamespace()
	return err == nil && expected.Namespace != "" && expected.Namespace == namespace
}

type Owned struct {
	processes  []processIdentity
	identities []Identity
}

func (o *Owned) Identities() []Identity         { return append([]Identity(nil), o.identities...) }
func (o *Owned) Close() error                   { return closeOwnedProcesses(o.processes) }
func (o *Owned) Stop(ctx context.Context) error { return reapOwnedProcesses(ctx, o.processes) }

func (b *Backend) AcquireTree(ctx context.Context, pane substrate.PaneID, expected Identity) (*Owned, error) {
	return b.acquireTree(ctx, pane, expected, readCurrentProcess)
}

func (b *Backend) acquireTree(ctx context.Context, pane substrate.PaneID, expected Identity, read func(int) (processRecord, error)) (*Owned, error) {
	boot, err := bootIdentity()
	if err != nil {
		return nil, err
	}
	if expected.BootID != boot {
		return &Owned{}, nil
	}
	if !CanReadIdentity(expected) {
		return nil, fmt.Errorf("registered process namespace is not visible")
	}
	r, err := read(expected.PID)
	if processGone(err) {
		return &Owned{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !sameIdentity(expected, r) {
		return &Owned{}, nil
	}
	if _, exists, err := b.registeredPane(ctx, string(pane)); err != nil {
		return nil, err
	} else if !exists {
		// The recorded root is still identifiable without tmux. Descendants
		// never captured before pane loss cannot be safely reconstructed here.
		return AcquireRecorded([]Identity{expected})
	}
	owned, err := b.ownedProcesses(ctx, pane)
	if err != nil {
		// The pane can close, or its process exit, between the registration
		// read and the process reads; its recorded root is then taken as above.
		if errors.As(err, new(*substrate.ExitedError)) {
			return AcquireRecorded([]Identity{expected})
		}
		if ctx.Err() == nil {
			if _, exists, rerr := b.registeredPane(ctx, string(pane)); rerr == nil && !exists {
				return AcquireRecorded([]Identity{expected})
			}
		}
		return nil, err
	}
	if len(owned) == 0 {
		return &Owned{}, nil
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
	identities, err := currentOwnedIdentities(owned, boot, readCurrentProcess)
	if err != nil {
		_ = closeOwnedProcesses(owned)
		return nil, err
	}
	out := &Owned{processes: owned, identities: identities}
	return out, nil
}

func currentOwnedIdentities(owned []processIdentity, boot string, read func(int) (processRecord, error)) ([]Identity, error) {
	namespace, err := nativeProcessNamespace()
	if err != nil {
		return nil, err
	}
	var identities []Identity
	for _, p := range owned {
		r, err := read(p.pid)
		if processGone(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if r.started != p.started {
			return nil, fmt.Errorf("process changed during teardown preparation")
		}
		identities = append(identities, Identity{PID: p.pid, Started: p.started, Version: r.version, UniqueID: r.uniqueID, BootID: boot, Namespace: namespace})
	}
	return identities, nil
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
	boot, err := bootIdentity()
	if err != nil {
		return nil, err
	}
	return acquireRecorded(ids, boot, observeProcess)
}

func acquireRecorded(ids []Identity, boot string, observe func(int) (processObservation, error)) (*Owned, error) {
	out := &Owned{}
	for _, id := range ids {
		if id.BootID != boot {
			continue
		}
		if !CanReadIdentity(id) {
			_ = out.Close()
			return nil, fmt.Errorf("recorded process namespace is not visible")
		}
		observation, err := observe(id.PID)
		if processGone(err) {
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
		if processGone(err) && closeErr == nil {
			continue
		}
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
	listedPanes, err := b.Panes(ctx)
	if err != nil {
		return err
	}
	for _, w := range listedPanes {
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
