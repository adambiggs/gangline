package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestRosterDisplaysRegisteredCollarName(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "claude")
	if err := f.cmd.roster([]string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var got rosterJSON
	decodeOutput(t, f, &got)
	if len(got.Agents) != 1 || got.Agents[0].Collar != "claude" {
		t.Fatalf("roster = %+v", got)
	}
}

func TestAttachStoppedTeamWithClaimedLeadAdvisesUp(t *testing.T) {
	f := newStateFixture(t)
	f.env["GANG_TMUX_SOCKET"] = filepath.Join(t.TempDir(), "absent.sock")
	fakeTmux := f.env["GANG_TMUX"]
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" +
		"if [ \"$1\" = -S ]; then [ ! -e \"$2\" ] || exit 92; shift 2; fi\n" +
		"case \"$1\" in list-panes|has-session) exit 1;; *) exit 91;; esac\n"
	if err := os.WriteFile(fakeTmux, []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	a := f.add(t, "lead-hitch", "lead", "codex")
	if err := f.cmd.roster(nil); err != nil {
		t.Fatal(err)
	}
	listed, err := f.run.resolve("lead")
	if err != nil {
		t.Fatal(err)
	}
	if listed.Status != core.Active || listed.Pane != a.Pane || listed.Evidence != "" {
		t.Fatalf("roster left claim in unexpected state: %+v", listed)
	}
	err = f.cmd.attach(nil)
	if err == nil || !strings.Contains(err.Error(), "start it with 'gang up'") {
		t.Fatalf("attach error = %v, want start advice", err)
	}
	retained, err := f.run.resolve("lead")
	if err != nil || retained.ID != a.ID || retained.Status != core.Active {
		t.Fatalf("lead claim changed: %+v, %v", retained, err)
	}
}

// A socket that answers nothing lists no panes, which says nothing of a pane
// the registering server may still run. The roster reports the agent unknown
// and leaves its record as it was.
func TestRosterWithoutTeamListingKeepsRecord(t *testing.T) {
	f := newStateFixture(t)
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" +
		"printf 'no server running on fixture\\n' >&2; exit 1\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	a := f.add(t, "a", "worker", "codex")
	f.setAgent(t, f.add(t, "b", "failed", "codex"), func(b *core.Agent) {
		b.Status, b.Activity, b.Evidence = core.Failed, core.Unknown, "boot deadline elapsed"
	})
	for range 2 {
		rows := rosterRows(t, f)
		if row := rows["worker"]; !strings.Contains(row, " active ") || !strings.Contains(row, " unknown ") || !strings.HasSuffix(row, "tmux lists no team session") {
			t.Fatalf("row = %q", row)
		}
		if row := rows["failed"]; !strings.HasSuffix(row, "boot deadline elapsed") {
			t.Fatalf("failed row = %q", row)
		}
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Active || got.Activity != core.Idle || got.Pane != a.Pane || got.Evidence != "" {
		t.Fatalf("record changed: status=%s activity=%s pane=%q evidence=%q", got.Status, got.Activity, got.Pane, got.Evidence)
	}
	if n := eventsOfType(t, f, "hitch_failed"); n != 0 {
		t.Fatalf("hitch_failed events = %d", n)
	}
}

func TestAttachWithoutLeadClaimKeepsStartAdvice(t *testing.T) {
	f := newStateFixture(t)
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err := f.cmd.attach(nil)
	if err == nil || !strings.Contains(err.Error(), "start it with 'gang up'") {
		t.Fatalf("attach error = %v, want start advice", err)
	}
}
