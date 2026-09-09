---
id: 0182
status: accepted
date: 2026-09-09
supersedes: []
superseded-by: []
tags: [testing]
---

# ADR-0182: Same-name test PATH shims share one fail-closed guard

## Context

A test executable named for the command it wraps can resolve itself as the supposed
real command. A wrapper that starts that result as a child grows one waiting process per
invocation and can exhaust the host process table before an elapsed-time limit reacts.
An absolute path prevents the ordinary lookup error, but a bad substitution or an alias
can still name the wrapper itself.

## Decision

Every same-name PATH delegator in the test suite obtains an absolute executable target
and passes that target, its own path, and its command label through
`test/path-shim-guard.sh` before invoking the target. A delegator may resolve to another
guarded shim already on PATH; file identity prevents a self-loop and inherited depth
bounds the resulting chain.

The shared guard refuses a non-absolute or non-executable target, refuses a target with
the shim's file identity, and carries one inherited delegation depth with a ceiling of
eight. The lead-evaluation recorder uses this shared implementation rather than keeping
a private copy of the same checks.

## Consequences

`test/path-shim-guard-test.sh` is part of the mandatory integration suite. Its
self-targeting `git` has the child-and-wait recursion shape, but enters a transient user
service whose `TasksMax=12` is read from inside the service. The launcher refuses before
starting the shim unless that observed ceiling is exactly 12.
The expected verdict is the guard's immediate self-file refusal; the task ceiling is a
host-safety backstop, not passing evidence.

A fixture with interception logic may call its real target more than once after the
single guard at entry. Repository convention requires a new same-name delegator to use
the same helper; the current inventory is reviewed rather than inferred by a lint rule.
A different local depth or identity mechanism would split the invariant again. The
helper and its test can be deleted when the suite no longer constructs same-name
delegators.
