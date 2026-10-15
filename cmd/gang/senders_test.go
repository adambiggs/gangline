package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func senderKeyFixture(t *testing.T, f *stateFixture) string {
	t.Helper()
	if err := f.cmd.execute([]string{"sender", "add", "relay"}); err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(f.out.String())
	f.out.Reset()
	return path
}

func TestRegisteredSenderDelivery(t *testing.T) {
	for _, environment := range []bool{false, true} {
		t.Run(map[bool]string{false: "flag", true: "environment"}[environment], func(t *testing.T) {
			f := newStateFixture(t)
			path := senderKeyFixture(t, f)
			a := f.add(t, "a", "worker", "codex")
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			args := []string{"send", "worker", "--as", "relay", "hello"}
			if environment {
				f.env["GANG_SENDER_CREDENTIAL"] = path
			} else {
				args = append(args, "--credential", path)
			}
			if err := f.cmd.execute(args); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.input.pasted, "[gang:registered:relay#") || !strings.Contains(f.out.String(), "delivered") {
				t.Fatalf("delivery: %q %q stderr=%q", f.input.pasted, f.out.String(), f.errOut.String())
			}
			if strings.Contains(f.input.pasted, "self-declared") {
				t.Fatal("registered send fell back")
			}
			f.input.pasted = ""
			if err := f.cmd.execute([]string{"send", "worker", "--from", "relay", "ordinary"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.input.pasted, "[gang:self-declared:relay#") {
				t.Fatal("ordinary send acquired registered authority")
			}
		})
	}
}

func TestRegisteredSenderRefusesCredentials(t *testing.T) {
	for _, kind := range []string{"missing", "wrong", "empty", "unreadable", "unknown", "revoked", "other-sender", "flag-overrides-env", "caller"} {
		t.Run(kind, func(t *testing.T) {
			f := newStateFixture(t)
			key := senderKeyFixture(t, f)
			a := f.add(t, "a", "worker", "codex")
			args := []string{"send", "worker", "--as", "relay", "hello"}
			switch kind {
			case "missing":
				key = ""
			case "wrong", "empty", "flag-overrides-env":
				if kind == "flag-overrides-env" {
					f.env["GANG_SENDER_CREDENTIAL"] = key
				}
				key = filepath.Join(t.TempDir(), "wrong")
				content := "wrong"
				if kind == "empty" {
					content = ""
				}
				if err := os.WriteFile(key, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				key = t.TempDir()
			case "unknown":
				args[3] = "unknown"
			case "revoked":
				copy := filepath.Join(t.TempDir(), "copy")
				data, err := os.ReadFile(key)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(copy, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := f.cmd.execute([]string{"sender", "remove", "relay"}); err != nil {
					t.Fatal(err)
				}
				f.out.Reset()
				if err := f.cmd.execute([]string{"sender", "add", "relay"}); err != nil {
					t.Fatal(err)
				}
				key = copy
			case "other-sender":
				if err := f.cmd.execute([]string{"sender", "add", "other"}); err != nil {
					t.Fatal(err)
				}
				args[3] = "other"
			case "caller":
				f.env["GANG_AGENT_ID"] = string(a.ID)
				f.env["GANG_AGENT_NONCE"] = "wrong"
			}
			f.out.Reset()
			if key != "" {
				args = append(args, "--credential", key)
			}
			err := f.cmd.execute(args)
			if err == nil || errorStatus(err) != exitRefused {
				t.Fatalf("credential %s: %v", kind, err)
			}
			if kind == "missing" && !strings.Contains(err.Error(), "credential is missing") {
				t.Fatalf("missing credential diagnosis: %v", err)
			}
			if kind == "unreadable" && !strings.Contains(err.Error(), "cannot read") {
				t.Fatalf("unreadable credential diagnosis: %v", err)
			}
			if kind != "caller" && !strings.Contains(err.Error(), "--credential FILE") {
				t.Fatalf("no allowed route: %v", err)
			}
			if f.input.submits != 0 {
				t.Fatal("refusal submitted input")
			}
			p, _ := f.run.team.Agent(a.ID)
			pending, err := p.ListNew()
			if err != nil || len(pending) != 0 {
				t.Fatalf("refusal published: %+v %v", pending, err)
			}
		})
	}
}

func TestSenderManagementAuthorityAndNames(t *testing.T) {
	for _, actor := range []string{"worker", "lead", "hitched-lead", "booting-lead", "bad-token", "operator"} {
		t.Run(actor, func(t *testing.T) {
			f := newStateFixture(t)
			if actor != "operator" {
				a := f.add(t, "a", "owner", "codex")
				p, _ := f.run.team.Agent(a.ID)
				if actor != "worker" {
					a.Role = "lead"
				}
				if actor == "hitched-lead" {
					a.HitchedBy = "parent"
				}
				if actor == "booting-lead" {
					a.Status = core.Booting
				}
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
				f.env["GANG_AGENT_ID"] = string(a.ID)
				if actor == "bad-token" {
					f.env["GANG_AGENT_NONCE"] = "wrong"
				}
			}
			err := f.cmd.execute([]string{"sender", "add", "relay"})
			allowed := actor == "operator" || actor == "lead"
			if (err == nil) != allowed {
				t.Fatalf("add %s: %v", actor, err)
			}
			if !allowed {
				delete(f.env, "GANG_AGENT_ID")
				f.env["GANG_AGENT_NONCE"] = "fixture-nonce"
				if err := f.cmd.execute([]string{"sender", "add", "relay"}); err != nil {
					t.Fatal(err)
				}
				f.env["GANG_AGENT_ID"] = "a"
				if actor == "bad-token" {
					f.env["GANG_AGENT_NONCE"] = "wrong"
				}
			}
			err = f.cmd.execute([]string{"sender", "remove", "relay"})
			if (err == nil) != allowed {
				t.Fatalf("remove %s: %v", actor, err)
			}
		})
	}
	for _, kind := range []string{"roster", "mailbox", "registered", "peer", "host"} {
		t.Run(kind, func(t *testing.T) {
			f := newStateFixture(t)
			switch kind {
			case "roster":
				f.add(t, "a", "relay", "codex")
			case "mailbox":
				if err := f.cmd.execute([]string{"outside", "register", "relay", "--credential-file", filepath.Join(t.TempDir(), "outside-key")}); err != nil {
					t.Fatal(err)
				}
			case "registered":
				senderKeyFixture(t, f)
			case "peer":
				f.env["GANG_PEERS"] = `{"RELAY":{"ssh":"x","gang":"/g","team":"x"}}`
			case "host":
				f.env["GANG_HOST"] = "RELAY"
			}
			if err := f.cmd.execute([]string{"sender", "add", "relay"}); err == nil {
				t.Fatal("collision accepted")
			}
		})
	}
}

func TestSenderManagementLifecycleAndNamespace(t *testing.T) {
	f := newStateFixture(t)
	path := senderKeyFixture(t, f)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode %v %v", info, err)
	}
	if !strings.HasPrefix(path, f.env["GANG_STATE_ROOT"]+string(os.PathSeparator)) {
		t.Fatal("key outside state root")
	}
	if err := f.cmd.execute([]string{"sender", "list"}); err != nil {
		t.Fatal(err)
	}
	if f.out.String() != "relay\n" {
		t.Fatalf("list %q", f.out.String())
	}
	if _, err := f.run.supersedeStoppedClaim(context.Background(), nil, core.Agent{Name: "relay"}); !errors.Is(err, store.ErrNameTaken) {
		t.Fatalf("stale replacement of registered sender: %v", err)
	}
	if err := f.cmd.execute([]string{"outside", "register", "relay", "--credential-file", filepath.Join(t.TempDir(), "outside-key")}); err == nil {
		t.Fatal("outside mailbox stole sender name")
	}
	if err := f.run.team.ClaimName("relay", "agent"); !errors.Is(err, store.ErrNameTaken) {
		t.Fatalf("agent collision %v", err)
	}
	f.add(t, "a", "worker", "codex")
	if err := f.cmd.rename([]string{"worker", "relay"}); err == nil {
		t.Fatal("rename stole sender name")
	}
	if err := f.cmd.drop([]string{"relay"}); err == nil || !strings.Contains(err.Error(), "gang sender remove") {
		t.Fatalf("drop sender: %v", err)
	}
	if _, err := f.run.resolve("relay"); err == nil || !strings.Contains(err.Error(), "not a recipient") {
		t.Fatalf("resolve sender %v", err)
	}
	if err := f.cmd.execute([]string{"sender", "remove", "worker"}); err == nil {
		t.Fatal("removed native agent")
	}
	if err := f.cmd.execute([]string{"sender", "remove", "relay"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("secret retained: %v", err)
	}
	if _, err := f.run.team.ResolveName("relay"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("name retained: %v", err)
	}
	if err := f.run.team.ClaimName("relay", "replacement"); err != nil {
		t.Fatal(err)
	}
}

func TestRegisteredSenderOutsideMailbox(t *testing.T) {
	f := newStateFixture(t)
	key := senderKeyFixture(t, f)
	paths, principal := registerOutsideFixture(t, f)
	if err := f.cmd.execute([]string{"send", "toby", "--as", "relay", "--credential", key, "mail"}); err != nil {
		t.Fatal(err)
	}
	pending, err := paths.ReadOutsidePending("unit", principal)
	if err != nil || len(pending) != 1 || senderLabel(pending[0].From) != "registered:relay" {
		t.Fatalf("mail: %+v %v", pending, err)
	}
	if err := f.cmd.execute([]string{"send", "toby", "--as", "relay", "--credential", t.TempDir(), "bad"}); err == nil {
		t.Fatal("mail accepted unreadable key")
	}
}

func TestRegisteredSenderOptionAndPrefixGuards(t *testing.T) {
	for _, args := range [][]string{
		{"worker", "--as", ""}, {"worker", "--as", "a:b"}, {"worker", "--as", "relay", "--from", "operator"},
		{"worker", "--credential", "key"}, {"worker", "--as", "relay", "--credential", ""},
		{"alpha:worker", "--credential", "key"},
		{"registered:relay", "hi"}, {"Registered:relay", "hi"},
	} {
		if _, err := parseSend(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	for _, name := range []string{"registered", "Registered", "REGISTERED"} {
		if validPeerHost(name) {
			t.Errorf("peer prefix %q accepted", name)
		}
	}
	f := newStateFixture(t)
	for _, args := range [][]string{{}, {"list", "extra"}, {"add"}, {"add", "bad:name"}, {"remove", "../x"}, {"unknown"}} {
		if err := f.cmd.senderCommand(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}

func TestRolelessLeadSenderRefusalNamesOperatorRecovery(t *testing.T) {
	f := newStateFixture(t)
	a := f.addHitched(t, "l", "lead", "", "")
	f.env["GANG_AGENT_ID"] = string(a.ID)
	if err := f.cmd.execute([]string{"whoami"}); err != nil || f.out.String() != "lead\n" {
		t.Fatalf("identity %q: %v", f.out.String(), err)
	}
	f.out.Reset()
	// Snooze and team-wide authority use the same role requirement.
	requireRefused(t, f.cmd.execute([]string{"snooze", "--clear", "another-intent"}), "only the lead")
	requireRefused(t, f.run.refuseTeamWide("down"), "not its lead")
	for _, action := range []string{"add", "remove"} {
		err := f.cmd.execute([]string{"sender", action, "relay"})
		requireRefused(t, err, "This caller has no role")
		for _, text := range []string{"from an operator shell", "gang drop lead --team", "gang hitch lead --team", "--role lead"} {
			if !strings.Contains(err.Error(), text) {
				t.Fatalf("recovery lacks %q: %v", text, err)
			}
		}
	}
	rows, err := f.run.team.ListSenders()
	if err != nil || len(rows) != 0 {
		t.Fatalf("refusal mutated registry: %+v %v", rows, err)
	}
}
