//go:build acceptance

package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func observationFixture(t *testing.T) (TeamPaths, AgentPaths, core.Event) {
	t.Helper()
	team := testTeam(t)
	a := testAgent("a", "worker")
	l, err := team.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() }) // Deliberately keep the native input lock held.
	e := core.Event{Type: "observation", HitchID: a.ID, Name: a.Name, At: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}
	return team, l.Paths, e
}

func observationBytes(t *testing.T, p AgentPaths) []byte {
	t.Helper()
	data, err := os.ReadFile(p.Log)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestObservationConcurrentRepeatsAndRestart(t *testing.T) {
	team, p, e := observationFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			if err := p.AppendObservation(e, "same"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	want := observationBytes(t, p)
	info, err := os.Stat(filepath.Join(p.Directory, "last-observation"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("receipt is not private: %v %v", info, err)
	}
	if bytes.Count(want, []byte("\n")) != 1 {
		t.Fatal("concurrent repeat grew audit")
	}
	// Other event writers need not share the observation lock.
	if err := team.Append(core.Event{Type: "native_hook", At: e.At, HitchID: e.HitchID, Status: "activity"}); err != nil {
		t.Fatal(err)
	}
	want = observationBytes(t, p)
	reopened, err := team.Agent(e.HitchID)
	if err != nil {
		t.Fatal(err)
	}
	e.At = e.At.Add(time.Minute)
	if err := reopened.AppendObservation(e, "same"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, observationBytes(t, p)) {
		t.Fatal("reopened writer forgot existing evidence")
	}
	if err := p.AppendObservation(e, "changed"); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(observationBytes(t, p), []byte("\n")) != 3 {
		t.Fatal("changed observation lost")
	}
	if err := p.AppendObservation(e, "same"); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(observationBytes(t, p), []byte("\n")) != 4 {
		t.Fatal("return to earlier state suppressed")
	}
}

func TestObservationRequiresExistingExactAuditEvidence(t *testing.T) {
	for _, mode := range []string{"missing", "empty", "replaced-same-length", "other-destination", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			_, p, e := observationFixture(t)
			if err := p.AppendObservation(e, "same"); err != nil {
				t.Fatal(err)
			}
			original := observationBytes(t, p)
			switch mode {
			case "missing":
				if err := os.Rename(p.Log, p.Log+".preserved"); err != nil {
					t.Fatal(err)
				}
			case "empty":
				if err := os.WriteFile(p.Log, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "replaced-same-length":
				replacement := bytes.ReplaceAll(original, []byte("worker"), []byte("other!"))
				if bytes.Equal(original, replacement) {
					t.Fatal("fixture did not replace event")
				}
				if err := os.WriteFile(p.Log, replacement, 0600); err != nil {
					t.Fatal(err)
				}
			case "other-destination":
				p.Log += ".new"
				if err := os.WriteFile(p.Log, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "truncated":
				if err := os.Truncate(p.Log, int64(len(original)-1)); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(p.Log)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := p.AppendObservation(e, "same"); err != nil {
				t.Fatal(err)
			}
			after := observationBytes(t, p)
			if !bytes.Equal(after, append(before, original...)) {
				t.Fatal("marker suppressed missing evidence or changed existing bytes")
			}
		})
	}
}

func TestObservationFailuresLeaveRetryableAudit(t *testing.T) {
	t.Run("append", func(t *testing.T) {
		_, p, e := observationFixture(t)
		if err := os.Mkdir(p.Log, 0700); err != nil {
			t.Fatal(err)
		}
		if err := p.AppendObservation(e, "same"); err == nil {
			t.Fatal("append failure ignored")
		}
		if _, err := os.Stat(filepath.Join(p.Directory, "last-observation")); !os.IsNotExist(err) {
			t.Fatal("failed append wrote marker")
		}
		if err := os.Remove(p.Log); err != nil {
			t.Fatal(err)
		} // Empty fixture directory.
		if err := p.AppendObservation(e, "same"); err != nil {
			t.Fatal(err)
		}
		if bytes.Count(observationBytes(t, p), []byte("\n")) != 1 {
			t.Fatal("retry lost observation")
		}
	})
	t.Run("marker", func(t *testing.T) {
		_, p, e := observationFixture(t)
		failure := errors.New("fixture marker write failure")
		if err := p.appendObservation(e, "same", func(string, any) error { return failure }); !errors.Is(err, failure) {
			t.Fatalf("marker failure not returned: %v", err)
		}
		if bytes.Count(observationBytes(t, p), []byte("\n")) != 1 {
			t.Fatal("marker failure lost appended evidence")
		}
		if err := p.AppendObservation(e, "same"); err != nil {
			t.Fatal(err)
		}
		if bytes.Count(observationBytes(t, p), []byte("\n")) != 2 {
			t.Fatal("marker failure retry did not retain duplicate")
		}
	})
	t.Run("marker-read", func(t *testing.T) {
		_, p, e := observationFixture(t)
		path := filepath.Join(p.Directory, "last-observation")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		// An unreadable marker fails visibly before append and never suppresses.
		if err := p.AppendObservation(e, "same"); err == nil {
			t.Fatal("marker read failure ignored")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := p.AppendObservation(e, "same"); err != nil {
			t.Fatal(err)
		}
		// Losing only the marker is safe: retry may duplicate, never lose history.
		if err := os.Rename(path, path+".preserved"); err != nil {
			t.Fatal(err)
		}
		if err := p.AppendObservation(e, "same"); err != nil {
			t.Fatal(err)
		}
		if bytes.Count(observationBytes(t, p), []byte("\n")) != 2 {
			t.Fatal("missing marker suppressed retry")
		}
	})
}
