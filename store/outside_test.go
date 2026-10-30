package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func outsideFixture(t *testing.T) (Paths, TeamPaths, string, OutsideRegistration, string) {
	t.Helper()
	p := Paths{Root: t.TempDir()}
	team, err := p.Team("throwaway")
	if err != nil {
		t.Fatal(err)
	}
	if err := team.Create(); err != nil {
		t.Fatal(err)
	}
	credential := filepath.Join(t.TempDir(), "credential")
	registration, err := p.RegisterOutside("throwaway", "toby", credential, credentialTestOrigin())
	if err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(credential)
	if err != nil {
		t.Fatal(err)
	}
	return p, team, credential, registration, strings.TrimSpace(string(token))
}
func TestOutsideMailboxCredentialLifecycle(t *testing.T) {
	p, team, credential, registration, token := outsideFixture(t)
	info, err := os.Stat(credential)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("credential permissions %o", info.Mode().Perm())
	}
	path, _ := team.outsidePath(registration.ID)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), token) {
		t.Fatal("state contains bearer credential")
	}
	principal, err := p.AuthenticateOutside("throwaway", token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Sender().Kind != core.SenderOutside || principal.Sender().Name != "toby" {
		t.Fatalf("sender %+v", principal.Sender())
	}
	if _, err := p.AuthenticateOutside("throwaway", token+"wrong"); !errors.Is(err, ErrOutsideUnauthorized) {
		t.Fatalf("invalid credential: %v", err)
	}
	if err := p.WithOutside("throwaway", principal, func(*OutsideMailbox) error {
		return p.RotateOutside("throwaway", "toby", filepath.Join(t.TempDir(), "busy"), credentialTestOrigin())
	}); !errors.Is(err, ErrLocked) {
		t.Fatalf("rotation bypassed auth transaction: %v", err)
	}
	rotated := filepath.Join(t.TempDir(), "rotated")
	if err := p.RotateOutside("throwaway", "toby", rotated, credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.AuthenticateOutside("throwaway", token); !errors.Is(err, ErrOutsideUnauthorized) {
		t.Fatalf("old credential: %v", err)
	}
	if _, err := p.ResolveOutsidePrincipal("throwaway", principal.String()); !errors.Is(err, ErrOutsideUnauthorized) {
		t.Fatalf("old OAuth principal: %v", err)
	}
	if err := p.WithOutside("throwaway", principal, func(*OutsideMailbox) error { t.Fatal("revoked principal called back"); return nil }); !errors.Is(err, ErrOutsideUnauthorized) {
		t.Fatalf("old principal: %v", err)
	}
	newToken, err := os.ReadFile(rotated)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := p.AuthenticateOutside("throwaway", strings.TrimSpace(string(newToken)))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveOutside("throwaway", "toby", credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ResolveOutsidePrincipal("throwaway", newer.String()); !errors.Is(err, ErrOutsideUnauthorized) {
		t.Fatalf("removed principal: %v", err)
	}
	if _, err := team.ResolveName("toby"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("name retained: %v", err)
	}
}
func TestOutsideMailboxQueueAckAndDurableMetadata(t *testing.T) {
	p, team, _, registration, token := outsideFixture(t)
	principal, err := p.AuthenticateOutside("throwaway", token)
	if err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "one", To: registration.Name, Recipient: registration.ID, From: core.Sender{Kind: core.SenderAgent, Name: "lead"}, Message: core.Message{Text: "review"}, CreatedAt: credentialTestOrigin().At}
	if err := p.QueueOutside("throwaway", "toby", e); err != nil {
		t.Fatal(err)
	}
	if err := p.QueueOutside("throwaway", "toby", e); !errors.Is(err, ErrEnvelopeQueued) {
		t.Fatalf("duplicate queue: %v", err)
	}
	err = p.WithOutside("throwaway", principal, func(m *OutsideMailbox) error {
		pending := m.Pending()
		if len(pending) != 1 || pending[0].Message.Text != "review" {
			t.Fatalf("pending %+v", pending)
		}
		if err := m.Put("receipt", map[string]string{"id": "one"}); err != nil {
			return err
		}
		return m.Ack([]core.EnvelopeID{"one"}, credentialTestOrigin().At)
	})
	if err != nil {
		t.Fatal(err)
	}
	p = Paths{Root: p.Root}
	err = p.WithOutside("throwaway", principal, func(m *OutsideMailbox) error {
		if len(m.Pending()) != 0 {
			t.Fatal("ack did not persist")
		}
		var receipt map[string]string
		found, err := m.Get("receipt", &receipt)
		if err != nil || !found || receipt["id"] != "one" {
			t.Fatalf("receipt %+v %t %v", receipt, found, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := team.ListOutside()
	if err != nil || len(list) != 1 || !reflect.DeepEqual(list[0], registration) {
		t.Fatalf("list %+v %v", list, err)
	}
	statePath, _ := team.outsidePath(registration.ID)
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateBytes), "receipt") || strings.Contains(string(stateBytes), "data") {
		t.Fatalf("mailbox state contains keyed history: %s", stateBytes)
	}
	dataDirectory := filepath.Join(team.Directory, "outside-data", string(registration.ID))
	if _, err := os.Stat(dataDirectory); err != nil {
		t.Fatal(err)
	}
	// A newly registered identity cannot receive an envelope built for its predecessor.
	if err := p.RemoveOutside("throwaway", "toby", credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dataDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("registration removal retained keyed records: %v", err)
	}
	if _, err := p.RegisterOutside("throwaway", "toby", filepath.Join(t.TempDir(), "replacement"), credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if err := p.QueueOutside("throwaway", "toby", e); err == nil {
		t.Fatal("stale recipient queued to replacement")
	}
}
func TestOutsideNameAndCredentialFileClaims(t *testing.T) {
	p, team, credential, registration, _ := outsideFixture(t)
	if _, err := p.RegisterOutside("throwaway", "toby", filepath.Join(t.TempDir(), "unused"), credentialTestOrigin()); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate name: %v", err)
	}
	if err := team.ClaimName("toby", "native"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("native bypassed outside name: %v", err)
	}
	if err := team.ClaimName("lead", "native"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RegisterOutside("throwaway", "lead", filepath.Join(t.TempDir(), "unused"), credentialTestOrigin()); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("outside bypassed native name: %v", err)
	}
	if err := p.RotateOutside("throwaway", "toby", credential, credentialTestOrigin()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("credential overwrite: %v", err)
	}
	if got, err := team.ResolveOutside("toby"); err != nil || !reflect.DeepEqual(got, registration) {
		t.Fatalf("failed rotate changed registration: %+v %v", got, err)
	}
}

func TestOutsideSettledRecordsDoNotAffectMailboxOperations(t *testing.T) {
	p, team, _, registration, token := outsideFixture(t)
	principal, err := p.AuthenticateOutside("throwaway", token)
	if err != nil {
		t.Fatal(err)
	}
	statePath, _ := team.outsidePath(registration.ID)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	err = p.WithOutside("throwaway", principal, func(m *OutsideMailbox) error {
		if err := m.Put("known", map[string]string{"receipt": "known-receipt"}); err != nil {
			return err
		}
		// Authentication and mailbox operations stay independent of unrelated
		// keyed records, including records that cannot be decoded.
		return os.WriteFile(m.dataPath("unrelated"), []byte("invalid json"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("keyed records rewrote the mailbox snapshot")
	}
	if _, err := p.AuthenticateOutside("throwaway", token); err != nil {
		t.Fatalf("auth read unrelated history: %v", err)
	}
	if _, err := p.ReadOutsidePending("throwaway", principal); err != nil {
		t.Fatalf("pending read unrelated history: %v", err)
	}
	envelope := core.Envelope{ID: "next", To: registration.Name, Recipient: registration.ID, From: core.Sender{Kind: core.SenderAgent, Name: "lead"}, Message: core.Message{Text: "new mail"}, CreatedAt: credentialTestOrigin().At}
	if err := p.QueueOutside("throwaway", "toby", envelope); err != nil {
		t.Fatalf("queue read unrelated history: %v", err)
	}
	err = p.WithOutside("throwaway", principal, func(m *OutsideMailbox) error {
		var record map[string]string
		found, err := m.Get("known", &record)
		if err != nil {
			return err
		}
		if !found || record["receipt"] != "known-receipt" {
			t.Fatalf("keyed lookup %+v %t", record, found)
		}
		if err := m.Delete("known"); err != nil {
			return err
		}
		if found, err := m.Get("known", &record); err != nil || found {
			t.Fatalf("keyed delete %t %v", found, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
