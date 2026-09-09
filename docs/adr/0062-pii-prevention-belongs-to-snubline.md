---
id: 0062
status: accepted
date: 2026-08-09
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0062: PII prevention belongs to Snubline

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Gangline carries no scanner, no scanning CI, and no scanning tests. The host-installed
pre-push gate owns the patterns, fixtures, and scan. A second copy is a second thing to
keep correct, and the copy that lags is the one that reports clean. Scanning stays
prospective — it reads what a push would add and never rewrites history, and any history
rewrite remains a separate explicit decision. Keep operator-specific denylist values
untracked; `.gitignore` covers the gate's `.pii-scan-denylist`, which the gate refuses
to let anyone commit.

## Consequences

The cost is accepted deliberately. Gangline's public CI runs no PII scanner, so a push
can reach Gangline without a public-CI PII check. That missing coverage does not justify
re-vendoring a scanner here.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
