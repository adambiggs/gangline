---
id: 0031
status: accepted
date: 2026-08-13
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0031: Occupancy is not authority

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Refuse ordinary input whenever a harness-owned UI occupies the composer, and do not
infer who may clear it. Occupancy recognition belongs in collars, as
`GANG_OCCUPIED_REGEX`, and unknown authority fails closed.

## Consequences

Gangline answered a collar-enumerated whole-block fingerprint through 1.x, with
authority language mechanically forbidden and directory trust as the one narrow
exception. 2.0 removes that: the registry, its per-dialog fingerprints, the key driving,
and both collars' records are gone. It bought dismissing one Codex wait screen and
repeating a directory-trust choice `hitch -d` had already made, and cost the most
version-fragile and security-sensitive TUI machinery in core — per-build strings that
rot into a silent fallback while we believe we have coverage, which is the same argument
that already refused a name-only registry. What remains is the simpler product that was
always underneath: occupied means occupied, whoever drew the screen, and answering one
is `gang attach`.

At hitch, positive evidence of a prompt without a composer is an operator outcome before
it is a launch failure: report `gang attach` once and spend the remaining original boot
bound waiting. A blank pane is only startup, not prompt evidence. Unknown prompts remain
manual, but clearing one leaves a healthy agent whose startup contract can be delivered
with `gang send`; only a composer that never appears calls for drop-and-re-hitch.

A native permission-request witness has no timed decay. The readable composer that
proves the dialog ended is its only closer; malformed evidence refuses instead of being
erased. A synthetic `remain-on-exit` survivor read identically before and after the
former bound, so the timer prevented nothing, while under tmux defaults a dead harness
removes its window and leaves no orphan to report.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
