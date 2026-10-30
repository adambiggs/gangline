//go:build acceptance

package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func credentialTestOrigin() core.CredentialOrigin {
	return core.CredentialOrigin{At: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC), Actor: core.CredentialActor{Kind: "operator_shell"}, Command: "store test"}
}

func TestCredentialMutationAuditFailureReportsCompletedChange(t *testing.T) {
	for _, action := range []string{"sender_add", "sender_remove", "outside_register", "outside_rotate", "outside_remove_direct", "outside_remove"} {
		t.Run(action, func(t *testing.T) {
			p := Paths{Root: t.TempDir()}
			team, err := p.Team("test")
			if err != nil {
				t.Fatal(err)
			}
			if err := team.Create(); err != nil {
				t.Fatal(err)
			}
			origin := credentialTestOrigin()
			if action == "sender_remove" {
				if _, err := team.AddSender("relay", origin); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasPrefix(action, "outside_") && action != "outside_register" {
				if _, err := p.RegisterOutside("test", "remote", filepath.Join(t.TempDir(), "initial"), origin); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(team.Log); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if err := os.Mkdir(team.Log, 0700); err != nil {
				t.Fatal(err)
			}
			credential := filepath.Join(t.TempDir(), "issued")
			switch action {
			case "sender_add":
				credential, err = team.AddSender("relay", origin)
			case "sender_remove":
				err = team.RemoveSender("relay", origin)
			case "outside_register":
				var registration OutsideRegistration
				registration, err = p.RegisterOutside("test", "remote", credential, origin)
				if registration.ID == "" {
					t.Fatal("completed registration omitted its identity")
				}
			case "outside_rotate":
				err = p.RotateOutside("test", "remote", credential, origin)
			case "outside_remove_direct":
				err = team.RemoveOutside("remote", origin)
			case "outside_remove":
				err = p.RemoveOutside("test", "remote", origin)
			}
			if err == nil || !strings.Contains(err.Error(), "completed, but credential audit append failed") {
				t.Fatalf("partial change not reported: %v", err)
			}
			if strings.Contains(action, "remove") {
				name := "remote"
				if action == "sender_remove" {
					name = "relay"
				}
				if _, err := team.ResolveName(name); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("removed claim retained: %v", err)
				}
			} else {
				token, err := os.ReadFile(credential)
				if err != nil {
					t.Fatalf("issued credential lost after audit failure: %v", err)
				}
				if action == "sender_add" {
					if _, err := team.AuthenticateSender("relay", token); err != nil {
						t.Fatalf("claimed sender is broken: %v", err)
					}
				} else if _, err := p.AuthenticateOutside("test", strings.TrimSpace(string(token))); err != nil {
					t.Fatalf("issued outside credential lost: %v", err)
				}
			}
		})
	}
}

func TestCredentialOldRegistrationsKeepUnknownCreation(t *testing.T) {
	p, team, _, outside, token := outsideFixture(t)
	path, _ := team.outsidePath(outside.ID)
	state, err := team.outsideState(outside.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Created = nil
	if err := atomicJSON(path, state); err != nil {
		t.Fatal(err)
	}
	if _, err := p.AuthenticateOutside("throwaway", token); err != nil {
		t.Fatal(err)
	}
	if err := p.RotateOutside("throwaway", "toby", filepath.Join(t.TempDir(), "rotated"), credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	read, err := team.ResolveOutside("toby")
	if err != nil || read.Created != nil {
		t.Fatalf("invented creation after rotation: %+v %v", read.Created, err)
	}
	key, err := team.AddSender("relay", credentialTestOrigin())
	if err != nil {
		t.Fatal(err)
	}
	sender, err := team.ReadSender("relay")
	if err != nil {
		t.Fatal(err)
	}
	sender.Created = nil
	if err := atomicJSON(filepath.Join(filepath.Dir(key), "sender.json"), sender); err != nil {
		t.Fatal(err)
	}
	secret, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := team.AuthenticateSender("relay", secret); err != nil {
		t.Fatal(err)
	}
	readSender, err := team.ReadSender("relay")
	if err != nil || readSender.Created != nil {
		t.Fatalf("invented sender creation: %+v %v", readSender.Created, err)
	}
	if err := team.RemoveSender("relay", credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if err := team.RemoveOutside("toby", credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialLifecycleAuditRetainsCreation(t *testing.T) {
	p, team, credential, registration, token := outsideFixture(t)
	rotated := filepath.Join(t.TempDir(), "rotated")
	if err := p.RotateOutside("throwaway", "toby", rotated, credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	current, err := team.ResolveOutside("toby")
	if err != nil || !reflect.DeepEqual(current.Created, registration.Created) {
		t.Fatalf("rotation changed creation: %+v %v", current, err)
	}
	if err := team.RemoveOutside("toby", credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(team.Log)
	if err != nil {
		t.Fatal(err)
	}
	rotatedToken, err := os.ReadFile(rotated)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{token, outsideHash(token), strings.TrimSpace(string(rotatedToken)), outsideHash(strings.TrimSpace(string(rotatedToken))), credential, rotated} {
		if strings.Contains(string(data), secret) {
			t.Fatal("audit retained credential material or credential path")
		}
	}
	var types []string
	if err := ReadLog(strings.NewReader(string(data)), func(e core.Event) error {
		if e.HitchID != registration.ID || e.Name != registration.Name || e.Actor == nil || *e.Actor != registration.Created.Actor || e.Command != registration.Created.Command || e.At != registration.Created.At {
			t.Fatalf("event lacks provenance: %+v", e)
		}
		types = append(types, e.Type)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(types, ",") != "outside_registered,outside_rotated,outside_removed" {
		t.Fatalf("events %v", types)
	}
}

func TestOutsideRemovalAuditsRevocationDespiteCleanupFailure(t *testing.T) {
	p, team, _, registration, token := outsideFixture(t)
	obstruction := filepath.Join(team.Directory, "outside-data")
	if err := os.WriteFile(obstruction, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	err := team.RemoveOutside("toby", credentialTestOrigin())
	var pathError *os.PathError
	if !errors.As(err, &pathError) {
		t.Fatalf("cleanup cause missing: %v", err)
	}
	if _, err := p.AuthenticateOutside("throwaway", token); !errors.Is(err, ErrOutsideUnauthorized) {
		t.Fatalf("identity not revoked: %v", err)
	}
	path, _ := team.outsidePath(registration.ID)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("mailbox cleanup skipped after data cleanup failure: %v", err)
	}
	data, err := os.ReadFile(team.Log)
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	if err := ReadLog(strings.NewReader(string(data)), func(e core.Event) error {
		if e.Type == "outside_removed" && e.HitchID == registration.ID {
			removed++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("revoked identity has %d removal events", removed)
	}
}

func TestCredentialOriginRequiredBeforeMutation(t *testing.T) {
	p := testTeam(t)
	if _, err := p.AddSender("relay", core.CredentialOrigin{}); err == nil {
		t.Fatal("accepted missing provenance")
	}
	if _, err := p.ResolveName("relay"); err == nil {
		t.Fatal("invalid provenance mutated state")
	}
}
