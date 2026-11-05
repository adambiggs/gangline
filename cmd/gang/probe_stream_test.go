package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/harness"
)

const probePrompt = "Reply with exactly READY."
const probeStarted = `{"hook_event_name":"UserPromptSubmit","session_id":"s","prompt":"Reply with exactly READY."}`
const probeSession = `{"hook_event_name":"SessionStart","session_id":"s"}`

func TestProbeTurnSelectsMatchingSubmit(t *testing.T) {
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			collar, err := harness.EmbeddedCollar(name)
			if err != nil {
				t.Fatal(err)
			}
			for _, prefix := range []string{"", probeSession + "\n", probeSession + "\n" + `{"hook_event_name":"UserPromptSubmit","prompt":"unrelated"}` + "\n"} {
				got, err := readProbeTurn(strings.NewReader(prefix+probeStarted+"\n"), collar, probePrompt)
				if err != nil || string(got) != probeStarted {
					t.Fatalf("selected=%q err=%v", got, err)
				}
			}
		})
	}
}

func TestProbeTurnDoesNotInventSubmit(t *testing.T) {
	collar, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", probeSession + "\n", `{"hook_event_name":"UserPromptSubmit","prompt":"unrelated"}` + "\n"} {
		if payload, err := readProbeTurn(strings.NewReader(input), collar, probePrompt); !errors.Is(err, io.EOF) || payload != nil {
			t.Fatalf("payload=%q err=%v", payload, err)
		}
	}
	for _, input := range []string{"malformed\n", `{"hook_event_name":"unknown"}` + "\n", `{"hook_event_name":"StopFailure","error":"Login expired"}` + "\n"} {
		if payload, err := readProbeTurn(strings.NewReader(input), collar, probePrompt); err == nil || payload == nil {
			t.Fatalf("invalid hook passed: payload=%q err=%v", payload, err)
		}
	}
}

func TestProbeHookFramesAreBounded(t *testing.T) {
	collar, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	input := append(bytes.Repeat([]byte("x"), maximumHookBytes+1), '\n')
	if _, err := readProbeTurn(bytes.NewReader(input), collar, probePrompt); err == nil {
		t.Fatal("oversized frame accepted")
	}
}

func TestNativeHookProbeWaitUsesDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		probe := &nativeHookProbe{result: make(chan nativeHookResult)}
		// Fake time advances across the full production probe budget; no real
		// deadline or polling is involved.
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := probe.await(ctx, func() error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("wait=%v", err)
		}
		if elapsed := time.Since(started); elapsed != 30*time.Second {
			t.Fatalf("fake deadline elapsed=%v, want exactly 30s", elapsed)
		}
	})
}

func TestProbeTurnReportsNativeFailureSeparately(t *testing.T) {
	collar, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	input := `{"hook_event_name":"StopFailure","error":"Login expired"}` + "\n"
	payload, err := readProbeTurn(strings.NewReader(input), collar, probePrompt)
	if payload == nil || !errors.Is(err, errProbeTurnFailed) || !strings.Contains(err.Error(), "Login expired") {
		t.Fatalf("native failure payload=%q err=%v", payload, err)
	}
	if _, err := readProbeTurn(strings.NewReader("malformed\n"), collar, probePrompt); err == nil || errors.Is(err, errProbeTurnFailed) {
		t.Fatalf("malformed hook confused with native failure: %v", err)
	}
}
