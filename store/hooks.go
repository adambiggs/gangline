package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// StatusHook retains native status observations awaiting reconciliation.
// Prompt receipts remain in Witness and never share this replacement policy.
type StatusHook struct {
	Sequence    uint64    `json:"sequence"`
	Kind        string    `json:"kind"`
	NativeEvent string    `json:"native_event"`
	At          time.Time `json:"at"`
	SessionID   string    `json:"session_id,omitempty"`
	TurnID      string    `json:"turn_id,omitempty"`
	Transcript  string    `json:"transcript,omitempty"`
	Failure     string    `json:"failure,omitempty"`
}

func (p AgentPaths) ReadStatusHooks() ([]StatusHook, error) {
	var records []StatusHook
	err := readJSON(filepath.Join(p.Directory, "status-hooks"), &records)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return records, err
}

// The short writer lock is independent of native input's agent lock. Both
// files live inside the agent directory and disappear with its registration.
func (p AgentPaths) WriteStatusHook(n StatusHook) error {
	switch n.Kind {
	case "turn-started", "turn-finished", "turn-failed", "activity", "permission-requested", "compaction-started", "compaction-finished":
	default:
		return fmt.Errorf("unsupported status hook %q", n.Kind)
	}
	f, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer f.Close()
	records, err := p.ReadStatusHooks()
	if err != nil {
		return err
	}
	a, err := p.Read()
	if err != nil {
		return err
	}
	terminal := n.Kind == "turn-finished" || n.Kind == "turn-failed" || n.Kind == "compaction-finished"
	var sequence uint64
	for _, old := range records {
		if old.Sequence > sequence {
			sequence = old.Sequence
		}
		if !terminal && old.Kind == n.Kind && old.At.After(n.At) {
			return nil
		}
	}
	n.Sequence = sequence + 1
	next := make([]StatusHook, 0, len(records)+1)
	for _, old := range records {
		if old.Kind != n.Kind || terminal && old.Sequence > a.Native.HookSequence {
			next = append(next, old)
		}
	}
	next = append(next, n)
	sort.Slice(next, func(i, j int) bool { return next[i].Sequence < next[j].Sequence })
	return atomicJSON(filepath.Join(p.Directory, "status-hooks"), next)
}

func (p AgentPaths) lockStatusHooks() (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(p.Directory, "status-hooks.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// Keep the latest reading of each kind and every unacknowledged boundary.
// Pruning after acknowledgment keeps settled history out of later sweeps.
func (p AgentPaths) PruneStatusHooks(sequence uint64) error {
	f, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer f.Close()
	records, err := p.ReadStatusHooks()
	if err != nil {
		return err
	}
	latest := map[string]uint64{}
	for _, n := range records {
		latest[n.Kind] = n.Sequence
	}
	kept := make([]StatusHook, 0, len(latest))
	for _, n := range records {
		if n.Sequence > sequence || latest[n.Kind] == n.Sequence {
			kept = append(kept, n)
		}
	}
	if len(kept) == len(records) {
		return nil
	}
	return atomicJSON(filepath.Join(p.Directory, "status-hooks"), kept)
}
