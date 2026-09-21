package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

var storeNow = time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
var storeDeadline = storeNow.Add(time.Minute)

func TestTeamPathsStayInsideRoot(t *testing.T) {
	paths, err := (Paths{Root: "/state/gangline"}).Team("example")
	if err != nil {
		t.Fatal(err)
	}
	if paths.Events != "/state/gangline/v1/example/events.jsonl" {
		t.Fatalf("events path = %q", paths.Events)
	}
	invalid := []string{"", ".", "..", "../other", "other/team"}
	for _, team := range invalid {
		if _, err := (Paths{Root: "/state/gangline"}).Team(team); err == nil {
			t.Fatalf("team %q passed path validation", team)
		}
	}
}

func TestTeamLockRefusesContentionImmediately(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	first, err := paths.Lock("example")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := paths.Lock("example"); !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock error = %v, want ErrLocked", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := paths.Lock("example")
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAppendLoadSnapshotAndRecoverIntent(t *testing.T) {
	team := lockedTeam(t)
	initial := core.NewState(core.Team{ID: "team-1", Name: "example"})
	hitch := core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work"}
	requested := core.HitchRequested{At: storeNow, Hitch: hitch, BootDeadline: storeDeadline}
	if err := team.Append(requested); err != nil {
		t.Fatal(err)
	}
	state, count, err := team.Load(initial)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || state.Hitches["h-1"].Status != core.HitchStarting {
		t.Fatalf("count=%d state=%#v", count, state)
	}
	if effects := core.PendingEffects(state); len(effects) != 1 || reflect.TypeOf(effects[0]) != reflect.TypeOf(core.SpawnHitch{}) {
		t.Fatalf("pending effects = %#v", effects)
	}

	spawned := core.HitchSpawned{At: storeNow, HitchID: "h-1", Pane: "%1"}
	if err := team.Append(spawned); err != nil {
		t.Fatal(err)
	}
	state, _, err = team.Load(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := team.SaveSnapshot(state); err != nil {
		t.Fatal(err)
	}
	if err := team.Append(core.HitchReady{At: storeNow, HitchID: "h-1"}); err != nil {
		t.Fatal(err)
	}
	state, count, err = team.Load(initial)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || state.Hitches["h-1"].Status != core.HitchActive || state.Hitches["h-1"].Activity != core.ActivityIdle {
		t.Fatalf("count=%d state=%#v", count, state)
	}
	if effects := core.PendingEffects(state); len(effects) != 0 {
		t.Fatalf("pending effects = %#v", effects)
	}
}

func TestLogAndReplayDataFunctions(t *testing.T) {
	initial := core.NewState(core.Team{ID: "team-1", Name: "example"})
	events := []core.Event{
		core.AdoptRequested{At: storeNow, Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work"}, Pane: "%1"},
		core.TurnStarted{At: storeNow, HitchID: "h-1"},
		core.WedgeDetected{At: storeNow, HitchID: "h-1", Evidence: "unchanged screen"},
	}
	var log bytes.Buffer
	for _, event := range events {
		data, err := core.EncodeEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		log.Write(data)
		log.WriteByte('\n')
	}
	entries, err := ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[2].Sequence != 3 || core.EventName(entries[2].Event) != "wedge_detected" {
		t.Fatalf("entries = %#v", entries)
	}
	state := Replay(initial, entries)
	if state.Hitches["h-1"].Activity != core.ActivityWedged || state.Hitches["h-1"].WedgeEvidence != "unchanged screen" {
		t.Fatalf("replayed state = %#v", state)
	}
}

func TestReadLogRejectsTornAndInvalidRecords(t *testing.T) {
	tests := []struct {
		name string
		log  string
	}{
		{name: "torn final record", log: `{"type":"wedge_cleared"}`},
		{name: "empty record", log: "\n"},
		{name: "invalid event", log: "{}\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReadLog(bytes.NewBufferString(test.log)); err == nil {
				t.Fatal("invalid log was accepted")
			}
		})
	}
}

func TestSnapshotRejectsStateNotDerivedFromLog(t *testing.T) {
	team := lockedTeam(t)
	state := core.NewState(core.Team{ID: "team-1", Name: "example"})
	state.Hitches["invented"] = core.Hitch{ID: "invented", Name: "invented", Status: core.HitchActive}
	if err := team.SaveSnapshot(state); err == nil {
		t.Fatal("snapshot accepted state not present in the event log")
	}
}

func TestLoadRejectsSnapshotWhoseLogPrefixChanged(t *testing.T) {
	team := lockedTeam(t)
	initial := core.NewState(core.Team{ID: "team-1", Name: "example"})
	event := core.AdoptRequested{At: storeNow, Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work"}, Pane: "%1"}
	if err := team.Append(event); err != nil {
		t.Fatal(err)
	}
	state, _, err := team.Load(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := team.SaveSnapshot(state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(team.paths.Events)
	if err != nil {
		t.Fatal(err)
	}
	data[0] = '['
	if err := os.WriteFile(team.paths.Events, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := team.Load(initial); err == nil {
		t.Fatal("snapshot accepted a changed event-log prefix")
	}
}

func TestSnapshotReplacementLeavesNoTemporaryFiles(t *testing.T) {
	team := lockedTeam(t)
	state := core.NewState(core.Team{ID: "team-1", Name: "example"})
	if err := team.SaveSnapshot(state); err != nil {
		t.Fatal(err)
	}
	if err := team.SaveSnapshot(state); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(team.paths.Directory, ".snapshot-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("snapshot temporary files remain: %v", matches)
	}
}

func TestSnapshotCanonicalizesTimesAndPreservesCurfew(t *testing.T) {
	team := lockedTeam(t)
	now := time.Now()
	deadline := now.Add(time.Hour)
	initial := core.NewState(core.Team{ID: "team-1", Name: "example"})
	event := core.CurfewSet{At: now, Deadline: deadline}
	if err := team.Append(event); err != nil {
		t.Fatal(err)
	}
	state, effects := core.Step(initial, event)
	if len(effects) != 0 {
		t.Fatalf("effects = %#v", effects)
	}
	if err := team.SaveSnapshot(state); err != nil {
		t.Fatalf("save state containing monotonic times: %v", err)
	}
	loaded, count, err := team.Load(initial)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !loaded.Team.Curfew.Equal(deadline) {
		t.Fatalf("count=%d curfew=%s", count, loaded.Team.Curfew)
	}
}

func lockedTeam(t *testing.T) *LockedTeam {
	t.Helper()
	team, err := (Paths{Root: t.TempDir()}).Lock("example")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := team.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return team
}
