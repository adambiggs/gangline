---
id: 0045
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0045: An abandoned turn bracket decays only under complete settled evidence

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A turn fact nobody will ever edit — a bracket abandoned by an interruption typed
straight into the pane, which no harness reports — decays instead of standing unknown
for the life of the window: once it passes its bound, the tiers beneath it answer.

## Consequences

Decay requires every leg, measured rather than assumed — a collar that does not declare
quiet-at-rest reports inactive by abstention, and an abstention is not a witness — and
applies only to a readable open bracket past its bound, since an unreadable or
future-stamped one is unknown, not abandoned. The pty clock and the screen are read
before the first tier and after the last, and a decay assembled while either moved is
refused. Inside its bound the bracket still outranks the tiers beneath it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
