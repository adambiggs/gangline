# Quota optimisation recommendations — 2026-09-17

Ranked by smallest safe change first, then by likely value within a similar
size. “Saved” means subscription/context usage avoided, not API dollars. Where
the provider has not published its accounting weights, the estimate says what
is measurable rather than converting tokens to a fictitious quota percentage.

## 1. Make pushed completion the only long-command waiting pattern

**Change:** doctrine/collar guidance, not a new `gang tick` supervisor. Start a
bounded command with `gang run` (or keep one yielded exec session and do other
work), end the inference turn, and resume from the pushed completion. Do not
re-poll a yielded command on a timer. Preserve the current rule that silence is
not health: `gang notify` may report provable input/permission stalls, but tick
must not infer task failure from age or idle state.

**Why first:** This is a doctrine-only change with no new machinery. A generic
tick stall detector would violate Constitution law 7 and conflate a long tool
with a stuck agent.

**Estimated saving:** unmeasured. Measure avoided re-poll turns and context
before claiming a percentage.

## 2. Cache Snubline `scan-text` verdicts by body and policy

**Change:** Snubline, with no Gangline cache. Its push path already caches by
payload; extend that existing cache to `scan-text`, keyed on body and every
verdict-changing input: backend/model, effective semantic prompt/policy,
scanner version, and relevant configuration. Expose hit/miss in Snubline's
ordinary output and fail closed on unreadable state. Add a scan-point setting
with `commit`, `push`, and `both`, defaulting to `both`. Commit scans populate
the shared cache; push verifies the same hash and scans only unseen content.

**Estimated saving:** one judge-model call per unchanged retry, i.e. 100% of
semantic-call usage on a cache hit. Fleet-wide share is not yet measured; add a
hit counter before claiming a percentage.

## 3. Fix cumulative-session double counting before adding time views

**Change:** Gangline. Keep per-hitch teardown snapshots, but aggregate repeated
native session IDs by a documented latest-snapshot or monotonic-delta rule.
Surface resets/decreases as unknown rather than subtracting blindly.

**Estimated saving:** 0 direct quota. This removes an overcount that would make
every later routing or pace decision systematically wrong.

## 4. Expose cap history and a labelled pace projection

**Change:** Gangline. Add a public history mode over cap's retained readings,
reachable as `gang limits --history` (or an equally direct documented command).
Group only samples with the same provider/reset/window identity. Show percentage
of the weekly window elapsed, recent observed slope, and a projected 100% time
when two increasing comparable samples exist. Say “insufficient history,”
“flat,” or “window changed” otherwise. Never label a projection as published
provider data. Never use local ccusage or usage-event totals: remote agents and
other machines consume the same account quota, so local usage is attribution
only.

**Estimated saving:** 0 automatic quota; this is the smallest decision surface
that can prevent a repeat of exhausting a weekly allowance days early. Its
real saving is the guarded-model consumption rerouted after an adverse
projection and must be measured from subsequent decisions.

## 5. Add `gang usage --daily` and `--since` without inventing agent history

**Change:** Gangline plus ccusage's date-filtered session surfaces. Join the
filtered aggregate to Gangline's native session IDs; where ccusage omits a
known session from that aggregate, use its exact-session `--id` entries for the
same date span under one bounded call budget. Deduplicate resumed native
session IDs and refuse attribution when one ID maps to several Gangline agents.
Do not assign a cumulative teardown snapshot to its teardown or last-activity
day.

The attribution table must group by Gangline agent and model and keep input,
output (including thinking/reasoning as ccusage reports it), and cache reads in
separate columns. Do not collapse them into a total: the split is the evidence
needed before doctrine treats reasoning effort as a quota lever.

**Estimated saving:** 0 automatic quota. It replaces bespoke lead analysis and
enables daily routing. Measure value as lead turns avoided and quota shifted
away from over-budget models, not as the tokens displayed.

## 6. Rank live-agent average/delta rates, then add a roster outlier flag

**Change:** Gangline. A first version can label cumulative session tokens divided
by hitch age as “average since hitch.” A “recent” rate needs two samples with the
same native session ID; retain only the minimal bounded sample needed. Define an
outlier relative to comparable live agents and make its threshold operator
configuration. Roster consumes the same computed verdict and does not resample.

**Estimated saving:** 0 until a lead acts. The upper bound is the post-alert
consumption of an agent the lead pauses, rotates, or moves to a cheaper model;
record those actions before claiming a fleet percentage.

## 7. Add an operator-configured quota guard at hitch

**Change:** operator doctrine/configuration plus Gangline. Configure the provider
models considered costly and warning/refusal thresholds. Start with a warning;
allow refusal only through explicit operator configuration. Require a fresh
published limit sample, and fail loud as unavailable rather than treating stale
or absent evidence as headroom. The refusal names the reset and the route to a
different model or changed operator policy.

**Estimated saving:** bounded by all guarded-model consumption after the
threshold. With a 70% threshold the theoretical allowance protected is at most
the remaining 30%, but the achievable value is lower and cannot be inferred
until Gangline records how many hitches were rerouted and what they consumed.

## 8. Treat per-turn cost as an upstream data-contract request

**Change:** ccusage (or another universal open surface), not harness branches in
Gangline. Request a session-plus-turn/time-bucket record for both Claude and
Codex carrying per-model input/output/cache counts and a stable native identity.
Gangline can join that identity to its agent. Until it exists, show the session
total and state plainly that last-turn cost is unavailable.

**Estimated saving:** 0 by itself. The signal can later support turn-level
rotation experiments, but building transcript parsers into Gangline would add
the churn ADR-0152 deliberately delegates to ccusage.

## Implementation choice for this arc

Implement the two requested smallest lead-facing surfaces: provider history
with pace projection, and a daily usage view that exposes ccusage's measured
session/day facts joined to Gangline agents without summing lifecycle snapshots.
The cumulative resume overcount remains a separate correctness issue and must
be fixed before any unfiltered view sums teardown snapshots.
