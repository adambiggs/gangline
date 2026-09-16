---
id: 0200
status: proposed
date: 2026-09-16
supersedes: []
superseded-by: []
tags: [messaging, provenance]
---

# ADR-0200: Hitch provenance is one vocabulary across every inspection surface

## Context

ADR-0130 resolves a hitcher by witnessed spool identity, but only `gang status`
read it. An operator comparing `gang roster` against `gang status` for the same
agent saw provenance in one and nothing in the other, and a pre-provenance or
otherwise unstamped agent read identically to one hitched by the operator: both
were silent.

## Decision

`gang roster` (human and `--porcelain`), and `gang explain`, resolve hitch
provenance through the same reader ADR-0130 established, and render one of
five states: a live parent, a parent whose window is gone, the operator
sentinel, nothing recorded, or an unknown reading. Nothing recorded is its own
explicit word, never silence and never folded into the operator case. A
window whose live-window enumeration itself fails reads `unknown`, never
`gone`: a failed check is not evidence of absence. `gang roster --porcelain`
carries this as two trailing columns, `hitcher_state` and `hitcher_name`,
appended after the existing fixed shape.

## Consequences

A script reading fewer than eight porcelain columns is unaffected; one reading
a fixed six-column shape by exact count must be updated. A future inspection
surface reads the same shared function rather than inventing its own
resolution. Falsifier: any of these commands ever names a parent for a window
whose `@gl_hitched_by` token does not resolve to it, or renders `gone` for a
window whose liveness it failed to check rather than ruled out.
