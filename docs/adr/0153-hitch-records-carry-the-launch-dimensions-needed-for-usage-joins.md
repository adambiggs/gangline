---
id: 0153
status: accepted
date: 2026-09-02
supersedes: []
superseded-by: []
tags: [usage, lifecycle]
---

# ADR-0153: Hitch records carry the launch dimensions needed for usage joins

## Context

Usage evidence crosses process and sandbox boundaries while accounting policy remains
operator-owned.

## Decision

The join needs the launch choices, so `hitch` and `adopt` register the model, effort,
directory, an optional opaque task label, and a wall-clock start on the window beside
the collar and session id already there.

## Consequences

They are records of what was asked for, written once at launch and re-written by a
resume; Gangline never reads them to decide anything.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
