package store

import "path/filepath"

// SessionConflict preserves the normalized witness rejected at a native session
// boundary. It is diagnostic evidence, never authority to change sessions.
type SessionConflict struct {
	SessionID  string  `json:"registered_session_id"`
	Transcript string  `json:"registered_transcript,omitempty"`
	Witness    Witness `json:"witness"`
}

// WriteSessionConflict replaces one private snapshot. Later submit hooks only
// replace Witness; this snapshot lasts until another conflict or agent removal.
func (p AgentPaths) WriteSessionConflict(c SessionConflict) error {
	return atomicJSON(filepath.Join(p.Directory, "native-session-conflict"), c)
}
