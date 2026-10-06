package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/adambiggs/gangline/core"
	"strings"
	"testing"
	"time"
)

func TestOperatorDiagnosticsUseNamesAndKeepJSONValid(t *testing.T) {
	names := &operatorNames{names: map[string]string{"%1": "lead", "%138": `window "desk" at pane position 2`, "$0": "team session", "@0": `window "desk"`}}
	var out bytes.Buffer
	w := operatorOutput{writer: &out, names: names}
	input := []byte(`{"reason":"pane %138 differs from %1 in $0 @0","exact":9007199254740993}` + "\n")
	if n, err := w.Write(input); err != nil || n != len(input) {
		t.Fatalf("write = %d %v", n, err)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if string(value["exact"]) != "9007199254740993" || tmuxHandle.Match(out.Bytes()) || !strings.Contains(out.String(), "lead") || !strings.Contains(out.String(), `\"desk\"`) {
		t.Fatalf("diagnostic JSON = %s", out.Bytes())
	}
	err := operatorError{error: errors.Join(refuseError("pane %%1 failed"), errors.New("pane %138 moved")), names: names}
	if errorStatus(err) != exitRefused || strings.Join(errorLines(err), "\n") != "pane lead failed\npane window \"desk\" at pane position 2 moved" {
		t.Fatalf("error status=%d lines=%q", errorStatus(err), errorLines(err))
	}
}

func TestLogShowsRecordedAgentNamesAndPreservesMessageText(t *testing.T) {
	const text = "literal %1 $100 @2"
	e := core.Event{Type: "send_queued", At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), HitchID: "id", Name: "worker", Pane: "%1", Envelope: &core.Envelope{ID: "msg", From: core.Sender{Kind: core.SenderAgent, Name: "lead"}, To: "worker", Recipient: "id", Message: core.Message{Text: text}, Reason: "pane %1 failed", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
	data, err := core.EncodeEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	names := &operatorNames{names: map[string]string{"%1": "worker"}}
	if err := writeFilteredLog(&out, bytes.NewReader(append(data, '\n')), logFilter{}, names.text); err != nil {
		t.Fatal(err)
	}
	got, err := core.DecodeEvent(bytes.TrimSpace(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Pane != "worker" || got.Envelope.Message.Text != text || got.Envelope.Reason != "pane worker failed" || e.Pane != "%1" {
		t.Fatalf("event changed: %+v", got)
	}
}

func TestSplitFlagsUseRegisteredNames(t *testing.T) {
	options, err := parseHitch([]string{"worker", "--split", "lead", "--vertical"}, "codex", "/tmp")
	if err != nil || options.Split != "lead" || !options.Vertical {
		t.Fatalf("split options = %+v %v", options, err)
	}
	for _, args := range [][]string{{"worker", "--split", "%1"}, {"worker", "--vertical"}, {"worker", "--recover", "--split", "lead"}} {
		if _, err := parseHitch(args, "codex", "/tmp"); err == nil {
			t.Fatalf("invalid split accepted: %v", args)
		}
	}
}
