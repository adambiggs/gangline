package harness

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ResolveTranscript(invocation Invocation, transcript, session string) (string, error) {
	if invocation.Name != "codex-session-log" {
		return "", fmt.Errorf("unknown transcript primitive %q", invocation.Name)
	}
	return resolveTranscript(transcript, session, transcriptIdentity, filepath.Glob)
}

func transcriptIdentity(path string) (string, error) {
	file, err := OpenTranscript(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	identity, _, err := codexTranscriptIdentity(file)
	return identity, err
}

func resolveTranscript(transcript, session string, identity func(string) (string, error), glob func(string) ([]string, error)) (string, error) {
	if session == "" || strings.ContainsAny(session, `/\\*?[]`) || session == "." || session == ".." {
		return "", fmt.Errorf("expected a native session ID")
	}
	current, err := identity(transcript)
	if err != nil {
		return "", err
	}
	if current == session {
		return transcript, nil
	}
	directory := strings.NewReplacer("\\", "\\\\", "[", "\\[", "*", "\\*", "?", "\\?").Replace(filepath.Dir(transcript))
	paths, err := glob(filepath.Join(directory, "rollout-*-"+session+".jsonl"))
	if err != nil {
		return "", err
	}
	resolved := ""
	for _, path := range paths {
		found, err := identity(path)
		if err != nil || found != session {
			continue
		}
		if resolved != "" {
			return "", fmt.Errorf("multiple session logs match native session %q", session)
		}
		resolved = path
	}
	if resolved == "" {
		return "", fmt.Errorf("session log belongs to %q; no verified log for native session %q beside %s", current, session, transcript)
	}
	return resolved, nil
}
