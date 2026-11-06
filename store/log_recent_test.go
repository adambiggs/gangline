package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// ReadRecent reads only the window at the end of the log, skips the line the
// window cuts, and decodes only the lines kept.
func TestReadRecentVisitsKeptEventsInTheWindow(t *testing.T) {
	event := func(kind, id string) string {
		return `{"type":"` + kind + `","at":"2026-11-06T00:00:00Z","id":"` + id + `"}` + "\n"
	}
	old := event("send_cancelled", "old")
	cut := event("send_cancelled", "cut")
	recent := event("send_cancelled", "recent") + event("native_hook", "skipped") + `{"type":"future_kind"}` + "\n" + event("send_cancelled", "last")
	p := TeamPaths{Log: filepath.Join(t.TempDir(), "log.jsonl")}
	if err := os.WriteFile(p.Log, []byte(old+cut+recent), 0o600); err != nil {
		t.Fatal(err)
	}
	var ids []string
	keep := func(line []byte) bool { return !bytes.Contains(line, []byte("native_hook")) }
	visit := func(e core.Event) error { ids = append(ids, e.ID); return nil }
	if err := p.ReadRecent(int64(len(recent)+len(cut)/2), keep, visit); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "recent,last" {
		t.Fatalf("visited %q", got)
	}
	ids = nil
	if err := p.ReadRecent(1<<20, keep, visit); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids, ","); got != "old,cut,recent,last" {
		t.Fatalf("whole log visited %q", got)
	}
	if err := (TeamPaths{Log: p.Log + ".missing"}).ReadRecent(1, keep, visit); !os.IsNotExist(err) {
		t.Fatalf("missing log: %v", err)
	}
}
