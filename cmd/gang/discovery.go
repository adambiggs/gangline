package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	gangline "github.com/adambiggs/gangline"
	"github.com/adambiggs/gangline/harness"
)

func (cmd command) collars(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("collars takes no arguments")
	}
	names, err := harness.EmbeddedCollarNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, err := fmt.Fprintln(cmd.stdout, name); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) collar(arguments []string) error {
	if len(arguments) != 2 || arguments[0] != "check" {
		return usageError("collar: expected 'check NAME'")
	}
	return pending("collar check")
}

func (cmd command) roles(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("roles takes no arguments")
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	shipped, err := gangline.RoleNames()
	if err != nil {
		return err
	}
	type role struct {
		name   string
		source string
	}
	roles := make(map[string]role, len(shipped))
	for _, name := range shipped {
		roles[name] = role{name: name, source: "shipped"}
	}
	operatorDir := filepath.Join(settings.ConfigDir, "roles")
	entries, readErr := os.ReadDir(operatorDir)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("list operator roles: %w", readErr)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(operatorDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read role %q: %w", name, err)
		}
		if len(data) == 0 || !utf8.Valid(data) {
			return fmt.Errorf("role %q must be non-empty UTF-8", name)
		}
		roles[name] = role{name: name, source: "operator"}
	}
	names := make([]string, 0, len(roles))
	for name := range roles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", name, roles[name].source); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) config(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("config takes no arguments")
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	rows := [][2]string{
		{"GANG_SESSION", settings.Session},
		{"GANG_COLLAR", settings.Collar},
		{"GANG_STATE_ROOT", settings.StateRoot},
		{"GANG_CONFIG_DIR", settings.ConfigDir},
		{"GANG_TMUX_SOCKET", valueOr(settings.Socket, "default")},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(cmd.stdout, "%s=%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	return nil
}
