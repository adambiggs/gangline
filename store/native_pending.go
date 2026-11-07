package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adambiggs/gangline/core"
)

// NativePending retains only unresolved native writes. It is separate from
// ordinary receipts so older binaries can still read the inbox on rollback.
type NativePending struct {
	CapacityFingerprint string        `json:"capacity_fingerprint,omitempty"`
	Envelope            core.Envelope `json:"envelope"`
	SessionID           string        `json:"session_id"`
	Prompt              string        `json:"prompt"`
	Witness             *Witness      `json:"witness,omitempty"`
}

func (p AgentPaths) NativePendingPath(id core.EnvelopeID) (string, error) {
	if err := segment(string(id)); err != nil {
		return "", err
	}
	return filepath.Join(p.Directory, "native-pending", string(id)+".json"), nil
}

// BeginNative records evidence before the first byte can reach the peer.
func (p AgentPaths) BeginNative(n NativePending) error {
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	path, err := p.NativePendingPath(n.Envelope.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return atomicJSON(path, n)
}

func (p AgentPaths) ReadNativePending(id core.EnvelopeID) (NativePending, error) {
	var n NativePending
	path, err := p.NativePendingPath(id)
	if err != nil {
		return n, err
	}
	err = readJSON(path, &n)
	if err == nil && n.Envelope.ID != id {
		err = fmt.Errorf("native pending ID does not match filename")
	}
	return n, err
}

func (p AgentPaths) ListNativePending() ([]NativePending, error) {
	entries, err := os.ReadDir(filepath.Join(p.Directory, "native-pending"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending []NativePending
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".write-") {
			continue
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			return nil, fmt.Errorf("unexpected native pending entry %s", entry.Name())
		}
		id := core.EnvelopeID(entry.Name()[:len(entry.Name())-5])
		n, err := p.ReadNativePending(id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		pending = append(pending, n)
	}
	return pending, nil
}

func (p AgentPaths) RemoveNativePending(id core.EnvelopeID) error {
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	path, err := p.NativePendingPath(id)
	if err != nil {
		return err
	}
	return removeIfPresent(path)
}

// Called under the status-hook lock, before replacing the ordinary witness.
// Each exact proof survives subsequent submit hooks and receipt cleanup.
func (p AgentPaths) witnessNativePending(a core.Agent, w Witness) error {
	pending, err := p.ListNativePending()
	if err != nil {
		return err
	}
	for _, n := range pending {
		if n.Witness != nil || w.SessionID == "" || w.SessionID != a.Native.SessionID || w.SessionID != n.SessionID || w.Prompt != n.Prompt || w.At.Before(n.Envelope.CreatedAt) {
			continue
		}
		n.Witness = &w
		path, _ := p.NativePendingPath(n.Envelope.ID)
		if err := atomicJSON(path, n); err != nil {
			return err
		}
	}
	return nil
}

// RetireNativePending removes attempts that can no longer gain a witness.
// A proof recorded before retirement remains available for reconciliation.
func (p AgentPaths) RetireNativePending(session string) error {
	if session == "" {
		return nil
	}
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	pending, err := p.ListNativePending()
	if err != nil {
		return err
	}
	for _, n := range pending {
		if n.SessionID == session || n.Witness != nil {
			continue
		}
		path, _ := p.NativePendingPath(n.Envelope.ID)
		if err := removeIfPresent(path); err != nil {
			return err
		}
	}
	return nil
}

// One fallback reason per episode keeps background downgrades observable
// without growing the audit on every tick. The agent lock serializes it.
func (p AgentPaths) ReadNativeFallback() (string, error) {
	var reason string
	err := readJSON(filepath.Join(p.Directory, "native-fallback"), &reason)
	return reason, err
}
func (p AgentPaths) WriteNativeFallback(reason string) error {
	return atomicJSON(filepath.Join(p.Directory, "native-fallback"), reason)
}
func (p AgentPaths) ClearNativeFallback() error {
	return removeIfPresent(filepath.Join(p.Directory, "native-fallback"))
}
