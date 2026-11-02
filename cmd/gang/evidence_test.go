//go:build acceptance

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/internal/evidence"
)

func TestEvidenceCommandDoesNotUseLiveState(t *testing.T) {
	t.Setenv("GANG_STATE_ROOT", filepath.Join(t.TempDir(), "must-not-exist"))
	t.Setenv("GANG_CAPACITY_TIMEOUT", "invalid")
	root := t.TempDir()
	path := filepath.Join(root, "chain.json")
	m := evidence.Manifest{Version: 1, Scope: evidence.Scope{Host: "host", Team: "team", Hitch: "hitch", Assignment: "task"}, Target: "missing"}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		mode   string
		status int
	}{{"verify", exitUnknown}, {"show", exitOK}} {
		var out, stderr bytes.Buffer
		got := run([]string{"evidence", tt.mode, "--assignment", "task", path}, strings.NewReader(""), &out, &stderr)
		if got != tt.status || !strings.Contains(out.String(), `"status": "incomplete"`) {
			t.Fatalf("%s: status %d output %s err %s", tt.mode, got, &out, &stderr)
		}
	}
	if _, err := os.Stat(os.Getenv("GANG_STATE_ROOT")); !os.IsNotExist(err) {
		t.Fatalf("live root touched: %v", err)
	}
}
func TestEvidenceInputRootAndWithheldErrors(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := []byte("SENTINEL-CREDENTIAL")
	if err := os.WriteFile(filepath.Join(outside, "secret"), secret, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(secret)
	m := evidence.Manifest{Version: 1, Scope: evidence.Scope{Host: "host", Team: "team", Hitch: "hitch", Assignment: "task"}, Target: "code", Records: []evidence.Record{
		{ID: "code", Kind: "code", Provenance: "manual_import", Code: &evidence.Code{Repository: "example/project", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("a", 40), CommitObject: "object", Run: "missing"}},
		{ID: "object", Kind: "artifact", Provenance: "manual_import", Artifact: &evidence.Artifact{Path: "escape", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(secret)), Status: "available"}},
	}}
	path := filepath.Join(root, "chain.json")
	data, _ := json.Marshal(m)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if status := run([]string{"evidence", "verify", "--assignment", "task", path}, strings.NewReader(""), &out, &stderr); status != exitUnknown || !strings.Contains(out.String(), "artifact unreadable") {
		t.Fatalf("status %d output %s err %s", status, &out, &stderr)
	}
	if strings.Contains(out.String()+stderr.String(), string(secret)) {
		t.Fatal("secret leaked")
	}
	if err := os.WriteFile(path, []byte(`{"version":"SENTINEL-CREDENTIAL"}`), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	run([]string{"evidence", "verify", "--assignment", "task", path}, strings.NewReader(""), &out, &stderr)
	if strings.Contains(out.String()+stderr.String(), string(secret)) {
		t.Fatal("decoder leaked raw input")
	}
}

func TestEvidenceDispatchNeverResolvesSettings(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chain.json")
	m := evidence.Manifest{Version: 1, Scope: evidence.Scope{Host: "host", Team: "team", Hitch: "hitch", Assignment: "task"}, Target: "missing"}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	trap := func(key string) string { t.Fatalf("settings/state route consulted: %s", key); return "" }
	var out, stderr bytes.Buffer
	cmd := command{stdout: &out, stderr: &stderr, getenv: trap, lookupEnv: func(key string) (string, bool) { trap(key); return "", false }, getwd: func() (string, error) { t.Fatal("working-directory settings consulted"); return "", nil }, userHomeDir: func() (string, error) { t.Fatal("home settings consulted"); return "", nil }}
	if err := cmd.execute([]string{"evidence", "show", "--assignment", "task", path}); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceUsageProjectsMetadataAtOutputBoundary(t *testing.T) {
	const sentinel = "INERT_COMMAND_ARGUMENT_SENTINEL"
	root := t.TempDir()
	m := evidence.Manifest{Version: 1, Scope: evidence.Scope{Host: "host", Team: "team", Hitch: "hitch", Assignment: "task"}, Records: []evidence.Record{{ID: "attempt", Kind: "usage", Provenance: "manual_import", Usage: &evidence.Usage{Summary: "summary", Charges: "charges"}}}}
	writeArtifact := func(id, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, id+".json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(data))
		r := evidence.Record{ID: id, Kind: "artifact", Provenance: "manual_import", Artifact: &evidence.Artifact{Path: id + ".json", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), Status: "available"}}
		for i := range m.Records {
			if m.Records[i].ID == id {
				m.Records[i] = r
				return
			}
		}
		m.Records = append(m.Records, r)
	}
	writeArtifact("summary", `{"session":{"id":"session"},"boundaries":[{"type":"task_started","utc":"2026-11-02T01:00:00Z"},{"type":"task_complete","utc":"2026-11-02T01:01:00Z"}],"generation_ids":["g"],"commands":[{"arguments":"`+sentinel+`"}],"unknown":"`+sentinel+`"}`)
	writeArtifact("charges", `{"native_session":"session","phase":"first","complete":true,"rows":[{"id":"g","requested_id":"g","http_status":200,"created_at":"2026-11-02T01:00:01Z","model":"test/model","provider_name":"test","total_cost_usd":"0.001","unknown":"`+sentinel+`"}]}`)
	path := filepath.Join(root, "usage.json")
	for _, broken := range []bool{false, true} {
		if broken {
			writeArtifact("summary", `{"session":{"id":{"commands":"`+sentinel+`"}}}`)
		}
		data, _ := json.Marshal(m)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"verify", "show"} {
			var out, stderr bytes.Buffer
			status := run([]string{"evidence", mode, "--require", "usage", "--assignment", "task", path}, strings.NewReader(""), &out, &stderr)
			want := exitOK
			if broken && mode == "verify" {
				want = exitRefused
			}
			if status != want || strings.Contains(out.String()+stderr.String(), sentinel) {
				t.Fatalf("status %d want %d, safe output required", status, want)
			}
			var result evidence.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !broken && (result.Usage.ActualUSD == nil || *result.Usage.ActualUSD != "0.001") {
				t.Fatal("supported charge lost")
			}
			if broken && result.Status != "fail" {
				t.Fatal("invalid metadata accepted")
			}
		}
	}
}
