---
id: 0156
status: accepted
date: 2026-09-02
supersedes: []
superseded-by: []
tags: [usage]
---

# ADR-0156: Quota and outcome policy stay outside usage accounting

## Context

Usage evidence crosses process and sandbox boundaries while accounting policy remains
operator-owned.

## Decision

Quota percentages stay with `gang limits`, cross-host aggregation stays with whatever
carries the file between hosts, and no success column exists because Gangline records no
outcome.

## Consequences

`docs/records/usage-spec.md` holds those follow-ups.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
