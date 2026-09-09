---
id: 0022
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0022: Queued is not delivered

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

A harness may accept the Enter and park the message in its own input queue — claude's
queue strand renders the parked body exactly like a submitted prompt and empties the
composer, so "the box changed" cannot prove entry into the session. The collar declares
the queue hint (`GANG_QUEUED_REGEX`), matched against the box reading only so a
delivered body quoting the hint can never trip it. When the hint already stands before a
delivery, it is current queue evidence and Gangline does not paste another body into it.
When the hint is first observed after Gangline presses Enter, it does not settle that
Enter's fate: the session can accept the turn before the next composer frame paints the
same hint. That outcome is unverified, distinct from both delivered and parked; Gangline
records the body for conditional recovery but does not prescribe a flush, re-send, or
drop until the transcript or current context confirms what happened. The same rule
applies after a recalled body and to deferred self-compaction, where an automatic retry
could compact twice. An unreadable verification capture remains ambiguity that fails
closed.

## Consequences

The contract is scoped to verified harness renderings: the pin is the composer hint
observed on claude-code 2.1.223, an unobserved version narrows the guarantee back to
box-change verification rather than refusing sends, and no session-record machinery is
built unless a reworded hint supplies the evidence to reopen that choice. A cleared
staged record is evidence the obstruction is gone, never retroactive proof the recorded
body was delivered.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
