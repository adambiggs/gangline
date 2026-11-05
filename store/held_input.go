package store

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// HeldInput retains a draft fingerprint, never its text, under the agent lock.
// It is separate from the agent record so older readers can ignore it.
type HeldInput struct {
	Fingerprint string    `json:"fingerprint"`
	Generation  string    `json:"generation"`
	Session     string    `json:"session"`
	SubmittedAt time.Time `json:"submitted_at"`
	Since       time.Time `json:"since"`
	Noticed     bool      `json:"noticed"`
}

func (l *LockedAgent) ReadHeldInput() (HeldInput, error) {
	var draft HeldInput
	err := readJSON(filepath.Join(l.Paths.Directory, "held-input.json"), &draft)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return draft, err
}

func (l *LockedAgent) SaveHeldInput(draft *HeldInput) error {
	path := filepath.Join(l.Paths.Directory, "held-input.json")
	if draft == nil {
		return removeIfPresent(path)
	}
	return atomicJSON(path, draft)
}
