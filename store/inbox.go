package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
)

func (p AgentPaths) EnvelopePath(dir string, id core.EnvelopeID) (string, error) {
	if err := segment(string(id)); err != nil {
		return "", err
	}
	if dir != "new" && dir != "cur" && dir != "failed" && dir != "tmp/drop" {
		return "", fmt.Errorf("invalid inbox directory %q", dir)
	}
	return filepath.Join(p.Inbox, dir, string(id)+".json"), nil
}
func (p AgentPaths) Publish(e core.Envelope) error {
	path, err := p.EnvelopePath("new", e.ID)
	if err != nil {
		return err
	}
	tmp, err := jsonTemp(filepath.Join(p.Inbox, "tmp"), e)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return os.Rename(tmp, path)
}

// Withdraw removes an undelivered envelope without a result, so the agent's
// retained receipts are untouched. The team log keeps its audit history.
func (l *LockedAgent) Withdraw(id core.EnvelopeID) error {
	path, err := l.Paths.EnvelopePath("new", id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
func (p AgentPaths) ReadEnvelope(dir string, id core.EnvelopeID) (core.Envelope, error) {
	var e core.Envelope
	path, err := p.EnvelopePath(dir, id)
	if err != nil {
		return e, err
	}
	err = readJSON(path, &e)
	if err == nil && e.ID != id {
		err = fmt.Errorf("decode %s: envelope ID does not match its filename", path)
	}
	return e, err
}
func (p AgentPaths) ListNew() ([]core.Envelope, error) { return p.list("new") }
func (p AgentPaths) list(dir string) ([]core.Envelope, error) {
	entries, err := os.ReadDir(filepath.Join(p.Inbox, dir))
	if err != nil {
		return nil, err
	}
	var envelopes []core.Envelope
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("unexpected inbox directory %s", entry.Name())
		}
		var e core.Envelope
		if err := readJSON(filepath.Join(p.Inbox, dir, entry.Name()), &e); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if entry.Name() != string(e.ID)+".json" {
			return nil, fmt.Errorf("inbox filename %s does not match envelope ID", entry.Name())
		}
		envelopes = append(envelopes, e)
	}
	sort.Slice(envelopes, func(i, j int) bool {
		if envelopes[i].CreatedAt.Equal(envelopes[j].CreatedAt) {
			left, right := string(envelopes[i].ID), string(envelopes[j].ID)
			// Context sequences preserve crossing order even when readings share a timestamp.
			if strings.HasPrefix(left, "context-") && strings.HasPrefix(right, "context-") {
				l, le := strconv.ParseUint(strings.TrimPrefix(left, "context-"), 10, 64)
				r, re := strconv.ParseUint(strings.TrimPrefix(right, "context-"), 10, 64)
				if (le == nil) != (re == nil) {
					return le == nil
				}
				if le == nil && re == nil && l != r {
					return l < r
				}
			}
			return left < right
		}
		return envelopes[i].CreatedAt.Before(envelopes[j].CreatedAt)
	})
	return envelopes, nil
}
func (l *LockedAgent) CleanResult(a *core.Agent) error {
	if a.Cleanup == nil {
		return nil
	}
	p, err := l.Paths.EnvelopePath(a.Cleanup.Directory, a.Cleanup.ID)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	a.Cleanup = nil
	return l.Save(*a)
}

// Settle records the prior result until deletion succeeds, so a crash cannot
// accumulate terminal envelopes. Audit history remains in the team log.
func (l *LockedAgent) Settle(a *core.Agent, e core.Envelope, outcome, reason string) error {
	if err := l.CleanResult(a); err != nil {
		return err
	}
	dir := "failed"
	previous := a.LastFailed
	if outcome == "delivered" {
		dir = "cur"
		previous = a.LastDelivered
	} else if outcome == "accepted" {
		dir = "cur"
		previous = a.LastAccepted
	}
	if err := l.fileResult(e, dir, outcome, reason); err != nil {
		return err
	}
	if previous != "" && previous != e.ID {
		a.Cleanup = &core.ResultRef{ID: previous, Directory: dir}
	}
	if outcome == "accepted" {
		a.LastAccepted = e.ID
	} else if dir == "cur" {
		a.LastDelivered = e.ID
	} else {
		a.LastFailed = e.ID
	}
	if err := l.Save(*a); err != nil {
		return err
	}
	return l.CleanResult(a)
}

// FileFailure files a failed result without taking the agent's receipt, so a
// retained failure stays in force.
func (l *LockedAgent) FileFailure(e core.Envelope, outcome, reason string) error {
	return l.fileResult(e, "failed", outcome, reason)
}
func (l *LockedAgent) fileResult(e core.Envelope, dir, outcome, reason string) error {
	e.Outcome, e.Reason = outcome, reason
	from, err := l.Paths.EnvelopePath("new", e.ID)
	if err != nil {
		return err
	}
	to, _ := l.Paths.EnvelopePath(dir, e.ID)
	if err := atomicJSON(from, e); err != nil {
		return err
	}
	return os.Rename(from, to)
}
func (l *LockedAgent) ClearScheduled(sender core.Sender) ([]core.Envelope, error) {
	envelopes, err := l.Paths.ListNew()
	if err != nil {
		return nil, err
	}
	var removed []core.Envelope
	for _, e := range envelopes {
		if e.NotBefore.IsZero() || !e.From.SameIdentity(sender) {
			continue
		}
		p, _ := l.Paths.EnvelopePath("new", e.ID)
		if err := os.Remove(p); err != nil {
			return removed, err
		}
		removed = append(removed, e)
	}
	return removed, nil
}
func (l *LockedAgent) SealInbox() ([]core.Envelope, error) {
	from, to := filepath.Join(l.Paths.Inbox, "new"), filepath.Join(l.Paths.Inbox, "tmp", "drop")
	if err := os.Rename(from, to); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if _, err := os.Stat(to); err != nil {
			return nil, err
		}
	}
	return l.Paths.list("tmp/drop")
}

type Witness struct {
	ID         string    `json:"id"`
	At         time.Time `json:"at"`
	Prompt     string    `json:"prompt"`
	SessionID  string    `json:"session_id,omitempty"`
	TurnID     string    `json:"turn_id,omitempty"`
	Transcript string    `json:"transcript,omitempty"`
	// A witness an earlier release wrote can carry joined; it is read and
	// discarded so that witness still decodes.
	Joined Ignored `json:"joined,omitzero"`
}

// Ignored reads any JSON value, keeps nothing, and is never written. It holds
// the place of a field that state an earlier release wrote can carry.
type Ignored struct{}

func (*Ignored) UnmarshalJSON([]byte) error { return nil }

func (p AgentPaths) WriteWitness(w Witness) error { return atomicJSON(p.Witness, w) }
func (p AgentPaths) ReadWitness() (Witness, error) {
	var w Witness
	err := readJSON(p.Witness, &w)
	return w, err
}

type CompactionWitness struct {
	At        time.Time `json:"at"`
	SessionID string    `json:"session_id"`
}

func (p AgentPaths) WriteCompactionWitness(w CompactionWitness) error {
	return atomicJSON(p.CompactionWitness, w)
}

func (p AgentPaths) ReadCompactionWitness() (CompactionWitness, error) {
	var w CompactionWitness
	err := readJSON(p.CompactionWitness, &w)
	return w, err
}

// Background is the number of background tasks the native harness reported
// pending when its last turn ended. Hooks replace it without the agent lock.
type Background struct {
	At    time.Time `json:"at"`
	Tasks int       `json:"tasks"`
}

func (p AgentPaths) WriteBackground(b Background) error { return atomicJSON(p.Background, b) }

// ReadBackground reports whether a count is recorded.
func (p AgentPaths) ReadBackground() (Background, bool, error) {
	var b Background
	err := readJSON(p.Background, &b)
	if errors.Is(err, os.ErrNotExist) {
		return Background{}, false, nil
	}
	return b, err == nil, err
}

func (p AgentPaths) RemoveBackground() error { return removeIfPresent(p.Background) }

// PermissionWitness records a native permission request that no later hook
// has followed. Hooks replace it without the agent lock.
type PermissionWitness struct {
	At        time.Time `json:"at"`
	SessionID string    `json:"session_id,omitempty"`
}

func (p AgentPaths) WritePermissionWitness(w PermissionWitness) error {
	return atomicJSON(p.Permission, w)
}

// ReadPermissionWitness reports whether a request awaits an answer.
func (p AgentPaths) ReadPermissionWitness() (PermissionWitness, bool, error) {
	var w PermissionWitness
	err := readJSON(p.Permission, &w)
	if errors.Is(err, os.ErrNotExist) {
		return PermissionWitness{}, false, nil
	}
	return w, err == nil, err
}

func (p AgentPaths) RemovePermissionWitness() error { return removeIfPresent(p.Permission) }

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
