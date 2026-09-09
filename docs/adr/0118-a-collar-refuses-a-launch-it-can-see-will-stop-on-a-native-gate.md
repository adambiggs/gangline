---
id: 0118
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [gates, collars]
---

# ADR-0118: A collar refuses a launch it can see will stop on a native gate

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

Gangline does not answer native dialogs, and `hitch` behaves correctly when one appears:
the screen is occupied, delivery parks, and the prompt is named for a person. What that
costs is a boot spent to learn something knowable beforehand.

## Consequences

Where a harness will answer, before it starts, whether it is about to gate its own boot,
the collar asks and refuses the launch instead — naming what is ungated and the exact
native command that clears it. Codex answers this about its hook trust through its
app-server, under the same overrides the launch will use, so nothing reproduces its hash
algorithm or reads its trust records behind its back.

The refusal grants nothing. Trust decides what may run outside the sandbox and stays the
operator's; the remediation is the operator answering the native menu once. A state the
collar cannot read is refused as well: launching blind would restore the stall the check
exists to remove, and a gate that quietly degrades while it is believed to hold is the
worse failure.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
