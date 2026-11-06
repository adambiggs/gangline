package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
)

// This projection deliberately has no free-text reason, envelope, prompt,
// native payload or compaction continuation fields. It is not an Event export.
type logMetadataUsage struct {
	Status       string           `json:"status"`
	Source       string           `json:"source"`
	SessionID    string           `json:"session_id,omitempty"`
	SourceOffset int64            `json:"source_offset,omitempty"`
	SourceAt     *time.Time       `json:"source_at,omitempty"`
	Usage        *core.TokenUsage `json:"usage,omitempty"`
}

type logMetadataBoundary struct {
	Kind         string     `json:"kind"`
	Source       string     `json:"source"`
	SessionID    string     `json:"session_id,omitempty"`
	SourceOffset int64      `json:"source_offset,omitempty"`
	SourceAt     *time.Time `json:"source_at,omitempty"`
}

type logMetadataEvent struct {
	Offset           int64                 `json:"offset"`
	End              int64                 `json:"end_offset"`
	Type             string                `json:"type"`
	At               time.Time             `json:"observed_at"`
	HitchID          core.HitchID          `json:"hitch_id,omitempty"`
	Name             core.AgentName        `json:"name,omitempty"`
	Activity         core.Activity         `json:"activity,omitempty"`
	Binding          *core.Binding         `json:"binding,omitempty"`
	Usage            []logMetadataUsage    `json:"usage,omitempty"`
	NativeBoundaries []logMetadataBoundary `json:"native_boundaries,omitempty"`
}

type logMetadataGap struct {
	Offset int64  `json:"offset"`
	Kind   string `json:"kind"`
}

type logMetadataPage struct {
	UnknownEvents int                `json:"unknown_event_count"`
	Schema        string             `json:"schema"`
	ReadAt        time.Time          `json:"read_at"`
	Size          int64              `json:"file_size_at_open"`
	Offset        int64              `json:"offset"`
	ReadEnd       int64              `json:"read_end"`
	Next          int64              `json:"next_offset"`
	SHA256        string             `json:"read_bytes_sha256"`
	More          bool               `json:"more"`
	Events        []logMetadataEvent `json:"events"`
	Gaps          []logMetadataGap   `json:"gaps"`
	Limitations   []string           `json:"limitations"`
}

func (cmd command) logMetadata(path string, filter logFilter) error {
	// A named FIFO must not block a metadata read or become a new stream route.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("log --metadata requires a regular audit file")
	}
	page, err := readLogMetadata(f, info.Size(), filter, cmd.now())
	if err != nil {
		return err
	}
	if filter.Summary {
		return writeJSON(cmd.stdout, summarizeLogMetadata(page))
	}
	return writeJSON(cmd.stdout, page)
}

// Read only this page and one boundary byte. A page is independently evidenced;
// its offsets/hash do not establish identity or continuity with another file.
func readLogMetadata(in io.ReaderAt, size int64, filter logFilter, now time.Time) (logMetadataPage, error) {
	if filter.Summary {
		filter.Offset = max(0, size-filter.MaxBytes)
	}
	page := logMetadataPage{Schema: "gangline.observability-log.v1", ReadAt: now, Size: size,
		Offset: filter.Offset, Next: filter.Offset, Events: []logMetadataEvent{}, Gaps: []logMetadataGap{},
		Limitations: []string{"host_and_team_scope_not_in_file", "cross_page_continuity_unverified",
			"activity_is_observation_not_compute_time", "counters_are_not_additive", "task_attribution_unavailable"}}
	if filter.Offset > size {
		return page, fmt.Errorf("log --metadata: offset exceeds file size; source may have been truncated or replaced")
	}
	partialStart := false
	if filter.Offset > 0 {
		var previous [1]byte
		if _, err := in.ReadAt(previous[:], filter.Offset-1); err != nil {
			return page, fmt.Errorf("log --metadata: cannot read page boundary")
		}
		if previous[0] != '\n' {
			if !filter.Summary {
				return page, fmt.Errorf("log --metadata: offset is not a record boundary")
			}
			partialStart = true
		}
	}
	count := min(filter.MaxBytes, size-filter.Offset)
	data := make([]byte, count)
	n, err := in.ReadAt(data, filter.Offset)
	if n != len(data) || (err != nil && err != io.EOF) {
		return page, fmt.Errorf("log --metadata: audit changed or could not be read")
	}
	page.ReadEnd = filter.Offset + int64(n)
	page.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	cursor := 0
	if partialStart {
		page.Gaps = append(page.Gaps, logMetadataGap{filter.Offset, "tail_starts_inside_record"})
		if newline := bytes.IndexByte(data, '\n'); newline >= 0 {
			cursor = newline + 1
			page.Next = filter.Offset + int64(cursor)
		} else {
			cursor = len(data)
		}
	}
	var prior time.Time
	continuity := newLogUsageContinuity()
	for cursor < len(data) {
		newline := bytes.IndexByte(data[cursor:], '\n')
		if newline < 0 {
			kind := "incomplete_tail"
			if page.ReadEnd < size {
				kind = "page_ends_inside_record"
				if cursor == 0 {
					kind = "record_exceeds_budget"
				}
			}
			page.Gaps = append(page.Gaps, logMetadataGap{filter.Offset + int64(cursor), kind})
			break
		}
		end := cursor + newline + 1
		startOffset := filter.Offset + int64(cursor)
		page.Next = filter.Offset + int64(end)
		e, err := core.DecodeEvent(data[cursor : end-1])
		cursor = end
		if errors.Is(err, core.ErrUnknownEventType) {
			page.UnknownEvents++
			page.Gaps = append(page.Gaps, logMetadataGap{startOffset, "unknown_event_kind"})
			continue
		}
		if err != nil {
			// Never relay decoder errors: they can contain rejected source text.
			page.Gaps = append(page.Gaps, logMetadataGap{startOffset, "invalid_audit_record"})
			continue
		}
		if !prior.IsZero() && e.At.Before(prior) {
			page.Gaps = append(page.Gaps, logMetadataGap{startOffset, "observation_time_out_of_order"})
		}
		prior = e.At
		if filtered, ok := filter.apply(e); ok {
			if projection, ok := projectLogMetadata(filtered); ok {
				projection.Offset, projection.End = startOffset, page.Next
				page.Gaps = append(page.Gaps, continuity.observe(projection)...)
				page.Events = append(page.Events, projection)
			}
		}
	}
	page.More = page.Next < size
	return page, nil
}

func projectLogMetadata(e core.Event) (logMetadataEvent, bool) {
	p := logMetadataEvent{Type: e.Type, At: e.At, HitchID: e.HitchID, Name: e.Name}
	switch e.Type {
	case "observation":
		for _, r := range e.Readings {
			if r.Source == "session-log" {
				kind := ""
				switch r.Kind {
				case "compaction-checkpoint", "compaction-finished":
					kind = r.Kind
				case "observation-gap":
					if r.NativeEvent == "source-window-skipped" || r.NativeEvent == "source-context-mismatch" {
						kind = r.NativeEvent
					}
				}
				if kind != "" {
					p.NativeBoundaries = append(p.NativeBoundaries, logMetadataBoundary{kind, "session-log", r.SessionID, r.SourceOffset, r.At})
				}
			}
			if r.Kind != "token-usage" {
				continue
			}
			u := logMetadataUsage{Status: "unknown", Source: "unknown", SessionID: r.SessionID,
				SourceOffset: r.SourceOffset, SourceAt: r.At}
			if r.Source == "status-line" || r.Source == "session-log" {
				u.Source = r.Source
			}
			if r.Status == "observed" || r.Status == "partial" || r.Status == "unknown" {
				u.Status = r.Status
			}
			if r.Usage != nil {
				copy := *r.Usage
				if copy.Fields != "info.total_token_usage" && copy.Fields != "context_window.total_input_tokens,total_output_tokens" {
					copy.Fields = "unknown"
				}
				u.Usage = &copy
			}
			p.Usage = append(p.Usage, u)
		}
		return p, len(p.Usage) > 0 || len(p.NativeBoundaries) > 0
	case "activity_observed":
		p.Activity = e.Activity
	case "session_bound":
		p.Binding = e.Binding
	case "hitch_claimed", "hitch_ready", "hitch_failed", "drop_started", "drop_finished",
		"clear_requested", "clear_completed", "clear_unverified", "rehitch_started", "rehitch_failed",
		"compaction_requested", "compaction_completed", "compaction_failed", "compaction_unverified",
		"watchdog_available", "watchdog_unavailable", "watchdog_failed", "tick_failed",
		"input_observation_outage", "input_observation_recovered":
	default:
		return p, false
	}
	return p, true
}
