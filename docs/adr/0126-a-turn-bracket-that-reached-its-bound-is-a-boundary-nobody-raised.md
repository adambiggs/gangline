---
id: 0126
status: accepted
date: 2026-08-27
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0126: A turn bracket that reached its bound is a boundary nobody raised

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Before the cooperative tick, every spool drain hung off an event the harness announced.
Measured on claude-code 2.1.241, a turn a person ends by declining a permission dialog
announces nothing at all: no `Stop`, no `StopFailure`, no `PermissionDenied`, no
`PostToolUseFailure`, and no late `Notification` — three denials by two routes, silent
for up to 160s against a pane visibly at rest, with an entry spooled before the denial
still queued 98s later. The harness's own code says why: `PermissionDenied` fires only
for an auto-mode classifier denial, `StopFailure` requires an error result, and `Stop`
runs at the normal end of a query loop that a denial aborts past. Registering an event,
which is what the report proposed, has nothing to register.

## Consequences

The cooperative tick now supplies the missing retry independently of that event. The
bracket's expiry is still Gangline's own fact and already licenses an idle verdict, so
an attempt may offer the window one delivery opportunity. It does NOT rewrite the
bracket: stamping it closed would convert `turn_witness`'s could-not-determine verdict
into a confident idle one, and a tool call longer than `GANG_TURN_LIMIT` is exactly the
turn that would then be typed into. What expiry buys is the attempt; the busy witness,
the collar's mid-turn declaration and the composer guards decide it, the same way they
decide an ordinary send against the same window.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
