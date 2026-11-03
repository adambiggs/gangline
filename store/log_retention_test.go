//go:build acceptance

package store

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentRetainLogUsesOneIdentity(t *testing.T) {
	p := testTeam(t)
	start := make(chan struct{})
	var wg sync.WaitGroup
	paths := make([]string, 8)
	for i := range paths {
		wg.Go(func() {
			<-start
			path, err := p.RetainLog()
			if err != nil {
				t.Error(err)
			}
			paths[i] = path
		})
	}
	close(start)
	wg.Wait()
	for _, path := range paths {
		if path == "" || path != paths[0] {
			t.Fatalf("different retained identities: %v", paths)
		}
	}
}

func TestRetainLogSurvivesRemovalAndTeamNameReuse(t *testing.T) {
	p := testTeam(t)
	original := []byte("{\"type\":\"old-event\"}\nunfinished original bytes")
	if err := os.WriteFile(p.Log, original, 0600); err != nil {
		t.Fatal(err)
	}
	writer, err := os.OpenFile(p.Log, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	path, err := p.RetainLog()
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.RetainLog()
	if err != nil || again != path {
		t.Fatalf("retry: path %q, error %v", again, err)
	}
	if err := os.RemoveAll(p.Directory); err != nil {
		t.Fatal(err)
	}
	// An already-open writer still writes the retained inode after teardown.
	late := []byte("\nlate append\n")
	if _, err := writer.Write(late); err != nil {
		t.Fatal(err)
	}
	if err := p.Create(); err != nil {
		t.Fatal(err)
	}
	newBytes := []byte("new incarnation\n")
	if err := os.WriteFile(p.Log, newBytes, 0600); err != nil {
		t.Fatal(err)
	}
	newPath, err := p.RetainLog()
	if err != nil || newPath == path {
		t.Fatalf("new incarnation: path %q, error %v", newPath, err)
	}
	for path, want := range map[string][]byte{path: append(original, late...), newPath: newBytes} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("retained %s = %q, error %v; want %q", path, got, err, want)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("retained permissions: %v, %v", info, err)
		}
	}
}

func TestRetainLogDoesNotOverwriteAnotherAudit(t *testing.T) {
	p := testTeam(t)
	path, err := p.RetainLog()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	other := []byte("different audit\n")
	if err := os.WriteFile(path, other, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RetainLog(); err == nil {
		t.Fatal("accepted conflicting archive")
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, other) {
		t.Fatalf("changed other audit: %q, %v", got, err)
	}
	if _, err := os.Stat(p.State); err != nil {
		t.Fatal(err)
	}
}

func TestRetainLogFailureKeepsOriginal(t *testing.T) {
	p := testTeam(t)
	original := []byte("existing audit\n")
	if err := os.WriteFile(p.Log, original, 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(p.Directory))
	if err := os.WriteFile(filepath.Join(root, "logs"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RetainLog(); err == nil {
		t.Fatal("retention unexpectedly succeeded")
	}
	if got, err := os.ReadFile(p.Log); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("changed original: %q, %v", got, err)
	}
	if err := os.Remove(filepath.Join(root, "logs")); err != nil {
		t.Fatal(err)
	}
	path, err := p.RetainLog()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("retry did not retain original: %q, %v", got, err)
	}
}

func TestRetainLogForLegacyTeamWithoutLog(t *testing.T) {
	p := testTeam(t)
	if err := os.WriteFile(p.State, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := p.RetainLog()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || len(got) != 0 {
		t.Fatalf("legacy empty audit: %q, %v", got, err)
	}
}

func TestRetainHoldLogsKeepEveryFailedLaunch(t *testing.T) {
	p := testTeam(t)
	a, err := p.Agent("hitch")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := p.RetainHoldLog("hitch"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(p.Directory))
	if _, err := os.Stat(filepath.Join(root, "logs")); !os.IsNotExist(err) {
		t.Fatalf("successful hold created log storage: %v", err)
	}
	for _, body := range []string{"first diagnostic\nexit status 1\n", "second diagnostic\nexit status 2\n"} {
		if err := os.WriteFile(filepath.Join(a.Directory, "hold"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := p.RetainHoldLog("hitch"); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(p.Directory); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "logs", "test", "*-hold-hitch-*.log"))
	if err != nil || len(paths) != 2 {
		t.Fatalf("diagnostics: %v, %v", paths, err)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		seen[string(body)] = true
	}
	if !seen["first diagnostic\nexit status 1\n"] || !seen["second diagnostic\nexit status 2\n"] {
		t.Fatalf("diagnostic bytes lost: %v", seen)
	}
}

func TestRetainHoldFailureKeepsDiagnostic(t *testing.T) {
	p := testTeam(t)
	a, _ := p.Agent("hitch")
	if err := os.MkdirAll(a.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(a.Directory, "hold")
	want := []byte("failed hold\nexit status 1\n")
	if err := os.WriteFile(source, want, 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(p.Directory))
	if err := os.WriteFile(filepath.Join(root, "logs"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.RetainHoldLog("hitch"); err == nil {
		t.Fatal("retention unexpectedly succeeded")
	}
	if got, err := os.ReadFile(source); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("diagnostic lost: %q, %v", got, err)
	}
}
