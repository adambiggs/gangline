package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestParseScreenAttributes(t *testing.T) {
	screen := parseFixture(t, "attributed.capture", substrate.Cursor{Row: 1, Column: 2, Visible: true})
	if got := screen.Rows[0][0]; got.Text != "B" || !got.Attributes.Bold {
		t.Fatalf("first cell = %#v, want bold B", got)
	}
	ghost := screen.Rows[0][5]
	if ghost.Text != "g" || !ghost.Attributes.Dim {
		t.Fatalf("ghost cell = %#v, want dim g", ghost)
	}
	red := screen.Rows[1][0]
	if got := red.Attributes.Foreground; got != (substrate.Color{Kind: substrate.ColorRGB, Red: 1, Green: 2, Blue: 3}) {
		t.Fatalf("red foreground = %#v", got)
	}
	blue := screen.Rows[1][1]
	if got := blue.Attributes.Background; got != (substrate.Color{Kind: substrate.ColorIndexed, Index: 42}) {
		t.Fatalf("blue background = %#v", got)
	}
	if screen.Cursor != (substrate.Cursor{Row: 1, Column: 2, Visible: true}) {
		t.Fatalf("cursor = %#v", screen.Cursor)
	}
}

func TestParseScreenCursorAndOverwrite(t *testing.T) {
	screen := parseFixture(t, "cursor.capture", substrate.Cursor{Row: 2, Column: 1, Visible: true})
	if got := screen.Rows[0][0].Text; got != "X" {
		t.Fatalf("overwritten cell = %q, want X", got)
	}
	if got := screen.Rows[1][2].Text; got != "t" {
		t.Fatalf("positioned cell = %q, want t", got)
	}
	if screen.Cursor != (substrate.Cursor{Row: 2, Column: 1, Visible: false}) {
		t.Fatalf("cursor = %#v", screen.Cursor)
	}
}

func TestParseScreenRefusesUnknownEscape(t *testing.T) {
	if _, err := parseScreen("\x1b]0;title\x07", substrate.Cursor{}); err == nil {
		t.Fatal("unknown terminal escape passed")
	}
}

func parseFixture(t *testing.T, name string, cursor substrate.Cursor) substrate.Screen {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.NewReplacer("\\e", "\x1b", "\\r", "\r").Replace(string(data))
	screen, err := parseScreen(raw, cursor)
	if err != nil {
		t.Fatal(err)
	}
	return screen
}
