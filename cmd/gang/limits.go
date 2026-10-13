package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

// limitsRow is one provider window as one collar observed it at one time.
type limitsRow struct {
	collar string
	window core.LimitWindow
	at     time.Time
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
		rows, err := cmd.queryLimits(c)
		if err != nil {
			return commandError{status: exitUnknown, text: err.Error()}
		}
		return cmd.writeLimits(rows)
	}
	name, err := observationName(positionals, "limits")
	if err != nil {
		return err
	}
	if name == "" {
		return cmd.allLimits()
	}
	return cmd.agentLimits(name)
}

func (cmd command) queryLimits(c harness.Collar) ([]limitsRow, error) {
	ctx, cancel := cmd.timeout(operationTimeout)
	defer cancel()
	windows, at, err := harness.QueryProviderLimits(ctx, c)
	if err != nil {
		return nil, err
	}
	if at.IsZero() {
		at = cmd.now()
	}
	rows := make([]limitsRow, 0, len(windows))
	for _, w := range windows {
		rows = append(rows, limitsRow{collar: c.Name, at: at, window: core.LimitWindow{Label: w.Label, UsedPercent: w.UsedPercent, ResetAt: w.ResetAt, WindowMinutes: w.WindowMinutes}})
	}
	return rows, nil
}

// allLimits reads every configured collar without an agent of it. Collars are
// queried in parallel. When a query fails, the newest reading a team agent of
// that collar recorded stands in with its age; a collar with neither is
// reported unknown after the others print.
func (cmd command) allLimits() error {
	s, err := cmd.settings()
	if err != nil {
		return err
	}
	names, err := collarNames(s)
	if err != nil {
		return err
	}
	rows := make([][]limitsRow, len(names))
	failures := make([]error, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		c, err := loadCollar(name, s)
		if err != nil {
			failures[i] = err
			continue
		}
		wg.Go(func() { rows[i], failures[i] = cmd.queryLimits(c) })
	}
	wg.Wait()
	var unknown []string
	for i, name := range names {
		if failures[i] == nil {
			continue
		}
		if recorded := cmd.recordedLimits(name); len(recorded) > 0 {
			rows[i] = recorded
			continue
		}
		unknown = append(unknown, fmt.Sprintf("%s: %v", name, failures[i]))
	}
	for _, collarRows := range rows {
		if err := cmd.writeLimits(collarRows); err != nil {
			return err
		}
	}
	if len(unknown) > 0 {
		return commandError{status: exitUnknown, text: strings.Join(unknown, "; ")}
	}
	return nil
}

// recordedLimits returns the newest observed reading any agent of the collar
// holds in the current team, or nothing when there is no team to read.
func (cmd command) recordedLimits(collar string) []limitsRow {
	run, err := cmd.runtime()
	if err != nil {
		return nil
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return nil
	}
	var best *core.Reading
	for _, a := range agents {
		r := a.Native.Limits
		if a.Collar != collar || r.Status != "observed" || r.At == nil || len(r.Limits) == 0 {
			continue
		}
		if best == nil || r.At.After(*best.At) {
			best = &r
		}
	}
	if best == nil {
		return nil
	}
	// A window that has reset since the reading describes a closed window.
	var rows []limitsRow
	for _, w := range best.Limits {
		if w.ResetAt > cmd.now().Unix() {
			rows = append(rows, limitsRow{collar: collar, window: w, at: *best.At})
		}
	}
	return rows
}

func (cmd command) agentLimits(name string) error {
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
		var rows []limitsRow
		for _, w := range r.Limits {
			row := limitsRow{collar: c.Name, window: w}
			if r.At != nil {
				row.at = *r.At
			}
			rows = append(rows, row)
		}
		return cmd.writeLimits(rows)
	}
	b, err := run.input()
	if err != nil {
		return err
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return err
	}
	now := cmd.now()
	readings, err := harness.ReadProviderLimits(c.Primitives.ProviderLimits, screen, now)
	if err != nil {
		return commandError{status: exitUnknown, text: err.Error()}
	}
	var rows []limitsRow
	for _, r := range readings {
		rows = append(rows, limitsRow{collar: c.Name, at: now, window: core.LimitWindow{Label: r.Label, UsedPercent: float64(r.UsedPercent), ResetAt: r.ResetAt.Unix()}})
	}
	return cmd.writeLimits(rows)
}

// writeLimits prints one line per window: collar, window, use, the elapsed
// share of the window, pace delta in points, reset, projected exhaustion at
// the window's average rate so far, and the reading's age. Pace is measured
// when the reading was taken, since its use is as old as it is.
func (cmd command) writeLimits(rows []limitsRow) error {
	now := cmd.now()
	for _, row := range rows {
		w := row.window
		reset := time.Unix(w.ResetAt, 0).UTC()
		measured := row.at
		if measured.IsZero() {
			measured = now
		}
		elapsed, pace, exhausts := "elapsed unknown", "pace unknown", "exhausts unknown"
		if p, ok := harness.UsagePace(w.UsedPercent, reset, w.Label, w.WindowMinutes, measured); ok {
			elapsed = fmt.Sprintf("%.0f%% elapsed", p.ElapsedPercent)
			pace = "pace " + pacePoints(p.Delta)
			exhausts = "exhausts after reset"
			if !p.Exhausts.IsZero() {
				exhausts = "exhausts " + p.Exhausts.UTC().Format(time.RFC3339)
			}
		}
		age := "age unknown"
		if !row.at.IsZero() {
			age = "age " + max(now.Sub(row.at), 0).Round(time.Second).String()
		}
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%s\t%.0f%% used\t%s\t%s\tresets %s\t%s\t%s\n", row.collar, w.Label, w.UsedPercent, elapsed, pace, reset.Format(time.RFC3339), exhausts, age); err != nil {
			return err
		}
	}
	return nil
}

// pacePoints signs a pace delta rounded to whole points, without a negative
// zero.
func pacePoints(delta float64) string {
	rounded := math.Round(delta)
	if rounded == 0 {
		return "+0"
	}
	return fmt.Sprintf("%+.0f", rounded)
}
