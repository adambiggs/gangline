package main

import (
	"encoding/json"
	"errors"
	"github.com/adambiggs/gangline/harness"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/store"
)

func TestHookRequiresRegisteredNonceBeforeWritingEvidence(t *testing.T) {
	for _, event := range []string{"UserPromptSubmit", "StopFailure", "PermissionRequest", "PreCompact", "PostCompact"} {
		for _, nonce := range []string{"", "another-agents-nonce", "fixture-nonce"} {
			t.Run(event+"/"+nonce, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "claude")
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				f.env["GANG_AGENT_NONCE"] = nonce
				detached := false
				f.cmd.detach = func(string, hookNotice) error { detached = true; return nil }
				payload, err := json.Marshal(map[string]string{"hook_event_name": event, "session_id": "s", "prompt": "work", "error": "forged failure", "transcript_path": f.transcript})
				if err != nil {
					t.Fatal(err)
				}
				f.cmd.stdin = strings.NewReader(string(payload))
				err = f.cmd.handleHook(nil)
				if nonce == "fixture-nonce" {
					if err != nil || !detached {
						t.Fatalf("registered hook: detached=%v err=%v", detached, err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "token does not match") {
					t.Fatalf("unauthenticated hook: %v", err)
				}
				p, _ := f.run.team.Agent(a.ID)
				hooks, err := p.ReadStatusHooks()
				if err != nil || len(hooks) != 0 || detached {
					t.Fatalf("forged hook wrote status or detached: hooks=%v detached=%v err=%v", hooks, detached, err)
				}
				w, err := p.ReadWitness()
				if !errors.Is(err, os.ErrNotExist) || w != (store.Witness{}) {
					t.Fatalf("forged hook wrote witness: %+v err=%v", w, err)
				}
				_, present, err := p.ReadPermissionWitness()
				if err != nil || present {
					t.Fatalf("forged permission: present=%v err=%v", present, err)
				}
			})
		}
	}
}

func TestHookRejectsEmptyNonceEvenWhenRegistrationHashesEmptyToken(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	a.Registration.TokenHash = tokenHash("")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	delete(f.env, "GANG_AGENT_NONCE")
	f.cmd.stdin = strings.NewReader(`{"hook_event_name":"StopFailure","session_id":"s","error":"forged"}`)
	if err := f.cmd.handleHook(nil); err == nil || !strings.Contains(err.Error(), "token does not match") {
		t.Fatalf("missing nonce admitted: %v", err)
	}
}

func TestNativeRefreshRejectsNonregularTranscript(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.Native.Transcript = "/dev/null"
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.run.refreshNative(l, &a, c); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("native refresh accepted special file: %v", err)
	}
}
