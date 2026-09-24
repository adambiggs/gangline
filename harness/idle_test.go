package harness

import (
	"testing"
	"time"
)

func TestIdleRequiresAnEmptyComposerWithoutNativeWork(t *testing.T) {
	collar := Collar{}
	collar.Primitives.Composer = Invocation{Name: "codex-composer"}
	collar.Primitives.Wedge = Invocation{Name: "stable-busy-screen", Params: map[string]string{"busy": "Working"}}
	for _, test := range []struct {
		name, status, composer string
		idle                   bool
	}{
		{"ready", "Ready", "› ", true},
		{"busy empty composer", "Working", "› ", false},
		{"occupied composer", "Ready", "› draft", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			screen := testScreen(testCells(test.status, false), testCells(test.composer, false))
			idle, err := Idle(collar, screen)
			if err != nil || idle != test.idle {
				t.Fatalf("idle = %v, error = %v", idle, err)
			}

		})
	}
	if _, err := Idle(collar, testScreen(testCells("unrecognized surface", false))); err == nil {
		t.Fatal("unrecognized surface accepted")
	}
}

func TestComposerSettlesWhileNativeWorkAnimates(t *testing.T) {
	collar, _ := EmbeddedCollar("codex")
	now := time.Unix(100, 0)
	observed := composerStability{}
	for i, status := range []string{"Working |", "Working /", "Working -"} {
		screen := testScreen(testCells(status, false), testCells("› steer this turn", false))
		composer, err := ReadComposer(collar.Primitives.Composer, screen)
		if err != nil {
			t.Fatal(err)
		}
		ready := observed.ready(composer.Text, now.Add(time.Duration(i)*200*time.Millisecond), 400*time.Millisecond)
		if ready != (i == 2) {
			t.Fatalf("frame %d ready=%v", i, ready)
		}
	}
	if observed.ready("changed input", now.Add(time.Second), 400*time.Millisecond) {
		t.Fatal("changed composer bypassed settling")
	}
}

func TestClaudeToolRunScreenIsBusy(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	screen := fixtureScreen(t, "claude-code-2.1.281-tool-run.txt")
	busy, err := Busy(collar, screen)
	if err != nil || !busy {
		t.Fatalf("busy = %v, error = %v", busy, err)
	}
	idle, err := Idle(collar, screen)
	if err != nil || idle {
		t.Fatalf("idle = %v, error = %v", idle, err)
	}
}

func TestClaudeCompletedProseDoesNotLookBusy(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	screen := testScreen(
		testCells("The completed response mentions running PostToolUse hook in prose.", false),
		testCells("I also described · Doing… as a status label.", false),
		testCells("────────────────────────────────────────────────────────────────────────────────", false),
		testCells("❯", false),
		testCells("────────────────────────────────────────────────────────────────────────────────", false),
	)
	idle, err := Idle(collar, screen)
	if err != nil || !idle {
		t.Fatalf("idle = %v, error = %v", idle, err)
	}
}
