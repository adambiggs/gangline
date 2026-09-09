---
id: 0166
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [ticks]
---

# ADR-0166: A tick owner runs at most one rerun

## Context

Cooperative retries must make progress without creating a resident coordinator or
unbounded owner.

## Decision

A contender that finds a live tick owner marks it dirty and exits, and the owner
consumes that edge with one more pass.

## Consequences

The owner formerly consumed every edge, so a marker set during the rerun started another
rerun and the owner's lifetime became a function of the team's command rate: once hooks
and commands arrived faster than a pass completed, one owner held the lock until the 60
s deadline killed it, failed health, and took every synchronous `gang tick` and delivery
verification folded into it down with it. The bound is now two passes. A marker set
during the rerun is handed to one successor `gang tick`, armed while the owner still
holds the lock and started in a session of its own: it is spawned already holding the
read end of a pipe whose write end the owner closes after release or drops at death, so
the promised pass begins after the contender's edge whether or not the owner survives,
the owner's deadline kill cannot reach it, and no file records the hand-over. The wait
needs nothing beyond Bash and Python, which Gangline already requires. A successor that
cannot be armed fails the tick and its health and leaves the marker for the next tick. A
busy team gets a chain of short workers instead of one that dies at the deadline.
Synchronous `gang tick` returns after at most two passes. Measure a pass with `time gang
tick` on a quiet team, and an owner's lifetime by the age of the tick lock symlink under
`GANG_LOCK_DIR/tick` while candidates arrive.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
