# ADR-0005: Context bands are absolute token counts

- **Status:** Superseded by [ADR-0006](0006-the-band-ladder-spans-absolute-bounds.md).
  The argument below is not reversed — it is why ADR-0006's floor and cap are both
  absolute token counts. What changed is that the default ladder is derived from
  those two bounds rather than written as a literal CSV. Proposals to make the
  default a fraction of the window are still refused, and this is still the page
  that says why.
- **Date:** 2026-07-30

## Context

`GANG_CONTEXT_BANDS` is the ladder `gang patrol` and the context hook use to warn an
agent as its context grows. The rungs can be expressed as absolute token counts or
as percentages of the agent's own window, and which one the **default** uses has now
been decided three times, in the same direction, twice by re-deriving it from
scratch because the previous decision was not written anywhere a proposal would
look.

This ADR exists so there is no fourth time.

The rung figures used as illustration below are the 2026-07-30 ladder this ADR
set — kept because the argument needs concrete numbers against a concrete
window, not because they are current; ADR-0006 derives its own rungs from a
floor and cap instead, and nothing in the argument turns on which set is in
effect.

### Why absolute is the design, not a limitation

Context rot tracks the absolute length of a context, not how full the window happens
to be. A 300k context is degraded whether the window is 200k or 1M. The ladder's job
is to warn before degradation, so the rung that matters is 300k on both — the same
amount of rot deserves the same warning regardless of the harness carrying it.

A proportional ladder inverts this exactly where it costs most. 85% of 1M is 850k
tokens: the largest windows would be warned last, far past the point a warning could
still be acted on. Stated the other way round, and this is the sentence to keep:
**a bigger window is a reason to warn on the same absolute schedule, not a licence to
fill it.**

### The argument that keeps being made for proportional

It is a real observation and it is not sufficient. Shipped harnesses differ in window
size by nearly 4x, so one absolute ladder means the top rung is unreachable on a
small window — a 350000 rung never fires on codex's 258k window — and a proposal
follows that the ladder should scale to each window so every rung fires everywhere.

An unreachable rung is correct behaviour, not a gap. **An agent that cannot hold 350k
tokens cannot suffer 350k-token rot.** A codex agent on a 258k window is warned at
120k, 180k and 250k, and that is the whole ladder that applies to it. "Every rung
should fire on every harness" is a statement about ladder symmetry; the ladder exists
to track degradation, and degradation is not symmetric across window sizes.

## Decision

This section is what ADR-0005 fixed on 2026-07-30 and is now history, not the
ladder `gang patrol` runs: [ADR-0006](0006-the-band-ladder-spans-absolute-bounds.md)
replaced it with a ladder derived from an absolute floor and cap. Read ADR-0006 for
the ladder actually in effect.

The default ladder was `120000,180000,250000,350000`, in absolute tokens. It
stopped at four rungs because past four ignored warnings another is noise rather
than advice, and it reached into the region where rot actually bites rather than
stopping short of it. ADR-0006 keeps neither number: its ladder is derived from
the floor and cap rather than named as a literal CSV.

**The `%` suffix stays in the syntax as an escape hatch** for an unusual window,
where a fraction is the operator's intent rather than a mistake. It is not a default
and must not become one.

Proposals to make the default proportional are refused. Reopening this requires new
evidence about how rot behaves — not a fresh restatement of the mixed-window
observation above, which is already accounted for.

## Consequences

- A rung above an agent's window never fires. Expected; see above.
- Deployments wanting different absolute rungs set `GANG_CONTEXT_BANDS`. The units do
  not change.
- The suite carries rung-boundary checks at more than one window size, and they are
  the executable form of this decision. **They must fail if the default becomes
  proportional.** A change that edits those checks to agree with a new default has
  removed the guard rather than passed it.
- `bin/gang`'s comment at the `BANDS` assignment points here, so the reasoning is
  reachable from the code without a search.

## History

Recorded because the absence of this record is what allowed the regression.

- **2026-07-26, `713f932`** — the default's last proportional rung (`90%`) was removed
  and every rung made absolute, with measurements at both window sizes showing the
  proportional rung producing a duplicate band on a 200k window and a 650k stretch
  with no signal at all on 1M. Six rung-boundary checks were added specifically so
  "the duplicate rung and the silent stretch cannot come back." The reasoning lived
  in that commit message and nowhere else.
- **2026-07-29, `c3ca002`** — filed as issue #11 from the mixed-window observation and
  merged, changing the default to `30%,50%,70%,85%`. The commit named the cost
  explicitly — "context rot tracks absolute length too" — and traded it away anyway.
  The registers held no decision to reconcile against, because there was none to
  find. The 2026-07-26 guards were rewritten in the same change to match the new
  behaviour, so nothing failed.
- **2026-07-30** — reverted, and this ADR written. The escape hatch is restored to
  what `713f932` decided, rather than hardened further.

The durable lesson is not about bands. A decision recorded only in a commit message
is not in a register, and the guards that enforce it are only a guard while a change
has to *pass* them rather than edit them.
