---
id: 0074
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0074: Direct mandatory suites require a settled tree identity

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`test/lint.sh` and `test/integration.sh` refuse a tree they would not own and name the
command that does.

## Consequences

The suite reads the tree's identity again at the end, so a run whose source moved
underneath it reports no verdict rather than a count about a tree that no longer exists.
What counts as the tree resolves the operator's own git configuration and never the
caller's environment: two reads in one run must be answering the same question. A
reading that cannot be taken refuses; an unknown is not ownership, and neither is an
index instructed not to look at a file.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
