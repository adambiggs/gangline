//go:build acceptance

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/adambiggs/gangline/harness"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestClaudeNativeSendIsUnverifiedWithoutKeystrokeReplay(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "native-receiver" })
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	_, _, got := claudeNativeSocketFixture(t, f, a, 2)
	// Scale the existing operation deadline without sleeping. The fake clock
	// starts with the complete budget and expires it in one event barrier.
	now := time.Now()
	f.cmd.clock = func() time.Time { return now }
	f.cmd.newTimeout = func(ctx context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
		return fakeDeadline{ctx, now.Add(budget)}, func() {}
	}
	f.cmd.newWatch = func(string) (changeWait, error) {
		return waitFixture{func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok || deadline.Sub(now) != operationTimeout {
				t.Fatalf("native wait margin=%s", deadline.Sub(now))
			}
			now = deadline
			return context.DeadlineExceeded
		}}, nil
	}
	_, _ = holdFor(t, f, a, core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, "first queued message")
	err := f.cmd.send([]string{"worker", "--from", "operator", "native envelope\nsecond line"})
	var unknown commandError
	if !errors.As(err, &unknown) || unknown.status != exitUnknown {
		t.Fatalf("send error = %v, want unverified", err)
	}
	if f.input.submits != 0 || f.input.pasted != "" {
		t.Fatalf("native send used keystrokes")
	}
	var targetSeen, heldSeen bool
	var targetPrompt, heldPrompt string
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
		if strings.Contains(frame.Message.Content, "native envelope\nsecond line") {
			targetPrompt = frame.Message.Content
		}
		heldSeen = heldSeen || strings.Contains(frame.Message.Content, "first queued message")
		if strings.Contains(frame.Message.Content, "first queued message") {
			heldPrompt = frame.Message.Content
		}
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
	if err != nil || settled.Outcome != "unverified" || !strings.Contains(settled.Reason, "awaiting exact native submit hook") {
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
	// The real peer prompt reaches UserPromptSubmit with its outer wrapper.
	// Only that exact hook, from the bound native session, confirms delivery.
	for i, prompt := range []string{heldPrompt, targetPrompt} {
		if err := p.WriteNativeWitness(store.Witness{ID: "peer-hook-" + strconv.Itoa(i), At: f.cmd.now(), SessionID: a.Native.SessionID, Prompt: prompt}, a.Native.SessionID); err != nil {
			t.Fatal(err)
		}
	}
	l, confirmed, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	audit, err = os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"type":"delivery_succeeded"`) != 2 || confirmed.LastFailed != "" {
		t.Fatalf("batch did not confirm both receipts: %s", audit)
	}
	if pending, err := p.ListNativePending(); err != nil || len(pending) != 0 {
		t.Fatalf("native pending=%d error=%v", len(pending), err)
	}
	if f.input.submits != 0 || f.input.pasted != "" {
		t.Fatal("native hook confirmation used the composer")
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

type captured struct {
	data []byte
	err  error
}

func claudeNativeSocketFixture(t *testing.T, f *stateFixture, a core.Agent, count int) (string, string, <-chan captured) {
	t.Helper()
	home, err := f.cmd.userHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(registry, 0700); err != nil {
		t.Fatal(err)
	}
	// Short root avoids the Unix socket pathname budget in nested test dirs.
	socketRoot, err := os.MkdirTemp("", "gang-peer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketRoot) })
	socket := filepath.Join(socketRoot, "receiver.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
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
	got := make(chan captured, count)
	go func() {
		for range count {
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
	return registry, key, got
}

func TestClaudeNativeSynchronousHookPreservesStampAndClearsCrossings(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "native-receiver" })
	_, _, got := claudeNativeSocketFixture(t, f, a, 1)
	sender := f.add(t, "sender", "lead", "codex")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	c, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	a.Native.Context = stampReading(f.cmd.now(), 950)
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	if len(a.ContextBands.Pending) == 0 {
		t.Fatal("fixture produced no crossing")
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	e, _ := holdFor(t, f, a, agentSender(sender), "native stamped input")
	e.Crossed = "unknown"
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.cmd.newWatch = func(string) (changeWait, error) {
		return waitFixture{func(context.Context) error {
			pending, err := p.ReadNativePending(e.ID)
			if err != nil {
				return err
			}
			return p.WriteNativeWitness(store.Witness{ID: "immediate-hook", At: f.cmd.now(), SessionID: a.Native.SessionID, TurnID: "current", Prompt: pending.Prompt}, a.Native.SessionID)
		}}, nil
	}
	f.run.cmd = f.cmd
	outcome, err := f.run.drain(a.ID, e.ID)
	if err != nil || outcome != "delivered" {
		t.Fatalf("outcome=%s err=%v", outcome, err)
	}
	capture := <-got
	if capture.err != nil {
		t.Fatal(capture.err)
	}
	var frame struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(capture.data)))
	var auth any
	if err := decoder.Decode(&auth); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&frame); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frame.Message.Content, "[gang ctx=") || !strings.Contains(frame.Message.Content, "crossed: save state and compact/clear") || !strings.Contains(frame.Message.Content, "crossed=") {
		t.Fatalf("native wire lost stamp: %s", frame.Message.Content)
	}
	state, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.ContextBands.Pending) != 0 {
		t.Fatalf("crossings remained: %+v", state.ContextBands)
	}
	requireNoDeliveredNotice(t, f, sender, e.ID)
	// A later receipt deletes the first receipt; a future grace tick must not
	// republish context-N after its action was carried by native ordinary mail.
	l, err = p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	later := stampMail(a, "later", f.cmd.now())
	if err := l.Settle(&state, later, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	now := f.cmd.now().Add(2 * watchdogTimeout)
	f.run.cmd.clock = func() time.Time { return now }
	if err := f.run.publishContextNotes(l, &state); err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("repeated crossing: %+v %v", queued, err)
	}
	if f.input.submits != 0 || f.input.pasted != "" {
		t.Fatal("native confirmation used composer")
	}
}

func TestClaudeNativePrewriteFailuresFallBack(t *testing.T) {
	for _, kind := range []string{"missing key", "key mode", "key FIFO", "ambiguous record", "socket mode"} {
		t.Run(kind, func(t *testing.T) {
			f, a, p := claudeRecipient(t)
			a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "s" })
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			registry, key, _ := claudeNativeSocketFixture(t, f, a, 0)
			switch kind {
			case "missing key":
				if err := os.Remove(filepath.Join(registry, key)); err != nil {
					t.Fatal(err)
				}
			case "key FIFO":
				if err := os.Remove(filepath.Join(registry, key)); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(filepath.Join(registry, key), 0600); err != nil {
					t.Fatal(err)
				}
			case "key mode":
				if err := os.Chmod(filepath.Join(registry, key), 0644); err != nil {
					t.Fatal(err)
				}
			case "ambiguous record":
				path := filepath.Join(registry, strconv.Itoa(os.Getpid())+".json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var peer claudePeerSession
				if err := json.Unmarshal(data, &peer); err != nil {
					t.Fatal(err)
				}
				peer.PID++
				data, err = json.Marshal(peer)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(registry, strconv.Itoa(peer.PID)+".json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "socket mode":
				data, err := os.ReadFile(filepath.Join(registry, strconv.Itoa(os.Getpid())+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var peer claudePeerSession
				if err := json.Unmarshal(data, &peer); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(peer.Socket, 0666); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.cmd.send([]string{"worker", "--from", "operator", "fallback"}); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 1 || !strings.Contains(f.errOut.String(), "using keystrokes") {
				t.Fatalf("no visible fallback: %s", f.errOut)
			}
			if pending, err := p.ListNativePending(); err != nil || len(pending) != 0 {
				t.Fatalf("prewrite pending=%d err=%v", len(pending), err)
			}
		})
	}
}

func TestClaudeNativePendingFailureLeavesMessageQueued(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "native-receiver" })
	_, _, got := claudeNativeSocketFixture(t, f, a, 1)
	f.run.cmd = f.cmd
	l, a, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Directory, "native-pending"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	e, _ := holdFor(t, f, a, core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, "queued")
	f.run.cmd = f.cmd
	_, handled, err := f.run.tryClaudeNative(l, &a, e, f.input, c, false)
	if handled {
		t.Fatal("unwritten message handled")
	}
	if err == nil {
		t.Fatal("pending persistence failure was hidden")
	}
	state, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if state.Input != nil || state.LastFailed != "" {
		t.Fatalf("unwritten message marked uncertain: %+v", state)
	}
	if _, err := p.ReadEnvelope("new", e.ID); err != nil {
		t.Fatal(err)
	}
	capture := <-got
	if capture.err != nil || len(capture.data) != 0 {
		t.Fatalf("persistence failure wrote native bytes: %d %v", len(capture.data), capture.err)
	}
}

func TestClaudeNativeBusyTargetDoesNotWaitForHook(t *testing.T) {
	f, a, p := claudeRecipient(t)
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID = "native-receiver"; a.Activity = core.Busy })
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	_, _, got := claudeNativeSocketFixture(t, f, a, 1)
	f.cmd.newWatch = func(string) (changeWait, error) {
		t.Fatal("busy native target consumed a witness wait")
		return nil, nil
	}
	before := f.cmd.now()
	err := f.cmd.send([]string{"worker", "--from", "probe", "busy queued input"})
	var unknown commandError
	if !errors.As(err, &unknown) || unknown.status != exitUnknown {
		t.Fatalf("busy outcome=%v", err)
	}
	if !f.cmd.now().Equal(before) {
		t.Fatal("busy target advanced operation budget")
	}
	capture := <-got
	if capture.err != nil || len(capture.data) == 0 {
		t.Fatal("busy target was not written")
	}
	state, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	n, err := p.ReadNativePending(state.LastFailed)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteNativeWitness(store.Witness{ID: "later", At: f.cmd.now(), SessionID: a.Native.SessionID, TurnID: "later-turn", Prompt: n.Prompt}, a.Native.SessionID); err != nil {
		t.Fatal(err)
	}
	f.run.cmd = f.cmd
	l, _, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	receipt, err := p.ReadEnvelope("cur", n.Envelope.ID)
	if err != nil || receipt.Outcome != "delivered" || f.input.submits != 0 {
		t.Fatalf("busy confirmation=%+v err=%v", receipt, err)
	}
}

func TestClaudeNativeFallbackAuditsEachEpisodeOnce(t *testing.T) {
	f, a, p := claudeRecipient(t)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	f.run.cmd = f.cmd
	for range 2 {
		if _, _, err := f.run.claudeNativeFallback(l, a, errors.New("synthetic unavailable registry")); err != nil {
			t.Fatal(err)
		}
	}
	audit, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"type":"native_delivery_fallback"`) != 1 {
		t.Fatalf("repeated fallback audit: %s", audit)
	}
	if err := p.ClearNativeFallback(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.run.claudeNativeFallback(l, a, errors.New("synthetic unavailable registry")); err != nil {
		t.Fatal(err)
	}
	audit, err = os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(audit), `"type":"native_delivery_fallback"`) != 2 {
		t.Fatalf("new episode not audited: %s", audit)
	}
}
