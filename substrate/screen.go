package substrate

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
