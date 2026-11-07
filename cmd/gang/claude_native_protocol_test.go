package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/adambiggs/gangline/core"
)

func claudeRecord(t *testing.T, pid int, session, tmux string, protocol int, socket string) []byte {
	t.Helper()
	data, err := json.Marshal(claudePeerSession{PID: pid, SessionID: session, Protocol: protocol, Socket: socket, Tmux: tmux})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFindClaudePeerRequiresSessionTmuxAndPIDMatch(t *testing.T) {
	files := fstest.MapFS{
		"101.json":       {Data: claudeRecord(t, 101, "native-wanted", "team:@4.7", 1, "/tmp/socket")},
		"102.json":       {Data: claudeRecord(t, 102, "native-other", "team:@4.7", 1, "/tmp/socket")},
		"103.json":       {Data: claudeRecord(t, 103, "native-wanted", "other:@4.7", 1, "/tmp/socket")},
		"104.json":       {Data: claudeRecord(t, 104, "native-wanted", "team:@4.8", 1, "/tmp/socket")},
		"105.json":       {Data: claudeRecord(t, 999, "native-wanted", "team:@4.7", 1, "/tmp/socket")},
		"not-a-pid.json": {Data: claudeRecord(t, 106, "native-wanted", "team:@4.7", 1, "/tmp/socket")},
	}
	got, ok, err := findClaudePeer(files, "native-wanted", "team", "7")
	if err != nil || !ok || got.PID != 101 || got.Tmux != "team:@4.7" {
		t.Fatalf("findClaudePeer = (%+v, %t, %v), want PID 101 match", got, ok, err)
	}
	for _, query := range []struct{ session, team, pane string }{{"native-absent", "team", "7"}, {"native-wanted", "missing-team", "7"}, {"native-wanted", "team", "9"}, {"", "team", "7"}} {
		if _, ok, err := findClaudePeer(files, query.session, query.team, query.pane); err != nil || ok {
			t.Fatalf("findClaudePeer(%q, %q, %q) = ok %t, err %v; want unavailable", query.session, query.team, query.pane, ok, err)
		}
	}
}

func TestFindClaudePeerUnavailableAndAmbiguous(t *testing.T) {
	if _, ok, err := findClaudePeer(fstest.MapFS{}, "native", "team", "1"); err != nil || ok {
		t.Fatalf("absent registry = ok %t, err %v; want unavailable", ok, err)
	}
	for name, record := range map[string]claudePeerSession{
		"unknown protocol": {PID: 1, SessionID: "native", Protocol: 2, Socket: "/tmp/socket", Tmux: "team:@1.2"},
		"missing socket":   {PID: 1, SessionID: "native", Protocol: 1, Tmux: "team:@1.2"},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			_, ok, err := findClaudePeer(fstest.MapFS{"1.json": {Data: data}}, "native", "team", "2")
			if err != nil || ok {
				t.Fatalf("findClaudePeer = ok %t, err %v; want unavailable", ok, err)
			}
		})
	}
	files := fstest.MapFS{
		"1.json": {Data: claudeRecord(t, 1, "native", "team:@1.2", 1, "/tmp/a")},
		"2.json": {Data: claudeRecord(t, 2, "native", "team:@1.2", 1, "/tmp/b")},
	}
	if _, ok, err := findClaudePeer(files, "native", "team", "2"); err == nil || ok || !strings.Contains(err.Error(), "multiple Claude records") {
		t.Fatalf("ambiguous lookup = ok %t, err %v; want ambiguity error", ok, err)
	}
}

func TestClaudePeerKeyUsesSyntheticSocketHashAndPrivateRegularFile(t *testing.T) {
	peer := claudePeerSession{PID: 345, Socket: "/tmp/team/../team/peer.sock"}
	hash := sha256.Sum256([]byte(path.Clean(peer.Socket)))
	name := strconv.Itoa(peer.PID) + "." + hex.EncodeToString(hash[:]) + ".key"
	const credential = "0123456789abcdef0123456789abcdef"
	good := fstest.MapFS{name: {Data: []byte(`{"peerToken":"` + credential + `"}`), Mode: 0600}}
	got, err := claudePeerKey(good, peer)
	if err != nil || got != credential {
		t.Fatalf("claudePeerKey = %q, %v; want synthetic token", got, err)
	}
	if _, err := claudePeerKey(fstest.MapFS{}, peer); err == nil {
		t.Fatal("missing synthetic key accepted")
	}
	for label, entry := range map[string]*fstest.MapFile{
		"private mode": {Data: []byte(`{"peerToken":"` + credential + `"}`), Mode: 0644},
		"directory":    {Data: []byte(`{"peerToken":"` + credential + `"}`), Mode: fs.ModeDir | 0700},
		"malformed":    {Data: []byte(`{"peerToken":"secret malformed"}`), Mode: 0600},
	} {
		t.Run(label, func(t *testing.T) {
			if _, err := claudePeerKey(fstest.MapFS{name: entry}, peer); err == nil {
				t.Fatal("invalid key accepted")
			}
		})
	}
	const malformedSecret = `{"peerToken":"sensitive-key-material",`
	_, err = claudePeerKey(fstest.MapFS{name: {Data: []byte(malformedSecret), Mode: 0600}}, peer)
	if err == nil || strings.Contains(err.Error(), "sensitive-key-material") || strings.Contains(err.Error(), malformedSecret) {
		t.Fatalf("malformed-key error = %v; want a sanitized error", err)
	}
}

func TestWriteClaudePeerWireFramesPreserveEnvelope(t *testing.T) {
	const token = "0123456789abcdef"
	body := "first line\nsecond line — café 🐈"
	tests := []struct {
		name, label string
		from        core.Sender
	}{
		{"agent", "lead", core.Sender{Kind: core.SenderAgent, Name: "lead", HitchID: "lead-hitch"}},
		{"self-declared", "self-declared:relay", core.Sender{Kind: core.SenderSelfDeclared, Name: "relay"}},
		{"registered", "registered:relay", core.Sender{Kind: core.SenderRegistered, Name: "relay", HitchID: "relay-hitch"}},
		{"peer agent", "host-a:relay", core.Sender{Kind: core.SenderPeer, Name: "relay", Host: "host-a", PeerClass: core.SenderAgent}},
		{"peer registered", "host-a:registered:relay", core.Sender{Kind: core.SenderPeer, Name: "relay", Host: "host-a", PeerClass: core.SenderRegistered}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envelope := core.Envelope{ID: "env-1", Token: token, From: tc.from, To: "worker", Recipient: "hitch-2", Message: core.Message{Text: body}}
			wire, err := envelopeText(envelope)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := writeClaudePeer(&out, "synthetic-token", "native-session", "message-1", wire); err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(&out)
			var auth struct {
				Type  string `json:"type"`
				Token string `json:"token"`
			}
			if err := decoder.Decode(&auth); err != nil {
				t.Fatal(err)
			}
			if auth.Type != "auth" || auth.Token != "synthetic-token" {
				t.Fatalf("auth frame = %+v", auth)
			}
			var frame struct {
				Version   int    `json:"msgV"`
				ID        string `json:"msg_id"`
				Type      string `json:"type"`
				SessionID string `json:"session_id"`
				Priority  string `json:"priority"`
				Message   struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			}
			if err := decoder.Decode(&frame); err != nil {
				t.Fatal(err)
			}
			if frame.Version != 1 || frame.ID != "message-1" || frame.Type != "user" || frame.SessionID != "native-session" || frame.Priority != "next" || frame.Message.Role != "user" {
				t.Fatalf("peer frame = %+v", frame)
			}
			wantWire := "[gang:" + tc.label + "#" + token + "] " + body + " [/gang:" + tc.label + "#" + token + "]"
			wantContent := "<cross-session-message>\n" + wantWire + "\n</cross-session-message>"
			if frame.Message.Content != wantContent {
				t.Fatalf("peer content = %q, want exact envelope %q", frame.Message.Content, wantContent)
			}
		})
	}
}

type failingClaudeWriter struct{ remaining int }

func (w *failingClaudeWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("synthetic credential leaked by writer")
	}
	w.remaining--
	return len(p), nil
}

func TestWriteClaudePeerFailuresAreSanitizedAndUnverified(t *testing.T) {
	const token, secretEnvelope = "secret-peer-token", `{"token":"secret-envelope"}`
	for _, tc := range []struct {
		name   string
		writer io.Writer
		want   string
	}{
		{"auth frame", &failingClaudeWriter{remaining: 0}, "write Claude peer authentication"},
		{"message frame", &failingClaudeWriter{remaining: 1}, "receipt unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := writeClaudePeer(tc.writer, token, "native", "message", secretEnvelope)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("write error = %v; want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), secretEnvelope) || strings.Contains(err.Error(), "synthetic credential leaked") {
				t.Fatalf("write error leaked sensitive detail: %v", err)
			}
		})
	}
}
