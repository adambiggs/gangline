//go:build acceptance

package main

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/adambiggs/gangline/substrate"
)

func TestTitleReadsBackOwnedComposerWithoutModeFooter(t *testing.T) {
	for _, footer := range []string{"  paste again to expand", ""} {
		t.Run(footer, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := titleFixture(t, "claude")
				closing := strings.Repeat("─", 40)
				empty := screenWithText(namedRule("Main session"), "❯ ", closing, "  40k/200k (20%)", "  ⏸ manual mode on")
				fixture.input.screen = empty
				fixture.input.onKeys = func(keys substrate.Keys) error {
					if keys.Text != "" {
						rows := []string{namedRule("Main session"), "❯ /rename title", closing}
						if footer != "" {
							rows = append(rows, "  40k/200k (20%)", footer)
						}
						fixture.input.screen = screenWithText(rows...)
					}
					return nil
				}
				fixture.input.submit = func(string) error {
					fixture.input.screen = empty
					return nil
				}
				fixture.cmd.settleInput = nil
				fixture.run.cmd = fixture.cmd
				err := fixture.cmd.execute([]string{"title", "worker", "title", "--team", "unit"})
				var result commandError
				if !errors.As(err, &result) || result.status != exitUnknown || !strings.Contains(err.Error(), "persistence unconfirmed") {
					t.Fatalf("result=%v", err)
				}
				if fixture.input.submits != 1 {
					t.Fatalf("submits=%d; want 1", fixture.input.submits)
				}
			})
		})
	}
}
