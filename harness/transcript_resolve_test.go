package harness

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTranscriptKeepsVerifiedSession(t *testing.T) {
	got, err := resolveTranscript("parent.jsonl", "parent", func(path string) (string, error) {
		return "parent", nil
	}, func(string) ([]string, error) {
		t.Fatal("verified transcript searched settled sessions")
		return nil, nil
	})
	if err != nil || got != "parent.jsonl" {
		t.Fatalf("resolved = %q, %v", got, err)
	}
}

func TestResolveTranscriptFindsVerifiedParent(t *testing.T) {
	for _, test := range []struct {
		name       string
		candidates map[string]string
		want       string
	}{
		{"parent", map[string]string{"rollout-first-parent.jsonl": "parent"}, "rollout-first-parent.jsonl"},
		{"misnamed", map[string]string{"rollout-first-parent.jsonl": "other"}, ""},
		{"missing", nil, ""},
		{"ambiguous", map[string]string{"rollout-first-parent.jsonl": "parent", "rollout-second-parent.jsonl": "parent"}, ""},
		{"unreadable", map[string]string{"rollout-first-parent.jsonl": ""}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			const directory = "/sessions/native[store]"
			child := filepath.Join(directory, "rollout-child.jsonl")
			identity := func(path string) (string, error) {
				session := "child"
				if path != child {
					session = test.candidates[filepath.Base(path)]
				}
				if session == "" {
					return "", fmt.Errorf("unreadable header")
				}
				found, _, err := codexTranscriptIdentity(strings.NewReader(`{"type":"session_meta","payload":{"id":"` + session + `"}}` + "\n"))
				return found, err
			}
			glob := func(pattern string) ([]string, error) {
				var paths []string
				for name := range test.candidates {
					path := filepath.Join(directory, name)
					if matched, err := filepath.Match(pattern, path); err != nil || !matched {
						t.Fatalf("candidate does not match escaped lookup: %s, %v", pattern, err)
					}
					paths = append(paths, path)
				}
				return paths, nil
			}
			got, err := resolveTranscript(child, "parent", identity, glob)
			if test.want == "" {
				if err == nil || got != "" {
					t.Fatalf("unverified parent accepted: %q, %v", got, err)
				}
			} else if err != nil || got != filepath.Join(directory, test.want) {
				t.Fatalf("parent = %q, %v", got, err)
			}
		})
	}
}

func TestResolveTranscriptRejectsInvalidSession(t *testing.T) {
	for _, session := range []string{"", "../parent", "*", "[parent]", ".."} {
		_, err := resolveTranscript("child.jsonl", session, func(string) (string, error) {
			t.Fatal("invalid session read a transcript")
			return "", nil
		}, nil)
		if err == nil {
			t.Errorf("invalid session %q accepted", session)
		}
	}
}

func TestResolveTranscriptSearchesOnlyRecordedDirectory(t *testing.T) {
	const directory = "/sessions/2026/10/30"
	calls := 0
	_, err := resolveTranscript(directory+"/rollout-child.jsonl", "parent", func(string) (string, error) {
		return "child", nil
	}, func(pattern string) ([]string, error) {
		calls++
		if pattern != directory+"/rollout-*-parent.jsonl" {
			t.Fatalf("lookup escaped recorded session directory: %s", pattern)
		}
		return nil, nil
	})
	if err == nil || calls != 1 {
		t.Fatalf("missing parent lookup: calls=%d err=%v", calls, err)
	}
}
