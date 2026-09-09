---
id: 0093
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [collars, models]
---

# ADR-0093: Real harness proof is one opt-in lane against a local model server

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Every mandatory test drives a fixture: a shell pretending to be a harness, an immediate
clock, a pane whose transitions are synchronous. That is the right trade for a gate
everyone runs, and it leaves the collar unproven — the pane regexes, the native hook
wiring, the turn bracket and the transcript readers all describe a harness no mandatory
test ever starts. `test/e2e.sh` boots the real one against `test/e2e/stub.py` instead of
a provider, so the proof costs no network, no account and no money.

## Consequences

It stays out of `test/gate.sh`. A real boot costs seconds and the lane holds a turn open
deliberately, which is exactly what the mandatory suite forbids; `test/lint.sh` grants
this one file the timing exemption and refuses if the file ever appears in the gate, so
the exemption cannot outlive its reason.

The stub answers on markers the lane puts in the prompt, hardcoded, rather than through
a scenario language. Its request log is the instrument: a claim that an envelope was
delivered is settled by finding it in what the harness actually sent, not by reading it
off a pane that shows only what was typed.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
