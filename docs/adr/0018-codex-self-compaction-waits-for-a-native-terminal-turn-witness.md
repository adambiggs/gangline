---
id: 0018
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [compaction]
---

# ADR-0018: Codex self-compaction waits for a native terminal-turn witness

## Context

Compaction crosses a native turn boundary where duplicated or lost input would corrupt
the agent's work.

## Decision

Deferral still needs a positive post-Stop native-idle boundary, and the boundary is the
harness's own record, not its screen.

## Consequences

Codex runs Stop inside its active task: its composer paints idle for the whole hook and
drops an Enter typed there, and its compact hooks give no command correlation. What
Codex does leave is its rollout's terminal turn record (`task_complete` or
`turn_aborted`, carrying the turn id the Stop payload names), appended only after every
Stop hook has returned and flushed at once. The collar declares
`GANG_SELF_COMPACT_WITNESS=native-idle` and reads that record through
`collar_native_idle`; the dispatcher started by Stop waits for it inside the boot budget
before it consults the composer, so a self-request from inside a Codex agent is deferred
exactly as it is on claude-code. A record that has not landed when the budget runs out,
or a rollout that cannot answer for the turn, refuses at that boundary and leaves the
request standing for the next Stop, which carries a fresh payload. A cooperative tick
carries no payload and asks about the newest turn. Nothing falls through to the pane.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
