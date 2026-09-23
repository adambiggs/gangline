package harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestClaudeExpandedComposerUsesActiveCursor(t *testing.T) {
	rows := [][]substrate.Cell{testCells("────────────────", false), testCells("❯ message opener", false)}
	for range 9 {
		rows = append(rows, testCells("  wrapped body", false))
	}
	rows = append(rows, testCells("────────────────", false), testCells("auto mode on", false))
	screen := testScreen(rows...)
	screen.Cursor = substrate.Cursor{Row: 10, Column: 14, Visible: true}
	got, err := ReadComposer(Invocation{Name: "claude-composer"}, screen)
	if err != nil || !strings.HasPrefix(got.Text, "message opener\n") || strings.Count(got.Text, "wrapped body") != 9 {
		t.Fatalf("expanded composer: text=%q err=%v", got.Text, err)
	}
	screen.Cursor.Row = 12
	if _, err := ReadComposer(Invocation{Name: "claude-composer"}, screen); !errors.Is(err, ErrNoComposer) {
		t.Fatalf("history frame mistaken for active composer: %v", err)
	}
	screen.Cursor.Row = 10
	screen.Cursor.Visible = false
	if _, err := ReadComposer(Invocation{Name: "claude-composer"}, screen); !errors.Is(err, ErrNoComposer) {
		t.Fatalf("unobserved cursor accepted tall frame: %v", err)
	}
}
