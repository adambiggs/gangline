package main

import (
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestTokenUnitsRoundsToThousandsThenTenthsOfMillions(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{0, "0k"},
		{499, "0k"},
		{500, "1k"},
		{102950, "103k"},
		{200000, "200k"},
		{999499, "999k"},
		{999500, "1M"},
		{1000000, "1M"},
		{1049999, "1M"},
		{1050000, "1.1M"},
		{2000000, "2M"},
		{10960000, "11M"},
	} {
		if got := tokenUnits(tc.n); got != tc.want {
			t.Errorf("tokenUnits(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestEnvelopeStampTextNamesRoundedContextAndUTCSecond(t *testing.T) {
	at := time.Date(2026, 11, 6, 1, 43, 9, 700_000_000, time.FixedZone("PDT", -7*3600))
	used, limit, percent := int64(102950), int64(1000000), 10.295
	observed := core.Reading{Kind: "context", Status: "observed", Used: &used, Limit: &limit, Percent: &percent}
	if got := envelopeStampText(at, observed); got != "ctx=103k/1M at=11-06T08:43:09Z" {
		t.Fatal(got)
	}
	padded := time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := envelopeStampText(padded, observed); got != "ctx=103k/1M at=01-02T03:04:05Z" {
		t.Fatal(got)
	}
	for name, r := range map[string]core.Reading{
		"empty":        {},
		"not observed": {Kind: "context", Status: "unknown", Used: &used, Limit: &limit, Percent: &percent},
		"no limit":     {Kind: "context", Status: "observed", Used: &used, Percent: &percent},
	} {
		if got := envelopeStampText(at, r); got != "ctx=unknown at=11-06T08:43:09Z" {
			t.Errorf("%s: %s", name, got)
		}
	}
}
