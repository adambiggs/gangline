package store

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/adambiggs/gangline/core"
)

func (p TeamPaths) Append(event core.Event) error {
	data, err := core.EncodeEvent(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p.Log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("append %s: %w", p.Log, err)
	}
	data = append(data, '\n')
	n, err := syscall.Write(int(f.Fd()), data)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("append %s: %w", p.Log, err)
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return closeErr
}
func ReadLog(reader io.Reader, visit func(core.Event) error) error {
	r := bufio.NewReader(reader)
	for line := 1; ; line++ {
		data, err := r.ReadBytes('\n')
		if len(data) > 0 {
			if data[len(data)-1] != '\n' {
				return fmt.Errorf("audit line %d is incomplete", line)
			}
			e, eventErr := core.DecodeEvent(data)
			if eventErr != nil {
				return fmt.Errorf("audit line %d: %w", line, eventErr)
			}
			if err := visit(e); err != nil {
				return err
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
