package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adambiggs/gangline/core"
)

// IdleState is optional notification state, separate from legacy team and agent
// records. Work revisions count new publications, never native callback IDs.
type IdleState struct {
	Work        map[core.HitchID]uint64 `json:"work,omitempty"`
	Pending     *IdlePublication        `json:"pending,omitempty"`
	Initialized bool                    `json:"initialized"`
	Lead        IdleMember              `json:"lead"`
	Members     []IdleMember            `json:"members,omitempty"`
	Armed       bool                    `json:"armed"`
	Notice      *IdleNotice             `json:"notice,omitempty"`
}

type IdlePublication struct {
	Recipient core.HitchID    `json:"recipient"`
	ID        core.EnvelopeID `json:"id"`
}

type IdleMember struct {
	ID         core.HitchID   `json:"id"`
	Name       core.AgentName `json:"name"`
	Generation string         `json:"generation"`
	Session    string         `json:"session"`
	Work       uint64         `json:"work"`
}

type IdleNotice struct {
	Envelope   core.Envelope `json:"envelope"`
	Generation string        `json:"generation"`
	Session    string        `json:"session"`
	Outcome    string        `json:"outcome,omitempty"`
}

// LockedIdle is acquired after agent locks. It never spans native input or
// pane capture. A snapshot takes agent locks nonblockingly before this lock.
type LockedIdle struct {
	team TeamPaths
	file *os.File
}

func (p TeamPaths) LockIdle() (*LockedIdle, error) {
	path := filepath.Join(p.Directory, "idle.lock")
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
		return nil, fmt.Errorf("idle lock changed while acquiring it")
	}
	if _, err := os.Stat(p.State); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &LockedIdle{team: p, file: f}, nil
}

func (l *LockedIdle) Read() (IdleState, error) {
	var s IdleState
	err := readJSON(filepath.Join(l.team.Directory, "idle.json"), &s)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	if s.Work == nil {
		s.Work = make(map[core.HitchID]uint64)
	}
	if s.Pending != nil {
		p, err := l.team.Agent(s.Pending.Recipient)
		if err != nil {
			return s, err
		}
		for _, dir := range []string{"new", "cur", "failed"} {
			_, err := p.ReadEnvelope(dir, s.Pending.ID)
			if err == nil {
				if s.Work[s.Pending.Recipient] == ^uint64(0) {
					return s, fmt.Errorf("idle work revision exhausted")
				}
				s.Work[s.Pending.Recipient]++
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return s, err
			}
		}
		s.Pending = nil
		if err := l.Save(s); err != nil {
			return s, err
		}
	}
	// Dropped registrations cannot accumulate work counters, even while no
	// unique lead exists or only targeted ticks run.
	pruned := false
	for id := range s.Work {
		p, err := l.team.Agent(id)
		if err != nil {
			return s, err
		}
		if _, err := os.Stat(p.State); errors.Is(err, os.ErrNotExist) {
			delete(s.Work, id)
			pruned = true
		} else if err != nil {
			return s, err
		}
	}
	if pruned {
		if err := l.Save(s); err != nil {
			return s, err
		}
	}
	return s, nil
}

func (l *LockedIdle) Save(s IdleState) error {
	if l == nil || l.file == nil {
		return fmt.Errorf("idle lock is not held")
	}
	return atomicJSON(filepath.Join(l.team.Directory, "idle.json"), s)
}
func (l *LockedIdle) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}

func (p AgentPaths) idleTeam() TeamPaths {
	d := filepath.Dir(filepath.Dir(p.Directory))
	return TeamPaths{Directory: d, State: filepath.Join(d, "team.json"), Agents: filepath.Join(d, "agents")}
}

// prepareIdlePublication records intent before the inbox link. Resolution under
// the same lock distinguishes a completed link from a crashed/failed publisher.
// Ordinary sends (including scheduled sends) arm work; automatic observations
// and continuations do not manufacture another episode.
func (p AgentPaths) prepareIdlePublication(e core.Envelope) (*LockedIdle, error) {
	l, err := p.idleTeam().LockIdle()
	if err != nil {
		return nil, err
	}
	s, err := l.Read()
	if err == nil {
		path, pathErr := p.EnvelopePath("new", e.ID)
		if pathErr != nil {
			err = pathErr
		} else if _, statErr := os.Stat(path); statErr == nil {
			err = ErrEnvelopeQueued
		} else if !errors.Is(statErr, os.ErrNotExist) {
			err = statErr
		}
	}
	if err == nil {
		s.Pending = &IdlePublication{Recipient: core.HitchID(filepath.Base(p.Directory)), ID: e.ID}
		err = l.Save(s)
	}
	if err != nil {
		_ = l.Close()
		return nil, err
	}
	return l, nil
}

// Resolve publication before any receipt can be pruned. A crash after the link
// but before this point must still count the accepted publication exactly once.
func (p AgentPaths) resolveIdlePublication() error {
	l, err := p.idleTeam().LockIdle()
	if err != nil {
		return err
	}
	defer l.Close()
	_, err = l.Read()
	return err
}
