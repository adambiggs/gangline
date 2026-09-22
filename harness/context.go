package harness

import (
	"path/filepath"
	"sort"
	"strings"
)

func ActiveContextBand(collar Collar, model string, reading ContextReading) *ContextBand {
	bands := modelContextBands(collar, model)
	var active *ContextBand
	for index := range bands {
		if reading.Percent < bands[index].At {
			break
		}
		band := bands[index]
		active = &band
	}
	return active
}

// CrossedContextBands returns every upward threshold crossing, in order.
func CrossedContextBands(collar Collar, model string, previous, current float64) []ContextBand {
	var crossed []ContextBand
	for _, band := range modelContextBands(collar, model) {
		if previous < band.At && current >= band.At {
			crossed = append(crossed, band)
		}
	}
	return crossed
}

func modelContextBands(collar Collar, model string) []ContextBand {
	selector := contextSelector(collar.ContextBands, model)
	bands := append([]ContextBand(nil), collar.ContextBands[selector]...)
	sort.Slice(bands, func(left, right int) bool { return bands[left].At < bands[right].At })
	return bands
}

func contextSelector(bands map[string][]ContextBand, model string) string {
	if model == "" {
		return ""
	}
	best := ""
	bestSpecificity := -1
	for selector := range bands {
		matched, err := filepath.Match(selector, model)
		if err != nil || !matched {
			continue
		}
		specificity := len(strings.ReplaceAll(selector, "*", ""))
		if specificity > bestSpecificity || specificity == bestSpecificity && selector < best {
			best = selector
			bestSpecificity = specificity
		}
	}
	return best
}
