package harness

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

type ProviderLimit struct {
	Label       string
	UsedPercent int
	ResetAt     time.Time
}

var (
	claudeLimitPattern = regexp.MustCompile(`^(Current [^:]+): ([0-9]+)% used .* resets ([A-Z][a-z]{2} [0-9]{1,2}, [0-9]{1,2}(?::[0-9]{2})?(?:am|pm)) \(([^()]+)\)$`)
	codexLimitPattern  = regexp.MustCompile(`(?i)^([^:]+):?[[:space:]]+([0-9]+)%[[:space:]]+(used|left).*resets[[:space:]]+(.+)$`)
)

func ReadProviderLimits(invocation Invocation, screen substrate.Screen, now time.Time) ([]ProviderLimit, error) {
	var limits []ProviderLimit
	for _, line := range screenLines(screen, true) {
		line = strings.TrimSpace(strings.Join(strings.Fields(line), " "))
		var limit ProviderLimit
		var ok bool
		switch invocation.Name {
		case "claude-screen-limits":
			limit, ok = parseClaudeLimit(line, now)
		case "codex-screen-limits":
			limit, ok = parseCodexLimit(line, now)
		default:
			return nil, fmt.Errorf("unknown provider-limit primitive %q", invocation.Name)
		}
		if ok {
			limits = append(limits, limit)
		}
	}
	if len(limits) == 0 {
		return nil, errors.New("screen carries no provider-limit reading")
	}
	return limits, nil
}

func parseClaudeLimit(line string, now time.Time) (ProviderLimit, bool) {
	match := claudeLimitPattern.FindStringSubmatch(line)
	if len(match) == 0 {
		return ProviderLimit{}, false
	}
	used, err := strconv.Atoi(match[2])
	if err != nil || used < 0 || used > 100 {
		return ProviderLimit{}, false
	}
	zone, err := time.LoadLocation(match[4])
	if err != nil {
		return ProviderLimit{}, false
	}
	clock := strings.ToUpper(match[3])
	format := "Jan 2, 3PM 2006"
	if strings.Contains(clock, ":") {
		format = "Jan 2, 3:04PM 2006"
	}
	reset, err := time.ParseInLocation(format, fmt.Sprintf("%s %d", clock, now.In(zone).Year()), zone)
	if err != nil {
		return ProviderLimit{}, false
	}
	if reset.Before(now.Add(-24 * time.Hour)) {
		reset = reset.AddDate(1, 0, 0)
	}
	return ProviderLimit{Label: match[1], UsedPercent: used, ResetAt: reset}, true
}

func parseCodexLimit(line string, now time.Time) (ProviderLimit, bool) {
	match := codexLimitPattern.FindStringSubmatch(line)
	if len(match) == 0 {
		return ProviderLimit{}, false
	}
	percent, err := strconv.Atoi(match[2])
	if err != nil || percent < 0 || percent > 100 {
		return ProviderLimit{}, false
	}
	if strings.EqualFold(match[3], "left") {
		percent = 100 - percent
	}
	reset, ok := parseReset(match[4], now)
	if !ok {
		return ProviderLimit{}, false
	}
	return ProviderLimit{Label: strings.TrimSpace(match[1]), UsedPercent: percent, ResetAt: reset}, true
}

func parseReset(text string, now time.Time) (time.Time, bool) {
	text = strings.TrimSpace(text)
	for _, format := range []string{time.RFC3339, "Jan 2, 3:04 PM 2006", "Jan 2, 3:04 PM", "3:04 PM"} {
		candidate := text
		if format == "Jan 2, 3:04 PM" {
			candidate += fmt.Sprintf(" %d", now.Year())
			format += " 2006"
		}
		reset, err := time.ParseInLocation(format, candidate, now.Location())
		if err != nil {
			continue
		}
		if format == "3:04 PM" {
			reset = time.Date(now.Year(), now.Month(), now.Day(), reset.Hour(), reset.Minute(), 0, 0, now.Location())
		}
		if reset.Before(now) {
			reset = reset.AddDate(0, 0, 1)
		}
		return reset, true
	}
	return time.Time{}, false
}
