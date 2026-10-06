package main

import (
	"context"
	"errors"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFailedSendWarningAfterWithdrawal(t *testing.T) {
	for _, withdrawn := range []bool{true, false} {
		t.Run(map[bool]string{true: "withdrawn", false: "unconfirmed"}[withdrawn], func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "claude")
			p, _ := f.run.team.Agent(a.ID)
			empty := screenWithText("────────", "❯ ", "────────")
			f.input.command, f.input.screen = "claude", empty
			f.input.onKeys = func(k substrate.Keys) error {
				if k.Text != "" {
					f.input.screen = pasteShown(strings.TrimSuffix(strings.TrimPrefix(k.Text, "\x1b[200~"), "\x1b[201~"))
				}
				if withdrawn && slices.Contains(k.Names, "C-u") {
					f.input.screen = empty
				}
				return nil
			}
			first := true
			f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
				if first {
					first = false
					return errors.New("test paste settle failure")
				}
				return nil
			}
			corrupt := false
			f.cmd.afterUnlock = func() {
				if corrupt {
					return
				}
				cur := filepath.Join(p.Inbox, "cur")
				if err := os.Rename(cur, cur+"-saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cur, []byte("test unreadable receipt directory"), 0600); err != nil {
					t.Fatal(err)
				}
				corrupt = true
			}
			f.cmd.stdin = strings.NewReader("hello")

			err := f.cmd.send([]string{"worker", "--from", "operator"})
			outcome := "failed"
			if withdrawn {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				outcome = "unverified"
				var ce commandError
				if !errors.As(err, &ce) || ce.status != exitUnknown {
					t.Fatalf("unconfirmed withdrawal: %v", err)
				}
			}
			if !strings.Contains(f.out.String(), outcome) || !strings.Contains(f.errOut.String(), outcome+" status") || strings.Contains(f.errOut.String(), "do not resend") == withdrawn || f.input.submits != 0 {
				t.Fatalf("out=%q stderr=%q submits=%d", f.out.String(), f.errOut.String(), f.input.submits)
			}
			t.Logf("stdout=%q; stderr=%q; Enter presses=%d", f.out.String(), f.errOut.String(), f.input.submits)
		})
	}
}
