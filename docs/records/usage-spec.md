# Token usage reporting — implementation record and follow-ups

> Status: v1 landed. The decision is "Token consumption is ccusage's reading,
> joined by Gangline" in `docs/DECISIONS.md`; `docs/reference.md` documents
> `gang usage`, the hitch flags, and the event file. This record keeps the
> measurements the design rests on and the work deliberately left out.

## What ccusage provides

Measured against ccusage 20.0.20 on 2026-09-02. `ccusage session --json
--no-cost --offline` prints `{"session": [...], "totals": {...}}` for every
harness it detects at once. Each row carries `agent`, `period`, `inputTokens`,
`outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`,
`modelsUsed`, `modelBreakdowns` (one object per model with the same four token
counts), and `metadata.lastActivity`.

- Claude Code rows use the session UUID as `period`; it equals the id Gangline
  stamps from the harness's own hook payload.
- Codex rows use the rollout file stem as `period`, ending in the thread UUID
  Gangline stamps. ccusage documents its Codex reader as experimental, and
  those rows carry `metadata.reasoningOutputTokens`, which Gangline does not
  record.
- `metadata.projectPath` appears only on pi rows in this version, so a
  project-path join has no consumer and is not implemented.
- `--id` prints a different, per-message shape and answers `null` for Codex
  thread ids, so Gangline loads the whole session report and joins locally.
- Loading the whole session report is cheap enough to do on every read. With
  `--offline` and `--no-cost` the command completes inside a network namespace
  that has only loopback.
- With no transcripts under `$HOME` the command exits 126 and prints nothing.
  Gangline reports that as a failed read with the exit status.

## What ccusage cannot cover

- Start time and duration: ccusage knows last activity only. Duration is
  Gangline's, from the wall-clock start registered at hitch.
- A session resumed across several hitches accrues under one native id; ccusage
  reports the total, and each hitch record that shares the id is shown with it.
- Output tokens for Codex: whether `outputTokens` includes reasoning tokens is
  ccusage's definition and was not verified here.
- Harnesses ccusage does not parse, or a session it has not yet flushed, appear
  as unmatched.

## Follow-ups

### Quota before and after

Gangline already has provider-quota plumbing: `gang limits` reads a collar's
non-interactive source into `@gl_usage_limit` as
`label<TAB>percent-used<TAB>reset-epoch<TAB>observed-epoch`, and usage lights
warn at operator thresholds. The event record could carry the most recent
reading at hitch and at drop as `quota_before` and `quota_after` with their
observation clocks. It was left out because a reading is only present when a
light or an explicit `gang limits` sampled it, so the column would be blank for
most rows until sampling at hitch and drop is decided.

### `gang optimize`

Recommendations need three inputs: consumption per task class (this record),
remaining quota per subscription (`gang limits`), and a task class per hitch
(the opaque task label, which Gangline does not classify). No consumer exists
for a recommendation surface yet.

### Cross-host aggregation

Each host writes its own `events.jsonl` with a `host` field. `gang usage
--all` reads one file. Merging files from several hosts is a concatenation of
JSON lines; how the files travel is not decided.

### An outcome signal

Gangline records no outcome, so no success or usefulness column is honest. The
smallest candidate is a lead's verdict on a report: the lead runs a command
naming the agent and `accept` or `return`, which appends one line to the same
event file keyed by agent and hitch time. It is a coordination statement, so it
belongs in doctrine and a lead role brief before any command exists.
