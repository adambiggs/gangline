//go:build acceptance

package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestTitleSubmitsWrappedCodexCommand(t *testing.T) {
	fixture := titleFixture(t, "codex")
	empty := screenWithText("READY", "› ", "", "  status")
	empty.Cursor = substrate.Cursor{Row: 1, Column: 2, Visible: true}
	fixture.input.screen = empty
	fixture.input.onKeys = func(keys substrate.Keys) error {
		if keys.Text != "" {
			fixture.input.screen = screenWithText("READY", "› /rename worker@team: This is a long", "  native-input reproduction", "", "  status")
			fixture.input.screen.Cursor = substrate.Cursor{Row: 2, Column: 28, Visible: true}
		}
		return nil
	}
	fixture.input.submit = func(string) error {
		fixture.input.screen = empty
		return nil
	}
	err := fixture.cmd.execute([]string{"title", "worker", "worker@team: This is a long native-input reproduction", "--team", "unit"})
	var result commandError
	if !errors.As(err, &result) || result.status != exitUnknown || !strings.Contains(err.Error(), "persistence unconfirmed") {
		t.Fatalf("result=%v", err)
	}
	if fixture.input.submits != 1 {
		t.Fatalf("submits=%d; want 1", fixture.input.submits)
	}
}
