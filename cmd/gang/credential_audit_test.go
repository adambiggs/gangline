package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func credentialTestOrigin() core.CredentialOrigin {
	return core.CredentialOrigin{At: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC), Actor: core.CredentialActor{Kind: "operator_shell"}, Command: "store test"}
}

func TestCredentialCommandsRetainCreationAndLifecycle(t *testing.T) {
	for _, caller := range []string{"operator_shell", "agent"} {
		t.Run(caller, func(t *testing.T) {
			f := newStateFixture(t)
			if caller == "agent" {
				a := f.add(t, "lead-id", "owner", "codex")
				a.Role = "lead"
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
				f.env["GANG_AGENT_ID"] = string(a.ID)
			}
			key := filepath.Join(t.TempDir(), "key")
			for _, args := range [][]string{{"outside", "register", "remote", "--credential-file", key}, {"sender", "add", "relay"}} {
				if err := f.cmd.execute(args); err != nil {
					t.Fatal(err)
				}
			}
			outside, err := f.run.team.ResolveOutside("remote")
			if err != nil {
				t.Fatal(err)
			}
			sender, err := f.run.team.ReadSender("relay")
			if err != nil {
				t.Fatal(err)
			}
			for _, registration := range []any{outside, sender} {
				data, err := json.Marshal(registration)
				if err != nil {
					t.Fatal(err)
				}
				var record map[string]json.RawMessage
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				if len(record["created"]) == 0 {
					t.Fatalf("registration lacks creation provenance: %s", data)
				}
				if !strings.Contains(string(record["created"]), caller) {
					t.Fatalf("creation lacks caller: %s", data)
				}
				var created core.CredentialOrigin
				if err := json.Unmarshal(record["created"], &created); err != nil {
					t.Fatal(err)
				}
				if !created.At.Equal(f.cmd.now()) || created.Command == "" {
					t.Fatalf("creation lacks command/time: %+v", created)
				}
			}
			for _, args := range [][]string{{"outside", "rotate", "remote", "--credential-file", filepath.Join(t.TempDir(), "rotated")}, {"drop", "remote"}, {"sender", "remove", "relay"}, {"outside", "register", "removed", "--credential-file", filepath.Join(t.TempDir(), "removed")}, {"outside", "remove", "removed"}} {
				if err := f.cmd.execute(args); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			var types, commands []string
			if err := store.ReadLog(strings.NewReader(string(data)), func(e core.Event) error {
				if strings.HasPrefix(e.Type, "outside_") || strings.HasPrefix(e.Type, "sender_") {
					if e.Actor == nil || e.Actor.Kind != caller || e.Command == "" || !e.At.Equal(f.cmd.now()) {
						t.Fatalf("audit lacks command provenance: %+v", e)
					}
					if caller == "agent" && (e.Actor.HitchID != "lead-id" || e.Actor.Name != "owner") {
						t.Fatalf("caller not attributed: %+v", e.Actor)
					}
					types = append(types, e.Type)
					commands = append(commands, e.Command)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			want := "outside_registered,sender_added,outside_rotated,outside_removed,sender_removed,outside_registered,outside_removed"
			if strings.Join(types, ",") != want {
				t.Fatalf("lifecycle %v; want %s", types, want)
			}
			if strings.Join(commands, ",") != "gang outside register,gang sender add,gang outside rotate,gang drop,gang sender remove,gang outside register,gang outside remove" {
				t.Fatalf("commands %v", commands)
			}
			if strings.Contains(string(data), sender.CredentialHash) {
				t.Fatal("audit contains sender hash")
			}
			if strings.Contains(string(data), "credential_hash") || strings.Contains(string(data), "generation") {
				t.Fatalf("audit contains credential material: %s", data)
			}
		})
	}
}

func TestOutsideCredentialCommandsRefuseUnverifiedCaller(t *testing.T) {
	for _, action := range []string{"register", "rotate", "remove", "drop"} {
		t.Run(action, func(t *testing.T) {
			f := newStateFixture(t)
			paths, principal := registerOutsideFixture(t, f)
			a := f.add(t, "caller", "caller", "codex")
			f.env["GANG_AGENT_ID"] = string(a.ID)
			f.env["GANG_AGENT_NONCE"] = "unverified"
			args := []string{"outside", action, "toby"}
			if action == "drop" {
				args = []string{"drop", "toby"}
			}
			if action == "register" {
				args[2] = "new-registration"
			}
			if action == "register" || action == "rotate" {
				args = append(args, "--credential-file", filepath.Join(t.TempDir(), "issued"))
			}
			if err := f.cmd.execute(args); err == nil {
				t.Fatal("unverified caller changed credentials")
			}
			if _, err := paths.ResolveOutsidePrincipal("unit", principal.String()); err != nil {
				t.Fatalf("refused command changed existing credentials: %v", err)
			}
			if _, err := f.run.team.ResolveName("new-registration"); err == nil {
				t.Fatal("refused command registered a name")
			}
		})
	}
}

func TestSenderRemovalAuditFailureIsExecutionFailure(t *testing.T) {
	for _, failure := range []string{"directory", "missing-target"} {
		t.Run(failure, func(t *testing.T) {
			f := newStateFixture(t)
			senderKeyFixture(t, f)
			if err := os.Remove(f.run.team.Log); err != nil {
				t.Fatal(err)
			}
			if failure == "directory" {
				if err := os.Mkdir(f.run.team.Log, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(filepath.Join(t.TempDir(), "missing", "audit"), f.run.team.Log); err != nil {
				t.Fatal(err)
			}
			err := f.cmd.execute([]string{"sender", "remove", "relay"})
			if err == nil || errorStatus(err) != exitError {
				t.Fatalf("completed removal reported as refused: %v (status %d)", err, errorStatus(err))
			}
			var pathError *os.PathError
			if !errors.As(err, &pathError) {
				t.Fatalf("audit cause lost: %v", err)
			}
			if failure == "missing-target" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("audit absence cause lost: %v", err)
			}
			if _, err := f.run.team.ResolveName("relay"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("sender remains registered: %v", err)
			}
		})
	}
}

func TestSenderRemovalInvalidRegistrationIsRefused(t *testing.T) {
	for _, name := range []string{"missing", "worker"} {
		t.Run(name, func(t *testing.T) {
			f := newStateFixture(t)
			f.add(t, "worker-id", "worker", "codex")
			err := f.cmd.execute([]string{"sender", "remove", name})
			if err == nil || errorStatus(err) != exitRefused {
				t.Fatalf("invalid registration not refused: %v", err)
			}
		})
	}
}
