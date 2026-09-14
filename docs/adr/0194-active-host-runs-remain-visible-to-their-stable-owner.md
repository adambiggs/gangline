---
id: 0194
status: proposed
date: 2026-09-13
supersedes: []
superseded-by: []
tags: [runs, recovery, ownership]
---

# ADR-0194: Active host runs remain visible to their stable owner

## Context

A transient host service can continue after the Codex sandbox that requested it
is interrupted. This includes a mandatory gate waiting on a shared host lock.
Its durable declaration already carries the requester's stable spool identity,
but the successor does not retain the run id printed by the predecessor and
cannot safely discover or cancel the service.

## Decision

`gang run --active` reads the calling pane's stable spool identity and lists
only active declarations owned by it, each with its exact cancellation command.
Roster and status expose the same run ids. A read reports an unreadable record
instead of declaring no work. Ending a turn does not cancel a host service.

## Consequences

An owner can recover queued and running host work from a new sandbox without
exposing another agent's declarations. Intentional long-running work survives a
compaction; explicit owner cancellation remains the irreversible action. This
decision is falsified if a successor cannot list and cancel its own active
record, or if it can list another owner's record.
