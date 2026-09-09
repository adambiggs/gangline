---
id: 0162
status: accepted
date: 2026-09-05
supersedes: []
superseded-by: []
tags: [tmux, lifecycle]
---

# ADR-0162: Agent commands resolve scopes only through the window record

## Context

tmux exposes mutable terminal state, so Gangline must not turn an ambiguous read into
authority to act.

## Decision

Drop, roster, usage, tick, and explain already resolve an agent to its window before
acting or observing.

## Consequences

They continue through that stable window identity and never synthesize a unit from the
current name; the window's `@gl_scope` is the only registration-to-unit record. A
resumed hitch overwrites it with its new unit, an unscoped hitch writes it empty, and a
newly adopted window gets an empty record because Gangline did not launch that process.
Re-adopting an already registered agent preserves the scope Gangline launched. The tmux
server's team scope remains `gangline-<session>.scope` because the team name is not
changed in place.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
