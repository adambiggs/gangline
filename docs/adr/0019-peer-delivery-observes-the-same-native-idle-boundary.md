---
id: 0019
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0019: Peer delivery observes the same native-idle boundary

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Peer delivery uses that witness too.

## Consequences

Before closing the turn bracket, a native Stop records its exact payload on the window;
while that payload's rollout turn is still open, `busy` treats the empty-looking
composer as busy, and an unanswerable rollout is unknown rather than idle. A default
send therefore parks in the attributed spool without typing. If mail was already waiting
when Stop fired, its detached drain waits inside the same boot budget and releases
itself after the terminal record lands; it does not depend on a cooperative tick racing
that append. Recording the payload before closing the bracket means a racing sender
always sees either the open bracket or the rollout witness, never the false-idle
composer alone. A new prompt or an interrupt retires the old payload only after opening
or clearing the turn bracket, respectively. The payload itself preserves the native-idle
binding across a collar rewrite: if the current collar has no reader, delivery is
unknown and cannot fall through to an apparently empty composer. A live native boundary
also outranks a collar's ordinary mid-turn-input permission: Codex accepts steering
during normal work but drops Enter while its Stop hook owns the turn.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
