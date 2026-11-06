package main

import (
	"errors"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
)

// undeliveredFrom lists the tokens of messages agent from sent to agent to
// that Gangline had not delivered to to at now: still queued and due, in the
// native queue without a submit witness (accepted), or typed without one
// (unverified). Delivered means a verified submit, so a later reconciliation
// does not change what was true at now.
func (run *runtime) undeliveredFrom(from core.HitchID, to core.Sender, now time.Time) (string, error) {
	tokens, err := run.undeliveredTokens(from, to, now)
	return strings.Join(tokens, ","), err
}

func (run *runtime) undeliveredTokens(from core.HitchID, to core.Sender, now time.Time) ([]string, error) {
	if to.Kind != core.SenderAgent || to.HitchID == "" {
		return nil, nil
	}
	p, err := run.team.Agent(to.HitchID)
	if err != nil {
		return nil, err
	}
	b, err := p.Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sent := func(e core.Envelope) bool {
		return e.From.Kind == core.SenderAgent && e.From.HitchID == from && e.Token != ""
	}
	var pending []core.Envelope
	queued, err := p.ListNew()
	if err != nil {
		return nil, err
	}
	for _, e := range queued {
		if sent(e) && !e.NotBefore.After(now) {
			pending = append(pending, e)
		}
	}
	for _, r := range []struct {
		dir, outcome string
		id           core.EnvelopeID
	}{{"cur", "accepted", b.LastAccepted}, {"failed", "unverified", b.LastFailed}} {
		if r.id == "" {
			continue
		}
		e, err := p.ReadEnvelope(r.dir, r.id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if sent(e) && e.Outcome == r.outcome {
			pending = append(pending, e)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].CreatedAt.Before(pending[j].CreatedAt) })
	var tokens []string
	for _, e := range pending {
		if !slices.Contains(tokens, e.Token) {
			tokens = append(tokens, e.Token)
		}
	}
	return tokens, nil
}
