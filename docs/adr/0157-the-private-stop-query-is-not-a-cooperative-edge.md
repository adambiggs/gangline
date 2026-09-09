---
id: 0157
status: accepted
date: 2026-09-03
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0157: The private Stop query is not a cooperative edge

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The private `reply-obligations` command is the read-only first stage of the Stop
adapter, not a separate activity edge.

## Consequences

Its terminal `gang hook` invocation supplies the cooperative tick. Launching from both
stages doubled whole-team passes for every clear Stop; under a busy team that
amplification kept the singleton dirty and spent both commands' foreground budgets. A
released re-Stop still runs the state-mutating `reply-released` command before its hook,
and each retains its tick. Other commands retain the one-tick rule.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
