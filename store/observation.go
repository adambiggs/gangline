package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adambiggs/gangline/core"
)

// observationReceipt is replaceable runtime state, not retained history.
// The record proves that suppression still has evidence in this audit stream.
type observationReceipt struct {
	Key    string `json:"key"`
	Log    string `json:"log"`
	Offset int64  `json:"offset"`
	Record []byte `json:"record"`
}

// AppendObservation suppresses an unchanged observation only while its exact
// previous record remains in the audit. key includes the caller's registration,
// native session and readings. The hook lock never takes the native input lock.
func (p AgentPaths) AppendObservation(event core.Event, key string) error {
	return p.appendObservation(event, key, atomicJSON)
}

func (p AgentPaths) appendObservation(event core.Event, key string, save func(string, any) error) error {
	if event.Type != "observation" || key == "" {
		return fmt.Errorf("observation requires a nonempty identity key")
	}
	data, err := core.EncodeEvent(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	lock, err := p.lockStatusHooks()
	if err != nil {
		return err
	}
	defer lock.Close()
	path := filepath.Join(p.Directory, "last-observation")
	var old observationReceipt
	if err := readJSON(path, &old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(p.Log, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if old.Key == key && old.Log == p.Log && old.Offset >= 0 && len(old.Record) > 0 {
		previous := make([]byte, len(old.Record))
		n, err := f.ReadAt(previous, old.Offset)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n == len(previous) && bytes.Equal(previous, old.Record) {
			return nil
		}
	}
	// O_APPEND chooses the write position atomically against other appenders.
	// This descriptor's offset identifies our record even if another writer
	// subsequently extends the log through a different descriptor.
	n, err := syscall.Write(int(f.Fd()), data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	end, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// A failed marker write leaves the audit intact; retry may repeat it.
	return save(path, observationReceipt{Key: key, Log: p.Log, Offset: end - int64(n), Record: data})
}
