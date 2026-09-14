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

Gangline appends structured, monotonic event lines to its existing event file
and exposes the current team's filtered stream through `gang log`. Events
carry the team, agent, kind, and event-specific facts; message bodies are not
recorded. Two fixed-size generations bound retained diagnostic evidence.

## Consequences

The stream gives a lead a causal timeline without retaining private message
bodies, but older evidence expires on rotation. An append failure is loud and
refuses the event-producing action. The decision is falsified if the stream
cannot account for a future observed Gangline decision or becomes a polling or
coordination component.
