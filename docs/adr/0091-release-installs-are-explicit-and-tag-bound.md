---
id: 0091
status: accepted
date: 2026-08-14
supersedes: []
superseded-by: []
tags: [release]
---

# ADR-0091: Release installs are explicit and tag-bound

## Context

Release automation must preserve the exact commit and tag selected by the release
process.

## Decision

`install.sh` and `gang upgrade` resolve the greatest stable semantic
`gangline-vMAJOR.MINOR.PATCH` tag and install that commit, never the moving `main`
branch.

## Consequences

`gang upgrade --check` is the sole availability probe; ordinary commands stay offline. A
named operator action keeps network failure visible and avoids adding a cache, timer,
startup tax, or self-watching component.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
