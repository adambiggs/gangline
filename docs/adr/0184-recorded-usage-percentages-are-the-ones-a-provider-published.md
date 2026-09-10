---
id: 0184
status: accepted
date: 2026-09-09
supersedes: []
superseded-by: []
tags: [usage]
---

# ADR-0184: Recorded usage percentages are the ones a provider published

## Context

Both harnesses meter an account against a weekly allowance and keep no history
of it. Codex writes its reading into every session file; Claude
prints the same reading only in answer to a live turn. Token volume is visible
for both, and a percentage derived from it reads as a measurement while being an
estimate.

## Decision

`gang cap` stores and prints only percentages a provider published, each
carrying the surface it was read from; nothing is derived from token counts,
prices or elapsed time. A record under another limit id belongs to a pool with
its own allowance, not the account's window. A provider that cannot be read is
recorded as unreadable with the defect named, and that row carries no
percentage. Replay input and stored state are admitted on the same terms.

## Consequences

Claude's history begins when sampling begins and cannot be reconstructed
backwards, and each reading costs one provider turn.

This record is false if a percentage reaches a person unpublished by a
provider: a stored row naming no provider surface, a percentage on a row marked
unreadable, or a pass that read nothing yet reported a figure.
