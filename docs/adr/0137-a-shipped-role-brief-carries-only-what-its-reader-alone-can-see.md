---
id: 0137
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [roles]
---

# ADR-0137: A shipped role brief carries only what its reader alone can see

## Context

Role prose must guide reusable team behavior without becoming coordination state in
Gangline.

## Decision

A brief is prompt: every line is charged against its reader's context for the life of
the agent, so a line earns its place by changing behaviour nothing else in that context
already governs. For the lead brief the boundary is the arc. Method inside an arc is the
owner's, and saying so once is all the brief owes it; what the brief carries is the
decisions between arcs, which no owner can see and none can be given.

## Consequences

The same boundary excludes the contract, which reaches every agent already.
`test/role-briefs.sh` refuses any sentence appearing in both a shipped brief and
`CONTRACT.md`. That is an exact-duplicate guard, not a semantic one.

Locking a brief's sentences is the limit of what the mandatory gate can say about its
prose. A lock catches a decision deleted or reworded away, not one negated by a sentence
added after it, and nothing in that gate can show a lead obeyed any of them. Measuring
conduct needs a separate opt-in lane, and only the decisions that turn on a window are
observable without a task manifest the eval supplies itself. See
`docs/records/lead-brief-revision-2026-08-30.md`.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
