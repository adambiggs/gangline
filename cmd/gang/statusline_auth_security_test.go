//go:build acceptance

package main

import (
	"bytes"
	"github.com/adambiggs/gangline/core"
	"os"
	"strings"
	"testing"
)

func TestStatuslineRequiresRegisteredNonceBeforeObservation(t *testing.T) {
	for _, nonce := range []string{"", "another-agents-nonce", "fixture-nonce"} {
		for _, locked := range []bool{false, true} {
			t.Run(nonce+map[bool]string{false: "/unlocked", true: "/locked"}[locked], func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "claude")
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				f.env["GANG_AGENT_NONCE"] = nonce
				f.input.command = "claude"
				f.input.screen = screenWithText("────────", "❯ ", "────────")
				detached := 0
				f.cmd.detach = func(string, hookNotice) error { detached++; return nil }
				p, _ := f.run.team.Agent(a.ID)
				if locked {
					l, err := p.TryLock()
					if err != nil {
						t.Fatal(err)
					}
					defer l.Close()
				}
				before, err := os.ReadFile(p.State)
				if err != nil {
					t.Fatal(err)
				}
				logBefore, err := os.ReadFile(f.run.team.Log)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				f.cmd.stdin = strings.NewReader(`{"session_id":"s","model":{"id":"claude-test"},"context_window":{"context_window_size":1000,"current_usage":{"input_tokens":990,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`)
				err = f.cmd.statusline(nil)
				if nonce == "fixture-nonce" {
					if err != nil {
						t.Fatal(err)
					}
					if locked {
						if detached != 1 {
							t.Fatalf("registered locked observation not deferred: detached=%d", detached)
						}
					} else {
						saved, err := p.Read()
						if err != nil || saved.Native.Context.Percent == nil || *saved.Native.Context.Percent != 99 {
							t.Fatalf("registered context not saved: %+v err=%v", saved.Native.Context, err)
						}
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "token does not match") {
					t.Fatalf("forged statusline: %v", err)
				}
				after, err := os.ReadFile(p.State)
				if err != nil || !bytes.Equal(before, after) || detached != 0 {
					t.Fatalf("forged statusline changed state: detached=%d err=%v", detached, err)
				}
				logAfter, err := os.ReadFile(f.run.team.Log)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if !bytes.Equal(logBefore, logAfter) {
					t.Fatal("forged statusline recorded an observation")
				}
			})
		}
	}
}

func TestStatuslineRejectsEmptyRegisteredNonce(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	saveAgent(t, f, a.ID, func(a *core.Agent) { a.Registration.TokenHash = tokenHash("") })
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	delete(f.env, "GANG_AGENT_NONCE")
	f.cmd.stdin = strings.NewReader(`{"session_id":"s","context_window":{"context_window_size":1000,"current_usage":{"input_tokens":990,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`)
	if err := f.cmd.statusline(nil); err == nil || !strings.Contains(err.Error(), "token does not match") {
		t.Fatalf("empty registered nonce admitted: %v", err)
	}
}
