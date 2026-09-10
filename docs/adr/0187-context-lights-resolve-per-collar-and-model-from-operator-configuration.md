---
id: 0187
status: proposed
date: 2026-09-10
supersedes: []
superseded-by: []
tags: [collars, models, configuration]
---

# ADR-0187: Context lights resolve per collar and model from operator configuration

## Context

This proposal is aimed at accepted ADR-0033, whose explicit spec overrides the
collar's per-model default for one agent or the whole team. A team-wide spec fits
no mixed team: a percentage right for one harness's window fires far too early in
another's. Changing one pair's thresholds must not mean writing a collar.

## Decision

`GANG_CONTEXT_LIGHTS` is a list of `COLLAR/MODEL=SPEC` entries, either half `*`.
The most specific match wins — collar and model, collar, model, `*` — then the
collar's default. MODEL matches `-m` exactly, so specificity is the only ranking
rule. A bare SPEC is the `*` entry and warns at hitch. `-l` replaces the whole
map for one agent. Every entry is validated at every hitch.

## Consequences

ADR-0033 still holds for two high thresholds, one notice per epoch, the agent
deciding, fractional per-model collar defaults, and no default lighting what its
collar cannot read. Its single team-wide spec does not. An alias and its model id
are separate entries, and a selector naming nothing real is accepted silently;
`gang config` and the hitch line show it. Falsified if a matching `*` entry
settles a hitch that a matching `COLLAR/MODEL` entry names.
