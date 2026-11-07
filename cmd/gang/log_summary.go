package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/adambiggs/gangline/core"
)

type logSummaryUsage struct {
	ObservedAt time.Time        `json:"observed_at"`
	Offset     int64            `json:"offset"`
	End        int64            `json:"end_offset"`
	Reading    logMetadataUsage `json:"reading"`
}

type logSummarySubject struct {
	HitchID        core.HitchID       `json:"hitch_id"`
	Name           core.AgentName     `json:"name,omitempty"`
	Activity       *logMetadataEvent  `json:"last_activity_observation"`
	Usage          []logSummaryUsage  `json:"last_usage_observations"`
	WindowUsage    *core.TokenUsage   `json:"window_usage"`
	StateDuration  *float64           `json:"state_duration_seconds"`
	Window         *logSummaryWindow  `json:"projected_observation_window"`
	StateIntervals []logStateInterval `json:"state_intervals"`
	coverageCursor logCoverageCursor
}

type logSummary struct {
	Schema      string              `json:"schema"`
	ReadAt      time.Time           `json:"read_at"`
	Size        int64               `json:"file_size_at_open"`
	Offset      int64               `json:"offset"`
	ReadEnd     int64               `json:"read_end"`
	Next        int64               `json:"next_offset"`
	SHA256      string              `json:"read_bytes_sha256"`
	OmittedHead bool                `json:"omitted_head"`
	More        bool                `json:"more"`
	Subjects    []logSummarySubject `json:"subjects"`
	Boundaries  []logMetadataEvent  `json:"boundaries"`
	Gaps        []logMetadataGap    `json:"gaps"`
	Limitations []string            `json:"limitations"`
}

// Summarize one bounded read, never extend a state to read time or add cumulative
// counters. These are the last records in append order, not a current roster.
// Retaining each source/session separately avoids inventing a current binding.
func summarizeLogMetadata(p logMetadataPage) logSummary {
	s := logSummary{Schema: "gangline.observability-summary.v1", ReadAt: p.ReadAt,
		Size: p.Size, Offset: p.Offset, ReadEnd: p.ReadEnd, Next: p.Next, SHA256: p.SHA256,
		OmittedHead: p.Offset > 0, More: p.More, Subjects: []logSummarySubject{},
		Boundaries: []logMetadataEvent{}, Gaps: p.Gaps,
		Limitations: append(append([]string{}, p.Limitations...),
			"last_records_in_append_order_not_current_roster", "audit_silence_is_not_freshness",
			"window_usage_and_state_durations_unknown", "session_and_source_groups_not_additive",
			"state_intervals_are_discrete_observation_pairs_not_state_durations",
			"before_first_and_after_last_observations_unknown")}
	subjects := map[core.HitchID]*logSummarySubject{}
	usage := map[core.HitchID]map[string]logSummaryUsage{}
	seen := map[logUsageCursor]map[string]bool{}
	gapIndex, globalBoundaries := 0, 0
	for _, e := range p.Events {
		for gapIndex < len(p.Gaps) && p.Gaps[gapIndex].Offset <= e.Offset {
			gapIndex++
		}
		if isLogSummaryBoundary(e) {
			s.Boundaries = append(s.Boundaries, e)
		}
		if e.HitchID == "" {
			if isLogSummaryBoundary(e) {
				globalBoundaries++
			}
			continue
		}
		a := subjects[e.HitchID]
		if a == nil {
			a = &logSummarySubject{HitchID: e.HitchID, Usage: []logSummaryUsage{}, StateIntervals: []logStateInterval{}}
			subjects[e.HitchID] = a
			usage[e.HitchID] = map[string]logSummaryUsage{}
		}
		a.observeWindow(e, gapIndex, globalBoundaries)
		if e.Name != "" {
			a.Name = e.Name
		}
		if e.Type == "activity_observed" {
			copy := e
			a.Activity = &copy
		}
		for i, r := range e.Usage {
			if r.Source == "session-log" && r.SessionID != "" && r.SourceOffset > 0 {
				cursor := logUsageCursor{logUsageIdentity{e.HitchID, r.Source, r.SessionID}, r.SourceOffset}
				if encoded, err := json.Marshal(r); err == nil {
					key := string(encoded)
					if seen[cursor][key] {
						// A failed state save can append an older source record
						// again. It must not replace a newer retained reading,
						// including a replay of an earlier conflicting record.
						continue
					}
					if seen[cursor] == nil {
						seen[cursor] = map[string]bool{}
					}
					seen[cursor][key] = true
				}
			}
			key := r.Source + "\x00" + r.SessionID
			// Missing identity must not collapse unrelated unknown readings.
			if r.SessionID == "" || r.Source == "unknown" {
				key = fmt.Sprintf("unknown:%d:%d", e.Offset, i)
			}
			usage[e.HitchID][key] = logSummaryUsage{ObservedAt: e.At, Offset: e.Offset, End: e.End, Reading: r}
		}
	}
	ids := make([]string, 0, len(subjects))
	for id := range subjects {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		a := subjects[core.HitchID(id)]
		a.finishCoverage(p.ReadAt)
		keys := make([]string, 0, len(usage[a.HitchID]))
		for key := range usage[a.HitchID] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			a.Usage = append(a.Usage, usage[a.HitchID][key])
		}
		s.Subjects = append(s.Subjects, *a)
	}
	return s
}
