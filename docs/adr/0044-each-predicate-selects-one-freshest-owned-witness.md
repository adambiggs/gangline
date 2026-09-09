---
id: 0044
status: accepted
date: 2026-08-27
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0044: Each predicate selects one freshest owned witness

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

For each fact, prefer the freshest owned event, then owned file state, then pane
scraping; witnesses do not vote.

## Consequences

Expired or contradictory evidence is unknown and surfaced, and hooks translate native
facts. No resident process continuously reconciles them; the cooperative tick takes one
bounded fresh reading per invocation. Unknown never vetoes an action that fresher direct
evidence proves safe — the action's own verification carries the residual risk — and
state the new evidence refutes is retired at that moment, never by a patrol. Retirement
applies only to state gang alone writes. No reader — delivery or status — repairs the
turn bracket because tmux offers no atomic compare-and-delete. A tick delivery into a
hook-enabled target is the one writer outside the hooks: it records the positive open
edge it creates before Enter, so a native UserPromptSubmit/Stop pair can overwrite even
when a tiny turn closes before verification returns. A malformed value is reported as
unreadable, never repaired, and eligibility is re-derived per action. A hookless window
without mid-turn input whose pane keeps a frozen busy marker therefore stays refused
until the marker scrolls off or the agent is renewed — fail-closed by intent.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
