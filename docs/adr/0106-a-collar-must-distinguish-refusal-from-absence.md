---
id: 0106
status: accepted
date: 2026-08-18
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0106: A collar must distinguish refusal from absence

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Producing status `3` is the collar's obligation and cannot be performed on its behalf.

## Consequences

Where a collar answers `1`, Gangline asks the pane directly, and that probe is worth
exactly one thing: a transport still refusing turns the absence back into an unknown.
The converse is not available. A pane that answers proves the transport is up at that
moment, and the collar never parsed that reading, so it is no evidence about the read
that already happened — a refusal that healed in between remains an absent box to a
predicate that looks once. Claiming otherwise would be the same fabrication one layer
up.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
