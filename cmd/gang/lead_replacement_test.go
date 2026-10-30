//go:build acceptance

package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestReplacementLeadCanDropEarlierLeadsChildren(t *testing.T) {
	for _, name := range []string{"lead", "coordinator"} {
		t.Run(name, func(t *testing.T) {
			f := newStateFixture(t)
			old := f.addHitched(t, "old-lead", "lead", "lead", "")
			child := f.addHitched(t, "child", "worker", "worker", old.ID)
			if err := f.cmd.drop([]string{"lead"}); err != nil {
				t.Fatal(err)
			}
			current := f.addHitched(t, "replacement-lead", name, "lead", "")
			f.env["GANG_AGENT_ID"] = string(current.ID)
			if err := f.cmd.drop([]string{"worker"}); err != nil {
				t.Fatalf("replacement lead could not drop earlier lead's child: %v", err)
			}
			p, err := f.run.team.Agent(child.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Read(); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("child registration remains: %v", err)
			}
		})
	}
}

func TestDropRefusalReportsCallerAuthority(t *testing.T) {
	for _, tc := range []struct{ role, hitcher, nonce, want string }{
		{"", "", "fixture-nonce", `role="", hitched_by="", pane_token_match=true`},
		{"worker", "", "fixture-nonce", `role="worker", hitched_by="", pane_token_match=true`},
		{"lead", "old-owner", "fixture-nonce", `role="lead", hitched_by="old-owner", pane_token_match=true`},
		{"lead", "", "wrong-token", `role="lead", hitched_by="", pane_token_match=false`},
	} {
		t.Run(tc.want, func(t *testing.T) {
			f := newStateFixture(t)
			caller := f.addHitched(t, "caller", "lead", tc.role, core.HitchID(tc.hitcher))
			f.addHitched(t, "child", "worker", "worker", "absent-old-lead")
			f.env["GANG_AGENT_ID"], f.env["GANG_AGENT_NONCE"] = string(caller.ID), tc.nonce
			err := f.cmd.drop([]string{"worker"})
			requireRefused(t, err, "drop refused")
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("missing caller authority: %v", err)
			}
		})
	}
}

func TestRolelessDropRefusalNamesOperatorRehitch(t *testing.T) {
	for _, session := range []string{"", "native-session"} {
		t.Run(session, func(t *testing.T) {
			f := newStateFixture(t)
			f.env["GANG_TMUX_SOCKET"] = ""
			if session != "" {
				f.env["GANG_TMUX_SOCKET"] = "/selected socket"
			}
			caller := f.addHitched(t, "caller", "lead", "", "")
			saveAgent(t, f, caller.ID, func(a *core.Agent) {
				a.Directory = "/work tree/it's $(literal)"
				a.Native.SessionID = session
			})
			f.addHitched(t, "child", "worker", "worker", "absent-old-lead")
			f.env["GANG_AGENT_ID"] = string(caller.ID)
			err := f.cmd.drop([]string{"worker"})
			requireRefused(t, err, "drop refused")
			for _, text := range []string{"This caller has no role", "from an operator shell", "gang drop lead --team 'unit'", `gang hitch lead --team 'unit' -c 'codex' -d '/work tree/it'\''s $(literal)' --role lead`} {
				if !strings.Contains(err.Error(), text) {
					t.Fatalf("missing %q: %v", text, err)
				}
			}
			if strings.Contains(err.Error(), "--resume") != (session != "") || session != "" && !strings.Contains(err.Error(), "--resume 'native-session'") {
				t.Fatalf("resume hint does not match known native session: %v", err)
			}
			_, command, ok := strings.Cut(err.Error(), "from an operator shell run ")
			if !ok {
				t.Fatal("missing re-hitch command")
			}
			drop, hitch, found := strings.Cut(command, ", then ")
			if !found {
				t.Fatal("missing recovery commands")
			}
			for i, part := range []string{drop, hitch} {
				shell := exec.Command("sh", "-c", `gang() { printf '%s\n' "$GANG_STATE_ROOT" "$GANG_TMUX_SOCKET" "$@"; }; `+part)
				shell.Env = append(os.Environ(), "GANG_STATE_ROOT=/wrong-root", "GANG_TMUX_SOCKET=/wrong-socket")
				out, shellErr := shell.CombinedOutput()
				want := []string{f.env["GANG_STATE_ROOT"], f.env["GANG_TMUX_SOCKET"], "drop", "lead", "--team", "unit"}
				if i == 1 {
					want = []string{f.env["GANG_STATE_ROOT"], f.env["GANG_TMUX_SOCKET"], "hitch", "lead", "--team", "unit", "-c", "codex", "-d", "/work tree/it's $(literal)", "--role", "lead"}
					if session != "" {
						want = append(want, "--resume", session)
					}
				}
				if shellErr != nil || string(out) != strings.Join(want, "\n")+"\n" {
					t.Fatalf("recovery command changed environment/arguments: %v %q", shellErr, out)
				}
			}

		})
	}
}

func TestDelegatedRolelessCallerDoesNotReceiveLeadRecoveryHint(t *testing.T) {
	f := newStateFixture(t)
	caller := f.addHitched(t, "caller", "worker", "", "lead-id")
	f.addHitched(t, "other", "other", "", "lead-id")
	f.env["GANG_AGENT_ID"] = string(caller.ID)
	err := f.cmd.drop([]string{"other"})
	requireRefused(t, err, "drop refused")
	if strings.Contains(err.Error(), "--role lead") || strings.Contains(err.Error(), "operator shell") {
		t.Fatalf("delegated worker received lead recovery hint: %v", err)
	}
}
