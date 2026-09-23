package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/adambiggs/gangline/substrate"
)

func parseScreen(raw string, cursor substrate.Cursor) (substrate.Screen, error) {
	parser := screenParser{cursor: cursor}
	for index := 0; index < len(raw); {
		if raw[index] == '\x1b' {
			index = parser.escape(raw, index)
			continue
		}
		runeValue, width := utf8.DecodeRuneInString(raw[index:])
		if runeValue == utf8.RuneError && width == 1 {
			return substrate.Screen{}, fmt.Errorf("invalid UTF-8 at byte %d", index)
		}
		if runeValue >= 0x80 && runeValue <= 0x9f {
			// An 8-bit C1 control is the ESC Fe sequence 0x40 below it.
			index = parser.sequence(raw, byte(runeValue-0x40), index+width)
			continue
		}
		if err := parser.control(runeValue); err != nil {
			return substrate.Screen{}, err
		}
		index += width
	}
	parser.ensure(cursor.Row, cursor.Column)
	return substrate.Screen{Rows: parser.rows, Cursor: parser.cursor}, nil
}

type screenParser struct {
	rows       [][]substrate.Cell
	row        int
	column     int
	attributes substrate.Attributes
	cursor     substrate.Cursor
}

func (parser *screenParser) control(value rune) error {
	switch value {
	case '\n':
		parser.row++
		parser.column = 0
	case '\r':
		parser.column = 0
	case '\b':
		if parser.column > 0 {
			parser.column--
		}
	case '\t':
		parser.column += 8 - parser.column%8
	case '\x0e', '\x0f':
		// SO and SI bracket charset cells in a capture; the cell text is
		// kept as the ASCII the terminal would map, since the screen is
		// read, not drawn.
	default:
		if value < 0x20 || value == 0x7f {
			return fmt.Errorf("unsupported control character U+%04X", value)
		}
		parser.ensure(parser.row, parser.column)
		parser.rows[parser.row][parser.column] = substrate.Cell{Text: string(value), Attributes: parser.attributes}
		parser.column++
	}
	return nil
}

// escape consumes the escape sequence at start and returns the index of the
// first byte after it. Sequences that do not render are skipped, and a
// malformed one is abandoned where it breaks so the capture still parses:
// the screen is observed, never driven, so a lost escape costs at most the
// attribute or position it carried.
func (parser *screenParser) escape(raw string, start int) int {
	if start+1 >= len(raw) {
		return len(raw)
	}
	intro := raw[start+1]
	if intro < 0x20 || intro > 0x7e {
		// A control byte or non-ASCII byte cannot introduce a sequence;
		// only the ESC is dropped.
		return start + 1
	}
	if intro <= 0x2f {
		// nF: intermediates then one final byte, such as ESC ( B.
		end := start + 2
		for end < len(raw) && raw[end] >= 0x20 && raw[end] <= 0x2f {
			end++
		}
		if end < len(raw) && raw[end] >= 0x30 && raw[end] <= 0x7e {
			end++
		}
		return end
	}
	return parser.sequence(raw, intro, start+2)
}

// sequence consumes the body of an escape introduced by intro, which has
// already been consumed, and returns the index of the first byte after it.
func (parser *screenParser) sequence(raw string, intro byte, body int) int {
	switch intro {
	case '[':
		return parser.csi(raw, body)
	case ']':
		return stringEnd(raw, body, true)
	case 'P', 'X', '^', '_':
		return stringEnd(raw, body, false)
	default:
		// Fp, Fs and single C1 controls such as ESC 7, ESC =, ESC M, NEL.
		return body
	}
}

// stringEnd skips an OSC, DCS, SOS, PM or APC string to its terminator: ST
// as ESC \ or U+009C, or BEL when bel is set. A bare ESC ends the string
// without being consumed, and an unterminated string runs to the capture's
// end, as it would on a terminal.
func stringEnd(raw string, body int, bel bool) int {
	for index := body; index < len(raw); index++ {
		switch {
		case raw[index] == '\x1b':
			if index+1 < len(raw) && raw[index+1] == '\\' {
				return index + 2
			}
			return index
		case bel && raw[index] == '\x07':
			return index + 1
		case strings.HasPrefix(raw[index:], "\u009c"):
			return index + len("\u009c")
		}
	}
	return len(raw)
}

// csi parses a control sequence whose body starts at body: parameter and
// intermediate bytes then one final byte. A control byte or ESC before the
// final abandons the sequence and is parsed in its own right.
func (parser *screenParser) csi(raw string, body int) int {
	end := body
	for end < len(raw) && raw[end] >= 0x20 && raw[end] <= 0x3f {
		end++
	}
	if end == len(raw) || raw[end] < 0x40 || raw[end] > 0x7e {
		return end
	}
	parameters := raw[body:end]
	switch raw[end] {
	case 'm':
		parser.sgr(parameters)
	case 'H', 'f':
		parser.position(parameters)
	case 'A', 'B', 'C', 'D':
		parser.move(raw[end], parameters)
	case 'G':
		if column, err := oneBased(parameters); err == nil {
			parser.column = column - 1
		}
	case 'h', 'l':
		if parameters == "?25" {
			parser.cursor.Visible = raw[end] == 'h'
		}
	case 'J':
		if parameters == "2" {
			parser.rows = nil
			parser.row = 0
			parser.column = 0
		}
	case 'K':
		if parameters != "" && parameters != "0" {
			break
		}
		if parser.row < len(parser.rows) && parser.column < len(parser.rows[parser.row]) {
			parser.rows[parser.row] = parser.rows[parser.row][:parser.column]
		}
	}
	return end + 1
}

// sgr applies the attribute parameters it knows. An unknown parameter, and a
// colon-separated group such as an underline style or colour, is ignored;
// a malformed extended colour ends the sequence early.
func (parser *screenParser) sgr(parameters string) {
	values := sgrValues(parameters)
	for index := 0; index < len(values); index++ {
		switch value := values[index]; {
		case value < 0:
		case value == 0:
			parser.attributes = substrate.Attributes{}
		case value == 1:
			parser.attributes.Bold = true
		case value == 2:
			parser.attributes.Dim = true
		case value == 3:
			parser.attributes.Italic = true
		case value == 4:
			parser.attributes.Underline = true
		case value == 7:
			parser.attributes.Reverse = true
		case value == 22:
			parser.attributes.Bold = false
			parser.attributes.Dim = false
		case value == 23:
			parser.attributes.Italic = false
		case value == 24:
			parser.attributes.Underline = false
		case value == 27:
			parser.attributes.Reverse = false
		case value >= 30 && value <= 37:
			parser.attributes.Foreground = indexed(uint8(value - 30))
		case value == 39:
			parser.attributes.Foreground = substrate.Color{}
		case value >= 40 && value <= 47:
			parser.attributes.Background = indexed(uint8(value - 40))
		case value == 49:
			parser.attributes.Background = substrate.Color{}
		case value >= 90 && value <= 97:
			parser.attributes.Foreground = indexed(uint8(value - 90 + 8))
		case value >= 100 && value <= 107:
			parser.attributes.Background = indexed(uint8(value - 100 + 8))
		case value == 38 || value == 48 || value == 58:
			// 58 is the underscore colour: its arguments are consumed
			// like a foreground's, and the colour itself is not kept.
			color, consumed, err := extendedColor(values[index+1:])
			if err != nil {
				return
			}
			switch value {
			case 38:
				parser.attributes.Foreground = color
			case 48:
				parser.attributes.Background = color
			}
			index += consumed
		}
	}
}

// sgrValues splits SGR parameters on ';'. An empty parameter is 0; a group
// carrying ':' subparameters or a value that is not a small non-negative
// integer is -1, which sgr ignores.
func sgrValues(parameters string) []int {
	if parameters == "" {
		return []int{0}
	}
	parts := strings.Split(parameters, ";")
	values := make([]int, len(parts))
	for index, part := range parts {
		switch value, err := strconv.Atoi(part); {
		case part == "":
			values[index] = 0
		case err != nil || value < 0 || strings.Contains(part, ":"):
			values[index] = -1
		default:
			values[index] = value
		}
	}
	return values
}

func (parser *screenParser) position(parameters string) {
	values, err := csiValues(parameters)
	if err != nil || len(values) > 2 {
		return
	}
	row, column := 1, 1
	if len(values) > 0 && values[0] != 0 {
		row = values[0]
	}
	if len(values) == 2 && values[1] != 0 {
		column = values[1]
	}
	parser.row = row - 1
	parser.column = column - 1
}

func (parser *screenParser) move(kind byte, parameters string) {
	distance, err := oneBased(parameters)
	if err != nil {
		return
	}
	switch kind {
	case 'A':
		parser.row -= distance
		if parser.row < 0 {
			parser.row = 0
		}
	case 'B':
		parser.row += distance
	case 'C':
		parser.column += distance
	case 'D':
		parser.column -= distance
		if parser.column < 0 {
			parser.column = 0
		}
	}
}

func (parser *screenParser) ensure(row, column int) {
	for len(parser.rows) <= row {
		parser.rows = append(parser.rows, nil)
	}
	for len(parser.rows[row]) <= column {
		parser.rows[row] = append(parser.rows[row], substrate.Cell{Text: " "})
	}
}

func csiValues(parameters string) ([]int, error) {
	if parameters == "" {
		return []int{0}, nil
	}
	parts := strings.Split(parameters, ";")
	values := make([]int, len(parts))
	for index, part := range parts {
		if part == "" {
			values[index] = 0
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil, fmt.Errorf("invalid parameter %q", part)
		}
		values[index] = value
	}
	return values, nil
}

func oneBased(parameters string) (int, error) {
	values, err := csiValues(parameters)
	if err != nil || len(values) != 1 {
		return 0, fmt.Errorf("invalid cursor distance %q", parameters)
	}
	if values[0] == 0 {
		return 1, nil
	}
	return values[0], nil
}

func extendedColor(values []int) (substrate.Color, int, error) {
	if len(values) < 2 {
		return substrate.Color{}, 0, fmt.Errorf("truncated extended color")
	}
	switch values[0] {
	case 5:
		if values[1] < 0 || values[1] > 255 {
			return substrate.Color{}, 0, fmt.Errorf("invalid indexed color %d", values[1])
		}
		return indexed(uint8(values[1])), 2, nil
	case 2:
		if len(values) < 4 || !channels(values[1:4]) {
			return substrate.Color{}, 0, fmt.Errorf("invalid RGB color")
		}
		return substrate.Color{Kind: substrate.ColorRGB, Red: uint8(values[1]), Green: uint8(values[2]), Blue: uint8(values[3])}, 4, nil
	default:
		return substrate.Color{}, 0, fmt.Errorf("unsupported extended color mode %d", values[0])
	}
}

func channels(values []int) bool {
	for _, value := range values {
		if value < 0 || value > 255 {
			return false
		}
	}
	return true
}

func indexed(value uint8) substrate.Color {
	return substrate.Color{Kind: substrate.ColorIndexed, Index: value}
}
