package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func memoryReader(files map[string][]byte) func(string, int64) ([]byte, error) {
	return func(path string, _ int64) ([]byte, error) {
		b, ok := files[path]
		if !ok {
			return nil, errors.New("private missing path")
		}
		return b, nil
	}
}

func TestCollectRestoresChainAndOpaqueInputs(t *testing.T) {
	f := fixtureChain()
	opaque := []byte("{\"type\":\"unknown_future_event\",\"payload\":\"PRIVATE-SENTINEL\"}\n")
	f.artifact("opaque", opaque)
	f.add("withheld", "artifact", &Artifact{Status: "withheld", Path: "must-not-read"})
	f.add("unavailable", "artifact", &Artifact{Status: "unavailable", Path: "must-not-read"})
	before, _ := json.Marshal(f.m)
	copied := map[string][]byte{}
	read := func(path string, size int64) ([]byte, error) {
		if path == "must-not-read" {
			t.Fatal("opened withheld/unavailable artifact")
		}
		return memoryReader(f.files)(path, size)
	}
	m, stats, err := Collect(f.m, "assignment", read, func(path string, b []byte) error { copied[path] = bytes.Clone(b); return nil })
	if err != nil || stats.Status != "collected" || stats.Artifacts != len(f.files) || stats.Withheld != 1 || stats.Unavailable != 1 {
		t.Fatalf("stats %+v err %v", stats, err)
	}
	after, _ := json.Marshal(f.m)
	if !bytes.Equal(before, after) {
		t.Fatal("source manifest changed")
	}
	if !bytes.Equal(copied["artifacts/opaque"], opaque) {
		t.Fatal("opaque record changed")
	}
	f.files = nil // Only bundle bytes can satisfy the reconstructed chain.
	got := Verify(m, "assignment", "ci", memoryReader(copied))
	if got.Status != "pass" {
		t.Fatalf("restored chain: %+v", got)
	}
	for _, source := range got.Sources {
		if !strings.HasPrefix(source.Path, "artifacts/") {
			t.Fatal("original locator retained")
		}
	}
	for _, r := range m.Records {
		if r.ID == "withheld" || r.ID == "unavailable" {
			if r.Artifact.Path != "must-not-read" {
				t.Fatal("withheld metadata rewritten")
			}
		}
	}
}

func TestCollectFailureRetainsInputsAndReturnsNoManifest(t *testing.T) {
	for _, mode := range []string{"changed", "missing", "write", "assignment", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			f := fixtureChain()
			assignment := "assignment"
			switch mode {
			case "changed":
				f.files["native.json"] = []byte("PRIVATE-SENTINEL")
			case "missing":
				delete(f.files, "native.json")
			case "assignment":
				assignment = "other"
			case "invalid":
				f.get("git").Artifact.Status = "unknown"
			}
			manifestBefore, _ := json.Marshal(f.m)
			filesBefore, _ := json.Marshal(f.files)
			writes := 0
			m, stats, err := Collect(f.m, assignment, memoryReader(f.files), func(_ string, _ []byte) error {
				writes++
				if mode == "write" {
					return errors.New("PRIVATE-SENTINEL")
				}
				return nil
			})
			if err == nil || stats.Status != "incomplete" || len(m.Records) != 0 || strings.Contains(err.Error(), "PRIVATE-SENTINEL") {
				t.Fatalf("stats %+v err %v", stats, err)
			}
			manifestAfter, _ := json.Marshal(f.m)
			filesAfter, _ := json.Marshal(f.files)
			if !bytes.Equal(manifestBefore, manifestAfter) || !bytes.Equal(filesBefore, filesAfter) {
				t.Fatal("source changed on failure")
			}
			if (mode == "missing" || mode == "changed") && writes != 1 {
				t.Fatal("did not exercise partial copy")
			}
		})
	}
}

func TestImportRecordsUsesExistingIdentityRules(t *testing.T) {
	f := fixtureChain()
	before, _ := json.Marshal(f.m)
	record := Record{ID: "extra", Kind: "artifact", Provenance: "manual_import", Artifact: &Artifact{Status: "withheld"}}
	data, _ := json.Marshal([]Record{record, f.m.Records[0]})
	m, err := Import(f.m, "assignment", data)
	if err != nil || len(m.Records) != len(f.m.Records)+1 || !reflect.DeepEqual(m.Records[len(m.Records)-1], record) {
		t.Fatalf("import: %v", err)
	}
	after, _ := json.Marshal(f.m)
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
	conflict := f.m.Records[0]
	conflict.Provenance = "manual_import"
	bad, _ := json.Marshal([]Record{conflict})
	for _, input := range [][]byte{bad, []byte(`[{"id":"private","unknown":"PRIVATE-SENTINEL"}]`), []byte(`null`), []byte(`{} `), []byte(`[] []`)} {
		if _, err := Import(f.m, "assignment", input); err == nil || strings.Contains(err.Error(), "PRIVATE-SENTINEL") {
			t.Fatalf("invalid import: %v", err)
		}
	}
	if _, err := Import(f.m, "other", []byte(`[]`)); err == nil {
		t.Fatal("wrong assignment accepted")
	}
}
