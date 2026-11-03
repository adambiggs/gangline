//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogMetadataCommandReadsNamedFileWithoutRuntime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "saved.jsonl")
	data := []byte(metadataUsage + "\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := command{stdout: &out, clock: func() time.Time { return time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC) }}
	if err := cmd.log([]string{"--metadata", path}); err != nil {
		t.Fatal(err)
	}
	var page logMetadataPage
	if err := json.Unmarshal(out.Bytes(), &page); err != nil || len(page.Events) != 1 || page.More {
		t.Fatal("named-file command did not deliver metadata", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("metadata read changed source audit")
	}
	if err := cmd.log([]string{"--metadata", root}); err == nil {
		t.Fatal("directory accepted as audit")
	}
	out.Reset()
	if err := cmd.log([]string{"--summary", path}); err != nil {
		t.Fatal(err)
	}
	var summary logSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil || len(summary.Subjects) != 1 || summary.Schema != "gangline.observability-summary.v1" {
		t.Fatal("named-file summary failed", err)
	}
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("summary changed source audit")
	}
	if err := cmd.log([]string{"--summary", root}); err == nil {
		t.Fatal("directory accepted for summary")
	}
	cmd.team = "other"
	if err := cmd.log([]string{"--metadata", path}); err == nil {
		t.Fatal("saved file accepted a conflicting team scope")
	}
}
