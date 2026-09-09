---
id: 0132
status: accepted
date: 2026-08-30
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0132: A free composer does not override native error state

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

A window whose turn ended without producing work draws a free composer, so every guard
Gangline keeps on the input box passes it. It reported `~idle~`; `send` typed into it
and reported delivered; `wait --until idle` returned satisfied.

## Consequences

The evidence was already being read. The claude-code fatal reader walks to the newest
semantic record and reaches the very record that proves the turn ended, then returns
absent because its `error` value is outside the two classes that reader owns.
Enumerating the classes that count makes every class the harness later ships a fresh
window that reads idle, arriving with no signal.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
