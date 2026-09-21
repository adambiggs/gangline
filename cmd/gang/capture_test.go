package main

import (
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestRenderScreenDropsTerminalDeadSpaceAndBoundsLines(t *testing.T) {
	screen := substrate.Screen{Rows: [][]substrate.Cell{
		{{Text: "o"}, {Text: "n"}, {Text: "e"}, {Text: " "}},
		{{Text: "t"}, {Text: "w"}, {Text: "o"}},
		{{Text: " "}},
	}}
	if got := screen.Text(1); got != "two" {
		t.Fatalf("rendered screen = %q", got)
	}
}
