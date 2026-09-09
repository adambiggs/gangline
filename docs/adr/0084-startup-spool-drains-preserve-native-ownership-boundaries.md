---
id: 0084
status: accepted
date: 2026-08-14
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0084: Startup spool drains preserve native ownership boundaries

## Context

Startup delivery crosses pane locking, native queues, active turns, compaction, and
operator-owned terminal modes.

## Decision

Lock before claiming startup mail, preserve oldest-first ownership across native
workers, wait for PostCompact during live compaction, never cancel operator-owned modes,
and fail closed when post-paste verification never sees a changed composer.

## Consequences

Unknown stable screens still fail loudly instead of being called startup prompts. Spool
drains take the pane delivery lock before the first claim, so crossed native workers
cannot split or reorder the oldest-first bundle. A mid-turn collar may declare `steer`:
the envelope commits to the attributed spool before any composer keystroke, then a free
composer may accept its claim as native steering. If the harness parks that Enter in its
native queue, Gangline retains the exact composer record for status and verified flush
recovery after retiring the attributed claim. PostToolUse is a delivery opportunity even
while the native turn record remains open. A live compaction is excluded: its queue is
the sanctioned landing only for the attributed continuation that owns the compaction,
while peer mail waits for PostCompact. `park`, an occupied composer, and tmux copy-mode
leave the entry live; Gangline never cancels operator-owned mode state. Post-paste
verification tolerates bounded unreadable or unchanged redraw frames, then opens the
staged-unknown record and fails closed if no changed composer appears.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
