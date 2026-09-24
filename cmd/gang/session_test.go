package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestStoppedTeamWithClaimedLeadExplainsRecovery(t *testing.T) {
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
	if listed.Status != core.Failed || listed.Pane != a.Pane || !strings.Contains(listed.Evidence, "registered pane is absent from tmux") {
		t.Fatalf("roster left claim in unexpected state: %+v", listed)
	}
	for name, invoke := range map[string]func() error{
		"attach": func() error { return f.cmd.attach(nil) },
		"up":     func() error { return f.cmd.up([]string{"-c", "codex"}) },
	} {
		t.Run(name, func(t *testing.T) {
			err := invoke()
			if err == nil || !strings.Contains(err.Error(), "gang drop lead") || !strings.Contains(err.Error(), "gang up NEW_NAME") {
				t.Fatalf("error = %v, want both recovery choices", err)
			}
			var refusal commandError
			if !errors.As(err, &refusal) || refusal.status != exitRefused {
				t.Fatalf("error = %v, want refused status", err)
			}
		})
	}
	retained, err := f.run.resolve("lead")
	if err != nil || retained.ID != a.ID || retained.Status != core.Failed {
		t.Fatalf("lead claim changed: %+v, %v", retained, err)
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
