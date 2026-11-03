package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func coverageActivity(at, activity string) string {
	return fmt.Sprintf(`{"type":"activity_observed","at":%q,"hitch_id":"h1","activity":%q,"reason":"PRIVATE"}`+"\n", at, activity)
}

func coverageDocument(t *testing.T, data string, budget int64) map[string]any {
	t.Helper()
	page := summaryPage(t, data, budget)
	for i := 1; i < len(page.Gaps); i++ {
		if page.Gaps[i].Offset < page.Gaps[i-1].Offset {
			t.Fatal("reader gap offsets are not ordered for the single-pass summary")
		}
	}
	b, err := json.Marshal(summarizeLogMetadata(page))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "PRIVATE") {
		t.Fatal("excluded text leaked")
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func coverageSubject(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	subjects := doc["subjects"].([]any)
	if len(subjects) != 1 {
		t.Fatalf("wanted one subject, got %d", len(subjects))
	}
	return subjects[0].(map[string]any)
}

func TestLogSummaryCoverageExposesUnknownStateIntervals(t *testing.T) {
	data := coverageActivity("2026-11-03T00:00:00Z", "busy") +
		coverageActivity("2026-11-03T00:00:30Z", "blocked") +
		coverageActivity("2026-11-03T00:01:00Z", "idle")
	a := coverageSubject(t, coverageDocument(t, data, int64(len(data))))
	w, ok := a["projected_observation_window"].(map[string]any)
	if !ok {
		t.Fatal("missing per-agent observation window and coverage")
	}
	if w["coverage"] != "partial" || w["freshness"] != "unknown" ||
		w["before_first"] != "unknown" || w["after_last"] != "unknown" ||
		w["first_projected_observed_at"] != "2026-11-03T00:00:00Z" || w["last_projected_observed_at"] != "2026-11-03T00:01:00Z" {
		t.Fatalf("wrong coverage: %#v", w)
	}
	intervals := a["state_intervals"].([]any)
	if len(intervals) != 2 {
		t.Fatalf("intermediate activity discarded: %d intervals", len(intervals))
	}
	for _, raw := range intervals {
		x := raw.(map[string]any)
		if x["observation_spacing_seconds"] != float64(30) || x["state_duration_seconds"] != nil || x["coverage"] != "unknown" || x["reason"] != "discrete_observations_only" {
			t.Fatalf("observation spacing promoted to state duration: %#v", x)
		}
	}
	if a["window_usage"] != nil || a["state_duration_seconds"] != nil {
		t.Fatal("invented aggregate")
	}
}

func TestLogSummaryCoveragePreservesCounterSemanticsAndSourceTime(t *testing.T) {
	line := strings.Replace(metadataUsage, `"source":"status-line"`, `"source":"status-line","at":"2026-11-02T23:00:00Z"`, 1) + "\n"
	data := strings.Repeat(line, 20)
	a := coverageSubject(t, coverageDocument(t, data, int64(len(data))))
	entries := a["last_usage_observations"].([]any)
	if len(entries) != 1 {
		t.Fatal("repeated counters multiplied")
	}
	u := entries[0].(map[string]any)
	if u["observed_at"] != "2026-11-03T00:00:00Z" {
		t.Fatal("age replaced observation time or used source time")
	}
	reading := u["reading"].(map[string]any)
	if reading["source_at"] != "2026-11-02T23:00:00Z" || reading["session_id"] != "s1" {
		t.Fatal("source identity/time lost")
	}
	counters := reading["usage"].(map[string]any)
	if counters["semantics"] != "session-cumulative" || counters["cache_inclusion"] != "unverified" || counters["input_tokens"] != nil || counters["output_tokens"] != float64(0) {
		t.Fatal("counter semantics changed")
	}
	if a["window_usage"] != nil || a["state_duration_seconds"] != nil || len(a["state_intervals"].([]any)) != 0 {
		t.Fatal("usage manufactured activity or totals")
	}
}

func TestLogSummaryCoverageDisorderAndFutureDoNotInventTime(t *testing.T) {
	for name, middle := range map[string]string{
		"subject rewind":            coverageActivity("2026-11-02T23:59:00Z", "blocked"),
		"other subject rewind":      strings.ReplaceAll(coverageActivity("2026-11-02T23:59:00Z", "blocked"), `"h1"`, `"h2"`),
		"unprojected record rewind": `{"type":"tick","source":"watchdog","at":"2026-11-02T23:59:00Z"}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			data := coverageActivity("2026-11-03T00:00:00Z", "busy") + middle + coverageActivity("2026-11-03T00:00:30Z", "idle")
			doc := coverageDocument(t, data, int64(len(data)))
			a := doc["subjects"].([]any)[0].(map[string]any)
			if a["projected_observation_window"].(map[string]any)["observation_times_ordered"] != (name != "subject rewind") {
				t.Fatal("timestamp disorder hidden")
			}
			for _, raw := range a["state_intervals"].([]any) {
				x := raw.(map[string]any)
				if (name == "subject rewind" && x["observation_spacing_seconds"] != nil) || x["state_duration_seconds"] != nil || x["coverage"] != "unknown" {
					t.Fatal("disordered intervals yielded time")
				}
			}
		})
	}
	data := coverageActivity("2026-11-03T02:00:00Z", "busy")
	a := coverageSubject(t, coverageDocument(t, data, int64(len(data))))
	if a["projected_observation_window"].(map[string]any)["freshness"] != "unknown" {
		t.Fatal("future timestamp became fresh")
	}
	futureUsage := strings.Replace(metadataUsage, "T00:00:00Z", "T02:00:00Z", 1) + "\n"
	a = coverageSubject(t, coverageDocument(t, futureUsage, int64(len(futureUsage))))
	if _, ok := a["last_usage_observations"].([]any)[0].(map[string]any)["observation_age_seconds"]; ok {
		t.Fatal("future counter has negative age")
	}
}

func TestLogSummaryCoverageBoundariesAndDamageStayUnknown(t *testing.T) {
	for name, middle := range map[string]string{
		"malformed":     "PRIVATE_BAD\n",
		"outage":        `{"type":"watchdog_unavailable","at":"2026-11-03T00:00:10Z","reason":"PRIVATE_OUTAGE"}` + "\n",
		"restart":       `{"type":"rehitch_started","at":"2026-11-03T00:00:10Z","hitch_id":"h1"}` + "\n",
		"session reset": `{"type":"session_bound","at":"2026-11-03T00:00:10Z","hitch_id":"h1","name":"fixture","binding":{"session_id":"s2","previous_session_id":"s1","collar":"codex"}}` + "\n",
		"native gap":    `{"type":"observation","at":"2026-11-03T00:00:10Z","hitch_id":"h1","readings":[{"kind":"observation-gap","source":"session-log","status":"unknown","session_id":"s1","source_offset":200,"native_event":"source-window-skipped"}]}` + "\n",
		"compaction":    `{"type":"observation","at":"2026-11-03T00:00:10Z","hitch_id":"h1","readings":[{"kind":"compaction-checkpoint","source":"session-log","status":"observed","session_id":"s1","source_offset":200}]}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			data := coverageActivity("2026-11-03T00:00:00Z", "busy") + middle + coverageActivity("2026-11-03T00:00:30Z", "idle")
			doc := coverageDocument(t, data, int64(len(data)))
			a := coverageSubject(t, doc)
			x := a["state_intervals"].([]any)[0].(map[string]any)
			if x["coverage"] != "unknown" || x["state_duration_seconds"] != nil || a["state_duration_seconds"] != nil {
				t.Fatal("gap bridged with a state duration")
			}
			wantGaps, wantBoundaries := 0, 1
			if name == "malformed" {
				wantGaps, wantBoundaries = 1, 0
			}
			if name == "native gap" {
				wantGaps = 1
			}
			if len(doc["gaps"].([]any)) != wantGaps || len(doc["boundaries"].([]any)) != wantBoundaries ||
				x["intervening_gap_records"] != float64(wantGaps) || x["intervening_projected_boundaries"] != float64(wantBoundaries) ||
				x["observation_spacing_seconds"] != nil {
				t.Fatalf("fixture boundary not decoded or counted as intended: gaps=%#v boundaries=%#v pair=%#v", doc["gaps"], doc["boundaries"], x)
			}
		})
	}
}

func TestLogSummaryCoverageDoesNotScanOmittedHistoryOrExtendTail(t *testing.T) {
	first := coverageActivity("2026-11-03T00:00:00Z", "busy")
	last := coverageActivity("2026-11-03T00:01:00Z", "idle")
	// An omitted observation never supplies the first endpoint. summaryPage
	// asserts the actual ReaderAt byte budget including the boundary byte.
	data := strings.Repeat(first, 2000) + last + "PRIVATE_TAIL"
	doc := coverageDocument(t, data, int64(len(last)+len("PRIVATE_TAIL")))
	a := coverageSubject(t, doc)
	if doc["omitted_head"] != true || doc["more"] != true || len(doc["gaps"].([]any)) != 1 {
		t.Fatal("tail truncation/omission hidden")
	}
	if len(a["state_intervals"].([]any)) != 0 || a["state_duration_seconds"] != nil {
		t.Fatal("single activity extended to read time or omitted history")
	}
	if a["projected_observation_window"].(map[string]any)["first_offset"] != float64(len(data)-len(last)-len("PRIVATE_TAIL")) {
		t.Fatal("window borrowed omitted observation")
	}
	empty := coverageDocument(t, "", 64)
	if len(empty["subjects"].([]any)) != 0 {
		t.Fatal("empty history became idle")
	}
	// Lifecycle-only subjects must not acquire activity or durations.
	boundary := `{"type":"drop_finished","at":"2026-11-03T00:01:00Z","hitch_id":"h1"}` + "\n"
	a = coverageSubject(t, coverageDocument(t, boundary, int64(len(boundary))))
	if a["last_activity_observation"] != nil || len(a["state_intervals"].([]any)) != 0 {
		t.Fatal("boundary became activity")
	}
}

func TestLogSummaryCoverageInterveningBreaksAreOnThePair(t *testing.T) {
	data := coverageActivity("2026-11-03T00:00:00Z", "busy") +
		`{"type":"clear_completed","at":"2026-11-03T00:00:10Z","hitch_id":"h1"}` + "\n" +
		"PRIVATE_DAMAGE\n" + coverageActivity("2026-11-03T00:00:30Z", "idle")
	a := coverageSubject(t, coverageDocument(t, data, int64(len(data))))
	x := a["state_intervals"].([]any)[0].(map[string]any)
	if x["intervening_projected_boundaries"] != float64(1) || x["intervening_gap_records"] != float64(1) {
		t.Fatalf("pair conceals its intervening break: %#v", x)
	}
}

func TestLogSummaryCoverageMaximumPageAndOutputDependOnWindow(t *testing.T) {
	line := strings.Replace(coverageActivity("2026-11-03T00:00:00Z", "busy"), `,"reason":"PRIVATE"`, "", 1)
	const budget = 1024 * 1024
	data := strings.Repeat(line, 3*budget/len(line))
	p := summaryPage(t, data, budget)
	s := summarizeLogMetadata(p)
	if !s.OmittedHead || len(s.Subjects) != 1 {
		t.Fatal("maximum page expanded to history")
	}
	a := s.Subjects[0]
	if a.Window.Records != len(p.Events) || len(a.StateIntervals) != len(p.Events)-1 || a.Window.Records > budget/len(line) {
		t.Fatal("output did not follow selected projected records")
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	// A fixture output bound detects accidental history duplication; it is not
	// a general response-size contract (the byte option bounds source reads).
	t.Logf("selected bytes=%d response bytes=%d ratio=%.3f", budget, len(encoded), float64(len(encoded))/budget)
	if len(encoded) > 9*budget/2+4096 {
		t.Fatalf("fixture output expanded unexpectedly: %d", len(encoded))
	}
	for _, x := range a.StateIntervals {
		if x.Spacing != nil || x.StateDuration != nil {
			t.Fatal("equal timestamp repeats became duration")
		}
	}
}

func TestLogSummaryCoverageSkipsUnprojectedWindowRecords(t *testing.T) {
	data := coverageActivity("2026-11-03T00:00:00Z", "busy") +
		`{"type":"native_hook","at":"2026-11-03T00:00:30Z","hitch_id":"h1"}` + "\n"
	a := coverageSubject(t, coverageDocument(t, data, int64(len(data))))
	w := a["projected_observation_window"].(map[string]any)
	if w["projected_records"] != float64(1) || w["last_projected_observed_at"] != "2026-11-03T00:00:00Z" {
		t.Fatalf("unprojected append became a window endpoint: %#v", w)
	}
	if _, exists := a["last_activity_age_seconds"]; exists {
		t.Fatal("redundant age field retained")
	}
}
