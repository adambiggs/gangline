---
id: 0063
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0063: Host-global contribution safety belongs to Snubline

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The host-installed pre-push gate remains outside Gangline, including its dispatcher,
deterministic PII scanner, installer, and behavioral suite. Gangline remains a consumer:
its repository-local pre-push hook delegates to an executable global hook before running
its own lint and commit gates, so opting into `.githooks` does not shadow the operator's
machine-wide gate. The dispatcher suppresses re-entry only when resolved hook identities
agree; the local hook never trusts an ambient recursion variable. An absent global hook
is a silent no-op. Gangline tracks no scanner of its own; outward delegation is the
whole of its participation.

## Consequences

The outer hook writes to the terminal, and Gangline does not capture it to replay the
verdict last. What replaying it bought is given up knowingly: the global gate's verdict
printed in place sits above every line this hook emits afterwards, and holding it back
made it the closing lines of a successful push, where a person reads the result. It is
given up because a host-global gate that spends minutes in inference reports its
progress while it runs, and withholding that until Gangline's own lint and suite finish
makes a live push indistinguishable from a hang. Fresher observation outranks closing
position.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
