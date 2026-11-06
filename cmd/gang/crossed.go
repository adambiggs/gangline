package main

import (
	"errors"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
)

// undeliveredFrom lists, oldest first and comma-separated, the tokens of
// messages agent from sent to agent to that Gangline had not delivered to
// to at now. It returns "unknown" when to's records cannot be read; the
// send itself does not depend on them.
func (run *runtime) undeliveredFrom(from core.HitchID, to core.Sender, now time.Time) string {
	if to.Kind != core.SenderAgent || to.HitchID == "" {
		return ""
	}
	pending, err := run.undeliveredMail(from, to.HitchID, now)
	if err != nil {
		return "unknown"
	}
	slices.SortStableFunc(pending, func(a, b core.Envelope) int { return a.CreatedAt.Compare(b.CreatedAt) })
	tokens := make([]string, 0, len(pending))
	for _, e := range pending {
		tokens = append(tokens, e.Token)
	}
	return strings.Join(tokens, ",")
}

// undeliveredMail reads to's queue and its last accepted and unverified
// receipts. Delivered means a submit witness matched. A queued message
// counts once due. An accepted or unverified receipt counts until to's
// next submit witness in its current native session: one that matches it
// delivered it, and one after the receipt was filed means to has taken
// input since, so it is no longer in flight.
func (run *runtime) undeliveredMail(from, to core.HitchID, now time.Time) ([]core.Envelope, error) {
	p, err := run.team.Agent(to)
	if err != nil {
		return nil, err
	}
	sent := func(e core.Envelope) bool {
		return e.From.Kind == core.SenderAgent && e.From.HitchID == from && e.Token != ""
	}
	// Read the queue before the receipts: a message settled in between
	// then appears in both, never in neither.
	queued, err := p.ListNew()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
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
	var pending []core.Envelope
	for _, e := range queued {
		if sent(e) && !e.NotBefore.After(now) {
			pending = append(pending, e)
		}
	}
	witness, err := p.ReadWitness()
	witnessed := err == nil && witness.SessionID != "" && witness.SessionID == b.Native.SessionID
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	c, err := loadCollar(b.Collar, run.settings)
	if err != nil {
		return nil, err
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
		if !sent(e) || e.Outcome != r.outcome || slices.ContainsFunc(pending, func(q core.Envelope) bool { return q.ID == e.ID }) {
			continue
		}
		if witnessed {
			matched, _, err := receiptMatches(c.Primitives.SubmitWitness, e, witness.Prompt)
			if err != nil {
				return nil, err
			}
			if matched || !e.FiledAt.IsZero() && witness.At.After(e.FiledAt) {
				continue
			}
		}
		pending = append(pending, e)
	}
	return pending, nil
}
