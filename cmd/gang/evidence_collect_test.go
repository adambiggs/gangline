//go:build acceptance

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/internal/evidence"
)

func collectionFixture(t *testing.T) (string, evidence.Manifest) {
	t.Helper()
	root := t.TempDir()
	m := evidence.Manifest{Version: 1, Scope: evidence.Scope{Host: "host", Team: "team", Hitch: "hitch", Assignment: "task"}, Target: "missing"}
	for _, name := range []string{"first", "second"} {
		b := []byte("PRIVATE-SENTINEL " + name)
		if err := os.WriteFile(filepath.Join(root, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		m.Records = append(m.Records, evidence.Record{ID: name, Kind: "artifact", Provenance: "manual_import", Artifact: &evidence.Artifact{Path: name, Size: int64(len(b)), SHA256: hex.EncodeToString(sum[:]), Status: "available"}})
	}
	writeCollectionManifest(t, root, m)
	return root, m
}
func writeCollectionManifest(t *testing.T, root string, m evidence.Manifest) {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "chain.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func evidenceCall(t *testing.T, args ...string) (int, []byte) {
	t.Helper()
	var out, stderr bytes.Buffer
	status := run(append([]string{"evidence"}, args...), strings.NewReader(""), &out, &stderr)
	if strings.Contains(out.String()+stderr.String(), "PRIVATE-SENTINEL") {
		t.Fatal("payload echoed")
	}
	return status, bytes.Clone(out.Bytes())
}
func TestEvidenceCollectPublicationAndCleanRestore(t *testing.T) {
	source, _ := collectionFixture(t)
	manifest := filepath.Join(source, "chain.json")
	original, _ := os.ReadFile(manifest)
	parent := t.TempDir()
	bundle := filepath.Join(parent, "bundle")
	trap := func(key string) string { t.Fatalf("settings/state consulted: %s", key); return "" }
	var out bytes.Buffer
	cmd := command{stdout: &out, stderr: &out, getenv: trap, lookupEnv: func(key string) (string, bool) { trap(key); return "", false }, getwd: func() (string, error) { t.Fatal("settings cwd consulted"); return "", nil }, userHomeDir: func() (string, error) { t.Fatal("home consulted"); return "", nil }}
	if err := cmd.execute([]string{"evidence", "collect", "--assignment", "task", manifest, bundle}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status string `json:"status"`
		Copied int    `json:"copied_artifacts"`
	}
	if json.Unmarshal(out.Bytes(), &result) != nil || result.Status != "collected" || result.Copied != 2 {
		t.Fatal("collection did not report its outcome")
	}
	saved, _ := os.ReadFile(filepath.Join(bundle, "source-manifest.json"))
	after, _ := os.ReadFile(manifest)
	if !bytes.Equal(original, saved) || !bytes.Equal(original, after) {
		t.Fatal("original manifest not retained exactly")
	}
	if err := filepath.WalkDir(bundle, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := fs.FileMode(0600)
		if entry.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Errorf("permissions %o want %o", info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	statuses := map[string]int{}
	outputs := map[string][]byte{}
	for _, mode := range []string{"verify", "show"} {
		statuses[mode], outputs[mode] = evidenceCall(t, mode, "--assignment", "task", filepath.Join(bundle, "manifest.json"))
	}
	restored := filepath.Join(parent, "restored")
	if err := os.CopyFS(restored, os.DirFS(bundle)); err != nil {
		t.Fatal(err)
	}
	// Remove both original locations from resolution without deleting evidence.
	if err := os.Rename(source, source+"-retained"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(source+"-retained", source) })
	if err := os.Rename(bundle, bundle+"-retained"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"verify", "show"} {
		status, output := evidenceCall(t, mode, "--assignment", "task", filepath.Join(restored, "manifest.json"))
		if status != statuses[mode] || !bytes.Equal(output, outputs[mode]) {
			t.Fatal("restore changed verification")
		}
	}
	for _, name := range []string{"first", "second"} {
		a, _ := os.ReadFile(filepath.Join(source+"-retained", name))
		b, _ := os.ReadFile(filepath.Join(restored, "artifacts", name))
		if !bytes.Equal(a, b) {
			t.Fatal("restore lost artifact")
		}
	}
	if statuses["verify"] != exitUnknown || statuses["show"] != exitOK {
		t.Fatal("collection falsely upgraded incomplete evidence")
	}
}

func TestEvidenceCollectFailureNeverPublishesOrChangesSource(t *testing.T) {
	for _, mode := range []string{"missing", "changed", "escape", "exists"} {
		t.Run(mode, func(t *testing.T) {
			source, _ := collectionFixture(t)
			path := filepath.Join(source, "second")
			switch mode {
			case "missing":
				if err := os.Rename(path, path+"-retained"); err != nil {
					t.Fatal(err)
				}
			case "changed":
				if err := os.WriteFile(path, []byte("PRIVATE-SENTINEL altered"), 0600); err != nil {
					t.Fatal(err)
				}
			case "escape":
				if err := os.Rename(path, path+"-retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(t.TempDir(), "secret"), path); err != nil {
					t.Fatal(err)
				}
			}
			original, _ := os.ReadFile(filepath.Join(source, "chain.json"))
			first, _ := os.ReadFile(filepath.Join(source, "first"))
			bundle := filepath.Join(t.TempDir(), "bundle")
			if mode == "exists" {
				if err := os.Mkdir(bundle, 0700); err != nil {
					t.Fatal(err)
				}
			}
			status, _ := evidenceCall(t, "collect", "--assignment", "task", filepath.Join(source, "chain.json"), bundle)
			if status == exitOK {
				t.Fatal("bad input accepted")
			}
			if _, err := os.Stat(filepath.Join(bundle, "manifest.json")); !os.IsNotExist(err) {
				t.Fatal("incomplete manifest published")
			}
			after, _ := os.ReadFile(filepath.Join(source, "chain.json"))
			firstAfter, _ := os.ReadFile(filepath.Join(source, "first"))
			if !bytes.Equal(original, after) || !bytes.Equal(first, firstAfter) {
				t.Fatal("source changed")
			}
			if mode != "exists" {
				if b, err := os.ReadFile(filepath.Join(bundle, "artifacts", "first")); err != nil || !bytes.Equal(b, first) {
					t.Fatal("partial copy discarded")
				}
			}
		})
	}
}

func TestEvidenceRecordAndRefusals(t *testing.T) {
	source, m := collectionFixture(t)
	// Imported paths resolve from the manifest, not the records file's directory.
	recordPath := filepath.Join(t.TempDir(), "records.json")
	records, _ := json.Marshal(m.Records[1:])
	if err := os.WriteFile(recordPath, records, 0600); err != nil {
		t.Fatal(err)
	}
	m.Records = m.Records[:1]
	m.Records = append(m.Records, evidence.Record{ID: "private", Kind: "artifact", Provenance: "manual_import", Artifact: &evidence.Artifact{Status: "withheld", Path: "not-present"}})
	writeCollectionManifest(t, source, m)
	bundle := filepath.Join(t.TempDir(), "bundle")
	status, _ := evidenceCall(t, "record", "--assignment", "task", filepath.Join(source, "chain.json"), recordPath, bundle)
	if status != exitOK {
		t.Fatal("record failed")
	}
	saved, err := os.ReadFile(filepath.Join(bundle, "imported-records.json"))
	if err != nil || !bytes.Equal(saved, records) {
		t.Fatal("import bytes not retained")
	}
	if _, err := os.Stat(filepath.Join(bundle, "artifacts", "second")); err != nil {
		t.Fatal("imported artifact absent")
	}
	if _, err := os.Stat(filepath.Join(bundle, "artifacts", "private")); !os.IsNotExist(err) {
		t.Fatal("withheld copied")
	}
	before, _ := os.ReadFile(filepath.Join(bundle, "manifest.json"))
	if status, _ := evidenceCall(t, "collect", "--assignment", "task", filepath.Join(source, "chain.json"), bundle); status == exitOK {
		t.Fatal("overwrite allowed")
	}
	after, _ := os.ReadFile(filepath.Join(bundle, "manifest.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("published bundle changed")
	}
	for _, args := range [][]string{
		{"collect", "--assignment", "task", "--require", "ci", filepath.Join(source, "chain.json"), filepath.Join(t.TempDir(), "unused")},
		{"collect", "--assignment", "other", filepath.Join(source, "chain.json"), filepath.Join(t.TempDir(), "unused")},
		{"record", "--assignment", "task"},
	} {
		if status, _ := evidenceCall(t, args...); status == exitOK {
			t.Fatal("invalid command accepted")
		}
	}
}
