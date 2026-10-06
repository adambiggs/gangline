package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSessionConflictReplacesPrivateSnapshot(t *testing.T) {
	p := AgentPaths{Directory: t.TempDir()}
	path := filepath.Join(p.Directory, "native-session-conflict")
	for _, session := range []string{"other", "later"} {
		want := SessionConflict{SessionID: "registered", Transcript: "registered.jsonl", Witness: Witness{
			ID: "receipt", SessionID: session, TurnID: "turn", Transcript: "other.jsonl",
			At: time.Date(2026, 10, 6, 17, 17, 38, 0, time.UTC), Prompt: "first\n\nsecond\x00",
		}}
		if err := p.WriteSessionConflict(want); err != nil {
			t.Fatal(err)
		}
		var got SessionConflict
		if err := readJSON(path, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("snapshot = %#v, want %#v", got, want)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot mode = %o", info.Mode().Perm())
	}
	entries, err := os.ReadDir(p.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "native-session-conflict" {
		t.Fatalf("snapshot left extra files: %v", entries)
	}
}
