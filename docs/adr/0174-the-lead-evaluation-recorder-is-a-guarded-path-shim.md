---
id: 0174
status: accepted
date: 2026-09-06
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0174: The lead-evaluation recorder is a guarded PATH shim

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The recorder is a shim named `gang` placed ahead of the real one on the staged lead's
PATH, and its three guards are mandatory.

## Consequences

It resolves the real binary from an absolute path baked in at write time and never
through PATH, because a lookup from a shim that is first on PATH returns the shim and
each call forks another copy — unbounded, self-accelerating, and fatal to every
unrelated process on the host. It carries a depth ceiling, because a hitched agent's own
harness invokes `gang` and legitimate nesting must be bounded rather than forbidden. It
refuses when the path it is about to run is itself, checking the conclusion rather than
trusting the substitution. Under an opt-in flag the two commands that would launch
something are recorded and answered without running, since scoring a dispatch decision
needs argv and nothing else; a scenario that needs live teammates must not set it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
