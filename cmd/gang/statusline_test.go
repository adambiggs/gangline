package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStatuslineInstallUsesNativeClaudeConfigDirectory(t *testing.T) {
	for _, test := range []struct {
		name       string
		configured string
		want       string
	}{
		{name: "configured", configured: "alternate-claude", want: filepath.Join("alternate-claude", "settings.json")},
		{name: "default", want: filepath.Join(".claude", "settings.json")},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			env := map[string]string{}
			if test.configured != "" {
				env["CLAUDE_CONFIG_DIR"] = filepath.Join(home, test.configured)
			}
			out := &bytes.Buffer{}
			cmd := command{stdout: out, getenv: func(k string) string { return env[k] }, userHomeDir: func() (string, error) { return home, nil }}
			want := filepath.Join(home, test.want)
			if err := cmd.statusline([]string{"--install"}); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != "Installed status line in "+want+"\n" {
				t.Fatalf("first install output = %q", got)
			}
			if _, err := os.Stat(want); err != nil {
				t.Fatal(err)
			}
			if test.configured != "" {
				if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
					t.Fatalf("default Claude directory touched while CLAUDE_CONFIG_DIR is set: %v", err)
				}
			}
			out.Reset()
			if err := cmd.statusline([]string{"--install"}); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != "Kept existing statusLine setting in "+want+"\n" {
				t.Fatalf("second install output = %q", got)
			}
		})
	}
}
