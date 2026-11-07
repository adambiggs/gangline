//go:build acceptance && claude_native

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestClaudeNativeSendIsUnverifiedWithoutKeystrokeReplay(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "native-receiver" })
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	home, err := f.cmd.userHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(registry, 0700); err != nil {
		t.Fatal(err)
	}
	// Short root avoids the Unix socket pathname budget in nested test dirs.
	socketRoot, err := os.MkdirTemp("", "c44-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketRoot)
	socket := filepath.Join(socketRoot, "receiver.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0600); err != nil {
		t.Fatal(err)
	}
	peer := claudePeerSession{PID: os.Getpid(), SessionID: a.Native.SessionID, Protocol: 1, Socket: socket, Tmux: "unit:@1." + a.Pane}
	record, _ := json.Marshal(peer)
	if err := os.WriteFile(filepath.Join(registry, strconv.Itoa(peer.PID)+".json"), record, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(socket))
	key := strconv.Itoa(peer.PID) + "." + hex.EncodeToString(hash[:]) + ".key"
	if err := os.WriteFile(filepath.Join(registry, key), []byte(`{"peerToken":"00000000000000000000000000000000"}`), 0600); err != nil {
		t.Fatal(err)
	}
	type captured struct {
		data []byte
		err  error
	}
	got := make(chan captured, 2)
	go func() {
		for range 2 {
			conn, err := listener.Accept()
			if err != nil {
				got <- captured{err: err}
				return
			}
			data, err := io.ReadAll(conn)
			_ = conn.Close()
			got <- captured{data, err}
		}
	}()
	_, _ = holdFor(t, f, a, core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, "first queued message")
	err = f.cmd.send([]string{"worker", "--from", "operator", "native envelope\nsecond line"})
	var unknown commandError
	if !errors.As(err, &unknown) || unknown.status != exitUnknown {
		t.Fatalf("send error = %v, want unverified", err)
	}
	if f.input.submits != 0 || f.input.pasted != "" {
		t.Fatalf("native send used keystrokes")
	}
	var targetSeen, heldSeen bool
	for range 2 {
		capture := <-got
		if capture.err != nil {
			t.Fatal(capture.err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(capture.data)))
		var auth struct {
			Type string `json:"type"`
		}
		var frame struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := decoder.Decode(&auth); err != nil {
			t.Fatal(err)
		}
		if err := decoder.Decode(&frame); err != nil {
			t.Fatal(err)
		}
		if auth.Type != "auth" || !strings.Contains(frame.Message.Content, "[gang:self-declared:operator#") {
			t.Fatal("native wire lost auth or envelope")
		}
		targetSeen = targetSeen || strings.Contains(frame.Message.Content, "native envelope\nsecond line")
		heldSeen = heldSeen || strings.Contains(frame.Message.Content, "first queued message")
	}
	if !targetSeen || !heldSeen {
		t.Fatal("drain did not write both queued messages")
	}
	if pending, err := p.ListNew(); err != nil || len(pending) != 0 {
		t.Fatalf("pending=%d error=%v", len(pending), err)
	}
	state, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	settled, err := p.ReadEnvelope("failed", state.LastFailed)
	if err != nil || settled.Outcome != "unverified" || !strings.Contains(settled.Reason, "no native delivery receipt") {
		t.Fatalf("settled outcome=%s, error=%v", settled.Outcome, err)
	}
	audit, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"type":"delivery_unverified"`) != 2 {
		t.Fatal("audit lacks two uncertain native outcomes")
	}
	f.run.cmd = f.cmd
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatal("uncertain native input was replayed through keystrokes")
	}
}

func TestClaudeNativeMissingRecordFallsBackToKeystrokes(t *testing.T) {
	f, a, _ := claudeRecipient(t)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
	if err := f.cmd.send([]string{"worker", "--from", "operator", "fallback envelope"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, "fallback envelope") || !strings.Contains(f.out.String(), "delivered") {
		t.Fatalf("fallback submit=%d output=%q", f.input.submits, f.out.String())
	}
}
