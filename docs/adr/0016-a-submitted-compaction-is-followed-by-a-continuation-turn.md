---
id: 0016
status: accepted
date: 2026-09-01
supersedes: []
superseded-by: []
tags: [compaction]
---

# ADR-0016: A submitted compaction is followed by a continuation turn

## Context

Compaction crosses a native turn boundary where duplicated or lost input would corrupt
the agent's work.

## Decision

Gang types a continuation behind every compaction command it elects to submit, so an
agent lands with a turn to take instead of an empty composer. It rides the composer, not
the PreCompact/PostCompact pair: a refused compaction raises PreCompact and never
PostCompact, so a continuation keyed on the bracket is lost exactly when the agent still
holds its full context. A compacting harness parks the continuation and submits it when
the compaction ends; a harness that refused takes it immediately. Both land, on every
collar declaring a compact command, which is why this does not split a team into collars
that resume and collars that sit.

## Consequences

The agent supplies the turn with `--resume`, or a default fires. The default is
orientation, never direction — "continue working" would make a finished agent invent
work.

Parking here is a landing rather than a failed delivery, which is the one exception to
*Queued is not delivered*: gang is using the harness's own queue as the delivery timer,
and claims entry only for a message it told a peer was sent.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
