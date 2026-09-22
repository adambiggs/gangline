package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvelopeBudgetIncludesEscapingAndFraming(t *testing.T) {
	for _, body := range []string{strings.Repeat("x", maximumMessageBytes), strings.Repeat("\"", maximumMessageBytes/2), strings.Repeat("\t", maximumMessageBytes/2)} {
		_, err := renderEnvelope("self-declared:operator", "msg-123", "", body)
		if err == nil || !strings.Contains(err.Error(), "state file") {
			t.Fatalf("oversized encoded envelope was not refused with state-file guidance: %v", err)
		}
	}
	empty, err := renderEnvelope("operator", "msg-123", "", "")
	if err != nil {
		t.Fatal(err)
	}
	overhead, _ := json.Marshal(empty)
	wire, err := renderEnvelope("operator", "msg-123", "", strings.Repeat("x", maximumMessageBytes-len(overhead)))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(wire)
	if len(encoded) != maximumMessageBytes {
		t.Fatalf("encoded boundary = %d", len(encoded))
	}
}

func TestOptionalProseRejectsOversizeDuringRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DOCTRINE.md")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maximumProseBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readOptionalProse(path); err == nil || data != nil {
		t.Fatalf("oversize prose returned %d bytes, error %v", len(data), err)
	}
}
