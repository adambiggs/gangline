---
id: 0086
status: accepted
date: 2026-08-13
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0086: State explanations instrument the classified read

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

`gang explain` records match/miss results only while the ordinary live state reader
evaluates collar-owned busy and occupancy rules, and prints the first matching line from
that exact capture.

## Consequences

It does not take a later diagnostic snapshot that could describe a different TUI frame.
Rules bypassed by stronger native evidence are named as not evaluated rather than
fabricated as misses; the diagnostic adds no stored state.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
