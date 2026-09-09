---
id: 0079
status: accepted
date: 2026-08-12
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0079: A fixture shell is hermetic

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Gangline's suite compresses gang's production waits, so the pane's own reaction to a
keystroke is the whole margin. A bash launched with `--rcfile` or `--init-file` still
reads `/etc/bash.bashrc`, where Debian installs a `command_not_found_handle` that runs a
Python program against a multi-megabyte apt database — and every envelope this suite
delivers is an unrunnable command, so that handler lands on the Enter path of every
submission gang verifies. It cost most of the compressed budget on an otherwise idle
box, and it made the suite's verdict depend on the operator's system configuration and
page cache.

## Consequences

Fixture rc files therefore reach their shell through `ENV` in posix mode, where bash
reads no system rc at all, or the shell takes none with `--norc`. `test/lint.sh` checks
it. Enforcement rather than convention is the decision: the property was already known
and written into most fixture rc files as a line authors copied, and the fixtures that
starved were the ones that had not.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
