---
id: 0064
status: accepted
date: 2026-08-21
supersedes: []
superseded-by: []
tags: [testing]
---

# ADR-0064: Mandatory tests are immediate

## Context

Mandatory evidence must distinguish the behavior under test from timing and fixture
artifacts.

## Decision

Mandatory tests do not sleep, poll, or test timeout behaviour; use immediate state,
event barriers, or fake clocks, and test real harnesses only in separate disposable tmux
sessions.

## Consequences

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
