---
id: 0012
status: accepted
date: 2026-08-21
---

# ADR-0012: Mandatory tests are immediate

## Context

The integration suite stubbed `sleep` on PATH, so liveness and timeout-budget
defects structurally could not fail it (#114), and wall-clock waits went flaky
under team load.

## Decision

Mandatory tests do not sleep, poll, or use wall-clock delay as evidence. They
assert state the command has already established, through immediate reads,
event barriers or fake clocks. `test/lint.sh` enforces the ban across `test/`.

## Consequences

Where the behaviour under test is a timeout, a fake clock may be scaled rather
than stopped, and the fixture records its measured margin. Real harness turns
run only in the opt-in e2e lane.
