---
id: 0090
status: accepted
date: 2026-08-13
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0090: The mandatory gate fits under a memory ceiling

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

Run ShellCheck per file and keep integration split into sourced parts so each mandatory
process stays below the gate's memory ceiling.

## Consequences

`test/lint.sh` runs one `shellcheck` per file, and `test/integration.sh` is split into
sourced parts for that reason alone. shellcheck holds an invocation's whole input at
once and its cost grows faster than that input does, so the set costs far more than the
sum of its files. Handed this repo as a single invocation it reached 6.1 GB; on
2026-08-12 the kernel OOM killer took it twice on an 11.6 GB host, and that host was
power-cycled hours later after a starvation livelock. A mandatory gate that is the
largest single allocation on the machine punishes the agent who runs it, which is the
opposite of what a gate everybody must run should do.

The rule is the ceiling, not a file count. The gate must pass under

```sh
systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0 -- test/gate.sh
```

and a file that grows until it alone will not fit is split. Parts are sourced rather
than executed, so the suite stays one program with one set of fixtures, counters and
ordering and the split moves no assertion. A part is a fragment, so shellcheck cannot
see across the boundary: a variable that crosses one carries a directive naming the file
at the other end, and that directive is the record that the crossing was deliberate.

Splitting re-keys `test/source-guards.allow`, whose fingerprints bind a statement to its
path on purpose. A move is settled by showing that the multiset of reviewed statements
is unchanged and carrying each review note onto its new key — never by regenerating the
ledger, which would launder an unreviewed guard through a mechanical step.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
