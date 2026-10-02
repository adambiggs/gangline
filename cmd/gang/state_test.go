package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

type inputFixture struct {
	screen           substrate.Screen
	command, pasted  string
	tmuxCommand      string
	foregroundErr    error
	processErr       error
	submits          int
	registrations    int
	captures         int
	captureErr       error
	releases         int
	released         tmux.PaneIdentity
	releaseErr       error
	paneClosed       bool
	keys             []string
	submit           func(string) error
	onKeys           func(substrate.Keys) error
	registeredSender harnessInput
}

func (b *inputFixture) RegisterPane(_ context.Context, p substrate.PaneID) (tmux.PaneIdentity, error) {
	b.registrations++
	return tmux.PaneIdentity{Generation: strings.Repeat("a", 64), Session: "$1", Pane: string(p)}, nil
}
func (b *inputFixture) Identity(context.Context, substrate.PaneID) (tmux.Identity, error) {
	return tmux.Identity{PID: 7, Started: "fixture", BootID: "fixture", Namespace: "fixture"}, nil
}
func (b *inputFixture) AcquireTree(context.Context, substrate.PaneID, tmux.Identity) (*tmux.Owned, error) {
	return &tmux.Owned{}, nil
}
func (b *inputFixture) RemoveRegisteredNativePane(context.Context, tmux.PaneIdentity, tmux.Identity) error {
	return nil
}
func (b *inputFixture) RemoveRegisteredPane(context.Context, tmux.PaneIdentity) error { return nil }
func (b *inputFixture) CheckPane(context.Context, tmux.PaneIdentity) (bool, error)    { return true, nil }
func (b *inputFixture) PaneClosed(context.Context, tmux.PaneIdentity, tmux.Identity) (bool, error) {
	return b.paneClosed, nil
}
func (b *inputFixture) ProcessVisibility(context.Context, substrate.PaneID) (bool, error) {
	return true, nil
}
func (b *inputFixture) VerifyCaller(context.Context, substrate.PaneID) error { return nil }
func (b *inputFixture) ReleaseRegisteredExit(_ context.Context, id tmux.PaneIdentity) error {
	b.releases++
	b.released = id
	return b.releaseErr
}
func (b *inputFixture) SendRegisteredKeys(ctx context.Context, id tmux.PaneIdentity, _ string, k substrate.Keys) error {
	if b.registeredSender != nil {
		return b.registeredSender.SendKeys(ctx, substrate.PaneID(id.Pane), k)
	}
	return b.SendKeys(ctx, substrate.PaneID(id.Pane), k)
}
func (b *inputFixture) ForegroundCommand(context.Context, substrate.PaneID) (string, error) {
	if b.tmuxCommand != "" {
		return b.tmuxCommand, b.foregroundErr
	}
	return b.command, b.foregroundErr
}

// Capture fails on a done context, as a tmux capture run under it does.
func (b *inputFixture) Capture(ctx context.Context, _ substrate.PaneID) (substrate.Screen, error) {
	b.captures++
	if err := ctx.Err(); err != nil {
		return substrate.Screen{}, err
	}
	return b.screen, b.captureErr
}
func (b *inputFixture) ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	if b.processErr != nil {
		return nil, b.processErr
	}
	return []substrate.Process{{PID: 7, Command: b.command}}, nil
}
func (b *inputFixture) SendKeys(_ context.Context, _ substrate.PaneID, k substrate.Keys) error {
	b.keys = append(b.keys, k.Names...)
	if b.onKeys != nil {
		if err := b.onKeys(k); err != nil {
			return err
		}
	}
	if k.Text != "" {
		b.pasted = strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~")
	}
	if k.Submit {
		b.submits++
		if b.submit != nil {
			return b.submit(b.pasted)
		}
	}
	return nil
}
func screenWithText(lines ...string) substrate.Screen {
	var rows [][]substrate.Cell
	for _, line := range lines {
		var row []substrate.Cell
		for _, r := range line {
			row = append(row, substrate.Cell{Text: string(r)})
		}
		rows = append(rows, row)
	}
	return substrate.Screen{Rows: rows}
}

type stateFixture struct {
	cmd         command
	run         *runtime
	env         map[string]string
	out, errOut *synchronizedBuffer
	input       *inputFixture
}

type synchronizedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

func (b *synchronizedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Len()
}

func (b *synchronizedBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Buffer.Reset()
}

func newStateFixture(t *testing.T) *stateFixture {
	t.Helper()
	root := t.TempDir()
	out, errOut := &synchronizedBuffer{}, &synchronizedBuffer{}
	env := map[string]string{"GANG_STATE_ROOT": root, "GANG_CONFIG_DIR": filepath.Join(root, "config"), "GANG_SESSION": "unit", "GANG_AGENT_NONCE": "fixture-nonce"}
	backend := &inputFixture{screen: screenWithText("READY", "› "), command: "codex"}
	cmd := command{newScheduler: func() watchdogScheduler { return nil }, stdin: strings.NewReader(""), stdout: out, stderr: errOut, getenv: func(k string) string { return env[k] }, getwd: func() (string, error) { return root, nil }, userHomeDir: func() (string, error) { return root, nil }, clock: func() time.Time { return time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC) }, paneBackend: backend, inputBackend: backend, settleInput: func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error { return nil }, detach: func(string, hookNotice) error { return nil }}
	cmd.awaitStartup = func(_ context.Context, _ substrate.PaneID, c harness.Collar) (harness.Startup, substrate.Screen, error) {
		startup, err := harness.InspectStartup(c, backend.screen)
		return startup, backend.screen, err
	}
	run, err := cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if err := run.team.Create(); err != nil {
		t.Fatal(err)
	}
	// This executable supplies a listing only. Tests never contact a tmux server.
	fakeTmux := filepath.Join(root, "tmux")
	if err := os.WriteFile(fakeTmux, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\ncase \"$1\" in list-panes) printf '%%1\\tworker\\n';; has-session) exit 0;; *) exit 91;; esac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	env["GANG_TMUX"] = fakeTmux
	f := &stateFixture{cmd: cmd, run: run, env: env, out: out, errOut: errOut, input: backend}
	backend.submit = func(prompt string) error {
		payload, _ := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt, "session_id": "s"})
		hook := f.cmd
		hook.stdin = bytes.NewReader(payload)
		return hook.hook(nil)
	}
	return f
}

func fakeCodexOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nexit 91\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func (f *stateFixture) add(t *testing.T, id, name, collar string) core.Agent {
	t.Helper()
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(name), Collar: collar, Directory: "/work", Pane: "%1", Registration: core.PaneRegistration{Generation: strings.Repeat("a", 64), Session: "$1", TokenHash: tokenHash("fixture-nonce")}, Process: core.ProcessIdentity{PID: 7, Started: "fixture", BootID: "fixture", Namespace: "fixture"}, Status: core.Active, Activity: core.Idle, CreatedAt: f.cmd.now(), ChangedAt: f.cmd.now()}
	l, err := f.run.team.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return a
}
func TestHooksCompleteWithAllAgentLocksHeld(t *testing.T) {
	for _, name := range []string{"codex", "claude"} {
		t.Run(name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", name)
			other := f.add(t, "b", "other", name)
			for _, id := range []core.HitchID{a.ID, other.ID} {
				p, _ := f.run.team.Agent(id)
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
			}
			otherPath, _ := f.run.team.Agent(other.ID)
			if err := os.WriteFile(otherPath.State, []byte("unreadable as state"), 0600); err != nil {
				t.Fatal(err)
			}
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			c, err := harness.EmbeddedCollar(name)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for native := range c.Hooks.Events {
				payload, _ := json.Marshal(map[string]string{"hook_event_name": native, "prompt": "witness", "session_id": "s"})
				cmd := f.cmd
				cmd.stdin = bytes.NewReader(payload)
				if err := cmd.hook(nil); err != nil {
					t.Fatal(err)
				}
				count++
			}
			if f.errOut.Len() != 0 {
				t.Fatal(f.errOut.String())
			}
			file, err := os.Open(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			seen := 0
			if err := store.ReadLog(file, func(e core.Event) error {
				if e.Type != "native_hook" {
					t.Fatalf("unexpected event %+v", e)
				}
				seen++
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if seen != count {
				t.Fatalf("completed hooks=%d records=%d", count, seen)
			}
		})
	}
}
func TestOneAgentCannotBlockAnotherDelivery(t *testing.T) {
	f := newStateFixture(t)
	x := f.add(t, "x", "blocked", "codex")
	y := f.add(t, "y", "worker", "codex")
	xp, _ := f.run.team.Agent(x.ID)
	held, err := xp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	f.env["GANGLINE_HITCH_ID"] = string(y.ID)
	cmd := f.cmd
	cmd.stdin = strings.NewReader("hello")
	if err := cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !strings.Contains(f.out.String(), "delivered") {
		t.Fatalf("submits=%d output=%s errors=%s", f.input.submits, f.out, f.errOut)
	}
}
func TestSendUsesTmuxForegroundWhenProcessTreeIsUnavailable(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.input.processErr = errors.New("read process tree: pane process 7 was not present")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.stdin = strings.NewReader("send once")
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 || f.input.submits != 1 {
		t.Fatalf("delivery: %+v, submits=%d", pending, f.input.submits)
	}
	if got := f.out.String(); !strings.Contains(got, "\tdelivered\n") {
		t.Fatalf("missing delivered receipt: %q", got)
	}
	if f.errOut.Len() != 0 {
		t.Fatal(f.errOut.String())
	}
}

func TestForeignTmuxForegroundRejectsMatchingProcessFallback(t *testing.T) {
	f := newStateFixture(t)
	f.input.tmuxCommand = "sh"
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	// A matching process-table entry cannot override tmux's foreground command.
	if err := requireHarnessForeground(context.Background(), f.input, "%1", c); err == nil {
		t.Fatal("foreign tmux command accepted")
	}
	f.input.command = "other"
	if err := requireHarnessForeground(context.Background(), f.input, "%1", c); err == nil {
		t.Fatal("foreign foreground process accepted")
	}
}

func TestFailedIdentityLookupDoesNotPublish(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.env["GANG_AGENT_ID"] = "missing"
	f.cmd.stdin = strings.NewReader("send once")
	if err := f.cmd.send([]string{"worker"}); err == nil || !strings.Contains(err.Error(), "hitch identity is not registered") {
		t.Fatalf("identity error = %v", err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 0 || f.input.submits != 0 {
		t.Fatalf("identity failure published: %+v, submits=%d, error=%v", pending, f.input.submits, err)
	}
}

func TestSendFromHitchEnvironmentHasVerifiedEnvelope(t *testing.T) {
	f := newStateFixture(t)
	sender := f.add(t, "sender-id", "lead", "codex")
	recipient := f.add(t, "recipient-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(sender.ID)
	f.env["GANGLINE_HITCH_ID"] = string(recipient.ID)
	f.cmd.stdin = strings.NewReader("one report")
	if err := f.cmd.send([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	id, _, ok := strings.Cut(f.out.String(), "\t")
	if !ok || !strings.Contains(f.out.String(), "\tdelivered\n") {
		t.Fatalf("receipt = %q", f.out.String())
	}
	p, err := f.run.team.Agent(recipient.ID)
	if err != nil {
		t.Fatal(err)
	}
	e, err := p.ReadEnvelope("cur", core.EnvelopeID(id))
	if err != nil {
		t.Fatal(err)
	}
	if e.From != (core.Sender{Kind: core.SenderAgent, Name: sender.Name, HitchID: sender.ID}) {
		t.Fatalf("sender = %+v", e.From)
	}
	wire, err := envelopeText(e)
	if err != nil || !strings.HasPrefix(wire, "[gang:lead#") {
		t.Fatalf("envelope = %q, error = %v", wire, err)
	}
}
func TestCommandsDoNotReadAuditLog(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	if err := os.WriteFile(f.run.team.Log, nil, 0200); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(f.run.team.Log); err == nil {
		t.Fatal("test requires unreadable audit log")
	}
	for _, args := range [][]string{{"roster"}, {"status", "worker"}, {"send", "worker", "--from", "operator"}, {"tick"}, {"drop", "worker"}} {
		cmd := f.cmd
		cmd.stdin = strings.NewReader("hello")
		if err := cmd.execute(args); err != nil {
			t.Fatalf("%v read or failed with unreadable log: %v", args, err)
		}
	}
}
func TestCrashRecoveryNeverRetypes(t *testing.T) {
	for _, settled := range []bool{false, true} {
		t.Run(fmt.Sprint(settled), func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			e := core.Envelope{ID: "crashed", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "only once"}, CreatedAt: f.cmd.now()}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if settled {
				if err := l.Settle(&a, e, "delivered", ""); err != nil {
					t.Fatal(err)
				}
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := f.run.drain(a.ID, ""); err != nil {
				t.Fatal(err)
			}
			got, err := p.ReadEnvelope("failed", e.ID)
			if err != nil {
				t.Fatal(err)
			}
			state, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != "unverified" || state.Input != nil || f.input.submits != 0 {
				t.Fatalf("recovery: %+v input=%+v submits=%d", got, state.Input, f.input.submits)
			}
		})
	}
}
func TestDrainChecksArrivalsDuringUnlock(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	e := core.Envelope{ID: "arrived", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "arrives before recheck"}, CreatedAt: f.cmd.now().Add(-time.Hour)}
	calls := 0
	f.run.cmd.afterUnlock = func() {
		if calls == 0 {
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
		}
		calls++
	}
	outcome, err := f.run.drain(a.ID, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != "delivered" || f.input.submits != 1 {
		t.Fatalf("release race stranded input: %s %d", outcome, f.input.submits)
	}
}
func TestSchedulingOnlyClearsSendersOwnEnvelopes(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	for _, from := range []string{"alice", "bob"} {
		e := core.Envelope{ID: core.EnvelopeID(from), Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: core.AgentName(from)}, Message: core.Message{Text: "later"}, CreatedAt: f.cmd.now(), NotBefore: f.cmd.now().Add(time.Hour)}
		if err := p.Publish(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.cmd.send([]string{"worker", "--from", "alice", "--clear"}); err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].From.Name != "bob" {
		t.Fatalf("cleared another sender: %+v", pending)
	}
}
func TestMismatchIsUnverifiedAndNeverRetyped(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.input.submit = func(string) error {
		return p.WriteWitness(store.Witness{ID: "wrong", At: f.cmd.now(), Prompt: "another message"})
	}
	cmd := f.cmd
	cmd.stdin = strings.NewReader("hello")
	err := cmd.send([]string{"worker", "--from", "operator"})
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitUnknown {
		t.Fatalf("mismatch result: %v", err)
	}
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("retyped %d times", f.input.submits)
	}
}
func TestHookFailureStillExitsZeroAndRecordsFailure(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.env["TMUX_PANE"] = "%77"
	cmd := f.cmd
	cmd.stdin = strings.NewReader("invalid JSON")
	if err := cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"type":"hook_failed"`)) {
		t.Fatalf("missing failure record: %s", data)
	}
	if !bytes.Contains(data, []byte(`"pane":"%77"`)) {
		t.Fatalf("missing hook pane: %s", data)
	}
}

func TestHitchRefusesPaneOnlyAStaleRecordNames(t *testing.T) {
	for _, tc := range []struct {
		name, generation string
		refused          bool
	}{
		{name: "other server", generation: strings.Repeat("b", 64), refused: true},
		{name: "same server", generation: strings.Repeat("a", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			fakeCodexOnPath(t)
			f.add(t, "a", "worker", "codex")
			script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\ncase \"$1 $2\" in\n'list-panes -a') printf '" + tc.generation + "\\t$1\\t%%1\\n';;\nlist-panes*) printf '%%1\\t?worker?\\n';;\ndisplay-message*) printf '$1\\n';;\nhas-session*) exit 0;;\n*) exit 91;;\nesac\n"
			if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			err := f.cmd.hitch([]string{"lead", "-c", "codex"})
			// The fake tmux refuses new-window, so an exempted pane ends at spawn.
			want := "spawn pane"
			if tc.refused {
				want = "unregistered pane %1"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("hitch error = %v, want %q", err, want)
			}
		})
	}
}

func TestHitchRefusesUnregisteredPaneBeforeClaim(t *testing.T) {
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	fakeTmux := f.env["GANG_TMUX"]
	if err := os.WriteFile(fakeTmux, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\ncase \"$1\" in list-panes) printf '%%1\\t?lead?\\n';; has-session) exit 0;; *) exit 91;; esac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err := f.cmd.hitch([]string{"lead", "-c", "codex"})
	if err == nil || !strings.Contains(err.Error(), "unregistered pane %1") {
		t.Fatalf("hitch error = %v, want unregistered pane refusal", err)
	}
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Fatalf("hitch claimed agents despite orphan pane: %+v", agents)
	}
}
