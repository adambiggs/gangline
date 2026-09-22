package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func testTeam(t *testing.T) TeamPaths {
	t.Helper()
	p, err := (Paths{Root: t.TempDir()}).Team("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Create(); err != nil {
		t.Fatal(err)
	}
	return p
}
func testAgent(id, name string) core.Agent {
	return core.Agent{ID: core.HitchID(id), Name: core.AgentName(name), Collar: "codex", Directory: "/work", Status: core.Active, Activity: core.Idle}
}
func TestNamesAreClaimedExactlyOnce(t *testing.T) {
	p := testTeam(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			l, err := p.CreateAgent(testAgent(fmt.Sprintf("a%d", i), "worker"))
			if l != nil {
				_ = l.Close()
			}
			results <- err
		}(i)
	}
	close(start)
	success, refused := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrNameTaken) {
			refused++
		} else {
			t.Fatal(err)
		}
	}
	agents, err := p.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := os.ReadDir(p.Agents)
	if err != nil {
		t.Fatal(err)
	}
	if success != 1 || refused != 1 || len(agents) != 1 || len(dirs) != 1 {
		t.Fatalf("claims: success=%d refused=%d agents=%d dirs=%d", success, refused, len(agents), len(dirs))
	}
}
func TestStrictAgentDecodeNamesFile(t *testing.T) {
	p := testTeam(t)
	l, err := p.CreateAgent(testAgent("a", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	data, err := os.ReadFile(l.Paths.State)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"id":"a"`, `"id":"a","extra":true`, 1))
	if err := os.WriteFile(l.Paths.State, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Paths.Read(); err == nil || !strings.Contains(err.Error(), l.Paths.State) {
		t.Fatalf("decode error lacks file: %v", err)
	}
}
func TestConcurrentAppendsAreCompleteLinesWithoutAgentLocks(t *testing.T) {
	p := testTeam(t)
	l, err := p.CreateAgent(testAgent("a", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var group sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		group.Go(func() {
			errs <- p.Append(core.Event{Type: "native_hook", At: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), HitchID: "a", Status: "activity"})
		})
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(p.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	count := 0
	if err := ReadLog(f, func(core.Event) error { count++; return nil }); err != nil {
		t.Fatal(err)
	}
	if count != 24 {
		t.Fatalf("audit lines=%d", count)
	}
}
func TestWatchSeesSameSizeAtomicReplacement(t *testing.T) {
	p := testTeam(t)
	l, err := p.CreateAgent(testAgent("a", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	watch, err := Watch(l.Paths.State)
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	a, err := l.Paths.Read()
	if err != nil {
		t.Fatal(err)
	}
	a.Activity = core.Busy
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := watch.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := l.Paths.Read()
	if err != nil || got.Activity != core.Busy {
		t.Fatalf("state after change: %+v %v", got, err)
	}
}
func TestSealedInboxRejectsPublisherAndCannotReappear(t *testing.T) {
	p := testTeam(t)
	l, err := p.CreateAgent(testAgent("a", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	e := core.Envelope{ID: "msg", Recipient: "a", To: "worker", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "hello"}, CreatedAt: time.Now()}
	staged, err := jsonTemp(filepath.Join(l.Paths.Inbox, "tmp"), e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.SealInbox(); err != nil {
		t.Fatal(err)
	}
	dest, _ := l.Paths.EnvelopePath("new", e.ID)
	if err := os.Rename(staged, dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("publisher crossed sealed inbox: %v", err)
	}
	if err := l.Paths.Publish(e); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("publisher recreated inbox: %v", err)
	}
	if err := os.RemoveAll(p.Directory); err != nil {
		t.Fatal(err)
	}
	if err := l.Paths.WriteWitness(Witness{ID: "w"}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hook recreated state: %v", err)
	}
	if err := p.Append(core.Event{Type: "native_hook", At: time.Now()}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("audit recreated state: %v", err)
	}
}

func TestRenameRecoveryHandlesNameReuse(t *testing.T) {
	p := testTeam(t)
	a := testAgent("a", "old")
	l, err := p.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a.RenameFrom, a.RenameTo = "old", "new"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	other, err := p.CreateAgent(testAgent("b", "new"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := p.FinishRename(l, &a); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("conflict=%v", err)
	}
	saved, err := l.Paths.Read()
	if err != nil {
		t.Fatal(err)
	}
	if saved.RenameTo != "" || saved.Name != "old" {
		t.Fatalf("conflict poisoned state: %+v", saved)
	}
	if err := p.RemoveName("new", "b"); err != nil {
		t.Fatal(err)
	}
	a.RenameFrom, a.RenameTo = "old", "new"
	if err := p.ClaimName("new", "a"); err != nil {
		t.Fatal(err)
	}
	a.Name = "new"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveName("old", "a"); err != nil {
		t.Fatal(err)
	}
	if err := p.ClaimName("old", "c"); err != nil {
		t.Fatal(err)
	}
	if err := p.FinishRename(l, &a); err != nil {
		t.Fatal(err)
	}
	if id, err := p.ResolveName("old"); err != nil || id != "c" {
		t.Fatalf("reused alias=%s %v", id, err)
	}
	if a.RenameTo != "" || a.Name != "new" {
		t.Fatalf("unfinished rename: %+v", a)
	}
}
