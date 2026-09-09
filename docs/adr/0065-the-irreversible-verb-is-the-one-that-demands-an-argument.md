---
id: 0065
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0065: The irreversible verb is the one that demands an argument

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`gang down` requires the session it ends and refuses from inside it.

## Consequences

Every other argument-taking command answers a bare invocation with its usage; `down` did
not, so the reflex that reads the manual — run it bare and see what it wants — executed
the teardown instead.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
