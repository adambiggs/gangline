package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLimitsCollarWithoutLiveAgent(t *testing.T) {
	f := newLimitsFixture(t)
	err := f.cmd.limits([]string{"-c", "claude-code"})
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitUnknown {
		t.Fatalf("limits without agent = %v, want explicit unknown for unsupported query", err)
	}
	if f.out.Len() != 0 || f.input.submits != 0 {
		t.Fatalf("unsupported query produced output or submitted input: %q, %d", f.out.String(), f.input.submits)
	}
}

func TestLimitsCollarRejectsMixedArguments(t *testing.T) {
	f := newLimitsFixture(t)
	for _, args := range [][]string{{"-c"}, {"-c", "codex", "worker"}, {"worker", "-c", "codex"}, {"-c", "--bad"}} {
		err := f.cmd.limits(args)
		var ce commandError
		if !errors.As(err, &ce) || ce.status != exitUsage {
			t.Fatalf("limits %v = %v", args, err)
		}
	}
}

func TestLimitsCollarQueriesWithoutTeam(t *testing.T) {
	f := newLimitsFixture(t)
	root := t.TempDir()
	executable := filepath.Join(root, "native-cli")
	script := `#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
[ "$*" = 'app-server --listen stdio://' ] || exit 80
printf '%s\n' '{"id":1,"result":{}}' '{"id":2,"result":{"rateLimits":{"limitId":"codex","primary":{"usedPercent":12,"resetsAt":1800000000}}}}'
cat >/dev/null
`
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	collar, err := os.ReadFile("../../harness/collars/codex.cue")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(collar), `command: "codex"`, "command: "+strconv.Quote(executable), 1)
	if err := os.WriteFile(filepath.Join(root, "codex.cue"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	f.env["GANG_COLLARS"] = root
	f.env["GANG_STATE_ROOT"] = filepath.Join(root, "no-team")
	if err := f.cmd.limits([]string{"-c", "codex"}); err != nil {
		t.Fatal(err)
	}
	if got, want := f.out.String(), "codex/primary\t12%\t2027-01-15T08:00:00Z\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if _, err := os.Stat(f.env["GANG_STATE_ROOT"]); !os.IsNotExist(err) {
		t.Fatalf("query touched team state: %v", err)
	}
	if f.input.submits != 0 {
		t.Fatal("query submitted native input")
	}
}

func newLimitsFixture(t *testing.T) *stateFixture {
	t.Helper()
	f := newStateFixture(t)
	f.cmd.newTimeout = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		return context.WithCancel(parent)
	}
	return f
}
