package store

import "testing"

func TestTeamPathsStayInsideRoot(t *testing.T) {
	paths, err := (Paths{Root: "/state/gangline"}).Team("example")
	if err != nil {
		t.Fatal(err)
	}
	if paths.Events != "/state/gangline/v1/example/events.jsonl" {
		t.Fatalf("events path = %q", paths.Events)
	}
	if _, err := (Paths{Root: "/state/gangline"}).Team("../other"); err == nil {
		t.Fatal("path traversal passed team validation")
	}
}
