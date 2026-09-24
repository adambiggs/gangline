package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	if len(args) > 0 && args[0] == "--widget" {
		return cmd.contextWidget(args[1:])
	}
	name, err := observationName(args, "context")
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
			return commandError{status: exitUnknown, text: valueOr(r.Reason, "native context has not been observed")}
		}
		bandName := "none"
		if band := harness.ActiveContextBand(c, r.Model, harness.ContextReading{Used: *r.Used, Limit: *r.Limit, Percent: *r.Percent / 100}); band != nil {
			bandName = band.Name
		}
		_, err = fmt.Fprintf(cmd.stdout, "%s\t%s/%s\t%.0f%%\t%s\n", a.Name, compactTokenCount(*r.Used), compactTokenCount(*r.Limit), *r.Percent, bandName)
		return err
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
		return commandError{status: exitUnknown, text: err.Error()}
	}
	model := ""
	if c.Models.Selected != nil {
		model, _ = harness.ReadSelectedModel(*c.Models.Selected, screen)
	}
	bandName := "none"
	if band := harness.ActiveContextBand(c, model, r); band != nil {
		bandName = band.Name
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%s/%s\t%.0f%%\t%s\n", a.Name, compactTokenCount(r.Used), compactTokenCount(r.Limit), r.Percent*100, bandName)
	return err
}
func (cmd command) limits(args []string) error {
	if len(args) > 0 && args[0] == "-c" {
		if len(args) != 2 {
			return usageError("limits -c: expected one collar and no agent")
		}
		s, err := cmd.settings()
		if err != nil {
			return err
		}
		c, err := loadCollar(args[1], s)
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
	name, err := observationName(args, "limits")
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
	composer := false
	if len(args) > 0 && args[0] == "--composer" {
		composer = true
		args = args[1:]
	}
	if len(args) > 2 {
		return usageError("capture: too many arguments")
	}
	name := ""
	if len(args) > 0 {
		name = args[0]
		if err := validateAgentName(name); err != nil {
			return err
		}
	}
	lines := 0
	if len(args) == 2 {
		if composer {
			return usageError("capture --composer does not accept a line count")
		}
		parsed, err := strconv.Atoi(args[1])
		if err != nil || parsed <= 0 {
			return usageError("capture: lines must be positive")
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
	collar := ""
	if name != "" || composer {
		a, err := run.resolve(name)
		if err != nil {
			return err
		}
		pane, collar = a.Pane, a.Collar
	}
	if pane == "" {
		return refuseError("capture without a name requires a tmux pane")
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(pane))
	if err != nil {
		return err
	}
	text := screen.Text(lines)
	if composer {
		c, err := loadCollar(collar, run.settings)
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
func (cmd command) contextWidget(args []string) error {
	if len(args) != 1 {
		return usageError("context --widget: expected NAME or off")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	b, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	if args[0] == "off" {
		return b.ContextWidget(context.Background(), "", "")
	}
	a, err := run.resolve(args[0])
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
