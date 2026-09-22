package store

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/adambiggs/gangline/core"
)

type LogEntry struct {
	Sequence uint64
	Event    core.Event
}

// ObserveLog reads complete appended records without competing with the writer's
// snapshot lock. An unfinished record is not evidence for an idle verdict.
func ObserveLog(path string, initial core.State) (core.State, int64, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return core.State{}, 0, false, err
	}
	end := bytes.LastIndexByte(data, '\n') + 1
	entries, err := ReadLog(bytes.NewReader(data[:end]))
	if err != nil {
		return core.State{}, 0, false, err
	}
	return Replay(initial, entries), int64(len(data)), end == len(data), nil
}

func (team *LockedTeam) Append(event core.Event) error {
	if err := team.checkLocked(); err != nil {
		return err
	}
	data, err := core.EncodeEvent(event)
	if err != nil {
		return fmt.Errorf("encode event for append: %w", err)
	}
	data = append(data, '\n')
	file, err := os.OpenFile(team.paths.Events, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	if err := writeFull(file, data); err != nil {
		_ = file.Close()
		return fmt.Errorf("append event log: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync event log: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close event log: %w", err)
	}
	if err := syncDirectory(team.paths.Directory); err != nil {
		return err
	}
	return nil
}

func (team *LockedTeam) Log() ([]LogEntry, error) {
	if err := team.checkLocked(); err != nil {
		return nil, err
	}
	file, err := os.Open(team.paths.Events)
	if errorsIsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()
	entries, err := ReadLog(file)
	if err != nil {
		return nil, fmt.Errorf("read event log: %w", err)
	}
	return entries, nil
}

func ReadLog(reader io.Reader) ([]LogEntry, error) {
	buffered := bufio.NewReader(reader)
	var entries []LogEntry
	for sequence := uint64(1); ; sequence++ {
		line, err := buffered.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				return nil, fmt.Errorf("event %d is not newline terminated", sequence)
			}
			line = bytes.TrimSuffix(line, []byte{'\n'})
			if len(line) == 0 {
				return nil, fmt.Errorf("event %d is empty", sequence)
			}
			event, decodeErr := core.DecodeEvent(line)
			if decodeErr != nil {
				return nil, fmt.Errorf("decode event %d: %w", sequence, decodeErr)
			}
			entries = append(entries, LogEntry{Sequence: sequence, Event: event})
		}
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read event %d: %w", sequence, err)
		}
	}
}

func Replay(initial core.State, entries []LogEntry) core.State {
	state := initial
	for _, entry := range entries {
		state, _ = core.Step(state, entry.Event)
	}
	return state
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open team directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync team directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close team directory after sync: %w", err)
	}
	return nil
}
