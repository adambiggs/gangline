package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// claudeCompactingScreen is a Claude Code pane captured mid-compaction. Claude
// paints the queued-message hint dim in an otherwise empty composer.
func claudeCompactingScreen(t *testing.T, c harness.Collar) substrate.Screen {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/claude-code-2.1.287-compacting.txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	screen := screenWithText(lines...)
	for i, line := range lines {
		if strings.Contains(line, "Press up to edit queued messages") {
			for j := 2; j < len(screen.Rows[i]); j++ {
				screen.Rows[i][j].Attributes.Dim = true
			}
		}
	}
	// Without the compaction pattern this screen reads as an idle harness,
	// which is the reading the pattern exists to correct.
	if idle, err := harness.Idle(c, screen); err != nil || !idle {
		t.Fatalf("compacting screen premise: idle=%v err=%v", idle, err)
	}
	return screen
}

func TestSendDuringClaudeCompactionStaysQueued(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.command = "claude"
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	f.input.screen = claudeCompactingScreen(t, c)
	f.cmd.stdin = strings.NewReader("status check")
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || len(f.input.keys) != 0 || !strings.Contains(f.out.String(), "queued") {
		t.Fatalf("submits=%d keys=%v output=%s", f.input.submits, f.input.keys, f.out)
	}
}

func TestClaudeCompactionReadsCompacting(t *testing.T) {
	f := newStateFixture(t)
	added := f.add(t, "a", "worker", "claude")
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	screen := claudeCompactingScreen(t, c)
	l, a, err := f.run.acquire(added.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.run.observeActivity(l, &a, c, screen); err != nil {
		t.Fatal(err)
	}
	if a.Activity != core.Compacting {
		t.Fatalf("activity %s: %s", a.Activity, a.Evidence)
	}
	a.Activity, a.InterruptDeadline = core.Interrupting, f.cmd.now().Add(time.Minute)
	if err := f.run.observeActivity(l, &a, c, screen); err != nil {
		t.Fatal(err)
	}
	if a.Activity != core.Interrupting {
		t.Fatalf("interrupt during compaction lost its state: %s", a.Activity)
	}
	if surface, err := classifyRecoverSurface(c, screen); err != nil || surface.kind != "busy" {
		t.Fatalf("recover surface: %+v %v", surface, err)
	}
}
