---
id: 0100
status: accepted
date: 2026-08-15
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0100: A budget that cannot be spent is not a bound

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The lane's bounded waits discarded exhaustion and let the following read decide, so a
wait that used its entire allowance still passed on the extra moment the last nap gave
it.

## Consequences

Exhaustion is now a failure in its own right, and the assertion after it stays only to
report what was true when the wait gave up. The same rule closes the run: cleanup that
fails, a signal handler that returns into a torn-down world, and a scenario list that
selects nothing are all ways of finishing without reporting, and each one now ends the
run with a reason.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
