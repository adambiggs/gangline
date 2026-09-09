---
id: 0080
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0080: A missing name is a self target

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Where a command's only required argument is an agent name, omitting that name targets
the calling window. An agent reading or stopping its own state does not have to know its
own name, and the pane already answers who it is. This holds for `usage`, `interrupt`
and `flush` as much as for `status` and `capture`: what the resolved target's state then
allows is a separate answer, and `usage` refusing its own mid-turn caller is that second
answer rather than a reason to withhold the first.

## Consequences

What is omitted is the name, not every argument. A leading flag is not a name, so `gang
interrupt -m "reason"` is a self-targeted stop carrying its reason; reading the flag as
a bad name made that the one self target an agent could not spell. The reason is
delivered to its own author on purpose — it is written to be read after the turn it
ended — where the same self-delivery on `send` stays refused as the accident it is
there. Gangline does not promise that delivery: the caller runs inside the turn it is
stopping, and a harness that ends that turn by killing the tool call takes the sending
process with it.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
