package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
			return envelopes[i].ID < envelopes[j].ID
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
	}
	e.Outcome, e.Reason = outcome, reason
	from, err := l.Paths.EnvelopePath("new", e.ID)
	if err != nil {
		return err
	}
	to, _ := l.Paths.EnvelopePath(dir, e.ID)
	if err := atomicJSON(from, e); err != nil {
		return err
	}
	if err := os.Rename(from, to); err != nil {
		return err
	}
	if previous != "" && previous != e.ID {
		a.Cleanup = &core.ResultRef{ID: previous, Directory: dir}
	}
	if dir == "cur" {
		a.LastDelivered = e.ID
	} else {
		a.LastFailed = e.ID
	}
	if err := l.Save(*a); err != nil {
		return err
	}
	return l.CleanResult(a)
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
}

func (p AgentPaths) WriteWitness(w Witness) error { return atomicJSON(p.Witness, w) }
func (p AgentPaths) ReadWitness() (Witness, error) {
	var w Witness
	err := readJSON(p.Witness, &w)
	return w, err
}
