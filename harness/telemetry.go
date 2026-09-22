package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Reading is a native observation, not a lifecycle transition. At is absent
// when the native payload supplies no measurement timestamp.
type Reading struct {
	Kind        string        `json:"kind"`
	Source      string        `json:"source"`
	NativeEvent string        `json:"native_event,omitempty"`
	At          *time.Time    `json:"at,omitempty"`
	Status      string        `json:"status"`
	Reason      string        `json:"reason,omitempty"`
	Model       string        `json:"model,omitempty"`
	Used        *int64        `json:"used,omitempty"`
	Limit       *int64        `json:"limit,omitempty"`
	Percent     *float64      `json:"percent,omitempty"`
	Limits      []LimitWindow `json:"limits,omitempty"`
}

type LimitWindow struct {
	Label       string  `json:"label"`
	UsedPercent float64 `json:"used_percent"`
	ResetAt     int64   `json:"reset_at"`
}

func UnknownReading(kind, source, reason string) Reading {
	return Reading{Kind: kind, Source: source, Status: "unknown", Reason: reason}
}

func contextReading(source string, used, limit int64) (Reading, error) {
	if used < 0 || limit <= 0 {
		return Reading{}, fmt.Errorf("%s context has invalid used/limit: %d/%d", source, used, limit)
	}
	percent := float64(used) / float64(limit) * 100
	return Reading{Kind: "context", Source: source, Status: "observed", Used: &used, Limit: &limit, Percent: &percent}, nil
}

// ReadStatusline consumes Claude's native stdin payload. Input/cache tokens
// occupy the prompt context; cumulative session totals and output do not.
func ReadStatusline(data []byte) (string, []Reading, error) {
	var p struct {
		SessionID string `json:"session_id"`
		Model     struct {
			ID string `json:"id"`
		} `json:"model"`
		Context *struct {
			Limit *int64 `json:"context_window_size"`
			Usage *struct {
				Input    *int64 `json:"input_tokens"`
				Creation *int64 `json:"cache_creation_input_tokens"`
				Read     *int64 `json:"cache_read_input_tokens"`
			} `json:"current_usage"`
		} `json:"context_window"`
		Limits map[string]*struct {
			Used  *float64 `json:"used_percentage"`
			Reset *int64   `json:"resets_at"`
		} `json:"rate_limits"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return "", nil, fmt.Errorf("decode status-line payload: %w", err)
	}
	if p.SessionID == "" {
		return "", nil, fmt.Errorf("status-line payload carries no session_id")
	}
	r := UnknownReading("context", "status-line", "native status-line has no current_usage yet")
	if p.Context == nil {
		return "", nil, fmt.Errorf("status-line payload carries no context_window")
	}
	if p.Context.Usage != nil {
		u := p.Context.Usage
		if p.Context.Limit == nil || u.Input == nil || u.Creation == nil || u.Read == nil {
			return "", nil, fmt.Errorf("status-line current_usage lacks input/cache tokens or context_window_size")
		}
		if *u.Input < 0 || *u.Creation < 0 || *u.Read < 0 {
			return "", nil, fmt.Errorf("status-line context has negative tokens")
		}
		var err error
		r, err = contextReading("status-line", *u.Input+*u.Creation+*u.Read, *p.Context.Limit)
		if err != nil {
			return "", nil, err
		}
	}
	r.Model = p.Model.ID
	limits := UnknownReading("provider-limits", "status-line", "native status-line exposes no provider limits")
	for _, label := range []string{"five_hour", "seven_day", "spend_limit"} {
		w := p.Limits[label]
		if w == nil {
			continue
		}
		if w.Used == nil || w.Reset == nil || *w.Used < 0 || *w.Reset <= 0 {
			return "", nil, fmt.Errorf("invalid status-line provider limit %s", label)
		}
		limits.Limits = append(limits.Limits, LimitWindow{Label: label, UsedPercent: *w.Used, ResetAt: *w.Reset})
	}
	if len(limits.Limits) > 0 {
		limits.Status = "observed"
		limits.Reason = ""
	}
	return p.SessionID, []Reading{r, limits}, nil
}

type Transcript struct {
	Offset   int64
	Readings []Reading
}

// ReadTranscript verifies the native session binding and consumes only complete
// records after offset. before excludes history from earlier hitches on resume.
func ReadTranscript(invocation Invocation, input io.ReadSeeker, session string, offset int64, before time.Time) (Transcript, error) {
	if invocation.Name != "codex-session-log" {
		return Transcript{}, fmt.Errorf("unknown transcript primitive %q", invocation.Name)
	}
	if session == "" {
		return Transcript{}, fmt.Errorf("session log requires a native session_id")
	}
	headerEnd, err := transcriptHeader(input, session)
	if err != nil {
		return Transcript{}, err
	}
	size, err := input.Seek(0, io.SeekEnd)
	if err != nil {
		return Transcript{}, err
	}
	if offset > size {
		return Transcript{}, fmt.Errorf("session log truncated: cursor %d exceeds size %d", offset, size)
	}
	if offset == 0 {
		offset = headerEnd
	}
	offset, err = boundedTranscriptStart(input, offset, size)
	if err != nil {
		return Transcript{}, err
	}
	if _, err := input.Seek(offset, io.SeekStart); err != nil {
		return Transcript{}, err
	}
	result := Transcript{Offset: offset}
	reader := bufio.NewReader(io.LimitReader(input, size-offset))
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return Transcript{}, err
		}
		result.Offset += int64(len(line))
		var record struct {
			At      time.Time       `json:"timestamp"`
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return Transcript{}, fmt.Errorf("decode session log at byte %d: %w", result.Offset, err)
		}
		if record.Type != "event_msg" && record.Type != "compacted" && record.Type != "turn_context" {
			continue
		}
		if record.At.IsZero() {
			return Transcript{}, fmt.Errorf("session event at byte %d has no timestamp", result.Offset)
		}
		if record.At.Before(before) {
			continue
		}
		var p struct {
			Model   string `json:"model"`
			Type    string `json:"type"`
			Message string `json:"message"`
			Reason  string `json:"reason"`
			Info    *struct {
				Usage *struct {
					Total *int64 `json:"total_tokens"`
				} `json:"last_token_usage"`
				Limit *int64 `json:"model_context_window"`
			} `json:"info"`
			RateLimits *struct {
				ID        string       `json:"limit_id"`
				Primary   *nativeLimit `json:"primary"`
				Secondary *nativeLimit `json:"secondary"`
			} `json:"rate_limits"`
		}
		if err := json.Unmarshal(record.Payload, &p); err != nil {
			return Transcript{}, err
		}
		if record.Type == "turn_context" {
			if p.Model == "" {
				return Transcript{}, fmt.Errorf("native turn_context carries no model")
			}
			result.Readings = append(result.Readings, Reading{Kind: "model", Source: "session-log", NativeEvent: "turn_context", At: &record.At, Status: "observed", Model: p.Model})
			continue
		}
		if record.Type == "compacted" {
			p.Type = "compaction_checkpoint"
		}
		base := Reading{Source: "session-log", NativeEvent: p.Type, At: &record.At, Status: "observed"}
		switch p.Type {
		case "token_count":
			if p.Info != nil {
				if p.Info.Usage == nil || p.Info.Usage.Total == nil || p.Info.Limit == nil {
					return Transcript{}, fmt.Errorf("token_count lacks last_token_usage.total_tokens or model_context_window")
				}
				r, err := contextReading("session-log", *p.Info.Usage.Total, *p.Info.Limit)
				if err != nil {
					return Transcript{}, err
				}
				r.At = &record.At
				r.NativeEvent = p.Type
				result.Readings = append(result.Readings, r)
			}
			if p.RateLimits != nil {
				base.Kind = "provider-limits"
				for i, w := range []*nativeLimit{p.RateLimits.Primary, p.RateLimits.Secondary} {
					if w == nil {
						continue
					}
					if w.Used == nil || w.Reset == nil || *w.Used < 0 || *w.Reset <= 0 {
						return Transcript{}, fmt.Errorf("token_count has invalid provider limit")
					}
					label := "primary"
					if i == 1 {
						label = "secondary"
					}
					if p.RateLimits.ID != "" {
						label = p.RateLimits.ID + "/" + label
					}
					base.Limits = append(base.Limits, LimitWindow{Label: label, UsedPercent: *w.Used, ResetAt: *w.Reset})
				}
				if len(base.Limits) > 0 {
					result.Readings = append(result.Readings, base)
				}
			}
		case "compaction_checkpoint":
			base.Kind = "compaction-checkpoint"
			result.Readings = append(result.Readings, base)
		case "context_compacted":
			base.Kind = "compaction-finished"
			result.Readings = append(result.Readings, base)
		case "task_started":
			base.Kind = "turn-started"
			result.Readings = append(result.Readings, base)
		case "task_complete":
			base.Kind = "turn-finished"
			result.Readings = append(result.Readings, base)
		case "turn_aborted":
			base.Kind = "turn-finished"
			base.Reason = p.Reason
			result.Readings = append(result.Readings, base)
		case "error":
			base.Kind = "error"
			base.Reason = p.Message
			if base.Reason == "" {
				return Transcript{}, fmt.Errorf("native error carries no message")
			}
			result.Readings = append(result.Readings, base)
		}
	}
}

type nativeLimit struct {
	Used  *float64 `json:"used_percent"`
	Reset *int64   `json:"resets_at"`
}

// StatuslineMeasurementTime corroborates an unversioned status-line reading
// with the latest assistant usage in the native transcript. A delayed callback
// cannot make an older matching reading newer than its native timestamp.
func StatuslineMeasurementTime(data []byte, r Reading) (*time.Time, error) {
	var payload struct {
		Transcript string `json:"transcript_path"`
		SessionID  string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Transcript == "" || r.Used == nil {
		return nil, nil
	}
	file, err := os.Open(payload.Transcript)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	start, err := boundedTranscriptStart(file, 0, size)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(io.LimitReader(file, size-start))
	var at *time.Time
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			return at, nil
		}
		if err != nil {
			return nil, err
		}
		var record struct {
			Type      string    `json:"type"`
			SessionID string    `json:"sessionId"`
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				Model string `json:"model"`
				Usage *struct {
					Input    *int64 `json:"input_tokens"`
					Creation *int64 `json:"cache_creation_input_tokens"`
					Read     *int64 `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("decode status-line transcript: %w", err)
		}
		if record.Type != "assistant" || record.Message.Usage == nil {
			continue
		}
		if record.SessionID != "" && record.SessionID != payload.SessionID {
			return nil, fmt.Errorf("status-line transcript belongs to another session")
		}
		u := record.Message.Usage
		if record.Timestamp.IsZero() || u.Input == nil || u.Creation == nil || u.Read == nil {
			return nil, fmt.Errorf("assistant usage lacks timestamp or input/cache tokens")
		}
		if *u.Input+*u.Creation+*u.Read == *r.Used && (r.Model == "" || r.Model == record.Message.Model) {
			copy := record.Timestamp
			at = &copy
		} else {
			at = nil
		}
	}
}

func TelemetryUsesTranscript(invocation Invocation) bool {
	return invocation.Name == "codex-session-log"
}
