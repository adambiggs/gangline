# ADR-0006: The band ladder spans absolute bounds

- **Status:** Accepted
- **Date:** 2026-07-30
- **Supersedes:** ADR-0005

## Context

ADR-0005 settled the units: the rungs of the context ladder are absolute token
counts, because context rot tracks how long a context is and not how full the window
happens to be. That reasoning is not reopened here. It is the reason both bounds
below are absolute.

Two things it left standing showed up in use.

**The unreachable top rung is consumed as a measurement.** ADR-0005 called a rung
above an agent's window correct rather than a gap, and for rot that is right. But
`final_exposure()` in `lib/context_events.py` returns `COULD-NOT-DETERMINE` whenever
the last rung exceeds the window, so every row from a 258k harness was permanently
uncountable — not "did not top out", but "unanswerable", forever (#32). A compliance
metric with a whole harness in a blind spot is not strict, it is partial.

**The widest gap sat where urgency was highest.** The rungs `120000,180000,250000,
350000` have gaps of 60k, 70k and 100k: they get *wider* as the context gets longer,
so the last stretch before the ceiling was the longest silent run. Observed live on
2026-07-30 — an agent crossed 250000 and 350000 and worked through both, with no
signal in the 100k between them.

Behind both is one number doing two jobs.

### Two hazards, not one

- **Rot** — degradation caused by context length. Absolute. A 300k context is
  degraded whether the window is 258k or 1M, and the rung that matters is the same
  on both. This is ADR-0005's subject.
- **Exhaustion** — running out of room to finish the turn. Relative by definition.
  26k remaining is 26k on any harness, but whether that is imminent depends entirely
  on where the window ends.

ADR-0005 answered rot correctly and had no answer for exhaustion, so one ladder was
spent on both. Separating them is what this decision does.

## Decision

**The ladder runs from an absolute floor to an absolute cap, and only the spacing
between them fits the window.**

- `GANG_CONTEXT_FLOOR` (default `120000`) — below this nobody is warned, on any
  harness. Rot onset is a property of context length, so this is the same number
  everywhere. It is the absolute claim of ADR-0005, kept.
- `GANG_CONTEXT_CAP` (default `350000`) — no agent goes unwarned past this, however
  large its window. ADR-0005's sentence stands unchanged: **a bigger window is a
  reason to warn on the same absolute schedule, not a licence to fill it.**
- `ceiling = min(0.9 × window, CAP)`. The 0.9 is exhaustion headroom: a rung at the
  window is a warning that arrives after the agent is already dead, and what remains
  after the last rung has to be enough to issue a compaction and have it land.
- **Five rungs at `0 · 308 · 555 · 783 · 1000` permille of `[FLOOR, ceiling]`.** The
  fractions are fixed, so relative spacing is identical on every harness, and they
  compress upward so warnings get closer together as the situation gets worse.
- `ceiling ≤ FLOOR` → a single rung at the ceiling. An agent whose window cannot
  reach rot onset can only be warned about exhaustion, and that one rung is the whole
  ladder that applies to it — ADR-0005's rule, unchanged, arrived at from the other
  side.
- An explicit `GANG_CONTEXT_BANDS` CSV bypasses derivation entirely. The `%` suffix
  remains what ADR-0005 made it: an escape hatch, never a default.

Both bounds are read from the environment, so a profile that exports them sets them
for its harness at no extra cost — `load_profile` already runs before `ladder` in
both nudge legs (`bin/gang`, `patrol_one` and the hook leg). There is **no per-model
table.** The window is read per agent from the live readout, so the span already
retunes itself for every model without configuration; a table would only add
override of the two deliberately universal numbers, and a per-model floor is one
commit away from a per-model fraction. A model that demonstrably rots earlier is a
finding for this ADR, not a row in a file.

### What this is not

It is not the proportional ladder refused three times and recorded in ADR-0005. That
proposal made **every** rung a fraction of the window, which warns a 1M agent at
850k — last, long past use. Here both ends are absolute token counts. Nothing is
warned below `FLOOR`; nothing goes unwarned above `CAP`; and a 1M window and a 400k
window get *identical* ladders, because both ceilings clamp to the cap. Only the
interior of a window too small to reach the cap scales, and there the hazard being
tracked is exhaustion, which is relative in its own right.

| window | rungs |
|---|---|
| ≥389k | 120k · 190k · 247k · 300k · 350k |
| 258k | 120k · 154k · 182k · 208k · 232k |
| 200k | 120k · 138k · 153k · 166k · 180k |
| 128k | 115k (single rung — cannot reach the floor) |

## Consequences

- Every agent can reach the last rung, so `final_exposure()` drops its
  `thresholds[-1] > window` bail-out and #32 closes with no special case. "Final
  band" means topped out, for everyone.
- The largest gap now sits at the bottom of the ladder where there is time, not at
  the top where there is not.
- A small window crosses several rungs in one turn. That is expected and already
  handled: `ladder` reports the highest rung crossed and both nudge legs send one
  note naming it.
- **The suite's rung-boundary checks change what they assert**, and ADR-0005's
  closing line requires that to carry an argument rather than a rewrite. This is the
  argument. The invariants they now hold are: the first rung is the same absolute
  number at every window size; no rung is below `FLOOR` or above `CAP`; and windows
  at and above `CAP / 0.9` get identical ladders. Each of those fails if the floor or
  the cap is ever made a fraction.
- `#24`'s context-compliance dataset takes a third regime boundary at the save.

## History

- **2026-07-30, ADR-0005** — the units decided, after the same conclusion had been
  reached three times and recorded nowhere a proposal would look. Superseded rather
  than reversed: its argument is why the floor and the cap are absolute.
- **2026-07-30** — this. Proposed by the operator, who added the floor to a draft
  that had only the cap; the floor is what makes every agent's first warning land at
  the same absolute token count, which is the property ADR-0005 was protecting.
