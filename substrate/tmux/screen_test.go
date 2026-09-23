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

// Every escape class a real terminal emits parses, and the text around it
// lands in the cells it would occupy on screen.
func TestParseScreenSkipsNonRenderingEscapes(t *testing.T) {
	cases := []struct {
		name   string
		escape string
	}{
		{"OSC BEL", "\x1b]0;title\x07"},
		{"OSC ST", "\x1b]8;;https://example.test\x1b\\"},
		{"OSC 8-bit ST", "\x1b]8;;\u009c"},
		{"charset G0", "\x1b(B"},
		{"charset G1", "\x1b)0"},
		{"keypad application", "\x1b="},
		{"keypad numeric", "\x1b>"},
		{"save cursor", "\x1b7"},
		{"restore cursor", "\x1b8"},
		{"reverse index", "\x1bM"},
		{"DECALN", "\x1b#8"},
		{"DCS", "\x1bPq#0;2;0;0;0\x1b\\"},
		{"APC", "\x1b_Gi=1\x1b\\"},
		{"PM", "\x1b^hidden\x1b\\"},
		{"SOS", "\x1bXtext\x1b\\"},
		{"C1 CSI", "\u009b0m"},
		{"C1 OSC", "\u009d0;title\x07"},
		{"C1 DCS", "\u0090q\u009c"},
		{"C1 APC", "\u009fG\u009c"},
		{"C1 PM", "\u009ex\u009c"},
		{"C1 single", "\u0085"},
		{"unknown CSI", "\x1b[?2004h"},
		{"unknown CSI final", "\x1b[3z"},
		{"unknown SGR", "\x1b[9;53m"},
		{"SGR subparameters", "\x1b[4:3;58:2::1:2:3m"},
		{"stray ST", "\x1b\\"},
		{"unterminated OSC", "\x1b]0;title"},
		{"unterminated CSI", "\x1b["},
		{"truncated ESC", "\x1b"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screen, err := parseScreen("ab"+testCase.escape, substrate.Cursor{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := screen.Rows[0][0].Text + screen.Rows[0][1].Text; got != "ab" {
				t.Fatalf("text before escape = %q, want ab", got)
			}
			if len(screen.Rows[0]) > 2 || len(screen.Rows) > 1 {
				t.Fatalf("escape rendered cells: %#v", screen.Rows)
			}
		})
	}
}

// A terminated escape consumes exactly its own bytes: the text after it
// still renders, and only the escape's own effect applies.
func TestParseScreenResumesAfterEscape(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"OSC BEL", "a\x1b]8;;https://example.test\x07b"},
		{"OSC ST", "a\x1b]8;;\x1b\\b"},
		{"two-byte", "a\x1b(Bb"},
		{"intermediate", "a\x1b#8b"},
		{"DCS", "a\x1bPq\x1b\\b"},
		{"C1 CSI", "a\u009b1mb"},
		{"C1 OSC", "a\u009d0;t\u009cb"},
		{"unknown CSI", "a\x1b[?2004hb"},
		{"unknown SGR", "a\x1b[9mb"},
		{"SGR with subparameters", "a\x1b[4:3;1mb"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			screen, err := parseScreen(testCase.raw, substrate.Cursor{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(screen.Rows) != 1 || len(screen.Rows[0]) != 2 {
				t.Fatalf("rows = %#v, want one row of two cells", screen.Rows)
			}
			if got := screen.Rows[0][0].Text + screen.Rows[0][1].Text; got != "ab" {
				t.Fatalf("text = %q, want ab", got)
			}
		})
	}
}

// A stray ESC before a control byte or a multibyte character skips only
// itself: the newline still ends the row and the character still renders.
func TestParseScreenStrayEscapeKeepsNextByte(t *testing.T) {
	screen, err := parseScreen("a\x1b\nb\x1b\u00e9", substrate.Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	if len(screen.Rows) != 2 || screen.Rows[0][0].Text != "a" || screen.Rows[1][0].Text != "b" || screen.Rows[1][1].Text != "\u00e9" {
		t.Fatalf("rows = %#v, want a, then b\u00e9 on the next row", screen.Rows)
	}
}

// An underscore colour (SGR 58) consumes its colour arguments the way 38 and
// 48 do, so they are not applied as attributes of their own.
func TestParseScreenSkipsUnderscoreColor(t *testing.T) {
	for _, raw := range []string{"\x1b[31m\x1b[58;2;0;0;0mx", "\x1b[31m\x1b[58;5;1mx", "\x1b[31m\x1b[58;5;7mx"} {
		screen, err := parseScreen(raw, substrate.Cursor{})
		if err != nil {
			t.Fatal(err)
		}
		cell := screen.Rows[0][0]
		if cell.Attributes.Foreground != indexed(1) || cell.Attributes.Bold || cell.Attributes.Reverse {
			t.Fatalf("%q cell = %#v, want plain red x", raw, cell)
		}
	}
}

// capture-pane -e brackets charset cells in SO and SI; neither renders.
func TestParseScreenSkipsShiftInOut(t *testing.T) {
	screen, err := parseScreen("\x0eq\x0fa", substrate.Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	if got := screen.Rows[0][0].Text + screen.Rows[0][1].Text; got != "qa" || len(screen.Rows[0]) != 2 {
		t.Fatalf("row = %#v, want qa", screen.Rows[0])
	}
}

// The -1 that sgrValues uses for an ignored parameter must not reach an
// extended colour as an index, where uint8 would turn it into 255.
func TestParseScreenRejectsNegativeExtendedColor(t *testing.T) {
	for _, raw := range []string{"\x1b[38;5;-1mx", "\x1b[38;5;4:3mx", "\x1b[38;2;1;-2;3mx"} {
		screen, err := parseScreen(raw, substrate.Cursor{})
		if err != nil {
			t.Fatal(err)
		}
		if got := screen.Rows[0][0].Attributes.Foreground; got != (substrate.Color{}) {
			t.Fatalf("%q foreground = %#v, want unset", raw, got)
		}
	}
}

// Known SGR parameters still apply when an unknown one shares the sequence.
func TestParseScreenKeepsKnownSGRBesideUnknown(t *testing.T) {
	screen, err := parseScreen("\x1b[9;1;4:3;31mx", substrate.Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	cell := screen.Rows[0][0]
	if !cell.Attributes.Bold || cell.Attributes.Foreground != indexed(1) {
		t.Fatalf("cell = %#v, want bold red x", cell)
	}
}

// The capture stays a failure only when its bytes are not text at all.
func TestParseScreenRefusesInvalidUTF8(t *testing.T) {
	if _, err := parseScreen("a\x9b0mb", substrate.Cursor{}); err == nil {
		t.Fatal("raw C1 byte passed as UTF-8")
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
