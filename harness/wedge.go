package harness

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

type WedgeObservation struct {
	Previous   substrate.Screen
	Current    substrate.Screen
	BusySince  time.Time
	ObservedAt time.Time
	TurnActive bool
}

type Wedge struct {
	Detected bool
	Since    time.Time
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
	if !reflect.DeepEqual(observation.Previous, observation.Current) {
		return Wedge{}, nil
	}
	plain := strings.Join(screenLines(observation.Current, true), "\n")
	if !busy.MatchString(plain) {
		return Wedge{}, nil
	}
	return Wedge{
		Detected: true,
		Since:    observation.BusySince,
		Evidence: boundedTail(plain, 20),
	}, nil
}

func boundedTail(text string, lines int) string {
	rows := strings.Split(text, "\n")
	if len(rows) > lines {
		rows = rows[len(rows)-lines:]
	}
	return strings.Join(rows, "\n")
}
