package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func guardedClaude(t *testing.T, refuse bool) harness.Collar {
	t.Helper()
	c, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	fiveHour := 0.9
	c.HitchGuard = &harness.HitchGuard{Models: []string{"costly"}, FiveHour: &fiveHour, Refuse: refuse}
	return c
}

// limitsReading gives an agent a provider reading taken age before the
// fixture clock with one five-hour window.
func (f *stateFixture) limitsReading(t *testing.T, a core.Agent, age time.Duration, used float64, reset time.Time) {
	t.Helper()
	at := f.cmd.now().Add(-age)
	f.setAgent(t, a, func(a *core.Agent) {
		a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", WindowMinutes: 300, UsedPercent: used, ResetAt: reset.Unix()}}}
	})
}

func TestHitchGuardIgnoresModelsItDoesNotName(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.limitsReading(t, a, time.Minute, 99, f.cmd.now().Add(time.Hour))
	unguarded, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		collar harness.Collar
		model  string
	}{{unguarded, "costly"}, {guardedClaude(t, true), ""}, {guardedClaude(t, true), "cheap"}} {
		if err := f.run.checkHitchGuard(check.collar, check.model); err != nil {
			t.Fatalf("model %q: %v", check.model, err)
		}
	}
	if f.errOut.Len() != 0 {
		t.Fatalf("stderr = %q", f.errOut.String())
	}
}

func TestHitchGuardWarnsOnFreshReadingOverThreshold(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.limitsReading(t, a, time.Minute, 92, f.cmd.now().Add(2*time.Hour))
	if err := f.run.checkHitchGuard(guardedClaude(t, false), "costly"); err != nil {
		t.Fatal(err)
	}
	got := f.errOut.String()
	for _, want := range []string{"warning: hitch:", `"costly"`, "worker's claude reading from 1m0s ago", "92% used (guard 90%)", "resets 2026-09-22T12:00:00Z", "another --model", "hitch_guard"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr %q lacks %q", got, want)
		}
	}
}

func TestHitchGuardRefusesOnlyWhenConfigured(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.limitsReading(t, a, time.Minute, 90, f.cmd.now().Add(2*time.Hour))
	err := f.run.checkHitchGuard(guardedClaude(t, true), "costly")
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), "90% used") {
		t.Fatalf("guarded hitch = %v", err)
	}
	if f.errOut.Len() != 0 {
		t.Fatalf("refusal also warned: %q", f.errOut.String())
	}
}

func TestHitchGuardNeverRefusesWithoutFreshReading(t *testing.T) {
	f := newStateFixture(t)
	if err := f.run.checkHitchGuard(guardedClaude(t, true), "costly"); err != nil {
		t.Fatalf("absent reading: %v", err)
	}
	a := f.add(t, "a", "worker", "claude")
	f.limitsReading(t, a, 6*time.Minute, 99, f.cmd.now().Add(time.Hour))
	if err := f.run.checkHitchGuard(guardedClaude(t, true), "costly"); err != nil {
		t.Fatalf("stale reading: %v", err)
	}
	if got := strings.Count(f.errOut.String(), "launching unchecked"); got != 2 {
		t.Fatalf("stderr = %q", f.errOut.String())
	}
}

func TestHitchGuardPassesBelowThresholdAndAfterReset(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	for _, reading := range []struct {
		used  float64
		reset time.Time
	}{{89, f.cmd.now().Add(time.Hour)}, {99, f.cmd.now()}} {
		f.limitsReading(t, a, time.Minute, reading.used, reading.reset)
		if err := f.run.checkHitchGuard(guardedClaude(t, true), "costly"); err != nil {
			t.Fatalf("%v%% resetting %s: %v", reading.used, reading.reset, err)
		}
	}
	if f.errOut.Len() != 0 {
		t.Fatalf("stderr = %q", f.errOut.String())
	}
}

func TestHitchGuardJudgesTheFreshestReading(t *testing.T) {
	// Both orders, so neither the first nor the last agent listed can win
	// by position.
	for _, newerOver := range []bool{false, true} {
		f := newStateFixture(t)
		first := f.add(t, "a", "first", "claude")
		second := f.add(t, "b", "second", "claude")
		older, newer := first, second
		if newerOver {
			older, newer = second, first
		}
		used := map[bool]float64{false: 10, true: 99}
		f.limitsReading(t, older, 2*time.Minute, used[!newerOver], f.cmd.now().Add(time.Hour))
		f.limitsReading(t, newer, time.Minute, used[newerOver], f.cmd.now().Add(time.Hour))
		err := f.run.checkHitchGuard(guardedClaude(t, true), "costly")
		if newerOver && (err == nil || !strings.Contains(err.Error(), string(newer.Name)+"'s")) || !newerOver && err != nil {
			t.Fatalf("newer reading over threshold = %v: %v", newerOver, err)
		}
	}
}

func TestHitchGuardHoldsEachWindowToItsOwnThreshold(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	at := f.cmd.now().Add(-time.Minute)
	reset := f.cmd.now().Add(time.Hour).Unix()
	f.setAgent(t, a, func(a *core.Agent) {
		a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{
			{Label: "five_hour", WindowMinutes: 300, UsedPercent: 99, ResetAt: reset},
			{Label: "seven_day", WindowMinutes: 10080, UsedPercent: 7, ResetAt: reset},
		}}
	})
	c := guardedClaude(t, true)
	weekly := 0.07
	c.HitchGuard.FiveHour, c.HitchGuard.Weekly = nil, &weekly
	err := f.run.checkHitchGuard(c, "costly")
	if err == nil || !strings.Contains(err.Error(), "seven_day 7% used (guard 7%)") || strings.Contains(err.Error(), "five_hour") {
		t.Fatalf("weekly guard = %v", err)
	}
}

func TestHitchGuardCountsOnlyObservedReadingsFromItsCollar(t *testing.T) {
	f := newStateFixture(t)
	f.limitsReading(t, f.add(t, "a", "other", "codex"), time.Minute, 99, f.cmd.now().Add(time.Hour))
	at := f.cmd.now().Add(-time.Minute)
	f.setAgent(t, f.add(t, "b", "failed", "claude"), func(a *core.Agent) {
		a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "unavailable", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", WindowMinutes: 300, UsedPercent: 99, ResetAt: f.cmd.now().Add(time.Hour).Unix()}}}
	})
	if err := f.run.checkHitchGuard(guardedClaude(t, true), "costly"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.errOut.String(), "launching unchecked") {
		t.Fatalf("stderr = %q", f.errOut.String())
	}
}

func TestHitchGuardCountsAReadingFiveMinutesOld(t *testing.T) {
	f := newStateFixture(t)
	f.limitsReading(t, f.add(t, "a", "worker", "claude"), hitchGuardFreshness, 99, f.cmd.now().Add(time.Hour))
	if err := f.run.checkHitchGuard(guardedClaude(t, true), "costly"); err == nil {
		t.Fatal("a reading at the freshness limit did not count")
	}
}

// guardedHitch runs hitch against a codex overlay whose native CLI answers the
// limits query with limits and whose guard names model "costly". The
// assignment reader is the barrier that proves the launch continued.
func guardedHitch(t *testing.T, refuse bool, limits string) (*resumeAssignmentReader, string, error) {
	t.Helper()
	directory := t.TempDir()
	executable := filepath.Join(directory, "codex")
	script := "#!/bin/sh\n[ \"$*\" = 'app-server --listen stdio://' ] || exit 80\nprintf '%s\\n' '{\"id\":1,\"result\":{}}' '" + limits + "'\ncat >/dev/null\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	overlay := "collar: {\n\tlaunch: command: " + strconv.Quote(executable) + "\n\thitch_guard: {models: [\"costly\"], five_hour: 0.9, refuse: " + strconv.FormatBool(refuse) + "}\n}\n"
	if err := os.WriteFile(filepath.Join(directory, "codex.cue"), []byte(overlay), 0600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GANG_STATE_ROOT": filepath.Join(directory, "state"),
		"GANG_CONFIG_DIR": filepath.Join(directory, "config"),
		"GANG_COLLARS":    directory,
		"GANG_SESSION":    "hitch-guard-test",
		"GANG_TMUX":       filepath.Join(directory, "must-not-run-tmux"),
	}
	reader := &resumeAssignmentReader{}
	var stderr bytes.Buffer
	cmd := command{
		stdin: reader, stdout: &bytes.Buffer{}, stderr: &stderr,
		getenv:      func(key string) string { return values[key] },
		lookupEnv:   func(key string) (string, bool) { value, ok := values[key]; return value, ok },
		getwd:       func() (string, error) { return directory, nil },
		userHomeDir: func() (string, error) { return directory, nil },
	}
	err := cmd.hitch([]string{"worker", "-c", "codex", "-m", "costly", "--stdin"})
	if _, statErr := os.Stat(values["GANG_STATE_ROOT"]); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("guard check created state: %v", statErr)
	}
	return reader, stderr.String(), err
}

const guardedFiveHour = `{"id":2,"result":{"rateLimits":{"limitId":"codex","primary":{"usedPercent":95,"windowDurationMins":300,"resetsAt":4102444800}}}}`

func TestHitchGuardQueriesProviderWhenNoAgentHoldsAReading(t *testing.T) {
	reader, stderr, err := guardedHitch(t, true, guardedFiveHour)
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), "codex limits query just now shows codex/primary 95% used (guard 90%), resets 2100-01-01T00:00:00Z") {
		t.Fatalf("guarded hitch = %v", err)
	}
	if reader.read {
		t.Fatal("refused hitch consumed its assignment")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	reader, stderr, err = guardedHitch(t, false, guardedFiveHour)
	if !passedPreflight(reader, err) || !strings.Contains(stderr, "warning: hitch: model \"costly\" is guarded") {
		t.Fatalf("warned hitch = %v; stderr = %q", err, stderr)
	}
}

func TestHitchGuardLaunchesWhenProviderQueryFails(t *testing.T) {
	reader, stderr, err := guardedHitch(t, true, `{"id":2,"error":{"message":"signed out"}}`)
	if !passedPreflight(reader, err) || !strings.Contains(stderr, "the provider limits query failed") || !strings.Contains(stderr, "launching unchecked") {
		t.Fatalf("hitch = %v; stderr = %q", err, stderr)
	}
}
