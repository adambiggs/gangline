---
id: 0205
status: proposed
date: 2026-09-17
supersedes: []
superseded-by: []
tags: [lifecycle]
---

# ADR-0205: Dropping a hitcher settles the windows it hitched

## Context

ADR-0198 lets an agent mark itself safe to drop, and only its direct live
hitcher drops it without ceremony. When that hitcher is dropped first,
marked children linger as orphans and unmarked ones lose the only agent that
could clear them. This proposal refines accepted ADR-0198.

## Decision

A drop settles everything the dropped window hitched before removing
anything. A child marked safe to drop goes with it, recursively, and its
`agent.dropped` event records `"scope": "cascade"`. A child that is unmarked,
or whose mark cannot be read, is never dropped: its provenance moves to the
nearest live agent above everything the drop removes, and the drop names it.
With no such agent the child stays an orphan for a root agent or the
operator. An unreadable registry refuses the drop.

## Consequences

Teardown follows ADR-0198's self-attested terminal fact and adds no task
state, so ADR-0001 and ADR-0005 hold. An adopter gains authority over work
it never started. The decision is falsified by a drop that removes an
unmarked child, or leaves a marked child of the dropped window live.
