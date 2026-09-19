---
id: 0003
status: accepted
date: 2026-08-11
---

# ADR-0003: The irreversible verb is the one that demands an argument

## Context

Every argument-taking command answered a bare invocation with its usage; `gang
down` did not, so running it bare to see what it wanted ended the team, and
`gang down lead` ignored its extra argument and ran a full teardown (#72).

## Decision

`gang down` requires the exact session it ends and refuses to run from inside
that session.

## Consequences

Ending a team always takes a deliberate, named argument. The cost is typing the
session name, which `gang teams` prints.
