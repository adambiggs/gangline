---
id: 0133
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0133: Blocked errors are classified from native shape

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

So the shape is `isApiErrorMessage`, which the harness sets on a record it wrote, and
the `error` value is a reason rather than a criterion.

## Consequences

A value the reader cannot recognise is reported as blocked with an unnamed reason, never
as absent. Only what the fatal reader positively claims is withheld, so a class neither
reader names is still reported.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
