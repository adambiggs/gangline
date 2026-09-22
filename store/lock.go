package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/adambiggs/gangline/core"
)

var ErrLocked = errors.New("agent is locked")

type LockedAgent struct {
	Paths AgentPaths
	file  *os.File
}

func (p AgentPaths) TryLock() (*LockedAgent, error)   { return p.lock(syscall.LOCK_EX | syscall.LOCK_NB) }
func (p AgentPaths) LockAgent() (*LockedAgent, error) { return p.lock(syscall.LOCK_EX) }
func (p AgentPaths) lock(flags int) (*LockedAgent, error) {
	f, err := os.OpenFile(p.Lock, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), flags); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLocked
		}
		return nil, err
	}
	// A drop may have removed the directory while a waiter owned this fd.
	if _, err := os.Stat(p.State); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &LockedAgent{p, f}, nil
}
func (l *LockedAgent) Save(a core.Agent) error {
	if l == nil || l.file == nil {
		return fmt.Errorf("agent lock is not held")
	}
	return atomicJSON(l.Paths.State, a)
}
func (l *LockedAgent) Held() bool { return l != nil && l.file != nil }
func (l *LockedAgent) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}
