---
id: 0142
status: accepted
date: 2026-08-31
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0142: A long Codex command needs no Gangline surface

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Issue #177 reported a command wrapper that self-terminates a long command part way
through. codex-cli 0.151.0 has no command wrapper: a model-invoked command becomes a
background terminal whose only bound is `background_terminal_max_timeout`, and a `!`
local-shell escape is not wrapped at all.

## Consequences

A command that outlives a wait slice therefore returns the slice rather than a signal,
and the model waits again. Nothing is cut off, so there is nothing for a collar to
survive and nothing for gang to surface; the error the harness makes easy is reading a
slice return as a completion, which is a rule for the agent rather than a surface for
the substrate.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
