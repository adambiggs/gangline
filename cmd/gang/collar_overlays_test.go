package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/harness"
)

func TestBundledCollarOverlayReplacesSelectedBands(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`package collars
collar: {
	context_bands: {
		"*": [
			{name: "early", at: 0.10, message: "custom {{agent_name}}"},
		]
	}
	primitives: {wedge: {params: {busy: "custom busy"}}}
}`)
	if err := os.WriteFile(filepath.Join(dir, "claude-code.cue"), data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadCollar("claude-code", settings{CollarDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if c.Launch.Command != "claude" || len(c.ContextBands["*"]) != 1 || c.ContextBands["*"][0].Message != "custom {{agent_name}}" || len(c.ContextBands["*haiku*"]) != 2 || c.Primitives.Wedge.Params["busy"] != "custom busy" || c.Primitives.Wedge.Params["after"] != "5m" {
		t.Fatalf("incomplete overlay: %+v", c)
	}
}

func TestOverlayPrimitiveNameReplacesParameters(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`collar: {models: {catalog: {name: "claude-help-models", params: {command: "claude"}}}}`)
	if err := os.WriteFile(filepath.Join(dir, "codex.cue"), data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadCollar("codex", settings{CollarDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if c.Models.Catalog.Name != "claude-help-models" || c.Models.Catalog.Params["command"] != "claude" || c.Models.Catalog.Params["args"] != "" {
		t.Fatalf("changed primitive retained bundled parameters: %+v", c.Models.Catalog)
	}
}

func TestOverlayRejectsUnknownMessageToken(t *testing.T) {
	dir := t.TempDir()
	for _, token := range []string{"{{used_precent}}", "{{Used_tokens}}", "{{ used_tokens }}"} {
		data := []byte(`collar: {context_bands: {"*": [{name: "early", at: 0.5, message: "` + token + `"}]}}`)
		if err := os.WriteFile(filepath.Join(dir, "codex.cue"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCollar("codex", settings{CollarDir: dir}); err == nil || !strings.Contains(err.Error(), strings.Trim(token, "{}")) {
			t.Fatalf("unknown message token %q: %v", token, err)
		}
	}
}

func TestOverlayRejectsCUEBytesAsString(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`collar: {launch: {command: 'claude'}}`)
	if err := os.WriteFile(filepath.Join(dir, "claude-code.cue"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCollar("claude-code", settings{CollarDir: dir}); err == nil {
		t.Fatal("CUE bytes passed string validation")
	}
}

func TestOverlayRejectsExactCUEThresholdAboveOne(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`collar: {context_bands: {"*": [{name: "early", at: 1.00000000000000001}]}}`)
	if err := os.WriteFile(filepath.Join(dir, "codex.cue"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCollar("codex", settings{CollarDir: dir}); err == nil {
		t.Fatal("CUE threshold above one passed after number conversion")
	}
}

func TestCustomCollarStillRequiresWholeDefinition(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude-code.cue"), []byte(`collar: {context_bands: {"*": [{name: "only", at: 0.5}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCollar("claude-code", settings{CollarDir: dir}); err != nil {
		t.Fatalf("partial bundled collar: %v", err)
	}
	base, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	base.Name = "custom"
	data, err := json.Marshal(map[string]any{"collar": base})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "custom.cue"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCollar("custom", settings{CollarDir: dir}); err != nil {
		t.Fatalf("whole custom collar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partial.cue"), []byte(`collar: {name: "partial"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCollar("partial", settings{CollarDir: dir}); err == nil {
		t.Fatal("partial custom collar passed validation")
	}
}

func TestBundledCollarOverlayValidatesUnifiedResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex.cue")
	if err := os.WriteFile(path, []byte(`collar: {context_bands: {"*": [{name: "valid", at: 0.5}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCollar("codex", settings{CollarDir: dir}); err != nil {
		t.Fatalf("valid partial overlay: %v", err)
	}
	for _, data := range []string{
		`collar: {context_bands: {"*": [{name: "red", at: 1.2}]}}`,
		`collar: {context_bands: {"*": [{name: "first", at: 0.8}, {name: "second", at: 0.7}]}}`,
		`collar: {unknown: true}`,
	} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCollar("codex", settings{CollarDir: dir}); err == nil || !strings.Contains(err.Error(), "collar") {
			t.Fatalf("invalid overlay %q: %v", data, err)
		}
	}
}
