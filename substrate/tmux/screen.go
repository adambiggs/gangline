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
			next, err := parser.escape(raw, index)
			if err != nil {
				return substrate.Screen{}, err
			}
			index = next
			continue
		}
		runeValue, width := utf8.DecodeRuneInString(raw[index:])
		if runeValue == utf8.RuneError && width == 1 {
			return substrate.Screen{}, fmt.Errorf("invalid UTF-8 at byte %d", index)
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

func (parser *screenParser) escape(raw string, start int) (int, error) {
	if start+1 >= len(raw) {
		return 0, fmt.Errorf("truncated escape at byte %d", start)
	}
	if raw[start+1] != '[' {
		return 0, fmt.Errorf("unsupported escape at byte %d", start)
	}
	end := start + 2
	for end < len(raw) && (raw[end] < 0x40 || raw[end] > 0x7e) {
		end++
	}
	if end == len(raw) {
		return 0, fmt.Errorf("unterminated CSI escape at byte %d", start)
	}
	parameters := raw[start+2 : end]
	switch raw[end] {
	case 'm':
		if err := parser.sgr(parameters); err != nil {
			return 0, err
		}
	case 'H', 'f':
		if err := parser.position(parameters); err != nil {
			return 0, err
		}
	case 'A', 'B', 'C', 'D':
		if err := parser.move(raw[end], parameters); err != nil {
			return 0, err
		}
	case 'G':
		column, err := oneBased(parameters)
		if err != nil {
			return 0, err
		}
		parser.column = column - 1
	case 'h', 'l':
		if parameters != "?25" {
			return 0, fmt.Errorf("unsupported CSI %q%c", parameters, raw[end])
		}
		parser.cursor.Visible = raw[end] == 'h'
	case 'J':
		if parameters != "2" {
			return 0, fmt.Errorf("unsupported erase display %q", parameters)
		}
		parser.rows = nil
		parser.row = 0
		parser.column = 0
	case 'K':
		if parameters != "" && parameters != "0" {
			return 0, fmt.Errorf("unsupported erase line %q", parameters)
		}
		if parser.row < len(parser.rows) && parser.column < len(parser.rows[parser.row]) {
			parser.rows[parser.row] = parser.rows[parser.row][:parser.column]
		}
	default:
		return 0, fmt.Errorf("unsupported CSI %q%c", parameters, raw[end])
	}
	return end + 1, nil
}

func (parser *screenParser) sgr(parameters string) error {
	values, err := csiValues(parameters)
	if err != nil {
		return fmt.Errorf("invalid SGR %q: %w", parameters, err)
	}
	for index := 0; index < len(values); index++ {
		switch value := values[index]; {
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
		case value == 38 || value == 48:
			color, consumed, err := extendedColor(values[index+1:])
			if err != nil {
				return err
			}
			if value == 38 {
				parser.attributes.Foreground = color
			} else {
				parser.attributes.Background = color
			}
			index += consumed
		default:
			return fmt.Errorf("unsupported SGR parameter %d", value)
		}
	}
	return nil
}

func (parser *screenParser) position(parameters string) error {
	values, err := csiValues(parameters)
	if err != nil || len(values) > 2 {
		return fmt.Errorf("invalid cursor position %q", parameters)
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
	return nil
}

func (parser *screenParser) move(kind byte, parameters string) error {
	distance, err := oneBased(parameters)
	if err != nil {
		return err
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
	return nil
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
		if values[1] > 255 {
			return substrate.Color{}, 0, fmt.Errorf("invalid indexed color %d", values[1])
		}
		return indexed(uint8(values[1])), 2, nil
	case 2:
		if len(values) < 4 || values[1] > 255 || values[2] > 255 || values[3] > 255 {
			return substrate.Color{}, 0, fmt.Errorf("invalid RGB color")
		}
		return substrate.Color{Kind: substrate.ColorRGB, Red: uint8(values[1]), Green: uint8(values[2]), Blue: uint8(values[3])}, 4, nil
	default:
		return substrate.Color{}, 0, fmt.Errorf("unsupported extended color mode %d", values[0])
	}
}

func indexed(value uint8) substrate.Color {
	return substrate.Color{Kind: substrate.ColorIndexed, Index: value}
}
