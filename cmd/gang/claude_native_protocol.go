package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

type claudePeerSession struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Protocol  int    `json:"peerProtocol"`
	Socket    string `json:"messagingSocketPath"`
	Tmux      string `json:"tmux"`
}

// A native session identity and the full tmux session/pane must agree. Pane
// numbers alone recur on separate tmux servers.
func findClaudePeer(files fs.FS, sessionID, team, pane string) (claudePeerSession, bool, error) {
	if sessionID == "" {
		return claudePeerSession{}, false, nil
	}
	entries, err := fs.ReadDir(files, ".")
	if errors.Is(err, fs.ErrNotExist) {
		return claudePeerSession{}, false, nil
	}
	if err != nil {
		return claudePeerSession{}, false, fmt.Errorf("read Claude session registry: %w", err)
	}
	var found claudePeerSession
	for _, entry := range entries {
		name := entry.Name()
		pid, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil || pid <= 0 || name != strconv.Itoa(pid)+".json" {
			continue
		}
		data, err := fs.ReadFile(files, name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return found, false, fmt.Errorf("read Claude session record: %w", err)
		}
		var record claudePeerSession
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		if record.SessionID != sessionID || record.PID != pid || !strings.HasPrefix(record.Tmux, team+":@") || !strings.HasSuffix(record.Tmux, "."+pane) {
			continue
		}
		if found.PID != 0 {
			return found, false, fmt.Errorf("multiple Claude records match the native session and pane")
		}
		found = record
	}
	return found, found.PID != 0 && found.Protocol == 1 && found.Socket != "", nil
}

func claudePeerKey(files fs.FS, peer claudePeerSession) (string, error) {
	sum := sha256.Sum256([]byte(path.Clean(peer.Socket)))
	name := strconv.Itoa(peer.PID) + "." + hex.EncodeToString(sum[:]) + ".key"
	file, err := files.Open(name)
	if err != nil {
		return "", fmt.Errorf("Claude peer key unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("Claude peer key unavailable")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("Claude peer key is not a private regular file")
	}
	var key struct {
		PeerToken string `json:"peerToken"`
	}
	// Claude's own key reader uses a 4096-byte budget. Never include data or
	// decoding errors in diagnostics: the source contains a credential.
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 || json.Unmarshal(data, &key) != nil {
		return "", fmt.Errorf("Claude peer key is malformed")
	}
	decoded, err := hex.DecodeString(key.PeerToken)
	if err != nil || len(decoded) != 16 || strings.ToLower(key.PeerToken) != key.PeerToken {
		return "", fmt.Errorf("Claude peer key is malformed")
	}
	return key.PeerToken, nil
}

// The peer socket has no synchronous acknowledgement. A successful write is
// still unverified; native status receipts use a separate return inbox.
func writeClaudePeer(w io.Writer, token, sessionID, messageID, envelope string) error {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}{"auth", token}); err != nil {
		return fmt.Errorf("write Claude peer authentication")
	}
	frame := struct {
		Version   int    `json:"msgV"`
		ID        string `json:"msg_id"`
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Priority  string `json:"priority"`
		Message   struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	}{Version: 1, ID: messageID, Type: "user", SessionID: sessionID, Priority: "next"}
	frame.Message.Role = "user"
	frame.Message.Content = claudePeerBody(envelope)
	if err := encoder.Encode(frame); err != nil {
		return fmt.Errorf("write Claude peer message; receipt unknown")
	}
	return nil
}

func claudePeerBody(envelope string) string {
	return "<cross-session-message>\n" + envelope + "\n</cross-session-message>"
}
