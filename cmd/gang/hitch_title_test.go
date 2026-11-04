package main

import "testing"

func TestHitchRejectsRemovedTitleOption(t *testing.T) {
	for _, args := range [][]string{{"worker", "--title", "display"}, {"--title=display", "worker"}, upHitchArguments("lead", []string{"--title", "display"})} {
		if _, err := parseHitch(args, "codex", "/work"); err == nil {
			t.Fatalf("accepted removed title option: %q", args)
		}
	}
}

func TestUpRejectsRemovedTitleOption(t *testing.T) {
	if err := (command{}).up([]string{"--title", "display"}); err == nil || err.Error() != "up: unknown option --title" {
		t.Fatalf("up title option: %v", err)
	}
}
