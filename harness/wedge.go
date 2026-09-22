package harness

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

type WedgeObservation struct {
	Previous   string
	Current    substrate.Screen
	BusySince  time.Time
	ObservedAt time.Time
	TurnActive bool
}

type Wedge struct {
	Detected bool
	Evidence string
}

func DetectWedge(invocation Invocation, observation WedgeObservation) (Wedge, error) {
	if invocation.Name != "stable-busy-screen" {
		return Wedge{}, fmt.Errorf("unknown wedge primitive %q", invocation.Name)
	}
	threshold, err := time.ParseDuration(invocation.Params["after"])
	if err != nil || threshold <= 0 {
		return Wedge{}, fmt.Errorf("wedge primitive has invalid after duration %q", invocation.Params["after"])
	}
	busy, err := regexp.Compile(invocation.Params["busy"])
	if err != nil {
		return Wedge{}, fmt.Errorf("compile wedge busy expression: %w", err)
	}
	if !observation.TurnActive || observation.BusySince.IsZero() || observation.ObservedAt.Before(observation.BusySince.Add(threshold)) {
		return Wedge{}, nil
	}
	if observation.Previous != ScreenFingerprint(observation.Current) {
		return Wedge{}, nil
	}
	plain := strings.Join(screenLines(observation.Current, true), "\n")
	if !busy.MatchString(plain) {
		return Wedge{}, nil
	}
	return Wedge{
		Detected: true,
		Evidence: boundedTail(plain, 20),
	}, nil
}

func ScreenFingerprint(screen substrate.Screen) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(screenLines(screen, true), "\n"))))
}

func boundedTail(text string, lines int) string {
	rows := strings.Split(text, "\n")
	if len(rows) > lines {
		rows = rows[len(rows)-lines:]
	}
	return strings.Join(rows, "\n")
}
