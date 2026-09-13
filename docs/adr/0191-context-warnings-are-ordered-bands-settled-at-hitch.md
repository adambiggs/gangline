---
id: 0191
status: proposed
date: 2026-09-12
supersedes: []
superseded-by: []
tags: [context, configuration, hooks]
---

# ADR-0191: Context warnings are ordered bands settled at hitch

## Context

The fixed yellow/red pair cannot express distinct operator checkpoints, and a
warning itself consumes recipient context. Reading configuration at every hook
would let a mid-climb edit redefine an already-fired policy. Discovering an
unknown template name at a hook would be late and leave a partial policy.

## Decision

`GANG_CONTEXT_BANDS` is opt-in beside the legacy decisions. When unset, the
legacy per-model context-light selection and wording remain byte-for-byte the
default. Otherwise it is a map with exact `COLLAR/MODEL`, then `COLLAR/*`, then
required `*` fallback. One entry holds ordered named
`BAND@THRESHOLD:TEMPLATE` values. Gangline validates every selector, template
placeholder, unit, and strict threshold order at configuration load, then
settles the selected literal list onto a window at hitch.

The hook emits every newly crossed band in configured order, remembers the
fired prefix for the climb, and resets only below the first band. It renders
only native context, cache-reader, identity, and clock values Gangline can
measure. Cache values without a usable reader are `unavailable`. A bounded
context-event ledger records timestamp, agent, band, threshold, and rendered
byte length, never the configured or rendered text.

## Consequences

Operators keep templates short because the rendered text enters recipient
context. A changed map affects only later hitches. `gang config` exposes
selector order and a non-live sample without exposing journal bodies. A map
requires its global fallback rather than silently falling back to a legacy pair.
The legacy `--lights` option remains a one-agent compatibility override.
