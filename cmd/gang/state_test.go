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
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type inputFixture struct {
	screen          substrate.Screen
	command, pasted string
	submits         int
	submit          func(string) error
}

func (b *inputFixture) Capture(context.Context, substrate.PaneID) (substrate.Screen, error) {
	return b.screen, nil
}
func (b *inputFixture) ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	return []substrate.Process{{PID: 7, Command: b.command}}, nil
}
func (b *inputFixture) SendKeys(_ context.Context, _ substrate.PaneID, k substrate.Keys) error {
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
	out, errOut *bytes.Buffer
	input       *inputFixture
}

func newStateFixture(t *testing.T) *stateFixture {
	t.Helper()
	root := t.TempDir()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	env := map[string]string{"GANG_STATE_ROOT": root, "GANG_CONFIG_DIR": filepath.Join(root, "config"), "GANG_SESSION": "unit"}
	backend := &inputFixture{screen: screenWithText("READY", "› "), command: "codex"}
	cmd := command{newScheduler: func() watchdogScheduler { return nil }, stdin: strings.NewReader(""), stdout: out, stderr: errOut, getenv: func(k string) string { return env[k] }, getwd: func() (string, error) { return root, nil }, userHomeDir: func() (string, error) { return root, nil }, clock: func() time.Time { return time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC) }, inputBackend: backend, settleInput: func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error { return nil }, detach: func(string, hookNotice) error { return nil }}
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
func (f *stateFixture) add(t *testing.T, id, name, collar string) core.Agent {
	t.Helper()
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(name), Collar: collar, Directory: "/work", Pane: "%1", Status: core.Active, Activity: core.Idle, CreatedAt: f.cmd.now(), ChangedAt: f.cmd.now()}
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
	for _, name := range []string{"codex", "claude-code"} {
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
	e := core.Envelope{ID: "arrived", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "arrives before recheck"}, CreatedAt: f.cmd.now().Add(-time.Hour)}
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
	if err := f.cmd.send([]string{"worker", "--from", "alice", "--at", "clear"}); err != nil {
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

func TestHitchRefusesUnregisteredPaneBeforeClaim(t *testing.T) {
	f := newStateFixture(t)
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
