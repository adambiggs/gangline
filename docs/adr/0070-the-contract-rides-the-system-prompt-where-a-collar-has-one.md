---
id: 0070
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [collars]
---

# ADR-0070: The contract rides the system prompt where a collar has one

## Context

The contract must survive compaction and pane-bound delivery limits.

## Decision

The standing terms live in `CONTRACT.md`, resolved operator-first and validated before a
window opens; a missing or non-prose contract refuses the hitch, because an agent sent
to read a contract that is not there would find that out alone, in a pane, with nobody
to tell. Where a collar declares `GANG_ROLE_PROMPT_OPT`, the contract passes through it:
the harness resends that prompt every turn, so the terms are unconditional and survive a
compaction without a re-read. Collars without the option point the startup contract at
the file instead, which is still better than a paste bounded by what a composer renders
and by pane geometry. A role brief joins the contract in that prompt wherever the collar
has the option, and is pasted where it does not; doctrine is pasted always. Neither can
be replaced by a pointer the way the contract can: they are per-hitch, and the pointer
buys them nothing.

## Consequences

Prose validation proves file shape, readability, NUL and control-byte absence, and
UTF-8. It does not cap bytes: the former threshold admitted a measured body the composer
could not render while refusing differently shaped prose a system prompt could carry.
Pane-bound prose fails at verified delivery; system-prompt prose meets the harness,
operating-system, and model-context limits that actually consume it. A mis-pointed
binary still reaches the content checks rather than being inferred from size.

A hitch may be role-less, but the contract is always present, so a role-less hitch still
carries a system prompt. Both share one option rather than passing it twice, because a
collar declares a single spelling and repeating it would guess at whether the harness
concatenates or keeps the last.

The startup contract does not repeat what the system prompt says. An agent that reads
the same assurance twice can still only act on it once, and Gangline cannot verify that
a launch option reached the model, so that line is spent on the recovery instead: where
the contract lives, and an instruction to report a missing attachment rather than
improvise.

Revisit if a supported collar preserves the contract across compaction without its
system-prompt option.
