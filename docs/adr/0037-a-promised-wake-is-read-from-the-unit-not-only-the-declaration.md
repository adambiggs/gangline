---
id: 0037
status: accepted
date: 2026-08-14
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0037: A promised wake is read from the unit, not only the declaration

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A pending declaration on a tmux window and the transient timer that keeps it can outlive
each other, and the declaration alone then reports a wake nothing will deliver.

## Consequences

Status reads the unit whenever the reset is still ahead, and separates gone from
unreadable rather than merging them into an answer.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
