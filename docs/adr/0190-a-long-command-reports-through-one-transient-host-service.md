---
id: 0190
status: proposed
date: 2026-09-12
supersedes: []
superseded-by: []
tags: [commands, lifecycle]
---

# ADR-0190: A long command reports through one transient host service

## Context

Codex command handles require repeated tool continuations to learn completion.
A sandboxed Codex turn cannot retain an ordinary detached child after the turn
ends. The local host user manager is, however, already reachable as
`systemd-run --user --machine=$USER@.host` from a workspace-write Codex
environment. The Codex sandbox is not this team's security boundary; bolster
is. Gangline therefore must not imply that this command creates or protects a
new execution boundary.

## Decision

Gangline accepts argv from a registered requesting agent, records it durably,
and starts one transient service through the current user's local-host systemd
manager. The service inherits the requester's working directory, `PATH`, and
`TMPDIR` (with a durable state-tree default for an absent `TMPDIR`). It writes
complete combined output and its normal exit result. An `ExecStopPost`
finalizer writes a fallback result for cancellation, timeout, OOM, or a killed
runner, then sends exactly one bounded completion through ordinary attributed
delivery. The finalizer clears the active declaration even if Gangline's
completion invocation itself fails, so a failed service cannot consume a team
slot indefinitely.

Every completed run appends one per-team audit line with requester, lossless
base64 argv, exit, duration, and output path. A run is cancellable only from a
live pane bearing its requester stable identity. Completion resolves that token,
so a rename still receives it; a gone or replaced requester receives no
substitute delivery and retains the named record instead.

## Consequences

`gang run` is a convenience wrapper over host execution that the team already
grants its agents; it is not a security boundary or a new capability. The
`systemd-run --user --machine=$USER@.host` path is the concrete host-execution
route on which that statement rests. It continues only while the host exposes
that user manager; otherwise Gangline refuses before accepting a command.

The service is ephemeral and no daemon, scheduler, or polling loop is
introduced. The manager and durable state directory are prerequisites. A result
that cannot enter delivery remains named beside its output and audit line. The
decision is falsified if a real Codex requester still needs a command-handle
continuation after its completion service exits.
