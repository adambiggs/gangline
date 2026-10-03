package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// upgradeFixture is an install root whose install.sh records the confirmation
// mode gang hands it and exits with a scripted status.
func upgradeFixture(t *testing.T, status string, terminal bool) (command, string) {
	t.Helper()
	root := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' \"$GANGLINE_UPGRADE_CONFIRM\" >\"$GANGLINE_HOME/confirm\"\nexit " + status + "\n"
	if err := os.WriteFile(filepath.Join(root, "install.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := command{
		stdin:         strings.NewReader(""),
		stdout:        &stdout,
		stderr:        &stderr,
		terminalInput: func() bool { return terminal },
		getenv: func(name string) string {
			if name == "GANGLINE_HOME" {
				return root
			}
			return ""
		},
		userHomeDir: func() (string, error) { return t.TempDir(), nil },
	}
	return cmd, root
}

func upgradeConfirmMode(t *testing.T, root string) string {
	t.Helper()
	mode, err := os.ReadFile(filepath.Join(root, "confirm"))
	if err != nil {
		t.Fatalf("installer did not run: %v", err)
	}
	return string(mode)
}

func TestUpgradeHandsInstallerItsConfirmation(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		terminal bool
		want     string
	}{
		{"yes", []string{"--yes"}, false, "yes"},
		{"y", []string{"-y"}, true, "yes"},
		{"terminal", nil, true, "ask"},
		{"no terminal", nil, false, "refuse"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd, root := upgradeFixture(t, "0", test.terminal)
			if err := cmd.upgrade(test.args); err != nil {
				t.Fatalf("upgrade error = %v", err)
			}
			if got := upgradeConfirmMode(t, root); got != test.want {
				t.Fatalf("confirmation = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUpgradeReportsInstallerRefusal(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		terminal bool
		refused  bool
		want     string
	}{
		{"no terminal", nil, false, true, "--yes"},
		{"declined", nil, true, true, "upgrade cancelled; no changes made"},
		{"yes", []string{"--yes"}, false, false, "exit status 3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd, _ := upgradeFixture(t, "3", test.terminal)
			err := cmd.upgrade(test.args)
			var commandErr commandError
			refused := errors.As(err, &commandErr) && commandErr.status == exitRefused
			if err == nil || refused != test.refused || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("upgrade error = %v, refused = %v", err, refused)
			}
		})
	}
}

func TestUpgradeRefusesAnInstallerThatCannotConfirm(t *testing.T) {
	cmd, root := upgradeFixture(t, "0", false)
	script := "#!/bin/sh\ntouch \"$GANGLINE_HOME/ran\"\n"
	if err := os.WriteFile(filepath.Join(root, "install.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	err := cmd.upgrade([]string{"--yes"})
	var commandErr commandError
	if !errors.As(err, &commandErr) || commandErr.status != exitRefused || !strings.Contains(err.Error(), "cannot confirm") {
		t.Fatalf("upgrade error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ran")); !os.IsNotExist(err) {
		t.Fatalf("installer ran: %v", err)
	}
}
