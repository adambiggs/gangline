---
id: 0152
status: accepted
date: 2026-09-03
supersedes: []
superseded-by: []
tags: [usage]
---

# ADR-0152: Gangline joins ccusage output by native session identity

## Context

Usage evidence crosses process and sandbox boundaries while accounting policy remains
operator-owned.

## Decision

Gangline knows which agent ran, on which harness and model, in which directory, for how
long, and under which native session id; it does not know what that session cost.
ccusage knows the cost: it parses each harness's local transcript format and follows
that format as it churns.

## Consequences

Gangline therefore never reads a transcript for tokens. `gang usage` runs `ccusage
session --json --no-cost --offline` when a `ccusage` executable is on `PATH` and joins
its rows to the team's hitch records by native session id: exactly, or, for a row
ccusage itself attributes to Codex, by the id suffix of its rollout-shaped period, so
one harness's tokens can never land on another's agent through a stray period. Nothing
else in Gangline changes when ccusage is absent: the same table prints without token
columns and says why.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
