---
id: 0192
status: proposed
date: 2026-09-13
supersedes: []
superseded-by: []
tags: [observability, events]
---

# ADR-0192: A team event stream retains observations and decisions

## Context

Gangline's usage rows preserve only teardown facts while context lights,
compaction state, delivery claims, classifications, tick state, and alerts are
mutable or short-lived. Incident reconstruction therefore cannot distinguish
an absent warning from a warning that was read and ignored. The record must
remain a small CLI artifact, not a watcher, database, or second transport.

## Decision

Gangline appends structured, monotonic event lines to its own event file and
exposes the current team's filtered stream through `gang log`. Events carry
the team, agent, kind, and event-specific facts; message bodies are not
recorded. The command that decides an event appends it directly; a context
reading is recorded only beside the band change it decided, because every hook
reads the context and a row per reading records nothing the next one would
not. Two fixed-size generations bound retained diagnostic evidence. The
separate unpruned usage and cost record remains `gang usage`'s only history.

## Consequences

The stream gives a lead a causal timeline without retaining private message
bodies, but older event evidence expires on rotation. It does not duplicate
the long-lived usage record, so investigations use `gang log` for decisions
and `gang usage` for costs and teardown history. An append failure is loud and
leaves an explicit diagnostic gap without stranding the lifecycle action. A
sandboxed agent records events only where its sandbox grants the event
directory; the operator grants it rather than every event paying a host-side
crossing. The
decision is falsified if the stream cannot account for a future observed
Gangline decision or becomes a polling or coordination component.
