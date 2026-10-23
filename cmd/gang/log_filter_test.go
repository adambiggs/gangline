package main

import (
	"bytes"
	"strings"
	"testing"
)

const filterTestEvent = `{"type":"native_hook","at":"2026-09-22T00:00:00Z","hitch_id":"a","name":"worker","status":"activity"}`

func TestFilteredLogSkipsUnselectedEventsWithoutValidating(t *testing.T) {
	log := `{"type":"invented","at":"2026-09-22T00:00:00Z","hitch_id":"b"}` + "\n" + filterTestEvent + "\n"
	var out bytes.Buffer
	if err := writeFilteredLog(&out, failWriter{t}, strings.NewReader(log), logFilter{Agent: "a"}, func(s string) string { return s }); err != nil {
		t.Fatal(err)
	}
	if out.String() != filterTestEvent+"\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestFilteredLogValidatesEverySelectedEvent(t *testing.T) {
	for _, c := range []struct {
		filter logFilter
		line   string
	}{
		{logFilter{Agent: "a"}, `{"type":"native_hook","at":"2026-09-22T00:00:00Z","hitch_id":"a","surprise":true}`},
		{logFilter{Agent: "a"}, `{"type":"native_hook","at":"2026-09-22T00:00:00Z","name":"a","surprise":true}`},
		{logFilter{Type: "usage"}, `{"type":"native_hook","at":"2026-09-22T00:00:00Z","readings":[{"kind":"usage","source":"s","status":"ok"}],"surprise":true}`},
		{logFilter{Agent: "a"}, `{"type":"native_hook",`},
	} {
		log := filterTestEvent + "\n" + c.line + "\n"
		var warn bytes.Buffer
		err := writeFilteredLog(&bytes.Buffer{}, &warn, strings.NewReader(log), c.filter, func(s string) string { return s })
		if err != nil || !strings.HasPrefix(warn.String(), "warning: audit line 2: ") {
			t.Fatalf("%+v %s: err = %v, warning %q", c.filter, c.line, err, warn.String())
		}
	}
}

// A write cut short by a full disk leaves a record without its end, and the
// next append continues on the same line. The export prints every event
// around that record and warns of its line once.
func TestFilteredLogRecoversTheEventAfterATornRecord(t *testing.T) {
	appended := `{"type":"send_queued","at":"2026-09-22T00:00:01Z","hitch_id":"a","name":"worker","envelope":{"id":"e","from":{"kind":"gangline","name":"hitch"},"to":"worker","recipient":"a","message":{"text":"{\"quoted\"} text"},"created_at":"2026-09-22T00:00:01Z"}}`
	for _, torn := range []string{
		`{"type":"native_hook","at":"2026-09-22T00:00:00Z","hitch_id":"a","id":"ab`,
		`{"type":"send_queued","at":"2026-09-22T00:00:00Z","envelope":{"id":"e","message":{"text":"x"}`,
		`{"type":"native_hook","at":"2026-09-22T00:00:00Z","hitch_id":"a","id":"ab` + `{"type":"native_hook","at":"20`,
	} {
		log := filterTestEvent + "\n" + torn + appended + "\n" + filterTestEvent + "\n"
		var out, warn bytes.Buffer
		err := writeFilteredLog(&out, &warn, strings.NewReader(log), logFilter{}, func(s string) string { return s })
		if err != nil || strings.Count(warn.String(), "warning: audit line 2: ") != 1 {
			t.Fatalf("%s: err = %v, warning %q", torn, err, warn.String())
		}
		if want := filterTestEvent + "\n" + appended + "\n" + filterTestEvent + "\n"; out.String() != want {
			t.Fatalf("%s: output = %q", torn, out.String())
		}
	}
}

// A damaged line with no intact event after the tear prints nothing.
func TestFilteredLogSkipsALineWithNoIntactEvent(t *testing.T) {
	log := filterTestEvent + "\n" + `{"type":"send_queued","envelope":{"id":"e"}}` + "\n" + filterTestEvent + "\n"
	var out, warn bytes.Buffer
	err := writeFilteredLog(&out, &warn, strings.NewReader(log), logFilter{}, func(s string) string { return s })
	if err != nil || !strings.HasPrefix(warn.String(), "warning: audit line 2: ") {
		t.Fatalf("err = %v, warning %q", err, warn.String())
	}
	if want := filterTestEvent + "\n" + filterTestEvent + "\n"; out.String() != want {
		t.Fatalf("output = %q", out.String())
	}
}

// failWriter fails a test that writes a warning.
type failWriter struct{ t *testing.T }

func (w failWriter) Write(p []byte) (int, error) {
	w.t.Errorf("unexpected warning %q", p)
	return len(p), nil
}
