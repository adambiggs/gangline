package store

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/adambiggs/gangline/core"
)

func (p TeamPaths) Append(event core.Event) error { return appendEvent(p.Log, event) }
func appendEvent(path string, event core.Event) error {
	data, err := core.EncodeEvent(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("append %s: %w", path, err)
	}
	data = append(data, '\n')
	n, err := syscall.Write(int(f.Fd()), data)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("append %s: %w", path, err)
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return closeErr
}

// ReadLog visits known events and returns the number of unknown event kinds
// skipped. Malformed records still return an error.
func ReadLog(reader io.Reader, visit func(core.Event) error) (skipped int, err error) {
	err = ScanLog(reader, func(line int, data []byte) error {
		e, err := core.DecodeEvent(data)
		if errors.Is(err, core.ErrUnknownEventType) {
			skipped++
			return nil
		}
		if err != nil {
			return fmt.Errorf("audit line %d: %w", line, err)
		}
		return visit(e)
	})
	return skipped, err
}

// ScanLog visits each complete line of a log, numbered from 1, without
// decoding it.
func ScanLog(reader io.Reader, visit func(line int, data []byte) error) error {
	r := bufio.NewReader(reader)
	for line := 1; ; line++ {
		data, err := r.ReadBytes('\n')
		if len(data) > 0 {
			if data[len(data)-1] != '\n' {
				return fmt.Errorf("audit line %d is incomplete", line)
			}
			if err := visit(line, data); err != nil {
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

// ReadRecent visits the events in the last window bytes of the team log
// whose lines pass keep, so the cost of a read does not grow with settled
// history. A line cut by the window is skipped.
func (p TeamPaths) ReadRecent(window int64, keep func([]byte) bool, visit func(core.Event) error) error {
	f, err := os.Open(p.Log)
	if err != nil {
		return err
	}
	defer f.Close()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	start := max(size-window, 0)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReader(io.LimitReader(f, size-start))
	if start > 0 {
		if _, err := r.ReadBytes('\n'); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
	return ScanLog(r, func(line int, data []byte) error {
		if !keep(data) {
			return nil
		}
		e, err := core.DecodeEvent(data)
		if errors.Is(err, core.ErrUnknownEventType) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("audit record: %w", err)
		}
		return visit(e)
	})
}
