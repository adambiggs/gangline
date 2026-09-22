package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrLocked = errors.New("team store is locked")

type LockedTeam struct {
	paths TeamPaths
	lock  *os.File
}

func (paths Paths) Lock(team string) (*LockedTeam, error) {
	return paths.lock(team, syscall.LOCK_EX|syscall.LOCK_NB)
}

// LockWait serializes short log transactions, including one-shot native hooks
// and delivery outcomes. No native effects hold this lock.
func (paths Paths) LockWait(team string) (*LockedTeam, error) {
	return paths.lock(team, syscall.LOCK_EX)
}

func (paths Paths) lock(team string, flags int) (*LockedTeam, error) {
	teamPaths, err := paths.Team(team)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(teamPaths.Directory, 0o700); err != nil {
		return nil, fmt.Errorf("create team state directory: %w", err)
	}
	lock, err := os.OpenFile(teamPaths.Lock, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open team lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), flags); err != nil {
		closeErr := lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLocked
		}
		if closeErr != nil {
			return nil, fmt.Errorf("lock team store: %v (close lock: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("lock team store: %w", err)
	}
	return &LockedTeam{paths: teamPaths, lock: lock}, nil
}

func (team *LockedTeam) Close() error {
	if team == nil || team.lock == nil {
		return nil
	}
	lock := team.lock
	team.lock = nil
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		_ = lock.Close()
		return fmt.Errorf("unlock team store: %w", err)
	}
	if err := lock.Close(); err != nil {
		return fmt.Errorf("close team lock: %w", err)
	}
	return nil
}

func (team *LockedTeam) checkLocked() error {
	if team == nil || team.lock == nil {
		return errors.New("team store is not locked")
	}
	return nil
}

// LockInput owns one hitch's native input until its outcome is recorded. Closing
// the descriptor releases ownership; the lock file dies with the team state.
func (paths Paths) LockInput(team, hitch string) (*os.File, error) {
	p, err := paths.Team(team)
	if err != nil {
		return nil, err
	}
	if hitch == "" || filepath.Base(hitch) != hitch || hitch == "." || hitch == ".." {
		return nil, fmt.Errorf("invalid input owner %q", hitch)
	}
	file, err := os.OpenFile(filepath.Join(p.Directory, "input-"+hitch+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return file, nil
}
