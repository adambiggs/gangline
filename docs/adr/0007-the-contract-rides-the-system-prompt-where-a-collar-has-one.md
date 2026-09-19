---
id: 0007
status: accepted
date: 2026-08-12
---

# ADR-0007: The contract rides the system prompt where a collar has one

## Context

A contract typed into a pane is lost to the next compaction, and a paste is
bounded by what the composer renders and by pane geometry. A byte cap on the
contract admitted a body the composer could not render while refusing prose a
system prompt could carry.

## Decision

The standing terms live in `CONTRACT.md`, resolved operator-first and validated
before a window opens; a missing or unreadable contract refuses the hitch. Where
a collar declares `GANG_ROLE_PROMPT_OPT`, the contract and role brief go into the
harness's system prompt. Otherwise the startup message points the agent at the
file. Doctrine is pasted.

## Consequences

The terms survive compaction without a re-read wherever the harness has the
option. A running agent keeps the contract it launched with, so an edit reaches
live agents only through a re-hitch or, for pointer collars, the next read of
the file.
