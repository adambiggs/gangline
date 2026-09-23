package harness

import (
	"strings"
	"testing"
)

func TestCodexRetryMenuRequiresPromptAndChoice(t *testing.T) {
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, title, choice string
		want                bool
	}{
		{"menu", "Giving this request a little extra thought", "› 1. Retry with a faster model", true},
		{"wrapped title", "Giving this request a little extra\nthought", "› 1. Retry with a faster model", true},
		{"quoted choice", "Giving this request a little extra thought", "Retry with a faster model", false},
		{"title only", "Giving this request a little extra thought", "", false},
		{"choice only", "", "› 1. Retry with a faster model", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, found, err := InputBlocked(c, testScreen(testCells(tc.title, false), testCells(tc.choice, false)))
			if err != nil || found != tc.want {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if found && !strings.Contains(got.Evidence, "Giving this request a little extra thought") {
				t.Fatalf("evidence=%q", got.Evidence)
			}
		})
	}
}
