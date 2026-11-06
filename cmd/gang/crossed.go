package main

import (
	"errors"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
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
// next submit witness in its native session, judged as delivery judges
// it: one that matches it delivered it, and one after the receipt was
// filed means to has taken input since, so it is no longer in flight.
// The reads take no lock, so a message settling at that moment can be
// missed.
func (run *runtime) undeliveredMail(from, to core.HitchID, now time.Time) ([]core.Envelope, error) {
	p, err := run.team.Agent(to)
	if err != nil {
		return nil, err
	}
	sent := func(e core.Envelope) bool {
		return e.From.Kind == core.SenderAgent && e.From.HitchID == from && e.Token != ""
	}
	queued, err := p.ListNew()
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(p.Directory); errors.Is(statErr, os.ErrNotExist) {
			return nil, nil
		}
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
	witnessed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var c harness.Collar
	if witnessed && (b.LastAccepted != "" || b.LastFailed != "") {
		if c, err = loadCollar(b.Collar, run.settings); err != nil {
			return nil, err
		}
		// As delivery does, a witness whose session cannot be followed
		// belongs to another session.
		bound := b
		same, err := followNativeSession(c, &bound, witness.SessionID, witness.Transcript)
		witnessed = err == nil && same
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
		// A witness older than the message cannot carry its token.
		if witnessed && !witness.At.Before(e.CreatedAt) {
			matched, _, err := receiptMatches(c.Primitives.SubmitWitness, e, witness.Prompt)
			if err != nil {
				return nil, err
			}
			// A receipt filed before filing times were recorded has a zero
			// FiledAt, so any later witness of the session ages it out.
			if matched || witness.At.After(e.FiledAt) {
				continue
			}
		}
		pending = append(pending, e)
	}
	return pending, nil
}
