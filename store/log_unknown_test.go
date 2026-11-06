package store

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestReadLogSkipsAndCountsUnknownKinds(t *testing.T) {
	known := `{"type":"native_hook","at":"2026-11-05T00:00:00Z","status":"activity"}` + "\n"
	unknown := `{"type":"future_kind","private_payload":{"anything":true}}` + "\n"
	visited := 0
	skipped, err := ReadLog(strings.NewReader(known+unknown+known), func(core.Event) error { visited++; return nil })
	if err != nil || skipped != 1 || visited != 2 {
		t.Fatalf("visited=%d skipped=%d err=%v", visited, skipped, err)
	}
	if _, err := ReadLog(strings.NewReader(unknown+"{broken}\n"+known), func(core.Event) error { return nil }); err == nil {
		t.Fatal("malformed JSON was skipped")
	}
}
