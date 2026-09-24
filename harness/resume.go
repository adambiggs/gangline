package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ValidateResume verifies supported native transcript identities before spawning.
// Collars without transcript discovery retain their native CLI resume behavior.
// A missing transcript is unknown: it cannot prove ownership by another collar.
func ValidateResume(collar Collar, session string) error {
	unknown := func(reason string) error {
		return fmt.Errorf("cannot verify resume session %q for collar %q: %s", session, collar.Name, reason)
	}
	if collar.Primitives.Telemetry == nil {
		return nil
	}
	if session == "" || strings.ContainsAny(session, `/\\*?[]`) || session == "." || session == ".." {
		return unknown("expected a native session ID")
	}
	if len(collar.Launch.ResumeArgs) == 0 {
		return unknown("collar has no supported native resume discovery")
	}
	env := func(key string) string {
		if value, ok := collar.Launch.Env[key]; ok {
			return value
		}
		return os.Getenv(key)
	}
	home := env("HOME")
	var root, pattern string
	switch collar.Primitives.Telemetry.Name {
	case "codex-session-log":
		root = env("CODEX_HOME")
		if root == "" && home != "" {
			root = filepath.Join(home, ".codex")
		}
		pattern = filepath.Join("sessions", "*", "*", "*", "rollout-*-"+session+".jsonl")
	case "claude-status-line":
		root = env("CLAUDE_CONFIG_DIR")
		if root == "" && home != "" {
			root = filepath.Join(home, ".claude")
		}
		pattern = filepath.Join("projects", "*", session+".jsonl")
	default:
		return nil
	}
	if root == "" {
		return unknown("native configuration directory is unknown")
	}
	// Escape the literal root; only the native directory layout is a glob.
	root = strings.NewReplacer("\\", "\\\\", "[", "\\[", "*", "\\*", "?", "\\?").Replace(root)
	paths, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return unknown(err.Error())
	}
	if len(paths) == 0 {
		return unknown("no matching native transcript found")
	}
	var evidenceErr error
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			evidenceErr = err
			continue
		}
		if collar.Primitives.Telemetry.Name == "codex-session-log" {
			_, err = transcriptHeader(file, session)
		} else {
			err = claudeResumeIdentity(file, session)
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			return nil
		}
		evidenceErr = err
	}
	return unknown(evidenceErr.Error())
}

func claudeResumeIdentity(input io.Reader, session string) error {
	scanner := bufio.NewScanner(io.LimitReader(input, transcriptWindow))
	scanner.Buffer(make([]byte, 4096), transcriptWindow)
	for scanner.Scan() {
		var record struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("decode native transcript identity: %w", err)
		}
		if record.SessionID != "" {
			if record.SessionID != session {
				return fmt.Errorf("native transcript belongs to another session")
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("native transcript carries no session identity in its header")
}
