package store

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adambiggs/gangline/core"
)

var ErrSenderUnauthorized = errors.New("registered sender credential is invalid or revoked")

func IsSenderID(id core.HitchID) bool { return strings.HasPrefix(string(id), "sender-") }

type SenderRegistration struct {
	ID             core.HitchID           `json:"id"`
	Name           core.AgentName         `json:"name"`
	CredentialHash string                 `json:"credential_hash"`
	Created        *core.CredentialOrigin `json:"created,omitempty"`
}

func (p TeamPaths) SenderCredential(id core.HitchID) (string, error) {
	if err := segment(string(id)); err != nil {
		return "", err
	}
	if !IsSenderID(id) {
		return "", ErrSenderUnauthorized
	}
	return filepath.Join(p.Directory, "senders", string(id), "key"), nil
}

func (p TeamPaths) ReadSender(name string) (SenderRegistration, error) {
	id, err := p.ResolveName(name)
	if err != nil {
		return SenderRegistration{}, err
	}
	key, err := p.SenderCredential(id)
	if err != nil {
		return SenderRegistration{}, err
	}
	var r SenderRegistration
	if err := readJSON(filepath.Join(filepath.Dir(key), "sender.json"), &r); err != nil {
		return r, err
	}
	if r.ID != id || string(r.Name) != name {
		return SenderRegistration{}, ErrSenderUnauthorized
	}
	return r, nil
}

// AddSender requires the team lock. ClaimName also fences concurrent renames.
func (p TeamPaths) AddSender(name string, origin core.CredentialOrigin) (path string, result error) {
	if err := validateCredentialOrigin(origin); err != nil {
		return "", err
	}
	if _, err := p.ReadTeam(); err != nil {
		return "", err
	}
	token, err := outsideRandom()
	if err != nil {
		return "", err
	}
	identity, err := outsideRandom()
	if err != nil {
		return "", err
	}
	r := SenderRegistration{ID: core.HitchID("sender-" + identity), Name: core.AgentName(name), CredentialHash: outsideHash(token), Created: &origin}
	path, err = p.SenderCredential(r.ID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	claimed := false
	defer func() {
		if result != nil && !claimed {
			result = errors.Join(result, os.RemoveAll(filepath.Dir(path)))
		}
	}()
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		return path, err
	}
	if err := atomicJSON(filepath.Join(filepath.Dir(path), "sender.json"), r); err != nil {
		return path, err
	}
	if err := p.ClaimName(r.Name, r.ID); err != nil {
		return path, err
	}
	claimed = true
	if err := p.auditCredential("sender_added", r.ID, r.Name, origin); err != nil {
		return path, fmt.Errorf("%w; credential file: %s", err, path)
	}
	return path, nil
}

func (p TeamPaths) ListSenders() ([]SenderRegistration, error) {
	entries, err := os.ReadDir(p.Names)
	if err != nil {
		return nil, err
	}
	result := []SenderRegistration{}
	for _, e := range entries {
		id, err := p.ResolveName(e.Name())
		if err != nil {
			return nil, err
		}
		if !IsSenderID(id) {
			continue
		}
		r, err := p.ReadSender(e.Name())
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// RemoveSender requires the team lock and leaves the claim until cleanup succeeds.
func (p TeamPaths) RemoveSender(name string, origin core.CredentialOrigin) error {
	if err := validateCredentialOrigin(origin); err != nil {
		return err
	}
	id, err := p.ResolveName(name)
	if errors.Is(err, os.ErrNotExist) {
		return ErrSenderUnauthorized
	}
	if err != nil {
		return err
	}
	key, err := p.SenderCredential(id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(key)); err != nil {
		return err
	}
	if err := p.RemoveName(core.AgentName(name), id); err != nil {
		return err
	}
	return p.auditCredential("sender_removed", id, core.AgentName(name), origin)
}

func (p TeamPaths) AuthenticateSender(name string, credential []byte) (core.Sender, error) {
	r, err := p.ReadSender(name)
	if err != nil {
		return core.Sender{}, fmt.Errorf("%w: %v", ErrSenderUnauthorized, err)
	}
	hash := outsideHash(strings.TrimSpace(string(credential)))
	if subtle.ConstantTimeCompare([]byte(hash), []byte(r.CredentialHash)) != 1 {
		return core.Sender{}, ErrSenderUnauthorized
	}
	return core.Sender{Kind: core.SenderRegistered, Name: r.Name, HitchID: r.ID}, nil
}
