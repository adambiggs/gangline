package main

import (
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func contextBandOverride(c harness.Collar, value string) (*core.ContextBandThresholds, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return nil, usageError("hitch: --context-bands requires EARLY,LATE percentages with 0 <= EARLY < LATE <= 100")
	}
	early, earlyErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	late, lateErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if earlyErr != nil || lateErr != nil || !(early >= 0 && early < late && late <= 100) {
		return nil, usageError("hitch: --context-bands requires EARLY,LATE percentages with 0 <= EARLY < LATE <= 100")
	}
	if len(c.ContextBands) == 0 {
		return nil, usageError("hitch: --context-bands requires a collar with two context bands per model selector")
	}
	for selector, bands := range c.ContextBands {
		if len(bands) != 2 {
			return nil, usageError("hitch: --context-bands requires two context bands for selector %q", selector)
		}
	}
	return &core.ContextBandThresholds{Early: early / 100, Late: late / 100}, nil
}

func agentContextCollar(a core.Agent, c harness.Collar) harness.Collar {
	if a.ContextBandThresholds == nil {
		return c
	}
	current := c.ContextBands
	c.ContextBands = make(map[string][]harness.ContextBand, len(current))
	for selector, bands := range current {
		copyBands := append([]harness.ContextBand(nil), bands...)
		if len(copyBands) == 2 {
			copyBands[0].At = a.ContextBandThresholds.Early
			copyBands[1].At = a.ContextBandThresholds.Late
		}
		c.ContextBands[selector] = copyBands
	}
	return c
}
