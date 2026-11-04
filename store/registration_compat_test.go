package store

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestRegistrationLaunchCommandCompatibility(t *testing.T) {
	for _, raw := range []string{
		`{"generation":"old","session":"session","token_hash":"token"}`,
		`{"generation":"old","session":"session","token_hash":"token","launch_command":"2.1.288"}`,
	} {
		var registration core.PaneRegistration
		if err := decodeJSON("registration", strings.NewReader(raw), &registration); err != nil {
			t.Fatal(err)
		}
		if registration.Generation != "old" || registration.Session != "session" || registration.TokenHash != "token" {
			t.Fatalf("registration identity changed: %+v", registration)
		}
	}
	var registration core.PaneRegistration
	if err := decodeJSON("registration", strings.NewReader(`{"unexpected_authority":"yes"}`), &registration); err == nil {
		t.Fatal("unrelated unknown fields must still be rejected")
	}
}
