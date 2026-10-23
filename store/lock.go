package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
)

var ErrLocked = errors.New("agent is locked")

type LockedAgent struct {
	Paths AgentPaths
	file  *os.File
	// bound is the saved state's BoundSession, once read.
	bound *string
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
	return &LockedAgent{Paths: p, file: f}, nil
}

// Save writes a's state. When a holds a native session the log has not bound
// to it, Save first appends a session_bound event, so every session a hitch
// held stays in the team log after its state is removed. Only a session the
// native harness reported with its transcript is bound: a requested resume
// session is not, until the harness confirms it. A saved session the log
// never bound, as from a build without bindings, is bound before it is
// replaced. The state records the bound session only once written: a failed
// append or state write leaves the binding to the next save, which may log it
// twice but never misses it.
func (l *LockedAgent) Save(a core.Agent) error {
	if l == nil || l.file == nil {
		return fmt.Errorf("agent lock is not held")
	}
	if l.bound == nil {
		var saved core.Agent
		if err := readJSON(l.Paths.State, &saved); err != nil {
			return err
		}
		bound := saved.Native.BoundSession
		if unbound(saved, bound) {
			if err := l.bind(saved, bound); err != nil {
				return err
			}
			bound = saved.Native.SessionID
		}
		l.bound = &bound
	}
	a.Native.BoundSession = *l.bound
	if unbound(a, *l.bound) {
		if err := l.bind(a, *l.bound); err != nil {
			return err
		}
		a.Native.BoundSession = a.Native.SessionID
	}
	if err := atomicJSON(l.Paths.State, a); err != nil {
		return err
	}
	l.bound = &a.Native.BoundSession
	return nil
}
func unbound(a core.Agent, bound string) bool {
	return a.Native.SessionID != "" && a.Native.Transcript != "" && a.Native.SessionID != bound
}
func (l *LockedAgent) bind(a core.Agent, previous string) error {
	return appendEvent(l.Paths.Log, core.Event{Type: "session_bound", At: time.Now(), HitchID: a.ID, Name: a.Name, Binding: &core.Binding{SessionID: a.Native.SessionID, PreviousSessionID: previous, Collar: a.Collar, HitchedBy: a.HitchedBy}})
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
