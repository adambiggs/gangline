package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LockTeam serializes registration with whole-team teardown. Its file lives
// outside the team directory because down removes that directory.
func (p Paths) LockTeam(name string) (*os.File, error) {
	if err := segment(name); err != nil {
		return nil, err
	}
	directory := filepath.Join(p.Root, "team-locks")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(directory, name+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("lock team %q: %w", name, err)
	}
	return f, nil
}
