package harness

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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

// Claude Code can hide the terminal cursor and paint a reverse-video cell at
// the reported cursor position. Six wrapped input rows cross the tall-frame
// threshold even in the default terminal size.
func claudeSoftwareCursorScreen() substrate.Screen {
	screen := testScreen(
		testCells("────────────────", false),
		testCells("❯ wrapped assignment", false),
		testCells("  second row", false),
		testCells("  third row", false),
		testCells("  fourth row", false),
		testCells("  fifth row", false),
		testCells("  final row ", false),
		testCells("────────────────", false),
		testCells("auto mode on", false),
	)
	screen.Cursor = substrate.Cursor{Row: 6, Column: 11}
	screen.Rows[6][11].Attributes.Reverse = true
	return screen
}

func TestClaudeExpandedComposerSoftwareCursorSettles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		screen := claudeSoftwareCursorScreen()
		collar, err := EmbeddedCollar("claude")
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = AwaitComposerSettle(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) { return screen, nil }, "%1", collar, 400*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		// Fake time: one 400ms stability window, 600ms before the deadline.
		if elapsed := time.Since(start); elapsed != 400*time.Millisecond {
			t.Fatalf("settled after %s", elapsed)
		}
		got, err := ReadComposer(collar.Primitives.Composer, screen)
		if err != nil || !strings.HasPrefix(got.Text, "wrapped assignment\n") || !strings.HasSuffix(strings.TrimSpace(got.Text), "final row") {
			t.Fatalf("composer=%+v err=%v", got, err)
		}
	})
}

func TestClaudeExpandedComposerRejectsUnwitnessedSoftwareCursor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*substrate.Screen)
	}{
		{"no reverse cell", func(s *substrate.Screen) { s.Rows[6][11].Attributes.Reverse = false }},
		{"cursor elsewhere", func(s *substrate.Screen) { s.Cursor.Column = 10 }},
		{"cursor in footer", func(s *substrate.Screen) { s.Cursor.Row = 8 }},
		{"dim cell", func(s *substrate.Screen) { s.Rows[6][11].Attributes.Dim = true }},
		{"selection", func(s *substrate.Screen) { s.Rows[6][10].Attributes.Reverse = true }},
		{"multiline selection", func(s *substrate.Screen) { s.Rows[2][3].Attributes.Reverse = true }},
		{"nonblank highlight", func(s *substrate.Screen) { s.Rows[6][11].Text = "x" }},
		{"visible footer cursor", func(s *substrate.Screen) { s.Cursor.Row = 8; s.Cursor.Visible = true }},
		{"negative row", func(s *substrate.Screen) { s.Cursor.Row = -1 }},
		{"negative column", func(s *substrate.Screen) { s.Cursor.Column = -1 }},
		{"past row", func(s *substrate.Screen) { s.Cursor.Column = 99 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen := claudeSoftwareCursorScreen()
			tc.change(&screen)
			if _, err := ReadComposer(Invocation{Name: "claude-composer"}, screen); !errors.Is(err, ErrNoComposer) {
				t.Fatalf("unsafe frame accepted: %v", err)
			}
		})
	}
}
