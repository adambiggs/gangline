package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

// ProcessVisibility establishes whether tmux's PIDs can safely be interpreted
// through this caller's process APIs, before looking up any pane process.
func (b *Backend) ProcessVisibility(ctx context.Context, pane substrate.PaneID) (bool, error) {
	if !numericTmuxID(string(pane), '%') {
		return false, fmt.Errorf("invalid pane id %q", pane)
	}
	out, err := b.run(ctx, "display-message", "-p", "-t", string(pane), "#{socket_path}\t#{pid}\t#{pane_id}")
	if err != nil {
		return b.visibilityFailure(ctx, pane, tmuxError("read tmux process namespace", err, out))
	}
	fields := strings.Split(strings.TrimSuffix(out, "\n"), "\t")
	if len(fields) != 3 || fields[0] == "" || fields[2] != string(pane) {
		return b.visibilityFailure(ctx, pane, fmt.Errorf("invalid tmux process namespace record %q", out))
	}
	pid, err := strconv.Atoi(fields[1])
	if err != nil || pid <= 0 {
		return false, fmt.Errorf("invalid tmux server pid %q", fields[1])
	}
	visible, err := serverProcessVisible(ctx, fields[0], pid)
	if err != nil {
		return b.visibilityFailure(ctx, pane, err)
	}
	if !visible {
		return false, nil
	}
	root, err := b.paneProcess(ctx, pane)
	if err != nil {
		return b.visibilityFailure(ctx, pane, err)
	}
	_, err = readCurrentProcess(root)
	return processReadVisibility(err)
}

func processReadVisibility(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if processGone(err) || errors.Is(err, os.ErrPermission) {
		return false, nil
	}
	return false, err
}

func (b *Backend) visibilityFailure(ctx context.Context, pane substrate.PaneID, failure error) (bool, error) {
	if ctx.Err() == nil {
		if _, exists, err := b.registeredPane(ctx, string(pane)); err == nil && !exists {
			return false, nil
		}
	}
	return false, failure
}

// VerifyCaller checks ancestry when host processes are visible. In an isolated
// PID namespace, registration and native hooks provide caller attribution.
func (b *Backend) VerifyCaller(ctx context.Context, pane substrate.PaneID) error {
	visible, err := b.ProcessVisibility(ctx, pane)
	if err != nil || !visible {
		return err
	}
	root, err := b.paneProcess(ctx, pane)
	if err != nil {
		return err
	}
	return verifyCallerAncestry(os.Getpid(), root, readCurrentProcess)
}

func verifyCallerAncestry(caller, root int, read func(int) (processRecord, error)) error {
	seen := map[int]bool{}
	for pid := caller; pid > 0 && !seen[pid]; {
		if pid == root {
			return nil
		}
		seen[pid] = true
		record, err := read(pid)
		if err != nil {
			return fmt.Errorf("read caller ancestry: %w", err)
		}
		pid = record.ParentPID
	}
	return fmt.Errorf("caller is outside registered pane process ancestry")
}
