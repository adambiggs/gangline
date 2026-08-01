# ADR-0009: Time bands are relative to a declared deadline

- **Status:** Accepted
- **Date:** 2026-08-01

## Context

An agent cannot feel elapsed time any more than it can feel context fullness, so
the substrate measures both or neither. The context ladder exists (ADR-0005,
ADR-0006); this page adds its counterpart for wall-clock time and answers the
unit question before it is asked, because for context that question was decided
three times before it was written down once.

The trap is symmetry. ADR-0005 made context rungs absolute and defended it hard,
so the tempting "consistent" answer is absolute time rungs — warn every agent at
minute 30, minute 60, minute 90. That transplants the conclusion while leaving
the argument behind. ADR-0006 split the hazards: **rot** is degradation caused
by the accumulating artifact and is absolute; **exhaustion** is running out of
room and is relative by definition. Time has no rot. Minute 60 does not degrade
an agent; the degradation that correlates with long sessions is context growth,
which has its own ladder, and a second ladder proxying the same hazard is noise
wearing a different unit. What time has is exhaustion only — not finishing
before a deadline — and a hazard that is relative in its own right gets a
relative ladder.

Stated the way ADR-0005 stated its inverse, and this is the sentence to keep:
**a longer budget is a licence to spend longer — that is what a budget is.**

## Decision

**The time ladder is fractions of a declared budget. There are no absolute
rungs, and there is no ladder without a declaration.**

- **Declared, never measured.** The deadline enters as operator intent —
  `gang hitch --deadline <duration|clock>` at hitch, or `gang deadline
  <duration|clock>` mid-flight (`gang deadline` shows it, `gang deadline clear`
  ends it). It is session-scoped: one line, one deadline, stored as the pair
  deadline-epoch and declared-at-epoch. There is no default. An undeclared
  budget produces silence, not a guess — a fabricated budget is fabricated
  status (law 8).

- **Substrate-only.** The clock is Gangline's own. No profile is involved, nothing
  is scraped, and the ladder behaves identically on every harness — including
  the ones whose whole portfolio is scrape. It is the cheapest band there is,
  and the only one with no rot treadmill at all.

- **One team, one deadline.** Per-agent budgets are refused. "Take ten minutes
  then report" is task prose in a lead's message — law 9's answer, prose in the
  prompt rather than code in the repo — not substrate state.

- **Span and rungs.** The usable span runs from declared-at to the deadline
  minus a reserve; the reserve defaults to 10% of the budget
  (`GANG_TIME_RESERVE`, a percentage or an absolute duration — banking takes
  roughly constant time, so an absolute reserve is a legitimate operator
  choice). Rungs sit at 308, 555, 783 and 1000 permille of the usable span —
  the interior fractions of the context ladder, compressing as pressure rises.
  There is no rung at zero because time has no onset hazard: a warning at the
  starting line warns about nothing. The top rung repeats every sweep;
  `ladder_at_top` was generalized for exactly this second axis before the axis
  existed (bin/gang:2231).

- **The reserve is banking room.** Work that exists only in an agent's head or
  its pane dies with the deadline; work on disk survives it. The last rung
  fires where the reserve begins, and its ask is durability: results written
  out, commits made, state recorded where the operator or a successor can read
  it. ADR-0006's sentence in time's costume: a warning at the deadline arrives
  after the deadline has already spent it.

- **The asks escalate by urgency, not volume.** In `band_note`'s discipline,
  one note per crossing: first an on-track check — is the current approach
  going to land inside what remains; then converge — stop widening, start
  closing; then bank — make progress durable; then, inside the reserve, bank
  now and take only work that improves what is banked.

- **Notes state remaining time plainly.** This is a deliberate asymmetry with
  context notes, which drop the window denominator so headroom cannot read as
  an allowance. Remaining budget *is* the pacing input — an agent told only
  "time is short" can neither rebalance nor triage. Concealing the number would
  gut the mechanism.

- **Two legs, one ledger.** `@gl_tband` sits beside `@gl_band`: per-agent
  warned-state, two writers, advance-if-higher, rebuild-not-refuse. Patrol is
  the primary leg, because time has the one hazard context never has — it
  advances while an agent is idle, so an idle agent with budget remaining is
  patrol's to nudge. The hook leg evaluates on event arrival wherever ADR-0008
  wiring exists.

- **Notes, never enforcement.** A passed deadline changes nothing mechanically.
  Stopping the sled is the musher's call, and the ladder's last word is still a
  note.

- **The escape hatch inverts.** An explicit `GANG_TIME_BANDS` CSV takes
  percentage rungs only. Absolute rungs are refused loudly — the exact mirror
  of `GANG_CONTEXT_BANDS`, where `%` is the escape hatch and absolute the
  design.

### What this is not

- Not absolute wall-clock rungs, and proposals to add them get this page.
  ADR-0005's argument does not transfer: it rests on rot tracking the artifact,
  and time accumulates no artifact.
- Not a scheduler or an estimator. Gangline never guesses how long work will
  take; it only reports how much declared budget remains.
- Not benchmark machinery. A deadline is any timeboxed session's fact — a
  sprint, a demo cut, a paid hour. The long-horizon benchmark entry that
  motivated this page is one consumer among those, and nothing in the ladder
  knows what a verifier is. Anything keyed to a particular benchmark's shape is
  refused here by name.

## Consequences

- `ladder` and `auto_rungs` generalize to a second axis with the fractions held
  in common; the suite gains time-rung boundary checks as the executable form
  of this decision, under ADR-0005's closing rule — a change that edits those
  checks to make time rungs absolute has removed the guard, not passed it.
- `band_note` grows a time voice with the four asks above; patrol rows and the
  patrol log gain a time verdict.
- The benchmark adapter, when it exists, passes the task budget through
  `--deadline` exactly as an operator would — it holds no machinery of its own.
- Declaring a deadline mid-flight re-spans the ladder from now to the deadline:
  fractions of what remains, which is the only budget there is.

## History

- **2026-08-01** — operator direction: enter the Long-Horizon Terminal-Bench
  (46 tasks, 90-minute wall-clock budgets, artifact-graded), with the explicit
  constraint recorded the same day that the design stay general — nothing that
  would look like gaming the benchmark. The "not benchmark machinery" bullet is
  that constraint made standing.
- **Prior art** — the LemonHarness technical report describes exposing "elapsed
  and remaining budget to the model, so it can rebalance exploration,
  implementation, and validation effort as time pressure shifts," and sits at
  the top of Terminal-Bench 2.0's community leaderboard with it (84.49% with
  GPT-5.3-Codex, 86.52% with GPT-5.5). Independent validation that budget
  exposure pays for itself; what this design adds is the escalating ask ladder,
  the banking reserve, and a patrol leg that reaches agents idling inside a
  live budget.
- ADR-0006's rot/exhaustion split is the frame this page stands on, and
  bin/gang:2231's comment reserved the seam — "a future wall-clock ladder" —
  before this page existed.
