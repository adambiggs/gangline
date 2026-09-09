---
id: 0098
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0098: A request log is only evidence once it separates whose request it was

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

An aggregated log answers "did these words reach the server", which is not the question
any assertion means to ask.

## Consequences

The harness counts tokens and titles sessions with bodies that quote the same prompt, so
a search over every record can be satisfied by a request the agent's turn never made —
and a log records arrival, so it can also be satisfied by a request the harness is still
blocked on. The stub therefore marks each record with whether it was the agent's own
turn and writes a second record when the answer is fully sent, and the lane reads only
completed agent turns. Two facts that must hold together are asserted against one
record, not against the log twice.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
