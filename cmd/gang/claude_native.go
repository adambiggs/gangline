package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// Native peer input bypasses the composer, including for Gangline notices.
// Startup and native control input retain their dedicated submit witnesses.
func claudeNativeEligible(a core.Agent, e core.Envelope, c harness.Collar) bool {
	return c.Primitives.NativeDelivery != nil && c.Primitives.NativeDelivery.Name == "claude-peer" &&
		e.Purpose == "" && e.Startup == nil && !capacityEnvelope(e) && a.Activity != core.Interrupting && a.Compaction == nil
}

func (run *runtime) tryClaudeNative(l *store.LockedAgent, a *core.Agent, e core.Envelope, b harnessInput, c harness.Collar, wait bool) (string, bool, error) {
	if !claudeNativeEligible(*a, e, c) {
		return "", false, nil
	}
	if err := run.checkRecipient(*a); err != nil {
		return "", false, err
	}
	// A busy or unobserved receiver can defer its hook for an entire turn.
	// Keep those writes asynchronous instead of holding its lock for that turn.
	wait = wait && a.Activity == core.Idle && !a.Native.SubmittedAt.After(a.Native.FinishedAt)
	home, err := run.cmd.userHomeDir()
	if err != nil {
		return run.claudeNativeFallback(l, *a, err)
	}
	root, err := os.OpenRoot(filepath.Join(home, ".claude", "sessions"))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return run.claudeNativeFallback(l, *a, fmt.Errorf("open Claude session registry: %w", err))
	}
	defer root.Close()
	peer, available, err := findClaudePeer(claudePeerRegistry{root}, a.Native.SessionID, run.settings.Session, a.Pane)
	if err != nil {
		return run.claudeNativeFallback(l, *a, err)
	}
	if !available {
		return "", false, nil
	}
	if !filepath.IsAbs(peer.Socket) || filepath.Clean(peer.Socket) != peer.Socket {
		return run.claudeNativeFallback(l, *a, fmt.Errorf("Claude peer socket path is not canonical and absolute"))
	}
	info, err := os.Lstat(peer.Socket)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return run.claudeNativeFallback(l, *a, fmt.Errorf("inspect Claude peer socket: %w", err))
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 {
		return run.claudeNativeFallback(l, *a, fmt.Errorf("Claude peer endpoint is not a private socket"))
	}
	token, err := claudePeerKey(claudePeerRegistry{root}, peer)
	if err != nil {
		return run.claudeNativeFallback(l, *a, err)
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", peer.Socket)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
		return "", false, nil
	}
	if err != nil {
		return run.claudeNativeFallback(l, *a, fmt.Errorf("connect Claude peer socket: %w", err))
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return run.claudeNativeFallback(l, *a, err)
		}
	}
	// Cancellation closes a stalled write without a second goroutine waiting
	// beyond the command's existing operation budget.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := l.Paths.ClearNativeFallback(); err != nil {
		return "", false, err
	}
	e, err = run.stampEnvelope(l, a, e, b, c)
	if err != nil {
		return "", false, err
	}
	wire, err := envelopeText(e)
	if err != nil {
		return "", false, err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", false, err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	messageID := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	if err := l.Paths.BeginNative(store.NativePending{Envelope: e, SessionID: a.Native.SessionID, Prompt: claudePeerBody(wire), CapacityFingerprint: a.Capacity.Fingerprint}); err != nil {
		return "", false, err
	}
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return "", false, errors.Join(err, l.Paths.RemoveNativePending(e.ID))
	}
	reason := "Claude peer socket written; awaiting exact native submit hook; do not resend"
	writeErr := writeClaudePeer(conn, token, peer.SessionID, messageID, wire)
	_ = conn.Close()
	if writeErr != nil {
		reason = writeErr.Error() + "; do not resend"
	}
	// Even a complete write can be held, refused or lost. Never replay through
	// keystrokes after attempting the native write.
	if err := run.finishInput(l, a, e, "unverified", reason); err != nil {
		return "unverified", true, err
	}
	if wait && writeErr == nil {
		if _, err := run.cmd.awaitNativeWitness(ctx, l.Paths, e.ID); err == nil {
			if err := run.reconcileNativePendingExcept(l, a, e.ID); err != nil {
				return "unverified", true, err
			}
			return "delivered", true, run.mark(*a)
		}
	}
	return "unverified", true, run.mark(*a)
}

// Nonblocking opens let the handle's type check reject a substituted FIFO.
type claudePeerRegistry struct{ root *os.Root }

func (r claudePeerRegistry) Open(name string) (fs.File, error) {
	return r.root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func (run *runtime) claudeNativeFallback(l *store.LockedAgent, a core.Agent, reason error) (string, bool, error) {
	previous, err := l.Paths.ReadNativeFallback()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	if previous != reason.Error() {
		if err := run.record(a, core.Event{Type: "native_delivery_fallback", Reason: reason.Error()}); err != nil {
			return "", false, err
		}
		if err := l.Paths.WriteNativeFallback(reason.Error()); err != nil {
			return "", false, err
		}
	}
	if run.cmd.stderr != nil {
		if _, err := fmt.Fprintf(run.cmd.stderr, "warning: Claude peer input unavailable: %v; using keystrokes\n", reason); err != nil {
			return "", false, err
		}
	}
	return "", false, nil
}
