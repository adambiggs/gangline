//go:build acceptance

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStatuslineRepeatedUsageDoesNotGrowAudit(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock() // Observation recording must remain independent of this lock.
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i := 0; i < 100; i++ {
		f.cmd.stdin = strings.NewReader(`{"session_id":"s","context_window":{"current_usage":null,"total_input_tokens":90,"total_output_tokens":0}}`)
		if err := f.cmd.statusline(nil); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(data, []byte(`"kind":"token-usage"`)); n != 1 {
		t.Fatalf("unchanged usage appended %d observations, want 1", n)
	}
}

func TestStatuslineRepeatedQuotaWithMovingClockDoesNotGrowAudit(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	now := f.cmd.now()
	f.cmd.clock = func() time.Time { now = now.Add(time.Second); return now }
	for i := 0; i < 100; i++ {
		f.cmd.stdin = strings.NewReader(`{"session_id":"s","context_window":{"current_usage":null,"total_input_tokens":90,"total_output_tokens":0},"rate_limits":{"five_hour":{"used_percentage":10,"resets_at":1800000000}}}`)
		if err := f.cmd.statusline(nil); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(data, []byte(`"kind":"token-usage"`)); n != 1 {
		t.Fatalf("unchanged usage with quota appended %d observations, want 1", n)
	}
}
