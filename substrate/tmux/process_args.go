package tmux

import (
	"encoding/binary"
	"fmt"
)

func parseDarwinArgv0(data []byte) (string, error) {
	if len(data) < 6 || binary.LittleEndian.Uint32(data[:4]) == 0 {
		return "", fmt.Errorf("native process arguments are incomplete")
	}
	data = data[4:]
	end := indexZero(data) // First C string is the executable path.
	if end < 0 {
		return "", fmt.Errorf("native process executable path is unterminated")
	}
	data = data[end+1:]
	for len(data) > 0 && data[0] == 0 {
		data = data[1:]
	}
	end = indexZero(data)
	if end <= 0 {
		return "", fmt.Errorf("native process argv[0] is unavailable")
	}
	return string(data[:end]), nil
}

func indexZero(data []byte) int {
	for index, value := range data {
		if value == 0 {
			return index
		}
	}
	return -1
}
