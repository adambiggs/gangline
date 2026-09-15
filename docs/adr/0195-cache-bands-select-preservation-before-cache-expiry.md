---
id: 0195
status: accepted
date: 2026-09-14
supersedes: []
superseded-by: []
tags: [cache, compaction, configuration]
---

# ADR-0195: Cache bands select preservation before cache expiry

## Context

ADR-0191's context bands answer how much runway remains and deliberately warn
the agent. ADR-0017's cache-expiry backstop needs only whether an idle warm
context is worth preserving. Reusing the warning map would couple those
judgments and turn a preservation policy into extra recipient text.

This proposed record documents the separately issued 2026-09-14 directive; that
directive, not this record, authorizes implementation pending acceptance.

## Decision

`GANG_CACHE_BANDS` is a separately settled ordered band map with ADR-0191's
grammar and placeholders. A selected map replaces the automatic backstop's
first-context-warning eligibility with its first crossed cache band. Its highest
crossed template replaces the built-in preservation instruction in the native
compaction command. A crossing emits no warning. Unset, whole-map `off`, and a
selected `off` entry retain the previous eligibility and instruction.

## Consequences

Operators can tune preservation independently from runway warnings. Cache bands
need the collar's native context reading and fail closed without it. The map is
visible through deterministic `gang config` samples but templates are not
journaled. This is falsified if crossing a cache band delivers an advisory, or
if an unset or off map changes automatic-compaction eligibility or instruction.
