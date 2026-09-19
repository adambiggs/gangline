---
id: 0005
status: accepted
date: 2026-08-11
---

# ADR-0005: Native continuation owns compaction recovery

## Context

A compaction left an agent at an empty composer holding only its summary. With
nothing to take the next turn, the agent sat idle and its work silently stopped
(f13e296).

## Decision

Every compaction Gangline submits is followed by a continuation turn that tells
the agent to re-read its brief and saved state. `gang compact --resume`
replaces that turn's text.

## Consequences

No compaction lands idle. What the continuation asks the agent to re-read is
operator prose, not Gangline state.
