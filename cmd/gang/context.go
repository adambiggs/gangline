package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func observationName(args []string, name string) (string, error) {
	if len(args) > 1 {
		return "", usageError("%s: expected at most one agent", name)
	}
	if len(args) == 0 {
		return "", nil
	}
	if err := validateAgentName(args[0]); err != nil {
		return "", err
	}
	return args[0], nil
}
func (cmd command) context(args []string) error {
	widget, clear, machine := "", false, false
	flags := boundFlagSet("context", map[string]any{"widget": &widget, "clear": &clear, "json": &machine})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("context: %v", err)
	}
	if machine && (clear || flagWasSet(flags, "widget")) {
		return usageError("context --json: does not combine with --widget or --clear")
	}
	if clear {
		if len(positionals) != 0 || flagWasSet(flags, "widget") {
			return usageError("context --clear: takes no agent or --widget")
		}
		return cmd.contextWidget("")
	}
	if flagWasSet(flags, "widget") {
		if len(positionals) != 0 {
			return usageError("context --widget: unexpected argument %q", positionals[0])
		}
		if err := validateAgentName(widget); err != nil {
			return err
		}
		return cmd.contextWidget(widget)
	}
	name, err := observationName(positionals, "context")
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	a, err = run.latest(a)
	if err != nil {
		return err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if c.Primitives.Telemetry != nil {
		r := a.Native.Context
		if r.Status != "observed" || r.Used == nil || r.Limit == nil || r.Percent == nil {
			return cmd.unknownContext(machine, contextJSON{Name: a.Name, Model: r.Model, Source: "telemetry", Reason: valueOr(r.Reason, "native context has not been observed")})
		}
		band := harness.ActiveContextBand(c, r.Model, harness.ContextReading{Used: *r.Used, Limit: *r.Limit, Percent: *r.Percent / 100})
		return cmd.writeContext(machine, contextJSON{Name: a.Name, Model: r.Model, Used: r.Used, Limit: r.Limit, Percent: r.Percent, Band: bandName(band), Source: "telemetry"})
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	r, err := harness.ReadContext(c.Primitives.Context, screen)
	if err != nil {
		return cmd.unknownContext(machine, contextJSON{Name: a.Name, Source: "screen", Reason: err.Error()})
	}
	model := ""
	if c.Models.Selected != nil {
		model, _ = harness.ReadSelectedModel(*c.Models.Selected, screen)
	}
	band := harness.ActiveContextBand(c, model, r)
	percent := r.Percent * 100
	return cmd.writeContext(machine, contextJSON{Name: a.Name, Model: model, Used: &r.Used, Limit: &r.Limit, Percent: &percent, Band: bandName(band), Source: "screen"})
}

func bandName(band *harness.ContextBand) *string {
	if band == nil {
		return nil
	}
	return &band.Name
}

// writeContext prints an observed reading. Percent is 0-100 in both forms.
func (cmd command) writeContext(machine bool, r contextJSON) error {
	r.Status = "observed"
	if machine {
		return writeJSON(cmd.stdout, r)
	}
	band := "none"
	if r.Band != nil {
		band = *r.Band
	}
	_, err := fmt.Fprintf(cmd.stdout, "%s\t%s/%s\t%.0f%%\t%s\n", r.Name, compactTokenCount(*r.Used), compactTokenCount(*r.Limit), *r.Percent, band)
	return err
}

// unknownContext reports a reading that could not be observed. The JSON form
// still prints, and the command exits unknown either way.
func (cmd command) unknownContext(machine bool, r contextJSON) error {
	r.Reason = cmd.operatorText(r.Reason)
	if machine {
		r.Status = "unknown"
		if err := writeJSON(cmd.stdout, r); err != nil {
			return err
		}
	}
	return commandError{status: exitUnknown, text: r.Reason}
}
func (cmd command) limits(args []string) error {
	collar := ""
	flags := boundFlagSet("limits", map[string]any{"c": &collar, "collar": &collar})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("limits: %v", err)
	}
	if flagWasSet(flags, "c") || flagWasSet(flags, "collar") {
		if len(positionals) != 0 {
			return usageError("limits --collar: expected one collar and no agent")
		}
		s, err := cmd.settings()
		if err != nil {
			return err
		}
		c, err := loadCollar(collar, s)
		if err != nil {
			return err
		}
		ctx, cancel := cmd.timeout(operationTimeout)
		defer cancel()
		limits, err := harness.QueryProviderLimits(ctx, c)
		if err != nil {
			return commandError{status: exitUnknown, text: err.Error()}
		}
		for _, w := range limits {
			if _, err := fmt.Fprintf(cmd.stdout, "%s\t%.0f%%\t%s\n", w.Label, w.UsedPercent, time.Unix(w.ResetAt, 0).UTC().Format(time.RFC3339)); err != nil {
				return err
			}
		}
		return nil
	}
	name, err := observationName(positionals, "limits")
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	a, err = run.latest(a)
	if err != nil {
		return err
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	if c.Primitives.Telemetry != nil {
		r := a.Native.Limits
		if r.Status != "observed" {
			return commandError{status: exitUnknown, text: valueOr(r.Reason, "native provider limits have not been observed")}
		}
		for _, w := range r.Limits {
			if _, err := fmt.Fprintf(cmd.stdout, "%s\t%.0f%%\t%s\n", w.Label, w.UsedPercent, time.Unix(w.ResetAt, 0).UTC().Format(time.RFC3339)); err != nil {
				return err
			}
		}
		return nil
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	readings, err := harness.ReadProviderLimits(c.Primitives.ProviderLimits, screen, cmd.now())
	if err != nil {
		return commandError{status: exitUnknown, text: err.Error()}
	}
	for _, r := range readings {
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%d%%\t%s\n", r.Label, r.UsedPercent, r.ResetAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}
func (cmd command) capture(args []string) error {
	composer, count := false, ""
	flags := boundFlagSet("capture", map[string]any{"n": &count, "lines": &count, "composer": &composer})
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("capture: %v", err)
	}
	if len(positionals) > 1 {
		return usageError("capture: unexpected argument %q", positionals[1])
	}
	name := ""
	if len(positionals) == 1 {
		name = positionals[0]
		if err := validateAgentName(name); err != nil {
			return err
		}
	}
	lines := 0
	if flagWasSet(flags, "n") || flagWasSet(flags, "lines") {
		if composer {
			return usageError("capture --composer does not accept --lines")
		}
		parsed, err := strconv.Atoi(count)
		if err != nil || parsed <= 0 {
			return usageError("capture: --lines must be a positive integer")
		}
		lines = parsed
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	pane := cmd.environment("TMUX_PANE")
	var record *core.Agent
	if name != "" || composer {
		a, err := run.resolve(name)
		if err != nil {
			return err
		}
		if a.Pane == "" {
			return refuseError("capture: %s has no pane to capture: it is %s. gang status %s shows its record", a.Name, a.Status, a.Name)
		}
		record = &a
	}
	if record == nil && pane == "" {
		return refuseError("capture without a name requires a tmux pane")
	}
	var screen substrate.Screen
	if record != nil {
		screen, err = run.captureAgentPane(context.Background(), b, *record)
	} else {
		screen, err = b.Capture(context.Background(), substrate.PaneID(pane))
	}
	if err != nil {
		return err
	}
	text := screen.Text(lines)
	if composer {
		c, err := loadCollar(record.Collar, run.settings)
		if err != nil {
			return err
		}
		r, err := harness.ReadComposer(c.Primitives.Composer, screen)
		if err != nil {
			return commandError{status: exitUnknown, text: err.Error()}
		}
		text = r.Text
	}
	if text != "" {
		_, err = fmt.Fprintln(cmd.stdout, text)
	}
	return err
}

// contextWidget shows name's context in the tmux widget, or clears the widget
// when name is empty.
func (cmd command) contextWidget(name string) error {
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	b, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	if name == "" {
		return b.ContextWidget(context.Background(), "", "")
	}
	a, err := run.resolve(name)
	if err != nil {
		return err
	}
	a, err = run.latest(a)
	if err != nil {
		return err
	}
	return b.ContextWidget(context.Background(), string(a.ID), contextWidgetText(string(a.Name), a.Native.Context))
}

// Compact only presentation; native readings retain their exact token counts.
func compactTokenCount(n int64) string {
	switch {
	case n >= 999500:
		return strings.TrimSuffix(strconv.FormatFloat(float64(n)/1000000, 'f', 1, 64), ".0") + "M"
	case n >= 1000:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

func contextUsageText(used, limit int64, percent float64) string {
	return fmt.Sprintf("%s/%s (%.0f%%)", compactTokenCount(used), compactTokenCount(limit), percent)
}
