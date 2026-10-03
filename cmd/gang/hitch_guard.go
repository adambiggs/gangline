package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

// A reading older than this does not describe the account at launch time.
// Too long a window lets an old reading refuse a hitch the account would
// accept; too short only leaves the guard silent, so it matches the recency
// the cap handling uses.
const hitchGuardFreshness = 5 * time.Minute

type guardReading struct {
	limits []core.LimitWindow
	at     time.Time
	agent  string
}

// checkHitchGuard applies the collar's operator-configured hitch guard to a
// requested model. Only a fresh reading over a threshold warns, or refuses
// when the guard says so; a stale or absent reading never stops a hitch.
func (run *runtime) checkHitchGuard(c harness.Collar, model string) error {
	g := c.HitchGuard
	if g == nil || model == "" || !slices.Contains(g.Models, model) {
		return nil
	}
	now := run.cmd.now()
	r, reason := run.hitchGuardReading(c, now)
	if r == nil {
		_, err := fmt.Fprintf(run.cmd.stderr, "warning: hitch: model %q is guarded, but %s; launching unchecked\n", model, reason)
		return err
	}
	var tripped []string
	for _, w := range r.limits {
		var threshold *float64
		switch harness.UsageWindowKind(w.Label, w.WindowMinutes) {
		case "five_hour":
			threshold = g.FiveHour
		case "weekly":
			threshold = g.Weekly
		}
		// Dividing the percent keeps an exact threshold exact: 7/100 is the
		// float 0.07, while 0.07*100 is not 7.
		if threshold == nil || w.ResetAt <= now.Unix() || w.UsedPercent/100 < *threshold {
			continue
		}
		tripped = append(tripped, fmt.Sprintf("%s %.0f%% used (guard %.0f%%), resets %s", w.Label, w.UsedPercent, *threshold*100, time.Unix(w.ResetAt, 0).UTC().Format(time.RFC3339)))
	}
	if len(tripped) == 0 {
		return nil
	}
	source := fmt.Sprintf("a %s limits query just now", c.Name)
	if r.agent != "" {
		source = fmt.Sprintf("%s's %s reading from %s ago", r.agent, c.Name, now.Sub(r.at).Round(time.Second))
	}
	detail := fmt.Sprintf("model %q is guarded and %s shows %s; choose another --model or change hitch_guard in the %s collar overlay", model, source, strings.Join(tripped, "; "), c.Name)
	if g.Refuse {
		return refuseError("%s", detail)
	}
	_, err := fmt.Fprintf(run.cmd.stderr, "warning: hitch: %s\n", detail)
	return err
}

// hitchGuardReading returns the freshest recorded reading from an agent on
// the same collar, or else asks the provider when the collar can, and
// otherwise says why there is none.
func (run *runtime) hitchGuardReading(c harness.Collar, now time.Time) (*guardReading, string) {
	agents, err := run.team.ListAgents()
	if err != nil {
		return nil, fmt.Sprintf("team state could not be read (%v)", err)
	}
	var best *guardReading
	for _, a := range agents {
		r := a.Native.Limits
		if a.Collar != c.Name || r.Status != "observed" || r.At == nil || r.At.After(now) || now.Sub(*r.At) > hitchGuardFreshness || len(r.Limits) == 0 {
			continue
		}
		if best == nil || r.At.After(best.at) {
			best = &guardReading{limits: r.Limits, at: *r.At, agent: string(a.Name)}
		}
	}
	if best != nil {
		return best, ""
	}
	if c.Primitives.LimitsQuery == nil {
		return nil, fmt.Sprintf("no %s agent holds a provider reading from the last %s", c.Name, hitchGuardFreshness)
	}
	ctx, cancel := run.cmd.timeout(operationTimeout)
	defer cancel()
	windows, err := harness.QueryProviderLimits(ctx, c)
	if err != nil {
		return nil, fmt.Sprintf("the provider limits query failed (%v)", err)
	}
	r := &guardReading{at: now}
	for _, w := range windows {
		r.limits = append(r.limits, core.LimitWindow{Label: w.Label, UsedPercent: w.UsedPercent, ResetAt: w.ResetAt, WindowMinutes: w.WindowMinutes})
	}
	return r, ""
}
