//go:build claude_native

package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// This experiment sends ordinary messages only. Startup, notices, resume and
// capacity input retain their existing exact submit-hook witnesses.
func (run *runtime) tryClaudeNative(l *store.LockedAgent, a *core.Agent, e core.Envelope, c harness.Collar) (string, bool, error) {
	if c.Name != "claude" || e.Purpose != "" || e.From.Kind == core.SenderGangline || a.Activity == core.Interrupting || a.Compaction != nil {
		return "", false, nil
	}
	home, err := run.cmd.userHomeDir()
	if err != nil {
		return "", false, err
	}
	root, err := os.OpenRoot(filepath.Join(home, ".claude", "sessions"))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("open Claude session registry: %w", err)
	}
	defer root.Close()
	peer, available, err := findClaudePeer(root.FS(), a.Native.SessionID, run.settings.Session, a.Pane)
	if err != nil || !available {
		return "", false, err
	}
	if !filepath.IsAbs(peer.Socket) || filepath.Clean(peer.Socket) != peer.Socket {
		return "", false, fmt.Errorf("Claude peer socket path is not canonical and absolute")
	}
	info, err := os.Lstat(peer.Socket)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect Claude peer socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 {
		return "", false, fmt.Errorf("Claude peer endpoint is not a private socket")
	}
	if err := run.checkRecipient(*a); err != nil {
		return "", false, err
	}
	wire, err := envelopeText(e)
	if err != nil {
		return "", false, err
	}
	token, err := claudePeerKey(root.FS(), peer)
	if err != nil {
		return "", false, err
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", peer.Socket)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("connect Claude peer socket: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return "", false, err
		}
	}
	// Cancellation closes a stalled write without a second goroutine waiting
	// beyond the command's existing operation budget.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", false, err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	messageID := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return "", false, err
	}
	reason := "Claude peer socket written; no native delivery receipt is available; do not resend"
	if err := writeClaudePeer(conn, token, peer.SessionID, messageID, wire); err != nil {
		reason = err.Error() + "; do not resend"
	}
	// Even a complete write can be held, refused or lost. Never replay through
	// keystrokes after attempting the native write.
	if err := run.finishInput(l, a, e, "unverified", reason); err != nil {
		return "unverified", true, err
	}
	return "unverified", true, run.mark(*a)
}
