package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LockedUsage serializes account-wide usage notices and agent wakes. The lock
// lives beside team.json so independent hooks agree on the same file.
type LockedUsage struct {
	state string
	file  *os.File
}

func (p TeamPaths) LockUsage() (*LockedUsage, error) {
	path := filepath.Join(p.Directory, "usage.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	held, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	current, err := os.Stat(path)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !os.SameFile(held, current) {
		_ = f.Close()
		return nil, fmt.Errorf("usage lock changed while acquiring it")
	}
	if _, err := os.Stat(p.State); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &LockedUsage{state: filepath.Join(p.Directory, "usage.json"), file: f}, nil
}

func (l *LockedUsage) Read(value any) error {
	if l == nil || l.file == nil {
		return fmt.Errorf("usage lock is not held")
	}
	err := readJSON(l.state, value)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (l *LockedUsage) Save(value any) error {
	if l == nil || l.file == nil {
		return fmt.Errorf("usage lock is not held")
	}
	return atomicJSON(l.state, value)
}

func (l *LockedUsage) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}
