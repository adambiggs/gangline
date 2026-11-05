//go:build acceptance

package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/harness"
)

func probeFIFO(t *testing.T) (string, *nativeHookProbe) {
	t.Helper()
	fifo := filepath.Join(t.TempDir(), "hooks.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	collar, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := startNativeHookProbe(fifo, collar, probePrompt)
	if err != nil {
		t.Fatal(err)
	}
	return fifo, probe
}

func writeProbeHook(fifo, payload string) error {
	cmd := command{stdin: strings.NewReader(payload)}
	return cmd.collarCheckHook(nil, fifo)
}

func TestNativeHookProbeSkipsStartupWriter(t *testing.T) {
	fifo, probe := probeFIFO(t)
	defer probe.Close()
	// The reader exists before launch can run SessionStart. Distinct hook
	// writers may close their descriptors without ending the frame stream.
	if err := writeProbeHook(fifo, "{\n \"hook_event_name\": \"SessionStart\", \"session_id\": \"s\"\n}"); err != nil {
		t.Fatal(err)
	}
	payload, err := probe.await(context.Background(), func() error { return writeProbeHook(fifo, probeStarted) })
	if err != nil || string(payload) != probeStarted {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
}

func TestNativeHookProbeClosesAfterTriggerFailure(t *testing.T) {
	fifo, probe := probeFIFO(t)
	triggerErr := errors.New("submission failed")
	if _, err := probe.await(context.Background(), func() error { return triggerErr }); !errors.Is(err, triggerErr) {
		t.Fatalf("trigger error=%v", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	// Close joins the reader; a late callback has no reader and must fail
	// immediately instead of blocking while opening the FIFO.
	if err := writeProbeHook(fifo, probeSession); !errors.Is(err, syscall.ENXIO) {
		t.Fatalf("late callback=%v", err)
	}
}

func TestNativeHookProbeSerializesLargeCallbacks(t *testing.T) {
	fifo, probe := probeFIFO(t)
	defer probe.Close()
	// Both frames exceed the FIFO's atomic-write budget and must remain valid
	// when two callback writers overlap.
	payload := `{"hook_event_name":"PostToolUse","detail":"` + strings.Repeat("x", 128<<10) + `"}`
	writes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { writes <- writeProbeHook(fifo, payload) }()
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-writes:
			if err != nil {
				t.Fatal(err)
			}
		case got := <-probe.result:
			t.Fatalf("callback framing failed: %q %v", got.payload, got.err)
		}
	}
	got, err := probe.await(context.Background(), func() error { return writeProbeHook(fifo, probeStarted) })
	if err != nil || string(got) != probeStarted {
		t.Fatalf("payload=%q err=%v", got, err)
	}
}
