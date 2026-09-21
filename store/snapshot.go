package store

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/adambiggs/gangline/core"
)

type Snapshot struct {
	Version    string     `json:"version"`
	EventCount uint64     `json:"event_count"`
	LogBytes   int64      `json:"log_bytes"`
	LogSHA256  string     `json:"log_sha256"`
	State      core.State `json:"state"`
}

func (team *LockedTeam) Load(initial core.State) (core.State, uint64, error) {
	if err := team.checkLocked(); err != nil {
		return core.State{}, 0, err
	}
	snapshot, found, err := team.readSnapshot()
	if err != nil {
		return core.State{}, 0, err
	}
	file, err := os.Open(team.paths.Events)
	if errorsIsNotExist(err) {
		if found {
			return core.State{}, 0, errors.New("snapshot exists without its event log")
		}
		return initial, 0, nil
	}
	if err != nil {
		return core.State{}, 0, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	state := initial
	start := uint64(0)
	if found {
		if snapshot.State.Team.ID != initial.Team.ID || snapshot.State.Team.Name != initial.Team.Name {
			return core.State{}, 0, errors.New("snapshot team does not match requested team")
		}
		if err := verifySnapshotPrefix(file, snapshot); err != nil {
			return core.State{}, 0, err
		}
		state = snapshot.State
		start = snapshot.EventCount
		if _, err := file.Seek(snapshot.LogBytes, io.SeekStart); err != nil {
			return core.State{}, 0, fmt.Errorf("seek past snapshot prefix: %w", err)
		}
	}
	entries, err := ReadLog(file)
	if err != nil {
		return core.State{}, 0, fmt.Errorf("read events after snapshot: %w", err)
	}
	for index := range entries {
		entries[index].Sequence += start
	}
	return Replay(state, entries), start + uint64(len(entries)), nil
}

func (team *LockedTeam) SaveSnapshot(state core.State) error {
	if err := team.checkLocked(); err != nil {
		return err
	}
	logFile, err := os.OpenFile(team.paths.Events, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open event log for snapshot: %w", err)
	}
	entries, err := ReadLog(logFile)
	if closeErr := logFile.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("validate event log for snapshot: %w", err)
	}
	initialTeam := state.Team
	initialTeam.Curfew = time.Time{}
	replayed := Replay(core.NewState(initialTeam), entries)
	replayedJSON, err := json.Marshal(replayed)
	if err != nil {
		return fmt.Errorf("encode replayed state for snapshot: %w", err)
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode state for snapshot: %w", err)
	}
	if !bytes.Equal(replayedJSON, stateJSON) {
		return errors.New("snapshot state does not match replayed event log")
	}
	count, size, digest, err := logIdentity(team.paths.Events)
	if err != nil {
		return err
	}
	snapshot := Snapshot{
		Version: LayoutVersion, EventCount: count, LogBytes: size,
		LogSHA256: digest, State: state,
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(team.paths.Directory, ".snapshot-*")
	if err != nil {
		return fmt.Errorf("create snapshot temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set snapshot permissions: %w", err)
	}
	if err := writeFull(temporary, data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}
	if err := os.Rename(temporaryName, team.paths.Snapshot); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	removeTemporary = false
	return syncDirectory(team.paths.Directory)
}

func (team *LockedTeam) readSnapshot() (Snapshot, bool, error) {
	data, err := os.ReadFile(team.paths.Snapshot)
	if errorsIsNotExist(err) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot Snapshot
	decoder := json.NewDecoder(bufio.NewReaderSize(bytes.NewReader(data), len(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, false, fmt.Errorf("decode snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Snapshot{}, false, errors.New("snapshot contains trailing data")
	}
	if snapshot.Version != LayoutVersion {
		return Snapshot{}, false, fmt.Errorf("snapshot version %q is not %q", snapshot.Version, LayoutVersion)
	}
	return snapshot, true, nil
}

func verifySnapshotPrefix(file *os.File, snapshot Snapshot) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat event log: %w", err)
	}
	if snapshot.LogBytes < 0 || snapshot.LogBytes > info.Size() {
		return errors.New("snapshot refers past the end of the event log")
	}
	hash := sha256.New()
	counting := &lineCounter{}
	if _, err := io.CopyN(io.MultiWriter(hash, counting), file, snapshot.LogBytes); err != nil {
		return fmt.Errorf("hash snapshot event prefix: %w", err)
	}
	if counting.trailing != 0 || counting.lines != snapshot.EventCount {
		return errors.New("snapshot event count does not match its log prefix")
	}
	if hex.EncodeToString(hash.Sum(nil)) != snapshot.LogSHA256 {
		return errors.New("snapshot event prefix does not match the event log")
	}
	return nil
}

func logIdentity(path string) (uint64, int64, string, error) {
	file, err := os.Open(path)
	if errorsIsNotExist(err) {
		empty := sha256.Sum256(nil)
		return 0, 0, hex.EncodeToString(empty[:]), nil
	}
	if err != nil {
		return 0, 0, "", fmt.Errorf("open event log for snapshot: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	counting := &lineCounter{}
	size, err := io.Copy(io.MultiWriter(hash, counting), file)
	if err != nil {
		return 0, 0, "", fmt.Errorf("read event log for snapshot: %w", err)
	}
	if counting.trailing != 0 {
		return 0, 0, "", errors.New("event log is not newline terminated")
	}
	return counting.lines, size, hex.EncodeToString(hash.Sum(nil)), nil
}

type lineCounter struct {
	lines    uint64
	trailing int
}

func (counter *lineCounter) Write(data []byte) (int, error) {
	for _, value := range data {
		counter.trailing++
		if value == '\n' {
			counter.lines++
			counter.trailing = 0
		}
	}
	return len(data), nil
}
