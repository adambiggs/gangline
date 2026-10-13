package store

import (
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
)

// ForeignSubmit retains the raw submit hook that a native session outside an
// agent's conversation sent to the agent's hitch. It is diagnostic evidence,
// never authority to change sessions, and it lives with the team so it
// outlasts the hitch's own state.
type ForeignSubmit struct {
	At         time.Time      `json:"at"`
	HitchID    core.HitchID   `json:"hitch_id"`
	Name       core.AgentName `json:"name"`
	Registered string         `json:"registered_session_id,omitempty"`
	SessionID  string         `json:"session_id"`
	Reason     string         `json:"reason"`
	Payload    string         `json:"payload"`
}

// ForeignSubmitPath is where the latest foreign submit for hitch id is kept.
func (p TeamPaths) ForeignSubmitPath(id core.HitchID) (string, error) {
	if err := segment(string(id)); err != nil {
		return "", err
	}
	return filepath.Join(p.Directory, "foreign-submits", string(id)+".json"), nil
}

// WriteForeignSubmit replaces the hitch's retained foreign submit and returns
// its path.
func (p TeamPaths) WriteForeignSubmit(f ForeignSubmit) (string, error) {
	path, err := p.ForeignSubmitPath(f.HitchID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	return path, atomicJSON(path, f)
}

// ReadForeignSubmit reads the hitch's retained foreign submit.
func (p TeamPaths) ReadForeignSubmit(id core.HitchID) (ForeignSubmit, error) {
	var f ForeignSubmit
	path, err := p.ForeignSubmitPath(id)
	if err != nil {
		return f, err
	}
	return f, readJSON(path, &f)
}
