---
id: 0092
status: accepted
date: 2026-08-14
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0092: Pre-push proves the fast boundary

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The repository pre-push hook runs the outer contribution gate, production and hook lint,
a command smoke, and commit-message checks against the pushed tree.

## Consequences

It names the test lint, checker self-tests, and integration suite it skipped; CI runs
full lint and integration on pushes to `main`. Release Please is a job in that same
workflow and needs both verdicts before it can publish. The local boundary stays quick
enough to use on every push without overlapping the memory-heavy shell linters, while
the complete gate also runs the smoke.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
