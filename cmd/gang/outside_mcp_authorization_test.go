package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/store"
)

func TestOutsideSubscriptionSurvivesAuthorizationReadFailure(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	dir := filepath.Join(f.run.team.Directory, "outside-mcp")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{"key": []byte(strings.Repeat("k", 32)), "subscriptions": map[string]any{"synthetic-sub": map[string]any{
		"id": "synthetic-sub", "principal": principal.String(), "url": "https://callback.example/events",
		"secret":    "whsec_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 32))),
		"createdAt": f.run.cmd.now().Add(-time.Second), "expiresAt": nil, "receipts": map[string]string{},
	}}}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "events.json")
	if err := os.WriteFile(statePath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := f.run.newOutsideService("https://mail.example", "client", "https://client.example/callback", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.reports.Close()
	registration := filepath.Join(f.run.team.Directory, "outside", string(principal.ID)+".json")
	if err := os.Rename(registration, registration+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(registration, 0700); err != nil {
		t.Fatal(err)
	}
	_, readErr := paths.ResolveOutsidePrincipal("unit", principal.String())
	var pathErr *os.PathError
	if !errors.As(readErr, &pathErr) || errors.Is(readErr, store.ErrOutsideUnauthorized) {
		t.Fatalf("expected registration read error, got %v", readErr)
	}
	for name, operation := range map[string]func(context.Context) error{"flush": service.flush, "watchMail": service.watchMail} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := operation(ctx); !errors.Is(err, pathErr.Err) {
				t.Fatalf("authorization read error did not stop %s: %v", name, err)
			}
		})
	}
	if saved, err := os.ReadFile(statePath); err != nil || !bytes.Equal(saved, raw) {
		t.Fatalf("authorization failure changed persisted subscriptions: %s, %v", saved, err)
	}
	if err := os.Remove(registration); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(registration+".saved", registration); err != nil {
		t.Fatal(err)
	}
	active, err := service.events.ActivePrincipals(context.Background())
	if err != nil || len(active) != 1 || active[0] != principal.String() {
		t.Fatalf("restored registration lost subscription: %v, %v", active, err)
	}
	if err := paths.RotateOutside("unit", string(principal.Name), filepath.Join(t.TempDir(), "replacement"), credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	active, err = service.events.ActivePrincipals(context.Background())
	if err != nil || len(active) != 0 {
		t.Fatalf("revoked subscription remained active: %v, %v", active, err)
	}
	saved, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var disk struct {
		Subscriptions map[string]json.RawMessage `json:"subscriptions"`
	}
	if err := json.Unmarshal(saved, &disk); err != nil || len(disk.Subscriptions) != 0 {
		t.Fatalf("revoked subscription remained persisted: %s, %v", saved, err)
	}
}
