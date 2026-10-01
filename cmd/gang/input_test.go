package main

import (
	"strings"
	"testing"
)

func TestReadBodyPreservesContentAndDropsOneTerminalNewline(t *testing.T) {
	got, err := readBody(strings.NewReader("first\nsecond\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "first\nsecond" {
		t.Fatalf("body = %q", got)
	}
}

func TestReadBodyRejectsEmptyInput(t *testing.T) {
	if _, err := readBody(strings.NewReader("")); err == nil {
		t.Fatal("empty input passed")
	}
}

func TestReadBodyRejectsTerminalControlBytes(t *testing.T) {
	for _, body := range []string{"contains\x00nul", "invalid\xffutf8", "\x1b[201~\rsubmit", "back\bspace", "carriage\rreturn", "c1\u009b201~", "delete\x7f"} {
		if _, err := readBody(strings.NewReader(body)); err == nil {
			t.Fatalf("body %q passed", body)
		}
	}
}

func TestEnvelopeRejectsTerminalControlsFromDirectArgumentsAndStartupSections(t *testing.T) {
	for _, text := range []string{"\x1b[201~\rsubmit", "carriage\rreturn", "\u009b201~"} {
		if _, err := renderEnvelope("lead", "0123abcd", "assignment", text); err == nil {
			t.Fatalf("unsafe envelope accepted: %q", text)
		}
	}
	if _, err := renderEnvelope("lead", "0123abcd", "assignment", "text\n\ttab"); err != nil {
		t.Fatal(err)
	}
}
