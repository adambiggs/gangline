package main

import (
	"strings"
	"testing"
)

func TestParseHitchUsesNameBeforeStdlibFlags(t *testing.T) {
	got, err := parseHitch([]string{"worker", "-c", "codex", "-d", "/work", "-m", "gpt", "--stdin"}, "claude-code", "/default")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "worker" || got.Collar != "codex" || got.Directory != "/work" || got.Model != "gpt" || !got.Stdin {
		t.Fatalf("options = %#v", got)
	}
}

func TestParseSendRejectsUnsafeCombinations(t *testing.T) {
	if _, err := parseSend([]string{"worker", "--live-only", "--at", "1h"}); err == nil {
		t.Fatal("--live-only with --at passed")
	}
	if _, err := parseSend([]string{"-worker"}); err == nil {
		t.Fatal("invalid recipient passed")
	}
}

func TestSendAtAcceptsClear(t *testing.T) {
	options, err := parseSend([]string{"worker", "--at", "clear"})
	if err != nil || options.At != "clear" {
		t.Fatalf("options=%+v err=%v", options, err)
	}
}

func TestParseSendAcceptsPositionalBody(t *testing.T) {
	got, err := parseSend([]string{"worker", "--from", "operator", "two\nlines"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body == nil || *got.Body != "two\nlines" {
		t.Fatalf("body = %v", got.Body)
	}
	if got, err := parseSend([]string{"worker", "--", "-dash"}); err != nil || got.Body == nil || *got.Body != "-dash" {
		t.Fatalf("dash body = %+v, %v", got, err)
	}
	if _, err := parseSend([]string{"worker", "first", "second"}); err == nil {
		t.Fatal("multiple message bodies passed")
	}
	if _, err := parseSend([]string{"worker", "--at", "clear", "body"}); err == nil {
		t.Fatal("body with clear passed")
	}
	if _, err := parseSend([]string{"worker", "body", "--from", "operator"}); err == nil || !strings.Contains(err.Error(), "options must precede BODY") {
		t.Fatalf("late option error = %v", err)
	}
}

func TestParseCompactAllowsSelfTarget(t *testing.T) {
	got, err := parseCompact([]string{"--resume", "continue"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "" || got.Resume != "continue" {
		t.Fatalf("options = %#v", got)
	}
}

func TestParseWaitAcceptsZeroAndRejectsNegativeTimeout(t *testing.T) {
	options, err := parseWait([]string{"worker", "--timeout", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Name != "worker" || options.Timeout != 0 {
		t.Fatalf("options = %#v", options)
	}
	if _, err := parseWait([]string{"worker", "--timeout", "-1s"}); err == nil {
		t.Fatal("negative timeout passed")
	}
}
