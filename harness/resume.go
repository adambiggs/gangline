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

// ValidateResume refuses a resume only when a native transcript stored under
// the session ID names a different session. Every state it cannot judge (no
// native store, no transcript, an unreadable header) is left to the native CLI
// and returned as unverified.
func ValidateResume(collar Collar, session string) (unverified string, err error) {
	refuse := func(reason string) error {
		return fmt.Errorf("refuse resume session %q for collar %q: %s", session, collar.Name, reason)
	}
	if collar.Primitives.Telemetry == nil {
		return "", nil
	}
	if session == "" || strings.ContainsAny(session, `/\\*?[]`) || session == "." || session == ".." {
		return "", refuse("expected a native session ID")
	}
	if len(collar.Launch.ResumeArgs) == 0 {
		return "", refuse("collar declares no native resume arguments")
	}
	env := func(key string) string {
		if value, ok := collar.Launch.Env[key]; ok {
			return value
		}
		return os.Getenv(key)
	}
	home := env("HOME")
	var root, pattern string
	var identity func(io.ReadSeeker) (string, error)
	switch collar.Primitives.Telemetry.Name {
	case "codex-session-log":
		root = env("CODEX_HOME")
		if root == "" && home != "" {
			root = filepath.Join(home, ".codex")
		}
		pattern = filepath.Join("sessions", "*", "*", "*", "rollout-*-"+session+".jsonl")
		identity = func(input io.ReadSeeker) (string, error) {
			id, _, err := codexTranscriptIdentity(input)
			return id, err
		}
	case "claude-status-line":
		root = env("CLAUDE_CONFIG_DIR")
		if root == "" && home != "" {
			root = filepath.Join(home, ".claude")
		}
		pattern = filepath.Join("projects", "*", session+".jsonl")
		identity = func(input io.ReadSeeker) (string, error) { return claudeTranscriptIdentity(input) }
	default:
		return "", nil
	}
	if root == "" {
		return "native configuration directory is unknown", nil
	}
	// Escape the literal root; only the native directory layout is a glob.
	root = strings.NewReplacer("\\", "\\\\", "[", "\\[", "*", "\\*", "?", "\\?").Replace(root)
	paths, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return err.Error(), nil
	}
	if len(paths) == 0 {
		return "no native transcript found", nil
	}
	foreign, unknown := "", ""
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			unknown = err.Error()
			continue
		}
		id, err := identity(file)
		file.Close()
		switch {
		case err != nil:
			unknown = fmt.Sprintf("%s: %v", path, err)
		case id == session:
			return "", nil
		default:
			foreign = id
		}
	}
	if unknown != "" {
		return unknown, nil
	}
	return "", refuse(fmt.Sprintf("its native transcript belongs to session %q", foreign))
}

// claudeTranscriptIdentity reads the native session named by the first record
// of a claude transcript that carries one.
func claudeTranscriptIdentity(input io.Reader) (string, error) {
	scanner := bufio.NewScanner(io.LimitReader(input, transcriptWindow))
	scanner.Buffer(make([]byte, 4096), transcriptWindow)
	for scanner.Scan() {
		var record struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return "", fmt.Errorf("decode native transcript identity: %w", err)
		}
		if record.SessionID != "" {
			return record.SessionID, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("native transcript carries no session identity in its header")
}
