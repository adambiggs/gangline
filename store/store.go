package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adambiggs/gangline/core"
)

type Paths struct{ Root string }
type TeamPaths struct{ Directory, State, Log, Names, Agents string }
type AgentPaths struct{ Directory, State, Lock, Witness, Inbox string }

func segment(value string) error {
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, "\\\x00") {
		return fmt.Errorf("invalid state path segment %q", value)
	}
	return nil
}
func (p Paths) Team(name string) (TeamPaths, error) {
	if p.Root == "" {
		return TeamPaths{}, fmt.Errorf("state root is empty")
	}
	if err := segment(name); err != nil {
		return TeamPaths{}, err
	}
	d := filepath.Join(p.Root, "teams", name)
	return TeamPaths{d, filepath.Join(d, "team.json"), filepath.Join(d, "log.jsonl"), filepath.Join(d, "names"), filepath.Join(d, "agents")}, nil
}
func (p TeamPaths) Agent(id core.HitchID) (AgentPaths, error) {
	if err := segment(string(id)); err != nil {
		return AgentPaths{}, err
	}
	d := filepath.Join(p.Agents, string(id))
	return AgentPaths{d, filepath.Join(d, "agent.json"), filepath.Join(d, "lock"), filepath.Join(d, "witness"), filepath.Join(d, "inbox")}, nil
}
func (p TeamPaths) Create() error {
	for _, d := range []string{p.Names, p.Agents} {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}
	tmp, err := jsonTemp(p.Directory, core.Team{})
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Link(tmp, p.State); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}
func (p TeamPaths) ReadTeam() (core.Team, error) {
	var t core.Team
	err := readJSON(p.State, &t)
	return t, err
}
func (p TeamPaths) WriteTeam(t core.Team) error { return atomicJSON(p.State, t) }

var ErrNameTaken = errors.New("name is already claimed")

func (p TeamPaths) ClaimName(name core.AgentName, id core.HitchID) error {
	if err := segment(string(name)); err != nil {
		return err
	}
	if err := segment(string(id)); err != nil {
		return err
	}
	if err := os.Symlink(string(id), filepath.Join(p.Names, string(name))); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrNameTaken
		}
		return err
	}
	return nil
}
func (p TeamPaths) ResolveName(name string) (core.HitchID, error) {
	if err := segment(name); err != nil {
		return "", err
	}
	id, err := os.Readlink(filepath.Join(p.Names, name))
	if err != nil {
		return "", err
	}
	if err := segment(id); err != nil {
		return "", err
	}
	return core.HitchID(id), nil
}
func (p TeamPaths) RemoveName(name core.AgentName, id core.HitchID) error {
	if name == "" {
		return nil
	}
	got, err := p.ResolveName(string(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if got != id {
		return nil
	}
	return os.Remove(filepath.Join(p.Names, string(name)))
}
func (p TeamPaths) CreateAgent(a core.Agent) (*LockedAgent, error) {
	ap, err := p.Agent(a.ID)
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(ap.Directory, 0700); err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(ap.Directory)
		}
	}()
	for _, name := range []string{"tmp", "new", "cur", "failed"} {
		if err := os.MkdirAll(filepath.Join(ap.Inbox, name), 0700); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(ap.Lock, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := atomicJSON(ap.State, a); err != nil {
		return nil, err
	}
	l, err := ap.TryLock()
	if err != nil {
		return nil, err
	}
	if err := p.ClaimName(a.Name, a.ID); err != nil {
		_ = l.Close()
		return nil, err
	}
	success = true
	return l, nil
}
func (p AgentPaths) Read() (core.Agent, error) {
	var a core.Agent
	if err := readJSON(p.State, &a); err != nil {
		return a, err
	}
	if string(a.ID) != filepath.Base(p.Directory) || a.Name == "" || a.Collar == "" {
		return a, fmt.Errorf("decode %s: invalid agent identity", p.State)
	}
	return a, nil
}
func (p TeamPaths) ListAgents() ([]core.Agent, error) {
	entries, err := os.ReadDir(p.Agents)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var agents []core.Agent
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ap, err := p.Agent(core.HitchID(entry.Name()))
		if err != nil {
			return nil, err
		}
		a, err := ap.Read()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		id, err := p.ResolveName(string(a.Name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if id == a.ID {
			agents = append(agents, a)
		}
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	return agents, nil
}
func (p TeamPaths) FinishRename(l *LockedAgent, a *core.Agent) error {
	if a.RenameTo == "" {
		return nil
	}
	id, err := p.ResolveName(string(a.RenameTo))
	if errors.Is(err, os.ErrNotExist) {
		err = p.ClaimName(a.RenameTo, a.ID)
		id = a.ID
	}
	if errors.Is(err, ErrNameTaken) || err == nil && id != a.ID {
		a.RenameFrom, a.RenameTo = "", ""
		if err := l.Save(*a); err != nil {
			return err
		}
		return ErrNameTaken
	}
	if err != nil {
		return err
	}
	a.Name = a.RenameTo
	if err := l.Save(*a); err != nil {
		return err
	}
	if err := p.RemoveName(a.RenameFrom, a.ID); err != nil {
		return err
	}
	a.RenameFrom, a.RenameTo = "", ""
	return l.Save(*a)
}
func readJSON(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode %s: trailing data", path)
	}
	return nil
}
func jsonTemp(dir string, value any) (string, error) {
	f, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := json.NewEncoder(f).Encode(value); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
func atomicJSON(path string, value any) error {
	name, err := jsonTemp(filepath.Dir(path), value)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(name)
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
