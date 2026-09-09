---
id: 0048
status: accepted
date: 2026-08-08
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0048: Server loss is a relaunch, not restoration

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Do not persist a Gangline roster.

## Consequences

`--resume` asks a collar's verified, explicit-id native command for the session stamped
in `@gl_session_id`, read from a surviving registered window or quoted from `gang
drop`'s parting output, and refuses without one; the operator supplies which agents and
ids to relaunch.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
