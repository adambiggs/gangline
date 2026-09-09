---
id: 0073
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0073: The mandatory gate runs from a private snapshot

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

Run the mandatory gate through `test/gate.sh`. It copies the working tree — tracked,
staged and untracked alike — into a private snapshot, commits it there, and runs lint
and the suite from that copy.

## Consequences

Two properties follow and both are load-bearing. The complete gate becomes runnable
before a commit, because the executable is clean against the snapshot's own HEAD and the
dirty-execution warning that one mandatory stderr assertion reads as failure never
fires. No assertion was relaxed to reach that, and no suite-only environment switch
exists to relax one later. And the run owns its tree: bash reads a script incrementally
and `gang` re-reads collars and roles at hitch time, so an edit landing mid-run
otherwise changes what executes.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
