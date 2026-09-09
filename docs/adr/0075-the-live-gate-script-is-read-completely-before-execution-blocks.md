---
id: 0075
status: accepted
date: 2026-08-11
supersedes: []
superseded-by: []
tags: [gates]
---

# ADR-0075: The live gate script is read completely before execution blocks

## Context

A mandatory gate must remain reproducible, bounded, and honest about evidence it cannot
obtain.

## Decision

`test/gate.sh` is the one file the snapshot cannot protect, because it is the file
running from the live tree while the copy is judged.

## Consequences

Its whole executable body is one function, called on the last line with the exit, so
bash has read the file before it blocks and a save landing mid-run reaches nothing. And
a relative symlink is judged by where it points, not by whether the source end of it
exists: a dangling one resolves against the destination's parent and can read bytes the
source never held.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
