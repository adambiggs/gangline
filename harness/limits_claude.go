package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// queryClaudeUsage runs Claude Code's /usage as a print-mode local command,
// which serves the account usage the native client caches in its global
// configuration, fetching it again when the cache is not recent. The result
// must report no model turn, so a prompt the model answered is never taken
// for a reading. The cache's fetch time is the reading's time.
func queryClaudeUsage(ctx context.Context, collar Collar) (measuredLimits, error) {
	env := func(key string) string {
		if value, ok := collar.Launch.Env[key]; ok {
			return value
		}
		return os.Getenv(key)
	}
	config := ""
	if dir := env("CLAUDE_CONFIG_DIR"); dir != "" {
		config = filepath.Join(dir, ".claude.json")
	} else if home := env("HOME"); home != "" {
		config = filepath.Join(home, ".claude.json")
	} else {
		return measuredLimits{}, fmt.Errorf("%s limits query: native configuration directory is unknown", collar.Name)
	}
	if err := ctx.Err(); err != nil {
		return measuredLimits{}, fmt.Errorf("%s limits query: %w", collar.Name, err)
	}
	child := exec.CommandContext(ctx, collar.Launch.Command, "-p", "/usage", "--no-session-persistence", "--output-format", "json")
	child.Env = os.Environ()
	for key, value := range collar.Launch.Env {
		child.Env = append(child.Env, key+"="+value)
	}
	// Print mode skips the folder-trust dialog, so the directory's project
	// settings and hooks would run unasked. A private empty directory has
	// none, and no other process can add them.
	dir, err := os.MkdirTemp("", "gangline-usage-")
	if err != nil {
		return measuredLimits{}, fmt.Errorf("%s limits query: %w", collar.Name, err)
	}
	defer os.RemoveAll(dir)
	child.Dir = dir
	stderr, err := newLimitsStderr()
	if err != nil {
		return measuredLimits{}, fmt.Errorf("%s limits query stderr: %w", collar.Name, err)
	}
	defer stderr.Close()
	child.Stderr = stderr
	output, err := child.Output()
	diagnostic, stderrErr := readLimitsStderr(stderr)
	if err := limitsQueryError(collar.Name, ctx.Err(), stderrErr, err, diagnostic); err != nil {
		return measuredLimits{}, err
	}
	var result struct {
		IsError *bool  `json:"is_error"`
		Turns   *int   `json:"num_turns"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return measuredLimits{}, fmt.Errorf("decode %s /usage result: %w", collar.Name, err)
	}
	if result.IsError == nil || *result.IsError || result.Subtype != "success" {
		return measuredLimits{}, fmt.Errorf("%s /usage did not succeed", collar.Name)
	}
	if result.Turns == nil || *result.Turns != 0 {
		return measuredLimits{}, fmt.Errorf("%s /usage ran a model turn instead of the local usage command", collar.Name)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		return measuredLimits{}, fmt.Errorf("read %s usage cache: %w", collar.Name, err)
	}
	return parseClaudeUsageCache(data)
}

type claudeUsageLimit struct {
	Kind    string   `json:"kind"`
	Group   string   `json:"group"`
	Percent *float64 `json:"percent"`
	Reset   string   `json:"resets_at"`
	Scope   *struct {
		Model *struct {
			Name string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// parseClaudeUsageCache reads the cached account usage and when it was
// fetched. Native group names fix each window's duration.
func parseClaudeUsageCache(data []byte) (measuredLimits, error) {
	var config struct {
		Cache *struct {
			FetchedAt   *int64 `json:"fetchedAtMs"`
			Utilization *struct {
				Limits []claudeUsageLimit `json:"limits"`
			} `json:"utilization"`
		} `json:"cachedUsageUtilization"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return measuredLimits{}, fmt.Errorf("decode native usage cache: %w", err)
	}
	cache := config.Cache
	if cache == nil || cache.FetchedAt == nil || cache.Utilization == nil {
		return measuredLimits{}, fmt.Errorf("native usage cache carries no fetched utilization")
	}
	var limits []LimitWindow
	for _, limit := range cache.Utilization.Limits {
		label, minutes := "", 0
		switch limit.Group {
		case "session":
			label, minutes = "five_hour", 300
		case "weekly":
			label, minutes = "seven_day", 10080
		default:
			continue
		}
		if limit.Kind == "weekly_scoped" {
			if limit.Scope == nil || limit.Scope.Model == nil || limit.Scope.Model.Name == "" {
				continue
			}
			label += "/" + strings.ToLower(limit.Scope.Model.Name)
		} else if limit.Kind != "session" && limit.Kind != "weekly_all" {
			label += "/" + limit.Kind
		}
		reset, err := time.Parse(time.RFC3339Nano, limit.Reset)
		if limit.Percent == nil || *limit.Percent < 0 || err != nil {
			return measuredLimits{}, fmt.Errorf("native usage cache carries an incomplete or invalid %s window", label)
		}
		limits = append(limits, LimitWindow{Label: label, UsedPercent: *limit.Percent, ResetAt: reset.Unix(), WindowMinutes: minutes})
	}
	if len(limits) == 0 {
		return measuredLimits{}, fmt.Errorf("native usage cache exposes no provider-limit windows")
	}
	return measuredLimits{windows: limits, at: time.UnixMilli(*cache.FetchedAt)}, nil
}
