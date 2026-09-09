# TD-0005: the dead PID-namespace probe races under host process churn

- **Status:** Open
- **Date:** 2026-09-09
- **Scope:** `test/integration-tick.sh` dead-namespace liveness fixture

## Problem

The mandatory assertion `no process remains in the dead namespace, which the
host reads as death` can report `expected [1], got [0]` while several other
integration suites create and retire processes concurrently. The same base and
changed trees pass the focused `cli,tick` lane in isolation, so the failure is
not attributable to the PATH-shim guard or its exported depth variable.

The assertion reads a live `/proc` surface after the namespace owner exits. PID
reuse and concurrent namespace churn can therefore make a valid dead-owner
fixture appear live during the full-suite context.

## Direction

Give the fixture a stable, fixture-owned observation boundary. Identify the
owner by namespace and process identity without allowing unrelated host churn
or PID reuse to satisfy the liveness probe, and keep the production
fail-closed behavior for genuinely unobservable namespaces.

## Acceptance

The focused tick lane and the full gate remain green under concurrent process
and PID-namespace churn. A live owner still retains its lock, a dead owner is
retired, and an unobservable namespace remains unknown rather than dead.
