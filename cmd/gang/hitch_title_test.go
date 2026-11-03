package main

import "testing"

func TestHitchTitleOptionalAndValidated(t *testing.T) {
	for _, args := range [][]string{{"worker"}, {"worker", "--title", "worker@ser5: cache refresh"}, {"--title=worker@mac: review", "worker"}} {
		got, err := parseHitch(args, "codex", "/work")
		if err != nil {
			t.Fatal(err)
		}
		if len(args) == 1 && got.Title != "" || len(args) > 1 && got.Title == "" {
			t.Fatalf("title lost: %+v", got)
		}
	}
	for _, title := range []string{"", "  ", "two\nlines", "two\tcolumns", "bad\x1btitle", "bad\rtitle"} {
		if _, err := parseHitch([]string{"worker", "--title", title}, "codex", "/work"); err == nil {
			t.Fatalf("invalid title accepted: %q", title)
		}
	}
	if _, err := parseHitch([]string{"worker", "--recover", "--title", "changed"}, "codex", "/work"); err == nil {
		t.Fatal("recovery silently accepted title")
	}
	args := upHitchArguments("lead", []string{"--title", "lead@ser5: delivery"})
	got, err := parseHitch(args, "codex", "/work")
	if err != nil || got.Title != "lead@ser5: delivery" || got.Role != "lead" {
		t.Fatalf("up title: %+v %v", got, err)
	}
}
