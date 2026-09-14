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

A transient host service can outlive its requesting sandbox. Its durable
declaration carries the requester's stable spool identity, but a successor may
lack the printed run id. A declaration written before manager acceptance can
also describe work that never started.

## Decision

`gang run` writes a `launching` declaration and promotes it only after the
manager accepts the unit. `gang run --active` reads the calling pane's stable
spool identity and lists only accepted records it owns, with exact
cancellation. It probes a leftover launching record: a live unit promotes,
positively absent retires, and an unreadable manager remains unknown. Roster
and status distinguish the two states. Ending a turn never cancels host work.

## Consequences

Owners recover host work without cross-agent visibility. Intentional work
survives a compaction; cancellation remains explicit. This decision is
falsified if a successor cannot settle an unaccepted record, cannot list and
cancel its own active record, or can list another owner's record.
