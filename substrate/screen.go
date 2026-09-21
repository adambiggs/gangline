package substrate

import "strings"

type Screen struct {
	Rows   [][]Cell
	Cursor Cursor
}

type Cell struct {
	Text       string
	Attributes Attributes
}

type Attributes struct {
	Bold       bool
	Dim        bool
	Italic     bool
	Underline  bool
	Reverse    bool
	Foreground Color
	Background Color
}

type Color struct {
	Kind  ColorKind
	Index uint8
	Red   uint8
	Green uint8
	Blue  uint8
}

type ColorKind uint8

const (
	ColorDefault ColorKind = iota
	ColorIndexed
	ColorRGB
)

type Cursor struct {
	Row     int
	Column  int
	Visible bool
}

func (screen Screen) Width() int {
	width := 0
	for _, row := range screen.Rows {
		if len(row) > width {
			width = len(row)
		}
	}
	return width
}

func (screen Screen) Height() int {
	return len(screen.Rows)
}

// Text renders visible cell text, trimming terminal dead space and old rows.
func (screen Screen) Text(lineCount int) string {
	rows := make([]string, len(screen.Rows))
	for rowNumber, row := range screen.Rows {
		var line strings.Builder
		for _, cell := range row {
			line.WriteString(cell.Text)
		}
		rows[rowNumber] = strings.TrimRight(line.String(), " ")
	}
	for len(rows) != 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	if lineCount > 0 && len(rows) > lineCount {
		rows = rows[len(rows)-lineCount:]
	}
	return strings.Join(rows, "\n")
}
