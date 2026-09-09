---
id: 0172
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [gates, models]
---

# ADR-0172: Model-behavior evaluation stays outside the mandatory gate

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

The lane is opt-in and never gates a commit.

## Consequences

Its subject is a model's choice, so one run is one draw, and a verdict from it is
evidence about the fixture before it is evidence about the brief. `test/gate.sh`
therefore does not invoke it, and `test/lint.sh` proves that: the wall-time exemption
naming this lane is refused if the gate ever calls it, so exempting a file and then
wiring it into the mandatory suite fails loudly instead of buying that file a licence to
sleep.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
