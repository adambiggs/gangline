---
id: 0017
status: accepted
date: 2026-09-06
---

# ADR-0017: A refused delivery is parked, a failed one is not

## Context

A refusal happens before any keystroke, so the body is still the sender's. A
failure after a paste has an unknown fate, and a second copy of a message that
may have landed is worse than one loud failure.

## Decision

A refused message is parked in the target's spool by default; `--live-only`
refuses instead. Entries are claimed out of the spool before delivery. An entry
whose delivery could not be verified, or whose drain died mid-flight, is held
and counted, never re-sent. A window's spool identity is minted at hitch and
adopt, where nothing can race it.

## Consequences

Gangline never sends a message twice on the chance the first did not arrive,
and never holds one without naming it. A boundary with no readable composer
records a drain failure and leaves every entry unclaimed.
