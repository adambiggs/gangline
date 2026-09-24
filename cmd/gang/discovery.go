package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/internal/prose"
)

var collarNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (cmd command) collars(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("collars takes no arguments")
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	names, err := collarNames(settings)
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

func (cmd command) models(arguments []string) error {
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	name, err := parseCollarFlags("models", arguments, settings.Collar)
	if err != nil {
		if err == flag.ErrHelp {
			return cmd.printHelp("models")
		}
		return usageError("models: %v", err)
	}
	collar, err := loadCollar(name, settings)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	catalog, err := harness.DiscoverModels(ctx, collar)
	if err != nil {
		return err
	}
	for _, model := range catalog.Models {
		efforts := strings.Join(model.Efforts, ",")
		if efforts == "" {
			efforts = "-"
		}
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\n", model.ID, efforts); err != nil {
			return err
		}
	}
	if !catalog.Complete {
		_, err = fmt.Fprintln(cmd.stdout, "(the harness accepts full model ids that its catalog does not enumerate)")
	}
	return err
}

func collarNames(settings settings) ([]string, error) {
	embedded, err := harness.EmbeddedCollarNames()
	if err != nil {
		return nil, err
	}
	unique := make(map[string]bool, len(embedded))
	for _, name := range embedded {
		unique[name] = true
	}
	if settings.CollarDir != "" {
		entries, err := os.ReadDir(settings.CollarDir)
		if err != nil {
			return nil, fmt.Errorf("list custom collars: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".cue" {
				name := strings.TrimSuffix(entry.Name(), ".cue")
				if !collarNamePattern.MatchString(name) {
					return nil, fmt.Errorf("custom collar filename %q is invalid", entry.Name())
				}
				unique[name] = true
			}
		}
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func loadCollar(name string, settings settings) (harness.Collar, error) {
	if !collarNamePattern.MatchString(name) {
		return harness.Collar{}, usageError("invalid collar name %q", name)
	}
	if settings.CollarDir != "" {
		filename := filepath.Join(settings.CollarDir, name+".cue")
		data, err := os.ReadFile(filename)
		if err == nil {
			collar, err := harness.LoadCollar(filename, data)
			if err != nil {
				return harness.Collar{}, err
			}
			if collar.Name != name {
				return harness.Collar{}, fmt.Errorf("collar %q declares name %q", filename, collar.Name)
			}
			return collar, nil
		}
		if !os.IsNotExist(err) {
			return harness.Collar{}, fmt.Errorf("read collar %q: %w", name, err)
		}
	}
	return harness.EmbeddedCollar(name)
}

func (cmd command) roles(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("roles takes no arguments")
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	shipped, err := prose.RoleNames()
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
		{"GANG_COLLARS", valueOr(settings.CollarDir, "unset")},
		{"GANG_LAUNCH_ARGS", valueOr(settings.LaunchArgsJSON, "unset")},
		{"GANG_CAPACITY_TIMEOUT", settings.CapacityTimeout.String()},
		{"GANG_STATE_ROOT", settings.StateRoot},
		{"GANG_CONFIG_DIR", settings.ConfigDir},
		{"GANG_TMUX_SOCKET", valueOr(settings.Socket, "default")},
	}
	for _, row := range rows {
		origin := valueOr(settings.Origins[row[0]], "default")
		if _, err := fmt.Fprintf(cmd.stdout, "%s=%s\t%s\n", row[0], row[1], origin); err != nil {
			return err
		}
	}
	return nil
}
