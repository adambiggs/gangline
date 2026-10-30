package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adambiggs/gangline/core"
)

var ErrOutsideUnauthorized = errors.New("outside credential is invalid or revoked")

// OutsideRegistration shares the team's name namespace, with a mailbox route.
type OutsideRegistration struct {
	ID      core.HitchID           `json:"id"`
	Name    core.AgentName         `json:"name"`
	Created *core.CredentialOrigin `json:"created,omitempty"`
}

type OutsidePrincipal struct {
	OutsideRegistration
	generation string
}

func (p OutsidePrincipal) String() string { return string(p.ID) + "." + p.generation }
func (p OutsidePrincipal) Sender() core.Sender {
	return core.Sender{Kind: core.SenderOutside, Name: p.Name, HitchID: p.ID}
}

type outsideState struct {
	OutsideRegistration
	CredentialHash string          `json:"credential_hash"`
	Generation     string          `json:"generation"`
	Pending        []core.Envelope `json:"pending"`
}

// OutsideMailbox is valid only during WithOutside. Its methods persist before
// returning, so a callback can publish an idempotent send after saving its intent.
type OutsideMailbox struct {
	Registration  OutsideRegistration
	state         outsideState
	path          string
	dataDirectory string
}

func (m *OutsideMailbox) Pending() []core.Envelope {
	return append([]core.Envelope{}, m.state.Pending...)
}
func (m *OutsideMailbox) Ack(ids []core.EnvelopeID) error {
	acknowledged := make(map[core.EnvelopeID]bool, len(ids))
	for _, id := range ids {
		acknowledged[id] = true
	}
	retained := make([]core.Envelope, 0, len(m.state.Pending))
	for _, e := range m.state.Pending {
		if !acknowledged[e.ID] {
			retained = append(retained, e)
		}
	}
	m.state.Pending = retained
	return atomicJSON(m.path, m.state)
}

// Keyed records live separately from the mailbox snapshot, so settled sends
// do not add work to authentication, fetches or later queue operations.
func (m *OutsideMailbox) dataPath(key string) string {
	return filepath.Join(m.dataDirectory, outsideHash(key)+".json")
}
func (m *OutsideMailbox) Get(key string, out any) (bool, error) {
	err := readJSON(m.dataPath(key), out)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
func (m *OutsideMailbox) Put(key string, value any) error {
	if err := os.MkdirAll(m.dataDirectory, 0700); err != nil {
		return err
	}
	return atomicJSON(m.dataPath(key), value)
}
func (m *OutsideMailbox) Delete(key string) error {
	err := os.Remove(m.dataPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func outsideRandom() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func outsideHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
func (p TeamPaths) outsidePath(id core.HitchID) (string, error) {
	if err := segment(string(id)); err != nil {
		return "", err
	}
	return filepath.Join(p.Directory, "outside", string(id)+".json"), nil
}
func (p TeamPaths) outsideState(id core.HitchID) (outsideState, error) {
	var state outsideState
	path, err := p.outsidePath(id)
	if err != nil {
		return state, err
	}
	if err := readJSON(path, &state); err != nil {
		return state, err
	}
	if state.ID != id || state.Name == "" || state.Generation == "" || len(state.CredentialHash) != 64 {
		return state, fmt.Errorf("invalid outside registration %s", id)
	}
	current, err := p.ResolveName(string(state.Name))
	if err != nil {
		return state, err
	}
	if current != id {
		return state, ErrOutsideUnauthorized
	}
	return state, nil
}
func (p TeamPaths) ResolveOutside(name string) (OutsideRegistration, error) {
	id, err := p.ResolveName(name)
	if err != nil {
		return OutsideRegistration{}, err
	}
	if !strings.HasPrefix(string(id), "outside-") {
		return OutsideRegistration{}, os.ErrNotExist
	}
	state, err := p.outsideState(id)
	return state.OutsideRegistration, err
}
func (p TeamPaths) ListOutside() ([]OutsideRegistration, error) {
	entries, err := os.ReadDir(filepath.Join(p.Directory, "outside"))
	if errors.Is(err, os.ErrNotExist) {
		return []OutsideRegistration{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []OutsideRegistration{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		state, err := p.outsideState(core.HitchID(strings.TrimSuffix(entry.Name(), ".json")))
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrOutsideUnauthorized) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, state.OutsideRegistration)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// RegisterOutside issues a bearer credential only into a newly created private
// file. State retains its hash; plaintext is never returned or logged.
func (p Paths) RegisterOutside(team, name, credentialFile string, origin core.CredentialOrigin) (OutsideRegistration, error) {
	if err := validateCredentialOrigin(origin); err != nil {
		return OutsideRegistration{}, err
	}
	lock, err := p.LockTeam(team)
	if err != nil {
		return OutsideRegistration{}, err
	}
	defer lock.Close()
	t, err := p.Team(team)
	if err != nil {
		return OutsideRegistration{}, err
	}
	if err := segment(name); err != nil {
		return OutsideRegistration{}, err
	}
	if _, err := t.ReadTeam(); err != nil {
		return OutsideRegistration{}, err
	}
	if _, err := t.ResolveName(name); err == nil {
		return OutsideRegistration{}, ErrNameTaken
	} else if !errors.Is(err, os.ErrNotExist) {
		return OutsideRegistration{}, err
	}
	id, err := outsideRandom()
	if err != nil {
		return OutsideRegistration{}, err
	}
	state := outsideState{OutsideRegistration: OutsideRegistration{ID: core.HitchID("outside-" + id), Name: core.AgentName(name), Created: &origin}, Pending: []core.Envelope{}}
	if err := os.MkdirAll(filepath.Join(t.Directory, "outside"), 0700); err != nil {
		return OutsideRegistration{}, err
	}
	path, _ := t.outsidePath(state.ID)
	if err := issueOutsideCredential(&state, credentialFile); err != nil {
		return OutsideRegistration{}, err
	}
	if err := atomicJSON(path, state); err != nil {
		return OutsideRegistration{}, errors.Join(err, os.Remove(credentialFile))
	}
	if err := t.ClaimName(state.Name, state.ID); err != nil {
		return OutsideRegistration{}, errors.Join(err, os.Remove(path), os.Remove(credentialFile))
	}
	return state.OutsideRegistration, t.auditCredential("outside_registered", state.ID, state.Name, origin)
}
func issueOutsideCredential(state *outsideState, path string) (result error) {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("credential file must be an absolute path")
	}
	token, err := outsideRandom()
	if err != nil {
		return err
	}
	generation, err := outsideRandom()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, f.Close())
		if result != nil {
			result = errors.Join(result, os.Remove(path))
		}
	}()
	if _, err := fmt.Fprintln(f, token); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	state.CredentialHash, state.Generation = outsideHash(token), generation
	return nil
}
func (p Paths) RotateOutside(team, name, credentialFile string, origin core.CredentialOrigin) error {
	if err := validateCredentialOrigin(origin); err != nil {
		return err
	}
	lock, err := p.LockTeam(team)
	if err != nil {
		return err
	}
	defer lock.Close()
	t, err := p.Team(team)
	if err != nil {
		return err
	}
	registration, err := t.ResolveOutside(name)
	if err != nil {
		return err
	}
	state, err := t.outsideState(registration.ID)
	if err != nil {
		return err
	}
	if err := issueOutsideCredential(&state, credentialFile); err != nil {
		return err
	}
	path, _ := t.outsidePath(state.ID)
	if err := atomicJSON(path, state); err != nil {
		return errors.Join(err, os.Remove(credentialFile))
	}
	return t.auditCredential("outside_rotated", state.ID, state.Name, origin)
}
func (p TeamPaths) RemoveOutside(name string, origin core.CredentialOrigin) error {
	if err := validateCredentialOrigin(origin); err != nil {
		return err
	}
	registration, err := p.ResolveOutside(name)
	if err != nil {
		return err
	}
	if err := p.RemoveName(registration.Name, registration.ID); err != nil {
		return err
	}
	auditErr := p.auditCredential("outside_removed", registration.ID, registration.Name, origin)
	dataErr := os.RemoveAll(filepath.Join(p.Directory, "outside-data", string(registration.ID)))
	path, _ := p.outsidePath(registration.ID)
	return errors.Join(dataErr, os.Remove(path), auditErr)
}
func (p Paths) RemoveOutside(team, name string, origin core.CredentialOrigin) error {
	lock, err := p.LockTeam(team)
	if err != nil {
		return err
	}
	defer lock.Close()
	t, err := p.Team(team)
	if err != nil {
		return err
	}
	return t.RemoveOutside(name, origin)
}
func (p Paths) AuthenticateOutside(team, token string) (OutsidePrincipal, error) {
	t, err := p.Team(team)
	if err != nil {
		return OutsidePrincipal{}, err
	}
	registrations, err := t.ListOutside()
	if err != nil {
		return OutsidePrincipal{}, err
	}
	digest := outsideHash(token)
	for _, registration := range registrations {
		state, err := t.outsideState(registration.ID)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrOutsideUnauthorized) {
			continue
		}
		if err != nil {
			return OutsidePrincipal{}, err
		}
		if subtle.ConstantTimeCompare([]byte(digest), []byte(state.CredentialHash)) == 1 {
			return OutsidePrincipal{registration, state.Generation}, nil
		}
	}
	return OutsidePrincipal{}, ErrOutsideUnauthorized
}
func (p Paths) ResolveOutsidePrincipal(team, principal string) (OutsidePrincipal, error) {
	id, generation, ok := strings.Cut(principal, ".")
	if !ok {
		return OutsidePrincipal{}, ErrOutsideUnauthorized
	}
	t, err := p.Team(team)
	if err != nil {
		return OutsidePrincipal{}, err
	}
	state, err := t.outsideState(core.HitchID(id))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrOutsideUnauthorized) {
		return OutsidePrincipal{}, ErrOutsideUnauthorized
	}
	if err != nil {
		return OutsidePrincipal{}, err
	}
	if subtle.ConstantTimeCompare([]byte(generation), []byte(state.Generation)) != 1 {
		return OutsidePrincipal{}, ErrOutsideUnauthorized
	}
	return OutsidePrincipal{state.OutsideRegistration, state.Generation}, nil
}

// WithOutside rechecks the registration under the same lock used for rotation,
// removal and registration. Identity remains valid for the callback's operation.
func (p Paths) WithOutside(team string, principal OutsidePrincipal, fn func(*OutsideMailbox) error) error {
	lock, err := p.LockTeam(team)
	if err != nil {
		return err
	}
	defer lock.Close()
	current, err := p.ResolveOutsidePrincipal(team, principal.String())
	if err != nil {
		return err
	}
	if current.Name != principal.Name {
		return ErrOutsideUnauthorized
	}
	t, _ := p.Team(team)
	state, err := t.outsideState(current.ID)
	if err != nil {
		return err
	}
	path, _ := t.outsidePath(current.ID)
	return fn(&OutsideMailbox{Registration: current.OutsideRegistration, state: state, path: path, dataDirectory: filepath.Join(t.Directory, "outside-data", string(current.ID))})
}
func (p Paths) QueueOutside(team, name string, e core.Envelope) error {
	lock, err := p.LockTeam(team)
	if err != nil {
		return err
	}
	defer lock.Close()
	t, err := p.Team(team)
	if err != nil {
		return err
	}
	registration, err := t.ResolveOutside(name)
	if err != nil {
		return err
	}
	if e.Recipient != registration.ID || e.To != registration.Name {
		return fmt.Errorf("outside recipient registration changed; retry")
	}
	state, err := t.outsideState(registration.ID)
	if err != nil {
		return err
	}
	for _, pending := range state.Pending {
		if pending.ID == e.ID {
			return ErrEnvelopeQueued
		}
	}
	state.Pending = append(state.Pending, e)
	path, _ := t.outsidePath(registration.ID)
	return atomicJSON(path, state)
}

// ReadOutsidePending reads an atomic mailbox snapshot without contending with
// writers. It checks the credential generation in that same snapshot.
func (p Paths) ReadOutsidePending(team string, principal OutsidePrincipal) ([]core.Envelope, error) {
	t, err := p.Team(team)
	if err != nil {
		return nil, err
	}
	state, err := t.outsideState(principal.ID)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrOutsideUnauthorized) {
		return nil, ErrOutsideUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if state.Name != principal.Name || subtle.ConstantTimeCompare([]byte(state.Generation), []byte(principal.generation)) != 1 {
		return nil, ErrOutsideUnauthorized
	}
	return append([]core.Envelope{}, state.Pending...), nil
}
