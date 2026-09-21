package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (cmd command) upgrade(arguments []string) error {
	if len(arguments) > 1 || (len(arguments) == 1 && arguments[0] != "--check") {
		return usageError("upgrade accepts only --check")
	}
	home, err := cmd.userHomeDir()
	if err != nil {
		return fmt.Errorf("locate home directory: %w", err)
	}
	installRoot := valueOr(cmd.getenv("GANGLINE_HOME"), filepath.Join(home, ".local", "share", "gangline"))
	installer := filepath.Join(installRoot, "install.sh")
	if info, err := os.Stat(installer); err != nil || info.IsDir() {
		return fmt.Errorf("upgrade requires an installer-managed release at %s", installRoot)
	}
	processArgs := []string{installer}
	if len(arguments) == 1 {
		processArgs = append(processArgs, "--check")
	}
	process := exec.Command("sh", processArgs...)
	process.Stdin = cmd.stdin
	process.Stdout = cmd.stdout
	process.Stderr = cmd.stderr
	process.Env = append(os.Environ(), "GANGLINE_UPGRADE=1", "GANGLINE_HOME="+installRoot)
	if err := process.Run(); err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	return nil
}
