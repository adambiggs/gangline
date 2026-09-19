---
id: 0008
status: accepted
date: 2026-08-13
---

# ADR-0008: Occupancy is not authority

## Context

Through 1.x, Gangline recognised each harness dialog by a per-build fingerprint
and drove keys to dismiss some of them. It bought dismissing one Codex wait
screen and repeating a trust choice `hitch -d` had already made, and cost the
most version-fragile TUI machinery in core: strings that rot into a silent
fallback while everyone believes coverage holds.

## Decision

When a harness-owned UI occupies the composer, ordinary input is refused and
Gangline does not decide who may clear it. Collars recognise occupancy with
`GANG_OCCUPIED_REGEX`; unknown authority fails closed.

## Consequences

An occupied composer is answered by a person through `gang attach`. 2.0 removed
the fingerprint registry, its per-dialog key driving and both collars' records.
