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
	if err := writeFilteredLog(&out, strings.NewReader(log), logFilter{Agent: "a"}); err != nil {
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
		err := writeFilteredLog(&bytes.Buffer{}, strings.NewReader(log), c.filter)
		if err == nil || !strings.Contains(err.Error(), "audit line 2") {
			t.Fatalf("%+v %s: err = %v", c.filter, c.line, err)
		}
	}
}
