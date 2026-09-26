package main

import (
	"testing"
	"time"
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
	if got, err := parseSend([]string{"worker", "body", "--from", "operator"}); err != nil || got.From != "operator" || got.Body == nil || *got.Body != "body" {
		t.Fatalf("late option = %+v, %v", got, err)
	}
}

func TestOptionsBeforeAndAfterNames(t *testing.T) {
	for _, args := range [][]string{{"-c", "codex", "worker"}, {"worker", "-c", "codex"}} {
		got, err := parseHitch(args, "claude-code", "/work")
		if err != nil || got.Name != "worker" || got.Collar != "codex" {
			t.Fatalf("hitch(%q) = %+v, %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"--from", "ext", "worker", "hi"}, {"worker", "hi", "--from", "ext"}} {
		got, err := parseSend(args)
		if err != nil || got.Name != "worker" || got.From != "ext" || got.Body == nil || *got.Body != "hi" {
			t.Fatalf("send(%q) = %+v, %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"--timeout", "1s", "worker"}, {"worker", "--timeout", "1s"}} {
		got, err := parseWait(args)
		if err != nil || got.Name != "worker" || got.Timeout != time.Second {
			t.Fatalf("wait(%q) = %+v, %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"-c", "codex", "worker"}, {"worker", "-c", "codex"}} {
		name, collar, err := parseAdopt(args)
		if err != nil || name != "worker" || collar != "codex" {
			t.Fatalf("adopt(%q) = %q, %q, %v", args, name, collar, err)
		}
	}
}

func TestTerminatorPreservesOperands(t *testing.T) {
	got, err := parseSend([]string{"--from", "ext", "--", "worker", "--help"})
	if err != nil || got.Name != "worker" || got.Body == nil || *got.Body != "--help" {
		t.Fatalf("send = %+v, %v", got, err)
	}
	filter, files, err := parseLogFilter([]string{"--", "-events.jsonl"}, true)
	if err != nil || filter != (logFilter{}) || len(files) != 1 || files[0] != "-events.jsonl" {
		t.Fatalf("log = %+v, %q, %v", filter, files, err)
	}
	if _, files, err := parseLogFilter([]string{"--"}, true); err != nil || len(files) != 0 {
		t.Fatalf("log terminator = %q, %v", files, err)
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
