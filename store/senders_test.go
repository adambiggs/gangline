package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestSenderAuthenticationBindsRegistration(t *testing.T) {
	for _, corruption := range []string{"id", "name", "hash", "missing", "native", "removed", "trailing"} {
		t.Run(corruption, func(t *testing.T) {
			p := testTeam(t)
			key, err := p.AddSender("relay")
			if err != nil {
				t.Fatal(err)
			}
			secret, err := os.ReadFile(key)
			if err != nil {
				t.Fatal(err)
			}
			original, err := p.AuthenticateSender("relay", secret)
			if err != nil || original.Kind != core.SenderRegistered {
				t.Fatalf("valid: %+v %v", original, err)
			}
			r, err := p.ReadSender("relay")
			if err != nil {
				t.Fatal(err)
			}
			switch corruption {
			case "trailing":
				data, err := os.ReadFile(filepath.Join(filepath.Dir(key), "sender.json"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(filepath.Dir(key), "sender.json"), append(data, []byte("{}")...), 0600); err != nil {
					t.Fatal(err)
				}
			case "id":
				r.ID = "sender-wrong"
			case "name":
				r.Name = "other"
			case "hash":
				r.CredentialHash = ""
			case "missing":
				if err := os.Remove(filepath.Join(filepath.Dir(key), "sender.json")); err != nil {
					t.Fatal(err)
				}
			case "native":
				if err := p.RemoveName(r.Name, r.ID); err != nil {
					t.Fatal(err)
				}
				if err := p.ClaimName("relay", "agent"); err != nil {
					t.Fatal(err)
				}
			case "removed":
				if err := p.RemoveSender("relay"); err != nil {
					t.Fatal(err)
				}
			}
			if corruption == "id" || corruption == "name" || corruption == "hash" {
				if err := atomicJSON(filepath.Join(filepath.Dir(key), "sender.json"), r); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := p.AuthenticateSender("relay", secret); !errors.Is(err, ErrSenderUnauthorized) {
				t.Fatalf("corrupt %s authenticated: %v", corruption, err)
			}
		})
	}
}

func TestSenderRemovalDoesNotCrossNamespace(t *testing.T) {
	p := testTeam(t)
	if err := p.ClaimName("worker", "native"); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveSender("worker"); !errors.Is(err, ErrSenderUnauthorized) {
		t.Fatalf("removed native claim: %v", err)
	}
	for _, id := range []core.HitchID{"../sender-x", "native"} {
		if _, err := p.SenderCredential(id); err == nil {
			t.Fatalf("accepted id %q", id)
		}
	}
	if _, err := p.AddSender("../escape"); err == nil {
		t.Fatal("accepted path traversal")
	}
	if _, err := p.AddSender("worker"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("accepted native claim: %v", err)
	}
}

func TestSenderRecreationGetsNewIdentity(t *testing.T) {
	p := testTeam(t)
	key, err := p.AddSender("relay")
	if err != nil {
		t.Fatal(err)
	}
	oldKey, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	old, err := p.AuthenticateSender("relay", oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveSender("relay"); err != nil {
		t.Fatal(err)
	}
	key, err = p.AddSender("relay")
	if err != nil {
		t.Fatal(err)
	}
	newKey, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	current, err := p.AuthenticateSender("relay", newKey)
	if err != nil {
		t.Fatal(err)
	}
	if old.SameIdentity(current) {
		t.Fatal("recreated sender retained scheduling authority")
	}
	if _, err := p.AuthenticateSender("relay", oldKey); !errors.Is(err, ErrSenderUnauthorized) {
		t.Fatalf("revoked key: %v", err)
	}
}
