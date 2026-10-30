//go:build acceptance

package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestTranscriptFileIsOpenedNonblocking(t *testing.T) {
	path := claudeTranscript(t, "0 user p1")
	f, err := OpenTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_NONBLOCK == 0 {
		t.Fatalf("transcript open can wait for a writer: flags=%x err=%v", flags, err)
	}
}

func TestTranscriptFileRejectsNonregularFiles(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "transcript.fifo")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	// Keep both ends present so a regressed blocking open cannot hang the test.
	keeper, err := os.OpenFile(fifo, os.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer keeper.Close()
	for _, path := range []string{fifo, t.TempDir(), "/dev/null"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := OpenTranscript(path)
			if f != nil {
				f.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Fatalf("transcript %s: %v", path, err)
			}

			payload, err := json.Marshal(map[string]string{"transcript_path": path, "session_id": "s"})
			if err != nil {
				t.Fatal(err)
			}
			used := int64(1)
			readers := map[string]func() error{
				"queue":        func() error { _, err := claudeQueueRecords(path); return err },
				"session move": func() error { _, err := claudeContinuedIn(path, "s"); return err },
				"statusline":   func() error { _, err := StatuslineMeasurementTime(payload, Reading{Used: &used}); return err },
			}
			for name, read := range readers {
				if err := read(); err == nil || !strings.Contains(err.Error(), "not a regular file") {
					t.Fatalf("%s bypassed regular-file guard: %v", name, err)
				}
			}
			collar, err := EmbeddedCollar("claude")
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			dir := filepath.Join(root, "projects", "fixture")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(path, filepath.Join(dir, "s.jsonl")); err != nil {
				t.Fatal(err)
			}
			collar.Launch.Env = map[string]string{"CLAUDE_CONFIG_DIR": root}
			unknown, err := ValidateResume(collar, "s")
			if err != nil || !strings.Contains(unknown, "not a regular file") {
				t.Fatalf("resume bypassed regular-file guard: unknown=%q err=%v", unknown, err)
			}

		})
	}
}
