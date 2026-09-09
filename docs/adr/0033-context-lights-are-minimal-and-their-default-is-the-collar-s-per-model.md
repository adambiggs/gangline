---
id: 0033
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [collars, models]
---

# ADR-0033: Context lights are minimal, and their default is the collar's per model

## Context

Supported harnesses expose different native controls and evidence, so core behavior
needs an explicit collar boundary.

## Decision

Expose exactly yellow and red at intentionally high thresholds, notify once per context
epoch, and leave the decision to compact with the agent. Place both thresholds below the
observed native automatic-compaction boundary while preserving most of the effective
window. Expose the same native reading as an on-demand query whether or not lights are
enabled, because signalling and asking are different acts.

## Consequences

One team-wide absolute pair cannot fit a mixed team: a red sized for the widest native
window cannot fire in the narrowest at all. So the thresholds an agent gets default to
the collar's own answer for the hitched model, and the collar answers per model because
one harness runs models whose windows differ several-fold and the same fraction leaves
very different absolute runway in each. Collars express those defaults as fractions,
since the window a model reports is the provider's to change while the fraction stays
correct. An explicit spec overrides that answer for one agent or for the team, and
absolute tokens remain available there for a single observed window.

A default never arms a light its own collar cannot take a reading for; an explicitly
configured threshold still does, because that was an ask. Where a collar wires its
native context source at launch, the collar is sourced already knowing the agent's
request, so the wiring and the armed thresholds cannot disagree.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
