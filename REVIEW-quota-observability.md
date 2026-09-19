# Quota observability gap review — 2026-09-17

Reviewed at `8a3010300719f6b2675bce71e20764cfef975975` against
ccusage 20.0.20. This is a dated review, not a statement that the sampled
percentages or roster still stand.

## Verdict

Gangline can answer **who is running, under which model, the cumulative tokens
ccusage assigns to that native session, and the last provider percentage it
sampled**. It cannot answer the questions a lead needs to manage a shared quota
over time: what was spent today or in this provider window, whether the window
is on pace to exhaust, which live agent is accelerating that spend, or what the
lead's last turn cost.

This is mostly missing product surface over data Gangline or ccusage already
has. One existing surface is wrong: the per-model roll-up double-counts
cumulative ccusage snapshots when a native session is resumed across hitches.

## Questions Gangline cannot answer today

| Lead question | What Gangline shows now | Gap | Classification |
|---|---|---|---|
| What did each model and agent spend per day or since the current quota window opened? | `gang usage` shows cumulative native-session totals for live agents and teardown snapshots for ended agents. The only selector is `--all`. | No daily or `--since` time axis. ccusage can filter native sessions and exact-session entries by date, but Gangline does not join that view to agents; teardown time is not consumption time. | Missing feature. |
| What provider-limit samples were seen during this window? | `gang limits` stores one ephemeral most-constrained reading on the tmux window. `gang cap` retains eight weeks of raw JSONL but `show` prints only each window's last state. | Gangline owns the history but exposes no history command. | Missing feature. |
| Are we on pace to exhaust before reset, and when? | Limits/cap print percent used, reset, and sample age. | No elapsed-window comparison, slope, or exhaustion projection. | Missing feature. |
| Which live agent is spending fastest right now? | `gang usage` joins live sessions to fresh cumulative totals; Gangline knows hitch start time. Roster flags provider threshold state/staleness only. | No rate, two-sample delta, ranking, or outlier flag. A one-sample total divided by hitch age can only be labelled an average, not "right now". | Missing feature. |
| What did the lead's own last turn cost? | The lead row is a cumulative session total. Usage events are written only at drop/down and are cumulative too. | No cross-harness per-turn token source. | Missing upstream/integration surface; Gangline must not add transcript parsers. |

The gap is structural: `gang usage` has no time filter, while `gang cap show`
exposes only the latest state and retained history has no public reader. A stale
provider reading remains an honest stale sample, not evidence of current
headroom.

## What the two data sources know

### Gangline usage events

Gangline's usage ledger is an append-only teardown record.
At review time it contained version-1 rows with:

- Gangline identity and launch context: host, team plus creation epoch, agent,
  collar, requested model/effort, directory, task, native session ID, and hitch
  time;
- lifecycle facts: end time, end cause, and duration;
- one ccusage join result: status/note, ccusage harness, per-model input/output,
  cache-read/cache-write counts, and last activity.

It does **not** contain a turn sequence, sample time separate from teardown,
daily buckets, provider-window identity, quota before/after, or token deltas.
Live rows are not in the file: `gang usage` constructs them from tmux launch
records and joins them to a fresh ccusage session report at read time.

The file cannot be summed as an event ledger. A resumed native session accrues
cumulatively under the same ID, so later teardowns include earlier tokens. The
host evidence includes one Codex session recorded at 268,915, 984,673, and
1,215,355 input tokens. A Claude session has two identical cumulative snapshots
before a later larger one. `libexec/gang-usage` currently sums every record's
`models` entry in its roll-up, so those sessions are counted repeatedly.

### ccusage

`ccusage session --json --no-cost --offline` knows every supported harness
session it can find, keyed by its native period/session ID. Each row includes
input, output, cache creation, cache reads, model breakdowns, and last activity.
It does not know Gangline's team, agent name, task, hitch interval, or outcome;
Gangline supplies those dimensions for the session join.

`ccusage daily` knows daily totals and per-model breakdowns, including cache
reads. Its day rows aggregate across sessions and identify harness families,
not Gangline agents, so they answer daily model spend but cannot by themselves
attribute a day to `lead`, `quotaobs`, or another Gangline registration.
The date-filtered session aggregate exposes Codex session rows; in ccusage
20.0.20 the same aggregate omits Claude rows, while `session --id` exposes the
selected Claude entries with model/input/output/cache fields. Gangline can use
the two native-session views together without parsing harness transcripts.
ccusage's documented Codex reader remains experimental, and there is still no
common Claude-plus-Codex per-turn row Gangline can join for last-turn cost
without taking transcript parsing back from ccusage, which “Gangline joins ccusage output by native session identity” forbids.

## Data Gangline already has but does not surface

- `gang cap` already retains `readings.jsonl` with provider, label/window,
  provider-published percentage, observation and append times, reset, status,
  source, and disposition. `cmd_show` reads only folded `state.json` and prints
  the latest window state.
- `gang usage` already takes a fresh ccusage snapshot for every live agent and
  has each hitch start. It can expose average rate immediately and true recent
  rate after retaining two comparable samples.
- Roster already has compact usage flags (`usage=yellow`, `usage=red`,
  `usage-stale`, `usage-unavailable`). It has a natural display location for a
  rate-outlier flag, but no rate measurement feeds one.
- The cap sample's provider reset is a stable window key. Samples sharing it can
  support a labelled slope/projection without deriving a provider percentage
  from tokens, preserving “Recorded usage percentages are the ones a provider published”.
- Account quota is shared with remote agents and other consumers. Local ccusage
  and usage events can attribute this host's activity, but can never be the
  denominator or pace input for the account-window projection.

## Product defects versus missing features

### Product defect

1. The per-model `gang usage` roll-up double-counts cumulative snapshots for a
   resumed native session. The per-hitch rows are truthful snapshots; their sum
   is not a truthful total.

### Missing features

1. Daily and since-window usage views.
2. Public provider-limit sample history.
3. Pace and projected-exhaustion reporting from comparable published samples.
4. Live-agent rate ranking and a roster outlier flag.
5. A universal/ccusage-owned per-turn delta source.
6. Operator-configured hitch warning/refusal for costly models when fresh quota
   evidence is past a threshold.
7. Snubline `scan-text` caching keyed by body and all verdict-changing inputs,
   plus one commit/push/both scan-point setting sharing the existing push cache.

### Not a Gangline feature

The requested generic "stall detection in `gang tick`" has no honest substrate
signal. Gangline already pushes provable permission/input stalls through
`gang notify`, and `gang run` pushes host-command completion. Its own reference
correctly says a long-running tool and a stalled agent both go quiet. Turning
age or idle-without-report into a verdict would either fabricate health state or
make the harness supervise task completion, violating Constitution laws 7 and
8. The quota-saving change belongs in doctrine and collar guidance: owners end
their turn while bounded work runs and resume from its pushed completion; leads
do not spend LLM turns polling. A new tick heuristic is not recommended.

## Evidence and reproducibility

The review used these public/read-only paths:

```sh
gang limits
gang usage
gang cap
gang roster
ccusage session --json --no-cost --offline
ccusage daily --json --no-cost --offline
gang limits --history
gang usage --all
```

The behavior is specified by “Provider usage is a collar-native observation”, “Gangline joins ccusage output by native session identity” through “Quota and outcome policy stay outside usage accounting”, “Recorded usage percentages are the ones a provider published”,
“A usage threshold alerts once inside the window it measures”, `docs/records/usage-spec.md`, and the `gang limits`, `gang usage`, and
`gang cap` sections of `docs/reference.md`. Implementation evidence is in
`bin/gang` (`cmd_limits`, `cmd_usage`, and roster's usage flags),
`libexec/gang-usage` (fresh join and unconditional model sum), and
`libexec/gang-cap` (retained readings versus latest-only `cmd_show`).

## Issue register

1. [#277 — resumed-session aggregate double counting](https://github.com/adambiggs/gangline/issues/277) — product defect.
2. [#278 — daily and since-window usage](https://github.com/adambiggs/gangline/issues/278) — missing feature.
3. [#279 — retained provider sample history](https://github.com/adambiggs/gangline/issues/279) — missing feature.
4. [#280 — quota pace projection](https://github.com/adambiggs/gangline/issues/280) — missing feature.
5. [#281 — live spend rate and outlier flag](https://github.com/adambiggs/gangline/issues/281) — missing feature.
6. [#282 — universal per-turn cost source](https://github.com/adambiggs/gangline/issues/282) — missing upstream/integration surface.
7. [#283 — quota-aware model guard at hitch](https://github.com/adambiggs/gangline/issues/283) — missing operator-configured feature.
8. [Snubline #59 — cache `scan-text` bodies and share scan-point policy](https://github.com/adambiggs/snubline/issues/59)
   — missing Snubline feature. Gangline
   [#284](https://github.com/adambiggs/gangline/issues/284) was closed because
   it was filed in the wrong tracker on a false premise.
