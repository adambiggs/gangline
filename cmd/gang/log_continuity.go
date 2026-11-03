package main

import (
	"reflect"

	"github.com/adambiggs/gangline/core"
)

type logUsageIdentity struct {
	Hitch           core.HitchID
	Source, Session string
}

type logUsageCursor struct {
	Identity logUsageIdentity
	Offset   int64
}

// Comparisons are local to this bounded page. Neither increasing counters nor
// a missing marker proves continuity, and no counter is added or subtracted.
type logUsageContinuity struct {
	last map[logUsageIdentity]logMetadataUsage
	seen map[logUsageCursor]logMetadataUsage
}

func newLogUsageContinuity() logUsageContinuity {
	return logUsageContinuity{map[logUsageIdentity]logMetadataUsage{}, map[logUsageCursor]logMetadataUsage{}}
}

func (c logUsageContinuity) observe(e logMetadataEvent) []logMetadataGap {
	var gaps []logMetadataGap
	for _, b := range e.NativeBoundaries {
		switch b.Kind {
		case "source-window-skipped":
			gaps = append(gaps, logMetadataGap{e.Offset, "source_window_skipped"})
		case "source-context-mismatch":
			gaps = append(gaps, logMetadataGap{e.Offset, "source_context_mismatch"})
		}
	}
	for _, r := range e.Usage {
		if e.HitchID == "" || r.SessionID == "" || r.Source == "unknown" {
			continue
		}
		key := logUsageIdentity{e.HitchID, r.Source, r.SessionID}
		// Unknown fields never become zero or establish a comparison baseline.
		if r.Usage == nil || r.Status == "unknown" || r.Usage.Fields == "unknown" {
			delete(c.last, key)
			continue
		}
		kind := ""
		if r.Source == "session-log" && r.SourceOffset > 0 {
			cursor := logUsageCursor{key, r.SourceOffset}
			if old, ok := c.seen[cursor]; ok {
				if reflect.DeepEqual(old, r) {
					// A replay after a failed state save is not a reset, even
					// if later source records were already retained.
					continue
				}
				kind = "source_offset_conflict"
			}
			c.seen[cursor] = r
		}
		if old, ok := c.last[key]; ok && kind == "" {
			switch {
			case r.Source == "session-log" && r.SourceOffset > 0 && old.SourceOffset > r.SourceOffset:
				kind = "source_offset_rewind"
			case old.Usage.Fields == r.Usage.Fields && countersDecrease(old.Usage, r.Usage):
				kind = "cumulative_counter_decreased"
			}
		}
		if kind != "" {
			gaps = append(gaps, logMetadataGap{e.Offset, kind})
		}
		c.last[key] = r
	}
	return gaps
}

func countersDecrease(old, current *core.TokenUsage) bool {
	less := func(a, b *int64) bool { return a != nil && b != nil && *b < *a }
	return less(old.Input, current.Input) || less(old.Output, current.Output) || less(old.CachedInput, current.CachedInput)
}
