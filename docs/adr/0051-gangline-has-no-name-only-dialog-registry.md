---
id: 0051
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0051: Gangline has no name-only dialog registry

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Keep no name-only dialog registry; a collar may authorize only a fully recognized
surface with a verified safe action, and every other screen remains operator-owned
occupancy.

## Consequences

A registry naming stall screens without declaring a safe keystroke was decided against.
It could only name screens somebody had already met, while the generic pointer gang
already prints — inspect it with `gang attach` — covers every screen including the unmet
ones. Each per-dialog fingerprint is one more per-build string that rots: when the
wording moves it stops matching and falls back to exactly that generic path, except that
by then we believe we have coverage. A guard that degrades to correct-but-silent is
fine; one that degrades while we think it holds is not.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
