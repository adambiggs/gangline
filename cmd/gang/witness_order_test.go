package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

func witnessFixture(t *testing.T) (command, *runtime, core.State, core.Envelope) {
	t.Helper()
	cmd, run := hookFixture(t, "")
	bin := t.TempDir()
	for name, body := range map[string]string{"tmux": "#!/bin/sh\nexit 0\n", "cat": "#!/bin/sh\nexit 91\n"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	getenv := cmd.getenv
	cmd.getenv = func(key string) string {
		if key == "GANG_TMUX" {
			return filepath.Join(bin, "tmux")
		}
		return getenv(key)
	}
	run.cmd = cmd
	now := time.Now()
	envelope := core.Envelope{ID: "msg-witness", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, To: "worker", Message: core.Message{Text: "hello"}, CreatedAt: now}
	locked, err := run.lock()
	if err != nil {
		t.Fatal(err)
	}
	if err := locked.Append(core.SendRequested{At: now, Envelope: envelope}); err != nil {
		t.Fatal(err)
	}
	if err := locked.Close(); err != nil {
		t.Fatal(err)
	}
	state, err := run.load()
	if err != nil {
		t.Fatal(err)
	}
	return cmd, run, state, envelope
}

type witnessBackend struct {
	cmd               command
	text              string
	submits, captures int
	collar            string
}

func (b *witnessBackend) Capture(context.Context, substrate.PaneID) (substrate.Screen, error) {
	b.captures++
	visible := b.text
	if len(visible) > 512 {
		visible = "[Pasted text]"
	}
	if b.collar == "claude-code" {
		return screenWithText("────────", "❯ "+visible, "────────", "auto mode on"), nil
	}
	return screenWithText("› " + visible), nil
}
func (b *witnessBackend) ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	command := "codex"
	if b.collar == "claude-code" {
		command = "claude"
	}
	return []substrate.Process{{PID: 1, Command: command}}, nil
}
func (b *witnessBackend) SendKeys(_ context.Context, _ substrate.PaneID, keys substrate.Keys) error {
	if keys.Text != "" {
		b.text = strings.TrimSuffix(strings.TrimPrefix(keys.Text, "\x1b[200~"), "\x1b[201~")
	}
	if !keys.Submit {
		return nil
	}
	b.submits++
	prompt := b.text
	if b.collar == "claude-code" {
		prompt = "<pasted_content id=\"1\">\n" + prompt + "\n</pasted_content id=\"1\">"
	}
	data, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt})
	if err != nil {
		return err
	}
	cmd := b.cmd
	cmd.stdin = strings.NewReader(string(data))
	return cmd.hook(nil)
}

func TestLargeEncodedNativeWitnessOnEveryCollar(t *testing.T) {
	for _, collar := range []string{"codex", "claude-code"} {
		t.Run(collar, func(t *testing.T) {
			cmd, run, state, envelope := witnessFixture(t)
			hitch := state.Hitches["h-1"]
			hitch.Collar = collar
			state.Hitches[hitch.ID] = hitch
			envelope.Message.Text = strings.Repeat("\"", maximumMessageBytes/2-80)
			synctest.Test(t, func(t *testing.T) {
				backend := &witnessBackend{cmd: cmd, collar: collar}
				event, err := run.deliver(state, backend, core.DeliverEnvelope{Envelope: envelope, Pane: "%1"})
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := event.(core.DeliverySucceeded); !ok {
					t.Fatalf("large native receipt: %#v", event)
				}
			})
		})
	}
}

func TestWitnessReaderReadyBeforeImmediateNativeHook(t *testing.T) {
	cmd, run, state, envelope := witnessFixture(t)
	synctest.Test(t, func(t *testing.T) {
		backend := &witnessBackend{cmd: cmd}
		event, err := run.deliver(state, backend, core.DeliverEnvelope{Envelope: envelope, Pane: "%1"})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := event.(core.DeliverySucceeded); !ok {
			t.Fatalf("immediate native receipt was lost: %#v", event)
		}
		if backend.submits != 1 {
			t.Fatalf("submits = %d", backend.submits)
		}
	})
}

func TestPendingWitnessWithoutReaderFailsLoudly(t *testing.T) {
	cmd, run, _, _ := witnessFixture(t)
	if err := syscall.Mkfifo(run.deliveryWitnessPath("h-1"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd.stdin = strings.NewReader(`{"hook_event_name":"UserPromptSubmit","prompt":"hello"}`)
	if err := cmd.hook(nil); err == nil {
		t.Fatal("pending receipt handoff silently succeeded without a reader")
	}
	state, err := run.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Deliveries["msg-witness"].Status != core.DeliveryUnverified {
		t.Fatalf("failed handoff left sender waiting: %#v", state.Deliveries["msg-witness"])
	}
}

func TestPublishedWitnessSurvivesTelemetryFailure(t *testing.T) {
	cmd, run, _, envelope := witnessFixture(t)
	reader, witnessed, err := openNativeWitness(run.deliveryWitnessPath("h-1"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	native := filepath.Join(t.TempDir(), "malformed.jsonl")
	if err := os.WriteFile(native, []byte("not json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "transcript_path": native, "prompt": "exact receipt"})
	if err != nil {
		t.Fatal(err)
	}
	cmd.stdin = strings.NewReader(string(payload))
	if err := cmd.hook(nil); err == nil {
		t.Fatal("telemetry parse failure was hidden")
	}
	receipt := <-witnessed
	if receipt.err != nil || string(receipt.data) != "exact receipt" {
		t.Fatalf("receipt=%q error=%v", receipt.data, receipt.err)
	}
	state, err := run.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Deliveries[envelope.ID].Status != core.DeliveryDelivering {
		t.Fatalf("telemetry failure erased native receipt: %#v", state.Deliveries[envelope.ID])
	}
}
