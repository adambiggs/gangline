---
id: 0179
status: accepted
date: 2026-09-07
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0179: The suite reaper runs outside the suite's execution boundary

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

The detached suite watcher is a transient systemd user service, armed before
`suite_reaper_start` returns. A new session changes signal routing but not cgroup
membership, and a process born in a child PID namespace cannot survive that namespace's
init. Either boundary can therefore kill a `setsid` watcher beside the parent it was
meant to observe.

## Consequences

The parent holds a fresh file inode under its claimed run root. The service finds the
inode's one holder through the host's `/proc`, opens a pidfd for that host process,
verifies that it occupies another cgroup and, when the parent is namespaced, another PID
namespace, then reports readiness through systemd's notify protocol. A namespace-local
PID is never translated or trusted. A host without pidfds, a readable Unix-socket table,
`systemd-run`, or a reachable user manager is refused rather than described as watched.

That PID-namespace check assumes the user manager is in the host namespace. A whole
container that places both the suite and its user manager under one PID namespace has no
surviving process inside that container when its namespace init dies; preserving
teardown across that boundary requires a manager outside the container and is not a
protection this reaper claims.

The transient unit is collected after the watcher exits. Its parent-identity file and
any launch-error record live under the run root and leave with the ordinary synchronous
or watched sweep; a refused launch removes them and the claim marker before returning.
If the watcher cannot remove the root, the root survives by definition, so the watcher
also leaves its removal diagnostic in `.suite-reaper-watch-error` rather than making the
user-manager journal its only audience.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
