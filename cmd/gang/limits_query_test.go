//go:build acceptance

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

	"github.com/adambiggs/gangline/core"
)

func TestLimitsCollarWithoutLiveAgent(t *testing.T) {
	f := newLimitsFixture(t)
	root := t.TempDir()
	collar, err := os.ReadFile("../../harness/collars/codex.cue")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(collar), `name: "codex"`, `name: "plain"`, 1)
	text = strings.Replace(text, `limits_query: {name: "codex-app-server-limits"}`, "", 1)
	if err := os.WriteFile(filepath.Join(root, "plain.cue"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	f.env["GANG_COLLARS"] = root
	err = f.cmd.limits([]string{"-c", "plain"})
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
	if got, want := f.out.String(), "codex\tcodex/primary\t12% used\telapsed unknown\tpace unknown\tresets 2027-01-15T08:00:00Z\texhausts unknown\tage 0s\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if _, err := os.Stat(f.env["GANG_STATE_ROOT"]); !os.IsNotExist(err) {
		t.Fatalf("query touched team state: %v", err)
	}
	if f.input.submits != 0 {
		t.Fatal("query submitted native input")
	}
}

// limitsCollars writes overlays that point both built-in collars at fixture
// CLIs. The codex fixture serves the app-server protocol; the claude fixture
// refreshes a usage cache in its HOME the way /usage does. A collar given an
// empty fixture fails its query.
func limitsCollars(t *testing.T, f *stateFixture, codex, claude string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	codexScript := "#!/bin/sh\nexit 1\n"
	if codex != "" {
		codexScript = "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n[ \"$*\" = 'app-server --listen stdio://' ] || exit 80\nprintf '%s\\n' '{\"id\":1,\"result\":{}}' '" + codex + "'\ncat >/dev/null\n"
	}
	claudeScript := "#!/bin/sh\nexit 1\n"
	if claude != "" {
		write("cache.json", claude)
		claudeScript = "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n[ \"$*\" = '-p /usage --no-session-persistence --output-format json' ] || exit 80\ncp \"$HOME/cache.json\" \"$HOME/.claude.json\"\nprintf '%s\\n' '{\"subtype\":\"success\",\"is_error\":false,\"num_turns\":0}'\n"
	}
	codexCLI, claudeCLI := write("codex-cli", codexScript), write("claude-cli", claudeScript)
	write("codex.cue", "collar: launch: command: "+strconv.Quote(codexCLI)+"\n")
	write("claude.cue", "collar: launch: {command: "+strconv.Quote(claudeCLI)+", env: {HOME: "+strconv.Quote(root)+", CLAUDE_CONFIG_DIR: \"\"}}\n")
	f.env["GANG_COLLARS"] = root
}

// With no agent of either collar hitched, gang limits reads both providers
// and reports each window's pace against the fixture clock. Claude's reading
// is as old as the usage cache /usage served.
func TestLimitsReportsEveryCollarWithoutAgents(t *testing.T) {
	f := newLimitsFixture(t)
	now := f.cmd.now()
	week := 7 * 24 * time.Hour
	codexReset := now.Add(week / 2).Unix()
	claudeReset := now.Add(4 * time.Hour).UTC().Format(time.RFC3339)
	limitsCollars(t, f,
		`{"id":2,"result":{"rateLimits":{"limitId":"codex","primary":{"usedPercent":75,"resetsAt":`+strconv.FormatInt(codexReset, 10)+`,"windowDurationMins":10080}}}}`,
		`{"cachedUsageUtilization":{"fetchedAtMs":`+strconv.FormatInt(now.Add(-30*time.Second).UnixMilli(), 10)+`,"utilization":{"limits":[{"kind":"session","group":"session","percent":10,"resets_at":"`+claudeReset+`"}]}}}`)
	if err := f.cmd.limits(nil); err != nil {
		t.Fatal(err)
	}
	want := "claude\tfive_hour\t10% used\t20% elapsed\tpace -10\tresets 2026-09-22T14:00:00Z\texhausts after reset\tage 30s\n" +
		"codex\tcodex/primary\t75% used\t50% elapsed\tpace +25\tresets 2026-09-25T22:00:00Z\texhausts 2026-09-23T14:00:00Z\tage 0s\n"
	if got := f.out.String(); got != want {
		t.Fatalf("output =\n%s\nwant\n%s", got, want)
	}
	if f.input.submits != 0 {
		t.Fatal("limits submitted native input")
	}
}

// A collar whose query fails falls back to the newest reading a team agent
// recorded, with its age; with no such reading the collar is unknown and the
// others still print.
func TestLimitsFallsBackToRecordedReading(t *testing.T) {
	f := newLimitsFixture(t)
	limitsCollars(t, f, `{"id":2,"result":{"rateLimits":{"limitId":"codex","primary":{"usedPercent":5,"resetsAt":1800000000}}}}`, "")
	err := f.cmd.limits(nil)
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitUnknown || !strings.Contains(ce.text, "claude:") {
		t.Fatalf("limits with a failed claude query and no reading = %v", err)
	}
	if !strings.HasPrefix(f.out.String(), "codex\tcodex/primary\t5% used") {
		t.Fatalf("other collars must still print: %q", f.out.String())
	}
	f.out.Reset()
	now := f.cmd.now()
	for i, age := range []time.Duration{time.Hour, 10 * time.Minute} {
		a := f.add(t, "claude-"+strconv.Itoa(i), "claude-"+strconv.Itoa(i), "claude")
		at := now.Add(-age)
		// The newest reading also holds a five-hour window that has since reset.
		a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "seven_day", UsedPercent: float64(30 + i), ResetAt: now.Add(84 * time.Hour).Unix()}, {Label: "five_hour", UsedPercent: 90, ResetAt: now.Add(-time.Minute).Unix()}}}
		p, _ := f.run.team.Agent(a.ID)
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Save(a); err != nil {
			t.Fatal(err)
		}
		_ = l.Close()
	}
	if err := f.cmd.limits(nil); err != nil {
		t.Fatal(err)
	}
	// Pace is measured at the reading's time, ten minutes before the clock.
	if want := "claude\tseven_day\t31% used\t50% elapsed\tpace -19\tresets 2026-09-25T22:00:00Z\texhausts after reset\tage 10m0s\ncodex"; !strings.HasPrefix(f.out.String(), want) {
		t.Fatalf("output = %q, want prefix %q", f.out.String(), want)
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

// A reading's pace is measured when it was taken: two hours ago a five-hour
// window had 20% elapsed against 60% used, which is 40 points ahead, not the
// on-pace line the same use would draw now.
func TestLimitsMeasuresPaceAtTheReadingTime(t *testing.T) {
	f := newLimitsFixture(t)
	at := f.cmd.now().Add(-2 * time.Hour)
	rows := []limitsRow{
		{collar: "claude", at: at, window: core.LimitWindow{Label: "five_hour", UsedPercent: 60, ResetAt: at.Add(4 * time.Hour).Unix()}},
		{collar: "claude", at: f.cmd.now(), window: core.LimitWindow{Label: "five_hour", UsedPercent: 59.7, ResetAt: f.cmd.now().Add(2 * time.Hour).Unix()}},
	}
	if err := f.cmd.writeLimits(rows); err != nil {
		t.Fatal(err)
	}
	want := "claude\tfive_hour\t60% used\t20% elapsed\tpace +40\tresets 2026-09-22T12:00:00Z\texhausts 2026-09-22T08:40:00Z\tage 2h0m0s\n" +
		"claude\tfive_hour\t60% used\t60% elapsed\tpace +0\tresets 2026-09-22T12:00:00Z\texhausts after reset\tage 0s\n"
	if got := f.out.String(); got != want {
		t.Fatalf("output =\n%s\nwant\n%s", got, want)
	}
}
