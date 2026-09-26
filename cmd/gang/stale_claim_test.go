package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// The fixture has durable registration files but no tmux session. Stop at
// new-session so the test cannot launch a native harness or touch a live team.
func stoppedClaimFixture(t *testing.T, name string) (*stateFixture, core.Agent) {
	t.Helper()
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	f.env["GANG_TMUX_SOCKET"] = filepath.Join(t.TempDir(), "absent.sock")
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" +
		"if [ \"$1\" = -S ]; then [ ! -e \"$2\" ] || exit 92; shift 2; fi\n" +
		"case \"$1\" in has-session) exit 1;; new-session) echo fixture-reached-new-session >&2; exit 91;; *) exit 92;; esac\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	old := f.add(t, "old-hitch", name, "codex")
	old.Process = core.ProcessIdentity{PID: os.Getpid(), BootID: "previous-boot", Started: "old-start"}
	old.Native = core.NativeState{SessionID: "retained-session", Transcript: "/retained/transcript"}
	saveStoppedClaim(t, f, old)
	return f, old
}

func TestUpSupersedesStoppedClaimPreservingRecord(t *testing.T) {
	for _, name := range []string{"lead", "captain"} {
		t.Run(name, func(t *testing.T) {
			f, old := stoppedClaimFixture(t, name)
			p, _ := f.run.team.Agent(old.ID)
			receipt := core.Envelope{ID: "retained", Recipient: old.ID, To: old.Name, Message: core.Message{Text: "retained assignment"}}
			if err := p.Publish(receipt); err != nil {
				t.Fatal(err)
			}
			if err := f.run.record(old, core.Event{Type: "hitch_claimed"}); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"-c", "codex"}
			if name != "lead" {
				args = append([]string{name}, args...)
			}
			err = f.cmd.up(args)
			if err == nil || !strings.Contains(err.Error(), "fixture-reached-new-session") {
				t.Fatalf("up = %v, want to reach new-session without manual drop", err)
			}
			current, err := f.run.resolve(name)
			if err != nil || current.ID == old.ID {
				t.Fatalf("replacement = %+v, %v", current, err)
			}
			retained, err := p.Read()
			if err != nil || retained.ID != old.ID || retained.Status != core.Failed || retained.Pane != "" || retained.Process != old.Process || retained.Native.SessionID != old.Native.SessionID || retained.Native.Transcript != old.Native.Transcript {
				t.Fatalf("retained = %+v, %v", retained, err)
			}
			got, err := p.ReadEnvelope("new", receipt.ID)
			if err != nil || got.Message != receipt.Message {
				t.Fatalf("retained receipt = %+v, %v", got, err)
			}
			after, err := os.ReadFile(f.run.team.Log)
			if err != nil || !bytes.HasPrefix(after, before) {
				t.Fatalf("old ledger changed: %v", err)
			}
			agents, err := f.run.team.ListAgents()
			if err != nil || len(agents) != 1 || agents[0].ID != current.ID {
				t.Fatalf("registered = %+v, %v", agents, err)
			}
		})
	}
}

func TestUpKeepsLockedStoppedClaim(t *testing.T) {
	f, old := stoppedClaimFixture(t, "lead")
	p, _ := f.run.team.Agent(old.ID)
	lock, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	err = f.cmd.up([]string{"-c", "codex"})
	var refusal commandError
	if !errors.As(err, &refusal) || refusal.status != exitRefused {
		t.Fatalf("locked claim = %v, want refusal", err)
	}
	id, err := f.run.team.ResolveName("lead")
	if err != nil || id != old.ID {
		t.Fatalf("claim = %s, %v", id, err)
	}
}

func TestUpKeepsRunningTeamClaim(t *testing.T) {
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	old := f.add(t, "old-hitch", "lead", "codex")
	err := f.cmd.up([]string{"-c", "codex"})
	if err == nil || !strings.Contains(err.Error(), "already claimed") {
		t.Fatalf("up = %v", err)
	}
	id, err := f.run.team.ResolveName("lead")
	if err != nil || id != old.ID {
		t.Fatalf("claim = %s, %v", id, err)
	}
}

func TestHitchKeepsStoppedClaim(t *testing.T) {
	f, old := stoppedClaimFixture(t, "lead")
	if err := f.cmd.hitch([]string{"lead", "-c", "codex"}); err == nil {
		t.Fatal("hitch replaced a claim")
	}
	id, err := f.run.team.ResolveName("lead")
	if err != nil || id != old.ID {
		t.Fatalf("claim = %s, %v", id, err)
	}
}

func TestUpRechecksStoppedTeamBeforeSuperseding(t *testing.T) {
	for _, response := range []string{"exit 0", "echo fixture-session-probe-failed >&2; exit 91"} {
		t.Run(response, func(t *testing.T) {
			f, old := stoppedClaimFixture(t, "lead")
			p, _ := f.run.team.Agent(old.ID)
			before, err := os.ReadFile(p.State)
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(f.env["GANG_STATE_ROOT"], "first-probe")
			program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" +
				"if [ \"$1\" = -S ]; then shift 2; fi\n" +
				"case \"$1\" in has-session) if [ -e '" + marker + "' ]; then " + response + "; else touch '" + marker + "'; exit 1; fi;; *) exit 92;; esac\n"
			if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.up([]string{"-c", "codex"}); err == nil {
				t.Fatal("up replaced an unconfirmed stale claim")
			}
			id, err := f.run.team.ResolveName("lead")
			if err != nil || id != old.ID {
				t.Fatalf("claim = %s, %v", id, err)
			}
			after, err := os.ReadFile(p.State)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("old state changed: %v", err)
			}
		})
	}
}

func saveStoppedClaim(t *testing.T, f *stateFixture, a core.Agent) {
	t.Helper()
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
}

func TestUpKeepsLiveProcessOnAbsentSocket(t *testing.T) {
	f, old := stoppedClaimFixture(t, "lead")
	program := fmt.Sprintf("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nif [ \"$1\" = -S ]; then shift 2; fi\ncase \"$1\" in has-session) exit 1;; display-message) echo '%d 0';; *) exit 92;; esac\n", os.Getpid())
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	b, err := f.cmd.tmux(f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := b.Identity(context.Background(), "%1")
	if err != nil {
		t.Fatal(err)
	}
	old.Process = storedIdentity(identity)
	saveStoppedClaim(t, f, old)
	err = f.cmd.up([]string{"-c", "codex"})
	if err == nil || !strings.Contains(err.Error(), "live recorded process") {
		t.Fatalf("up = %v", err)
	}
	got, err := f.run.resolve("lead")
	if err != nil || got.ID != old.ID || got.Status != old.Status || got.Pane != old.Pane {
		t.Fatalf("live claim changed: %+v, %v", got, err)
	}
}

func TestUpKeepsUnfinishedTeardown(t *testing.T) {
	for _, status := range []core.Status{core.Dropping, core.Failed} {
		t.Run(string(status), func(t *testing.T) {
			f, old := stoppedClaimFixture(t, "lead")
			old.Status = status
			old.Teardown = []core.ProcessIdentity{old.Process}
			saveStoppedClaim(t, f, old)
			err := f.cmd.up([]string{"-c", "codex"})
			if err == nil || !strings.Contains(err.Error(), "unfinished teardown") {
				t.Fatalf("up = %v", err)
			}
			got, err := f.run.resolve("lead")
			if err != nil || got.ID != old.ID || got.Status != old.Status || len(got.Teardown) != 1 {
				t.Fatalf("teardown changed: %+v, %v", got, err)
			}
		})
	}
}
