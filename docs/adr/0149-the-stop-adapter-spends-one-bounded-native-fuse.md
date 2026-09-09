---
id: 0149
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0149: The Stop adapter spends one bounded native fuse

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Every stage of that adapter is bounded by one native fuse rather than by a constant of
its own.

## Consequences

The query keeps its attempt and deadline and the release report its small fixed bound,
but each is clamped to what the fuse has left, and no stage starts on a budget it cannot
fit; a reserve keeps the verdict printed before the fuse rather than at it. Stage
constants spent independently add up past the fuse, and a verdict printed after it is a
verdict the harness never reads at all.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
