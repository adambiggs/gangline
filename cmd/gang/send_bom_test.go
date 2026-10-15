package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCodexSendByteOrderMarkRefusedBeforePublication(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.input.screen = screenWithText("esc to interrupt", "› ")
	text := "before\ufeffafter"
	f.cmd.stdin = strings.NewReader(text)
	err := f.cmd.send([]string{"worker", "--from", "operator"})
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), "U+FEFF") {
		t.Fatalf("send refusal=%v; stdout=%q; stderr=%q", err, f.out.String(), f.errOut.String())
	}
	if f.out.Len() != 0 || f.errOut.Len() != 0 {
		t.Fatalf("stdout=%q; stderr=%q", f.out.String(), f.errOut.String())
	}
	p, _ := f.run.team.Agent(a.ID)
	requireNothingTyped(t, f, p)
	if _, err := os.Stat(f.run.team.Log); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused send created an audit log: %v", err)
	}
	t.Logf("send refusal=%q; no stdout, warning, inbox message, audit event, or native input", err)
}
