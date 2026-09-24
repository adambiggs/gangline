package main

import (
	"encoding/json"
	"fmt"
	"github.com/adambiggs/gangline/harness"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// Exercise the real tick, inbox, envelope and native submit-witness path.
// No native harness, tmux server, sleep or deadline is involved.
func TestContextBandNotesCrossings(t *testing.T) {
	for _, collar := range []string{"codex", "claude-code"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			f.env["GANGLINE_HITCH_ID"] = "a"
			model := "gpt-test"
			low, high := 75.0, 90.0
			highName := "red"
			if collar == "claude-code" {
				f.input.command = "claude"
				f.input.screen = screenWithText("────────", "❯ ", "────────")
				model, low, high = "claude-opus-test", 10, 20
				highName = "late"
			}
			p, _ := f.run.team.Agent(a.ID)
			observe := func(percent float64, status string, at time.Time) {
				t.Helper()
				l, err := p.TryLock()
				if err != nil {
					t.Fatal(err)
				}
				a, err := p.Read()
				if err != nil {
					t.Fatal(err)
				}
				used, limit := int64(percent*100), int64(10000)
				a.Native.Context = core.Reading{Kind: "context", Source: "fixture", Model: model, Status: status, At: &at, Used: &used, Limit: &limit, Percent: &percent}
				if err := l.Save(a); err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
			}
			at := f.cmd.now()
			observe(low-1, "observed", at)
			if f.input.submits != 0 {
				t.Fatalf("below first band submitted %d notes", f.input.submits)
			}
			observe(high, "observed", at.Add(time.Second))
			if f.input.submits != 2 {
				t.Fatalf("jump across yellow and red submitted %d notes, want 2", f.input.submits)
			}
			if !strings.Contains(f.input.pasted, highName) || !strings.Contains(f.input.pasted, "[gang:gangline:context-band#") || !strings.Contains(f.input.pasted, "run `gang compact --resume") {
				t.Fatalf("band envelope: %s", f.input.pasted)
			}
			observe(high+1, "observed", at.Add(2*time.Second))
			observe(0, "unknown", at.Add(3*time.Second))
			observe(high+1, "observed", at.Add(4*time.Second))
			if f.input.submits != 2 {
				t.Fatalf("repeated/unknown reading repeated notes: %d", f.input.submits)
			}
			observe(low, "observed", at.Add(5*time.Second))
			observe(high, "observed", at.Add(6*time.Second))
			if f.input.submits != 3 {
				t.Fatalf("red recross submitted %d total notes, want 3", f.input.submits)
			}
			if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", At: at.Add(7 * time.Second)}, false); err != nil {
				t.Fatal(err)
			}
			observe(high, "observed", at.Add(8*time.Second))
			if f.input.submits != 3 {
				t.Fatalf("first reading after compaction asked for another compaction: %d total notes, want 3", f.input.submits)
			}
			observe(low-1, "observed", at.Add(9*time.Second))
			observe(high, "observed", at.Add(10*time.Second))
			if f.input.submits != 5 {
				t.Fatalf("post-compaction crossings submitted %d total notes, want 5", f.input.submits)
			}
			f.out.Reset()
			if err := f.cmd.log(nil); err != nil {
				t.Fatal(err)
			}
			bands, delivered := 0, 0
			if err := store.ReadLog(strings.NewReader(f.out.String()), func(e core.Event) error {
				if e.Type == "context_band_crossed" {
					bands++
					if e.HitchID != a.ID || e.Envelope == nil || e.ID != string(e.Envelope.ID) || len(e.Readings) != 1 || e.Readings[0].Percent == nil {
						t.Fatalf("incomplete band lifecycle event: %+v", e)
					}
				}
				if e.Type == "delivery_succeeded" {
					delivered++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if bands != 5 || delivered != 5 {
				t.Fatalf("lifecycle band=%d delivered=%d, want 5 each", bands, delivered)
			}
		})
	}
}

func TestContextBandNotesTranscriptBatch(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = "a"
	p, _ := f.run.team.Agent(a.ID)
	at := f.cmd.now().Format(time.RFC3339)
	transcript := fmt.Sprintf("{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n{\"timestamp\":%q,\"type\":\"turn_context\",\"payload\":{\"model\":\"gpt-test\"}}\n", at)
	for _, used := range []int{750, 900, 100, 900} {
		transcript += fmt.Sprintf("{\"timestamp\":%q,\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"last_token_usage\":{\"total_tokens\":%d},\"model_context_window\":1000}}}\n", at, used)
	}
	path := filepath.Join(f.env["GANG_STATE_ROOT"], "transcript.jsonl")
	if err := os.WriteFile(path, []byte(transcript), 0600); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Native.SessionID, a.Native.Transcript = "s", path
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
	}
	if f.input.submits != 4 {
		t.Fatalf("batch crossings submitted %d notes, want 4", f.input.submits)
	}
}

func TestContextBandNotesStatuslineWhileLocked(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude-code")
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.input.command, f.input.screen = "claude", screenWithText("────────", "❯ ", "────────")
	p, _ := f.run.team.Agent(a.ID)
	var deferred []hookNotice
	f.cmd.detach = func(id string, n hookNotice) error {
		if id != "a" {
			t.Fatalf("wrong recipient %s", id)
		}
		deferred = append(deferred, n)
		return nil
	}
	status := func(used int) {
		t.Helper()
		cmd := f.cmd
		cmd.stdin = strings.NewReader(fmt.Sprintf(`{"session_id":"s","model":{"id":"claude-haiku-test"},"context_window":{"context_window_size":1000,"current_usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`, used))
		if err := cmd.statusline(nil); err != nil {
			t.Fatal(err)
		}
	}
	status(400) // Haiku's first threshold differs from the default model.
	if len(deferred) != 0 {
		t.Fatalf("below-band reading woke delivery: %+v", deferred)
	}
	status(450)
	if len(deferred) != 1 {
		t.Fatalf("statusline crossing did not wake delivery: %+v", deferred)
	}
	if err := f.run.tickAgent(a.ID, deferred[0], false); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	status(650) // Must finish without waiting for the lock we hold.
	if len(deferred) != 2 {
		t.Fatalf("locked statusline lost its reading: %+v", deferred)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, deferred[1], false); err != nil {
		t.Fatal(err)
	}
	status(650)
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 2 {
		t.Fatalf("statusline crossings submitted %d notes, want 2", f.input.submits)
	}
}

func TestContextBandNotesScreenCollarAndBlockedDelivery(t *testing.T) {
	f := newStateFixture(t)
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	c.Name, c.Primitives.Telemetry = "screen-fixture", nil
	c.ContextBands = map[string][]harness.ContextBand{"*": {{Name: "checkpoint", At: 0.5}}}
	data, err := json.Marshal(map[string]any{"collar": c})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.env["GANG_STATE_ROOT"], "collars")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, c.Name+".cue"), data, 0600); err != nil {
		t.Fatal(err)
	}
	f.env["GANG_COLLARS"] = dir
	f.run.settings.CollarDir = dir
	a := f.add(t, "a", "worker", c.Name)
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.input.screen = screenWithText("gpt-test ·", "ctx 500/1000 50%", "Would you like to run this command?", "Yes, proceed", "› ")
	for range 2 {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
	}
	p, _ := f.run.team.Agent(a.ID)
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || f.input.submits != 0 {
		t.Fatalf("blocked crossing pending=%d submits=%d", len(pending), f.input.submits)
	}
	f.input.screen = screenWithText("gpt-test ·", "ctx 500/1000 50%", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, "checkpoint") {
		t.Fatalf("screen note submits=%d: %s", f.input.submits, f.input.pasted)
	}
}

func TestContextBandNotesPublicationRecovery(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(fmt.Sprint(published), func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			f.env["GANGLINE_HITCH_ID"] = "a"
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			used, limit, percent := int64(800), int64(1000), 80.0
			a.Native.Context = core.Reading{Kind: "context", Source: "fixture", Model: "gpt-test", Status: "observed", Used: &used, Limit: &limit, Percent: &percent}
			c, err := harness.EmbeddedCollar("codex")
			if err != nil {
				t.Fatal(err)
			}
			f.run.noteContextBands(&a, c)
			if len(a.ContextBands.Pending) != 1 {
				t.Fatalf("no durable intent: %+v", a.ContextBands)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if published {
				if err := p.Publish(a.ContextBands.Pending[0].Envelope); err != nil {
					t.Fatal(err)
				}
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
			}
			if f.input.submits != 1 {
				t.Fatalf("recovered publication submitted %d notes", f.input.submits)
			}
		})
	}
}

// A live reader may already be above the newly configured first threshold,
// without having published any note under the previous policy.
func TestClaudeEarlyContextBandFromStatusline(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude-code")
	f.env["GANGLINE_HITCH_ID"] = "a"
	f.input.command, f.input.screen = "claude", screenWithText("────────", "❯ ", "────────")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.ContextBands.Model, a.ContextBands.Percent = "claude-opus-test", 11
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	var notices []hookNotice
	f.cmd.detach = func(id string, n hookNotice) error {
		notices = append(notices, n)
		return nil
	}
	status := func(used int) {
		t.Helper()
		cmd := f.cmd
		cmd.stdin = strings.NewReader(fmt.Sprintf(`{"session_id":"s","model":{"id":"claude-opus-test"},"context_window":{"context_window_size":1000000,"current_usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`, used))
		if err := cmd.statusline(nil); err != nil {
			t.Fatal(err)
		}
		for _, n := range notices {
			if err := f.run.tickAgent(a.ID, n, false); err != nil {
				t.Fatal(err)
			}
		}
		notices = nil
	}
	status(110000)
	if f.input.submits != 1 || !strings.Contains(f.input.pasted, "early crossed (threshold 10%)") {
		t.Fatalf("first eligible reading: submits=%d, wire=%q", f.input.submits, f.input.pasted)
	}
	status(110000)
	if f.input.submits != 1 {
		t.Fatalf("same reading repeated note: %d", f.input.submits)
	}
	status(200000)
	if f.input.submits != 2 || !strings.Contains(f.input.pasted, "late crossed (threshold 20%)") {
		t.Fatalf("late reading: submits=%d, wire=%q", f.input.submits, f.input.pasted)
	}
}
