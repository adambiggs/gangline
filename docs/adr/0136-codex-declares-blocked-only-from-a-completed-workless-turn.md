---
id: 0136
status: accepted
date: 2026-09-02
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0136: Codex declares blocked only from a completed workless turn

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Codex binds a rollout and records a native turn bracket, but its reader does not guess
an error vocabulary the rollout lacks.

## Consequences

It declares a blocked window only when the newest completed turn has no reply, no first
token, and no work beyond the input and known bookkeeping. An unclosed turn is absent
and an unclassified payload is unknown, so new rollout shapes cannot silently become
blocked.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
