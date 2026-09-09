---
id: 0087
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [collars, models]
---

# ADR-0087: Model discovery and validation stay collar-native

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

A complete native model catalog is a collar reader, normalized to exact model ids with
optional per-model efforts.

## Consequences

`gang models` prints it and hitch requires an exact match before opening a window. A
harness with no complete catalog may publish only documented aliases and a native
recognition check; discovery says the list is incomplete, and recognition never claims
provider or account availability. Failed, empty, duplicate, or malformed evidence is
unknown and refused. Core carries no harness model vocabulary.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
