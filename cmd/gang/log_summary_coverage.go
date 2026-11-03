package main

import (
	"github.com/adambiggs/gangline/core"
	"time"
)

// A change-only audit establishes observation points, not continuous state.
// The window includes only this hitch's projected records, not all appends.
type logSummaryWindow struct {
	FirstAt     time.Time `json:"first_projected_observed_at"`
	LastAt      time.Time `json:"last_projected_observed_at"`
	FirstOffset int64     `json:"first_offset"`
	LastOffset  int64     `json:"last_offset"`
	Records     int       `json:"projected_records"`
	Ordered     bool      `json:"observation_times_ordered"`
	Coverage    string    `json:"coverage"`
	Freshness   string    `json:"freshness"`
	BeforeFirst string    `json:"before_first"`
	AfterLast   string    `json:"after_last"`
}

type logActivityPoint struct {
	Offset   int64         `json:"offset"`
	End      int64         `json:"end_offset"`
	At       time.Time     `json:"observed_at"`
	Activity core.Activity `json:"activity"`
}

func activityPoint(e logMetadataEvent) logActivityPoint {
	return logActivityPoint{e.Offset, e.End, e.At, e.Activity}
}

type logStateInterval struct {
	From          logActivityPoint `json:"from"`
	To            logActivityPoint `json:"to"`
	Spacing       *float64         `json:"observation_spacing_seconds"`
	StateDuration *float64         `json:"state_duration_seconds"`
	Boundaries    int              `json:"intervening_projected_boundaries"`
	Gaps          int              `json:"intervening_gap_records"`
	Coverage      string           `json:"coverage"`
	Reason        string           `json:"reason"`
}

type logCoverageCursor struct {
	boundaries               int
	activityBoundaries       int
	activityGlobalBoundaries int
	activityGaps             int
}

func isLogSummaryBoundary(e logMetadataEvent) bool {
	return e.Type != "observation" && e.Type != "activity_observed" || len(e.NativeBoundaries) > 0
}

func (a *logSummarySubject) observeWindow(e logMetadataEvent, gaps, globalBoundaries int) {
	if a.Window == nil {
		a.Window = &logSummaryWindow{FirstAt: e.At, FirstOffset: e.Offset,
			Ordered: !e.At.IsZero(), Coverage: "partial", Freshness: "unknown",
			BeforeFirst: "unknown", AfterLast: "unknown"}
	} else if e.At.IsZero() || e.At.Before(a.Window.LastAt) {
		a.Window.Ordered = false
	}
	a.Window.LastAt, a.Window.LastOffset = e.At, e.Offset
	a.Window.Records++
	c := &a.coverageCursor
	if isLogSummaryBoundary(e) {
		c.boundaries++
	}
	if e.Type != "activity_observed" {
		return
	}
	if a.Activity != nil {
		a.StateIntervals = append(a.StateIntervals, logStateInterval{From: activityPoint(*a.Activity), To: activityPoint(e),
			Boundaries: c.boundaries - c.activityBoundaries + globalBoundaries - c.activityGlobalBoundaries,
			Gaps:       gaps - c.activityGaps, Coverage: "unknown", Reason: "discrete_observations_only"})
	}
	c.activityGaps, c.activityBoundaries, c.activityGlobalBoundaries = gaps, c.boundaries, globalBoundaries
}

func (a *logSummarySubject) finishCoverage(now time.Time) {
	for i := range a.StateIntervals {
		x := &a.StateIntervals[i]
		switch {
		case !a.Window.Ordered || x.From.At.IsZero() || !x.To.At.After(x.From.At):
			x.Reason = "observation_order_unverified"
		case now.IsZero() || x.To.At.After(now):
			// Reader and writer clocks may differ. Do not diagnose clock failure.
			x.Reason = "endpoint_after_read_time"
		case x.Boundaries > 0 || x.Gaps > 0:
			x.Reason = "intervening_gap_or_boundary"
		default:
			// Avoid time.Duration saturation on distant but valid timestamps.
			seconds := float64(x.To.At.Unix()-x.From.At.Unix()) + float64(x.To.At.Nanosecond()-x.From.At.Nanosecond())/1e9
			x.Spacing = &seconds
		}
	}
}
