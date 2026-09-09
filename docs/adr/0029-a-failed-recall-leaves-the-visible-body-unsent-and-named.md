---
id: 0029
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0029: A failed recall leaves the visible body unsent and named

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Missing evidence, a park recorded without its body, a key that loads nothing, and a
readback that does not match are all refusals with nothing pressed.

## Consequences

A refusal that lands AFTER the recall has changed the world, so it reports what gang can
see and the part it cannot: the loaded body is visible and unsent, the Enter was not
pressed, and whether a copy is still waiting in the harness's own queue was never read —
which is why submitting the visible draft by hand may deliver it twice, and why gang
does not promise it drains on its own either. The post-Enter proof is one shared
implementation, because two copies of it would drift and the drifted one would report a
submission nobody saw.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
