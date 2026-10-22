package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func rehitchFixture(t *testing.T) (*stateFixture, core.Agent, store.AgentPaths) {
	t.Helper()
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	t.Setenv("CODEX_HOME", t.TempDir())
	a := f.add(t, "a", "worker", "codex")
	f.env["GANG_AGENT_ID"], f.env["GANGLINE_HITCH_ID"] = string(a.ID), string(a.ID)
	a.Native.SessionID = "s"
	a.Native.Transcript = f.transcript
	a.Native.SubmittedAt = f.cmd.now().Add(-time.Minute)
	a.Native.FinishedAt = f.cmd.now()
	a.Rehitch = &core.Message{Text: "read saved work and continue"}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return f, a, p
}

func TestRehitchQueuesOnlyForAuthenticatedSelf(t *testing.T) {
	for _, variant := range []string{"valid", "no identity", "wrong token", "unknown session", "pending", "failed"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := rehitchFixture(t)
			a.Rehitch = nil
			switch variant {
			case "no identity":
				delete(f.env, "GANG_AGENT_ID")
			case "wrong token":
				f.env["GANG_AGENT_NONCE"] = "other"
			case "unknown session":
				a.Native.SessionID = ""
			case "pending":
				a.Rehitch = &core.Message{Text: "first request"}
			case "failed":
				a.Status = core.Failed
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			_ = l.Close()
			err = f.cmd.rehitch([]string{"--note", "continue saved work"})
			got, readErr := p.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if variant == "valid" {
				if err != nil || got.Rehitch == nil || got.Rehitch.Text != "continue saved work" || got.Pane != a.Pane || !strings.Contains(f.out.String(), "restart queued") {
					t.Fatalf("request = %+v err=%v output=%s", got, err, f.out.String())
				}
			} else if err == nil {
				t.Fatalf("invalid request accepted: %+v", got)
			}
		})
	}
}

func TestRehitchRequiresContinuation(t *testing.T) {
	for _, args := range [][]string{nil, {"worker", "--note", "work"}, {"--note", " "}, {"--unknown"}} {
		f := newStateFixture(t)
		if err := f.cmd.rehitch(args); errorStatus(err) != exitUsage {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestRehitchWaitsForFinishedTurnAndClearComposer(t *testing.T) {
	for _, variant := range []string{"open turn", "busy", "draft", "permission", "compaction", "input", "unknown finish"} {
		t.Run(variant, func(t *testing.T) {
			f, a, p := rehitchFixture(t)
			switch variant {
			case "open turn":
				a.Native.SubmittedAt = a.Native.FinishedAt.Add(time.Second)
			case "busy":
				a.Activity = core.Busy
			case "draft":
				f.input.screen = screenWithText("READY", "› unfinished text")
			case "permission":
				f.input.screen = screenWithText("Do you want to allow this command?", "1. Yes, proceed", "2. No, and tell Codex what to do differently")
			case "compaction":
				a.Compaction = &core.Compaction{Status: "queued"}
			case "input":
				a.Input = &core.InputIntent{ID: "uncertain"}
			case "unknown finish":
				a.Native.FinishedAt = time.Time{}
			}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			_ = l.Save(a)
			_ = l.Close()
			next := a
			locked, err := f.run.prepareRehitch(a, &next)
			if locked != nil {
				_ = locked.Close()
				t.Fatal("restart consumed unfinished work")
			}
			if !errors.Is(err, errRehitchDeferred) {
				t.Fatalf("restart = %v", err)
			}
			got, _ := p.Read()
			if got.Rehitch == nil || got.Pane != a.Pane {
				t.Fatalf("deferred request changed: %+v", got)
			}
		})
	}
}

func TestRehitchRetainsInboxAndNativeCursor(t *testing.T) {
	f, a, p := rehitchFixture(t)
	a.Native.Offset = 123
	a.Native.HookSequence = 4
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Save(a)
	_ = l.Close()
	e := core.Envelope{ID: "pending", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "pending work"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{p.Witness, p.CompactionWitness, p.Background, p.Permission} {
		if err := os.WriteFile(file, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	next := core.Agent{ID: a.ID, Name: a.Name, Collar: a.Collar, Directory: a.Directory, Status: core.Starting}
	l, err = f.run.prepareRehitch(a, &next)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	got, err := p.Read()
	if err != nil || got.Rehitch != nil || got.Status != core.Starting || got.Pane != "" || got.Native.SessionID != "s" || got.Native.Offset != 123 || got.Native.HookSequence != 4 {
		t.Fatalf("prepared state = %+v: %v", got, err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 1 || pending[0].ID != e.ID {
		t.Fatalf("pending inbox = %+v: %v", pending, err)
	}
	for _, file := range []string{p.Witness, p.CompactionWitness, p.Background, p.Permission} {
		if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("old witness %s retained: %v", filepath.Base(file), err)
		}
	}
}

func TestRehitchPreflightFailureConsumesRequestWithoutEndingHarness(t *testing.T) {
	f, a, p := rehitchFixture(t)
	// Hitch preflight refuses this missing directory before native teardown.
	a.Directory = filepath.Join(t.TempDir(), "missing")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Save(a)
	_ = l.Close()
	if err := f.run.rehitchPending(a.ID); err == nil {
		t.Fatal("missing working directory accepted")
	}
	got, err := p.Read()
	if err != nil || got.Rehitch != nil || got.Status != core.Active || got.Pane != a.Pane {
		t.Fatalf("failed preflight changed harness: %+v %v", got, err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 1 || !strings.Contains(pending[0].Message.Text, "could not restart") {
		t.Fatalf("failure notice: %+v %v", pending, err)
	}
}

func TestRehitchStartupPrecedesRetainedDueMail(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	p, _ := f.run.team.Agent(a.ID)
	pending := core.Envelope{ID: "mail", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "due mail"}, CreatedAt: f.cmd.now().Add(-time.Minute)}
	startup := core.Envelope{ID: "startup", Token: "fedcba9876543210", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderAgent, Name: a.Name, HitchID: a.ID}, Purpose: "assignment", Message: core.Message{Text: "continue saved work"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(pending); err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(startup); err != nil {
		t.Fatal(err)
	}
	var submitted []string
	submit := f.input.submit
	f.input.submit = func(prompt string) error { submitted = append(submitted, prompt); return submit(prompt) }
	l, a, err := f.run.acquire(a.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.run.drainFrom(l, a, startup.ID); err != nil {
		t.Fatal(err)
	}
	if len(submitted) != 2 || !strings.Contains(submitted[0], startup.Message.Text) || !strings.Contains(submitted[1], pending.Message.Text) {
		t.Fatalf("startup ordering = %q", submitted)
	}
}
