---
id: 0046
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0046: Screen movement is evidence of presence

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Movement seen during a decision is presence, not indeterminacy: a harness paints the
opening of a turn with its composer still empty, so an empty box read out of a moving
screen is one frame of something in motion rather than a settled reading.

## Consequences

That verdict carries its reason so delivery can refuse on it alone.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
