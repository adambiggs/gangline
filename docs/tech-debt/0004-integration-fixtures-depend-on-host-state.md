# TD-0004: two integration fixtures depend on host state outside their snapshot

- **Status:** Open
- **Date:** 2026-08-29
- **Scope:** `test/integration-cli.sh` and the integration fixture environment

## Problem

Two mandatory integration assertions fail under the same host conditions on
both the clean `b61ae83` tree and a snapshot carrying the E1/E2 remediation:

    the captured NUX immediately before a composer hides that box
    the Codex effort vocabulary binds the configured model hitch will launch

The first expects the Claude NUX capture to hide the composer (`expected [1],
got [0]`). The second expects the configured Codex model to expose `low`,
`medium`, and `xhigh`, but receives no vocabulary. The gate therefore cannot
give a green verdict for either tree in this environment.

## Evidence

`test/gate.sh` ran first from the remediation checkout and then from a clean,
detached worktree at `b61ae838b8e9541b9812bed683e5676dedf8ab03`. Both runs
passed the command-surface smoke phase and emitted the same two assertion
failures from the integration phase with the values above. The remediation
does not modify either fixture surface.

## Direction

Make each fixture declare and provide every input it needs: the NUX capture
must not depend on an ambient tmux or harness state, and the configured-model
probe must bind a fixture-owned Codex configuration and native CLI response.
Keep both assertions capable of producing their stated red result; do not turn
an absent capture or unobservable vocabulary into a pass.

## Acceptance

The full gate is green on both an ordinary developer host and a host that has
an active Gangline team. Each assertion fails when its nominated fixture input
is deliberately removed or malformed.
