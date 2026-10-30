//go:build acceptance

package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A cache shaped like the one Claude Code's /usage writes to its global
// configuration, with the fields gang does not read left in place.
const claudeUsageCache = `{"numStartups": 3, "cachedUsageUtilization": {"fetchedAtMs": FETCHED, "accountUuid": "account",
 "utilization": {"five_hour": {"utilization": 2, "resets_at": "2026-10-13T02:30:00.301515+00:00"},
  "limits": [
   {"kind": "session", "group": "session", "percent": 2, "severity": "normal", "resets_at": "2026-10-13T02:30:00.301515+00:00", "scope": null},
   {"kind": "weekly_all", "group": "weekly", "percent": 26, "resets_at": "2026-10-14T17:00:00.301537+00:00", "scope": null},
   {"kind": "weekly_scoped", "group": "weekly", "percent": 40, "resets_at": "2026-10-14T17:00:00.301697+00:00", "scope": {"model": {"id": null, "display_name": "Fable"}, "surface": null}},
   {"kind": "monthly", "group": "spend", "percent": 0, "resets_at": "2026-11-05T07:59:00+00:00"}
  ]}}}`

func usageCache(fetched string) []byte {
	return []byte(strings.Replace(claudeUsageCache, "FETCHED", fetched, 1))
}

func TestClaudeUsageCacheMapsNativeWindows(t *testing.T) {
	measured, err := parseClaudeUsageCache(usageCache("1791856775387"))
	if err != nil {
		t.Fatal(err)
	}
	if !measured.at.Equal(time.UnixMilli(1791856775387)) {
		t.Fatalf("reading time = %v, want the cache's fetch time", measured.at)
	}
	limits := measured.windows
	want := []LimitWindow{
		{Label: "five_hour", UsedPercent: 2, ResetAt: 1791858600, WindowMinutes: 300},
		{Label: "seven_day", UsedPercent: 26, ResetAt: 1791997200, WindowMinutes: 10080},
		{Label: "seven_day/fable", UsedPercent: 40, ResetAt: 1791997200, WindowMinutes: 10080},
	}
	if len(limits) != len(want) {
		t.Fatalf("limits = %#v", limits)
	}
	for i := range want {
		if limits[i] != want[i] {
			t.Fatalf("limit %d = %#v, want %#v", i, limits[i], want[i])
		}
	}
}

func TestClaudeUsageCacheRefusesInvalidReadings(t *testing.T) {
	for name, data := range map[string][]byte{
		"no cache":        []byte(`{"numStartups": 3}`),
		"no fetch time":   []byte(`{"cachedUsageUtilization": {"utilization": {"limits": []}}}`),
		"no windows":      []byte(`{"cachedUsageUtilization": {"fetchedAtMs": 1791856775387, "utilization": {"limits": []}}}`),
		"missing percent": []byte(`{"cachedUsageUtilization": {"fetchedAtMs": 1791856775387, "utilization": {"limits": [{"kind": "session", "group": "session", "resets_at": "2026-10-13T02:30:00Z"}]}}}`),
		"bad reset":       []byte(`{"cachedUsageUtilization": {"fetchedAtMs": 1791856775387, "utilization": {"limits": [{"kind": "session", "group": "session", "percent": 2, "resets_at": "soon"}]}}}`),
	} {
		if limits, err := parseClaudeUsageCache(data); err == nil {
			t.Fatalf("%s: limits = %#v, want an error", name, limits)
		}
	}
}

// The fixture CLI checks its exact arguments, refreshes the cache in its
// HOME, and prints the print-mode result of a local command.
func claudeUsageFixture(t *testing.T, result, fetched string) Collar {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cache.json"), usageCache(fetched), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "claude")
	script := `#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
[ "$*" = '-p /usage --no-session-persistence --output-format json' ] || exit 80
cp "$HOME/cache.json" "$HOME/.claude.json"
printf '%s\n' '` + result + `'
`
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return Collar{Name: "claude", Launch: Launch{Command: executable, Env: map[string]string{"HOME": root, "CLAUDE_CONFIG_DIR": ""}}, Primitives: Primitives{LimitsQuery: &Invocation{Name: "claude-usage-command"}}}
}

const localResult = `{"type":"result","subtype":"success","is_error":false,"num_turns":0,"total_cost_usd":0,"result":"Current session: 2% used"}`

// /usage fetches again only when its cache is not recent, so the reading is
// as old as the cache, and says so.
func TestClaudeUsageQueryReadsTheCacheAtItsFetchTime(t *testing.T) {
	collar := claudeUsageFixture(t, localResult, "1791856775387")
	limits, at, err := QueryProviderLimits(context.Background(), collar)
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(time.UnixMilli(1791856775387)) {
		t.Fatalf("reading time = %v, want the cache's fetch time", at)
	}
	if len(limits) != 3 || limits[2].Label != "seven_day/fable" || limits[2].UsedPercent != 40 {
		t.Fatalf("limits = %#v", limits)
	}
}

// Print mode skips Claude Code's folder-trust dialog, so the project settings
// and hooks of the query's directory would run unasked. A shared temporary
// directory is writable by other processes; the query must run in a private
// empty directory of its own and remove it afterwards.
func TestClaudeUsageQueryRunsOutsideTheSharedTemporaryDirectory(t *testing.T) {
	shared := t.TempDir()
	if err := os.MkdirAll(filepath.Join(shared, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", shared)
	collar := claudeUsageFixture(t, localResult, "1791856775387")
	record := filepath.Join(collar.Launch.Env["HOME"], "query-directory")
	script := `#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
pwd -P > "$HOME/query-directory"
[ -z "$(ls -A .)" ] || exit 81
[ "$(ls -ld . | cut -c1-10)" = drwx------ ] || exit 82
cp "$HOME/cache.json" "$HOME/.claude.json"
printf '%s\n' '` + localResult + `'
`
	if err := os.WriteFile(collar.Launch.Command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := QueryProviderLimits(context.Background(), collar); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	directory := strings.TrimSpace(string(data))
	if sharedPath, err := filepath.EvalSymlinks(shared); err != nil {
		t.Fatal(err)
	} else if directory == sharedPath {
		t.Fatalf("query ran in the shared temporary directory %s", directory)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("query directory %s remains after the query: %v", directory, err)
	}
}

func TestClaudeUsageQueryRefusesUnwitnessedReadings(t *testing.T) {
	for name, collar := range map[string]Collar{
		"model answered":        claudeUsageFixture(t, strings.Replace(localResult, `"num_turns":0`, `"num_turns":1`, 1), "4102444800000"),
		"turn count absent":     claudeUsageFixture(t, strings.Replace(localResult, `"num_turns":0,`, ``, 1), "4102444800000"),
		"command failed":        claudeUsageFixture(t, strings.Replace(localResult, `"is_error":false`, `"is_error":true`, 1), "4102444800000"),
		"result is not success": claudeUsageFixture(t, strings.Replace(localResult, `"success"`, `"error_during_execution"`, 1), "4102444800000"),
	} {
		if limits, _, err := QueryProviderLimits(context.Background(), collar); err == nil {
			t.Fatalf("%s: limits = %#v, want an error", name, limits)
		}
	}
}
