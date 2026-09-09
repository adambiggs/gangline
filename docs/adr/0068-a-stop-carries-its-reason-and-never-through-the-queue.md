---
id: 0068
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [messaging]
---

# ADR-0068: A stop carries its reason, and never through the queue

## Context

Messages cross native harness boundaries where acceptance, attribution, and delivery are
distinct facts.

## Decision

`gang interrupt -m` stops a turn and delivers its reason at the boundary that stop
creates, under one continuous pane lock.

## Consequences

It is never spooled. `--supersede` retires the sender's own waiting entries and stamps
the replacement now, which sorts it behind every other sender's — so a stop sent that
way arrives after the work it was meant to stop.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
