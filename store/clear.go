package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
)

// SessionStart proves which hooks the running native process actually emits.
// Registration prevents a previous launch's witness from authorizing clear.
type SessionStart struct {
	At           time.Time       `json:"at"`
	SessionID    string          `json:"session_id"`
	Transcript   string          `json:"transcript"`
	Source       string          `json:"source"`
	Registration string          `json:"registration"`
	ClearID      core.EnvelopeID `json:"clear_id,omitempty"`
}

func (p AgentPaths) ReadSessionStart() (SessionStart, error) {
	var n SessionStart
	err := readJSON(filepath.Join(p.Directory, "session-start"), &n)
	return n, err
}

func (p AgentPaths) WriteSessionStart(n SessionStart) error {
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	a, err := p.Read()
	if err != nil {
		return err
	}
	if n.Registration == "" || n.Registration != a.Registration.Generation {
		return fmt.Errorf("session start belongs to another registration")
	}
	if n.SessionID == "" || n.Transcript == "" {
		return fmt.Errorf("session start has no native identity")
	}
	if pending := a.Clear; pending != nil {
		if (pending.Status == "queued" || pending.Status == "completed") && n.SessionID != a.Native.SessionID && n.Source != "compact" {
			return fmt.Errorf("session start is not an authorized native transition")
		}
		if n.SessionID == pending.PreviousSession && !pending.StartedAt.IsZero() {
			return fmt.Errorf("session start belongs to the retired native conversation")
		}
		old, readErr := p.ReadSessionStart()
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		if readErr == nil && pending.Status != "completed" && old.ClearID == pending.Envelope.ID && old.SessionID != n.SessionID {
			return fmt.Errorf("clear already confirmed another native conversation")
		}
	}
	if pending := a.Clear; pending != nil && (pending.Status == "submitted" || pending.Status == "continuing" || pending.Status == "unverified") && n.Source == "clear" && n.SessionID != pending.PreviousSession && !n.At.Before(pending.StartedAt) {
		n.ClearID = pending.Envelope.ID
	}
	if n.ClearID != "" {
		old, err := p.ReadSessionStart()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if old.ClearID != n.ClearID {
			if err := errors.Join(p.RemoveBackground(), p.RemovePermissionWitness()); err != nil {
				return err
			}
		}
	}
	return atomicJSON(filepath.Join(p.Directory, "session-start"), n)
}

type ClearAdmission struct {
	ID        core.EnvelopeID `json:"id"`
	SessionID string          `json:"session_id"`
}

func (p AgentPaths) ReadClearAdmission() (ClearAdmission, error) {
	var n ClearAdmission
	err := readJSON(filepath.Join(p.Directory, "clear-admission"), &n)
	return n, err
}

// AdmitClear reserves the continuation once without taking the agent lock:
// delivery holds that lock while its synchronous submit hook runs.
func (p AgentPaths) AdmitClear(id core.EnvelopeID, session, registration string) error {
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	a, err := p.Read()
	if err != nil {
		return err
	}
	pending := a.Clear
	if pending == nil || pending.Envelope.ID != id || pending.Status != "continuing" || a.Registration.Generation != registration {
		return fmt.Errorf("clear continuation is not pending in this registration")
	}
	n, err := p.ReadSessionStart()
	if err != nil {
		return err
	}
	if n.ClearID != id || n.Source != "clear" || n.Registration != registration || n.SessionID != session || session == "" || session == pending.PreviousSession || n.At.Before(pending.StartedAt) {
		return fmt.Errorf("clear continuation has no matching new native session")
	}
	old, err := p.ReadClearAdmission()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if old.ID == id {
		return fmt.Errorf("clear continuation already admitted; do not resend")
	}
	return atomicJSON(filepath.Join(p.Directory, "clear-admission"), ClearAdmission{ID: id, SessionID: session})
}

// WriteDisplayHook serializes display side files with clear confirmation. A
// hook already running when clear completes cannot remove the new session's
// permission request or restore the old session's background count.
func (p AgentPaths) WriteDisplayHook(n StatusHook, background *int, observedSession string) error {
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	a, err := p.Read()
	if err != nil {
		return err
	}
	if a.Clear != nil {
		start, err := p.ReadSessionStart()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		matchedClear := err == nil && start.ClearID == a.Clear.Envelope.ID && n.SessionID == start.SessionID
		if observedSession != a.Native.SessionID && n.SessionID != a.Native.SessionID && !matchedClear {
			return nil
		}
		if err == nil && start.ClearID == a.Clear.Envelope.ID && n.SessionID == a.Clear.PreviousSession {
			return nil
		}
	}
	if n.Kind == "permission-requested" {
		if err := p.WritePermissionWitness(PermissionWitness{At: n.At, SessionID: n.SessionID}); err != nil {
			return err
		}
	} else if err := p.RemovePermissionWitness(); err != nil {
		return err
	}
	switch n.Kind {
	case "turn-finished":
		if background != nil && *background > 0 {
			return p.WriteBackground(Background{At: n.At, Tasks: *background})
		}
	case "turn-started", "turn-failed":
	default:
		return nil
	}
	return p.RemoveBackground()
}
