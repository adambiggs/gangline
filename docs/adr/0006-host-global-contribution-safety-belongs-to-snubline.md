---
id: 0006
status: accepted
date: 2026-08-12
---

# ADR-0006: Host-global contribution safety belongs to Snubline

## Context

The operator runs one host-installed pre-push gate, including its PII scanner,
across every repository. A second scanner copy inside Gangline is a second thing
to keep correct, and the copy that lags is the one that reports clean.

## Decision

Gangline carries no scanner, scanning CI or scanning tests. Its pre-push hook
delegates to the executable global hook first and then runs its own lint and
commit checks; an absent global hook is a no-op. The outer gate writes straight
to the terminal rather than being captured and replayed.

## Consequences

A clone without the operator's gate pushes unscanned, and public CI runs no PII
scan; that gap is accepted rather than closed with a vendored copy. Because the
outer gate can spend minutes in inference, its progress shows while it runs
instead of reading as a hang.
