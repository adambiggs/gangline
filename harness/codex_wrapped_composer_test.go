package harness

import (
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestCodexComposerReadsWrappedInputAtCursor(t *testing.T) {
	for _, test := range []struct {
		name   string
		cursor substrate.Cursor
		want   string
		tail   bool
	}{
		{"wrapped input", substrate.Cursor{Row: 1, Column: 20, Visible: true}, "/rename worker@team: This is a long\nnative-input reproduction", false},
		{"cursor elsewhere", substrate.Cursor{Row: 0, Column: 2, Visible: true}, "/rename worker@team: This is a long", true},
		{"hidden cursor", substrate.Cursor{Row: 1, Column: 20}, "/rename worker@team: This is a long", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			screen := testScreen(
				testCells("› /rename worker@team: This is a long", false),
				testCells("  native-input reproduction", false),
				nil,
				testCells("  model · project", false),
			)
			screen.Cursor = test.cursor
			composer, err := ReadComposer(Invocation{Name: "codex-composer"}, screen)
			if err != nil || composer.Text != test.want || composer.TailOccupied != test.tail {
				t.Fatalf("composer=%+v error=%v; want text=%q tail=%v", composer, err, test.want, test.tail)
			}
		})
	}
}

func TestCodexWrappedComposerBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		rows   [][]substrate.Cell
		cursor int
		want   string
		tail   bool
	}{
		{"cursor before input tail", [][]substrate.Cell{testCells("› first", false), testCells("  second", false), testCells("  third", false), nil, testCells("  status", false)}, 1, "first\nsecond", true},
		{"unindented row", [][]substrate.Cell{testCells("› first", false), testCells("second", false), nil, testCells("  status", false)}, 1, "first", true},
		{"cursor in footer", [][]substrate.Cell{testCells("› first", false), testCells("  second", false), nil, testCells("  status", false)}, 3, "first", true},
		{"cursor above input", [][]substrate.Cell{testCells("history", false), testCells("› first", false), nil, testCells("  status", false)}, 0, "first", true},
		{"placeholder with continuation", [][]substrate.Cell{testCells("› [Pasted Content 123 chars]", false), testCells("  second", false), nil, testCells("  status", false)}, 1, "[Pasted Content 123 chars]\nsecond", false},
		{"dim continuation", [][]substrate.Cell{testCells("› first", false), append(testCells("  second", false), testCells(" ghost", true)...), nil, testCells("  status", false)}, 1, "first\nsecond", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			screen := testScreen(test.rows...)
			screen.Cursor = substrate.Cursor{Row: test.cursor, Column: 2, Visible: true}
			composer, err := ReadComposer(Invocation{Name: "codex-composer"}, screen)
			if err != nil || composer.Text != test.want || composer.TailOccupied != test.tail || composer.CollapsedChars != 0 {
				t.Fatalf("composer=%+v error=%v; want text=%q tail=%v", composer, err, test.want, test.tail)
			}
		})
	}
}
