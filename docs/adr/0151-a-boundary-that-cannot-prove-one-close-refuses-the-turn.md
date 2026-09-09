---
id: 0151
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0151: A boundary that cannot prove one close refuses the turn

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A boundary that runs out of time still refuses, on the re-Stop as on the first, because
the alternative loses an obligation silently.

## Consequences

An unclosed boundary leaves the turn bracket open; a prompt arriving under an open
bracket is steering, so the replies that turn read stay answerable, and the next message
the agent sends can be correlated to one of them — its recipient is then owed no reply
and no record says why. A refusal is loud, is bounded by the native cap, and ends a
session the operator can see. Only a boundary that can prove it closed exactly once may
release the turn.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
