package store

import (
	"os"
	"reflect"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// bindings are the session_bound records in p's log, by hitch.
func bindings(t *testing.T, p TeamPaths) []core.Event {
	t.Helper()
	f, err := os.Open(p.Log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var found []core.Event
	if err := ReadLog(f, func(e core.Event) error {
		if e.Type == "session_bound" {
			found = append(found, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return found
}

// A session requested for resume is bound only once the native harness
// reports it with its transcript.
func TestRequestedSessionIsBoundOnceReported(t *testing.T) {
	p := testTeam(t)
	a := testAgent("a", "worker")
	a.Native.SessionID = "r"
	l, err := p.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a.Activity = core.Busy
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if got := bindings(t, p); len(got) != 0 {
		t.Fatalf("requested session bound before the harness reported it: %+v", *got[0].Binding)
	}
	a.Native.Transcript = "r.jsonl"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	var got []core.Binding
	for _, e := range bindings(t, p) {
		got = append(got, *e.Binding)
	}
	if want := []core.Binding{{SessionID: "r", Collar: "codex"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bindings = %+v", got)
	}
}

// Every native session a hitch holds is recorded once, when it is first
// saved, with the collar and parent hitch, and stays in the team log after the
// agent is dropped and its name reused.
func TestSavedSessionChangesAreBound(t *testing.T) {
	p := testTeam(t)
	a := testAgent("a", "worker")
	a.HitchedBy, a.Native.SessionID, a.Native.Transcript = "l", "s", "s.jsonl"
	l, err := p.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	a.Activity = core.Busy
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	a.Native.SessionID, a.Native.Transcript = "n", "n.jsonl"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	ap, err := p.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err = ap.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Activity = core.Idle
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(ap.Directory); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveName(a.Name, a.ID); err != nil {
		t.Fatal(err)
	}
	b := testAgent("b", "worker")
	b.Collar = "claude"
	l, err = p.CreateAgent(b)
	if err != nil {
		t.Fatal(err)
	}
	b.Native.SessionID, b.Native.Transcript = "c", "c.jsonl"
	if err := l.Save(b); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	var got []core.Binding
	var hitches []core.HitchID
	for _, e := range bindings(t, p) {
		if e.Name != "worker" {
			t.Fatalf("binding names %q", e.Name)
		}
		got, hitches = append(got, *e.Binding), append(hitches, e.HitchID)
	}
	want := []core.Binding{
		{SessionID: "s", Collar: "codex", HitchedBy: "l"},
		{SessionID: "n", PreviousSessionID: "s", Collar: "codex", HitchedBy: "l"},
		{SessionID: "c", Collar: "claude"},
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(hitches, []core.HitchID{"a", "a", "b"}) {
		t.Fatalf("bindings = %+v for %v", got, hitches)
	}
}

// A session saved before the log bound it, as by a build without bindings or
// an append that failed, is bound at the next save, before any session that
// replaces it, and only once.
func TestUnboundSessionIsBoundAtTheNextSave(t *testing.T) {
	p := testTeam(t)
	a := testAgent("a", "worker")
	a.Native.Transcript = "t.jsonl"
	l, err := p.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	ap, err := p.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	save := func(a core.Agent) error {
		l, err := ap.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		return l.Save(a)
	}
	for _, c := range []struct{ saved, next string }{{"old", "old"}, {"old", "new"}} {
		a.Native.SessionID = c.saved
		if err := atomicJSON(ap.State, a); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(p.Log, p.Log+".kept"); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Mkdir(p.Log, 0700); err != nil {
			t.Fatal(err)
		}
		a.Native.SessionID = c.next
		if err := save(a); err == nil {
			t.Fatal("save succeeded without its binding")
		}
		if saved, err := ap.Read(); err != nil || saved.Native.SessionID != c.saved {
			t.Fatalf("saved session = %q, %v", saved.Native.SessionID, err)
		}
		if err := os.Remove(p.Log); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(p.Log+".kept", p.Log); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := save(a); err != nil {
			t.Fatal(err)
		}
		if err := save(a); err != nil {
			t.Fatal(err)
		}
		want := []core.Binding{{SessionID: "old", Collar: "codex"}}
		if c.next != c.saved {
			want = append(want, core.Binding{SessionID: c.next, PreviousSessionID: c.saved, Collar: "codex"})
		}
		var got []core.Binding
		for _, e := range bindings(t, p) {
			got = append(got, *e.Binding)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%+v: bindings = %+v", c, got)
		}
		if err := os.Remove(p.Log); err != nil {
			t.Fatal(err)
		}
	}
}
