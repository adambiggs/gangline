---
id: 0196
status: proposed
date: 2026-09-13
supersedes: []
superseded-by: []
tags: [gates, release]
---

# ADR-0196: The mandatory gate measures contributions and preserves release integration

## Context

The contribution gate must be observed under five minutes. Full serial
integration takes much longer; terminating it would refuse contributions.
Deleting assertions would misreport coverage. Releases still need the complete
proof.

## Decision

Gangline runs lint and smoke in the snapshot contribution gate and prints queue
time, total wall time after heavy-lock acquisition, and per-part timing at its
end. The unchanged full integration suite runs in `test/release.sh` with lint
and smoke on one settled tree under the same lock. A check stays mandatory only
when its healthy aggregate is observed under five minutes; otherwise it moves
intact to this pre-release lane. The `release` job runs it for Release Please
pull requests. A merger must confirm its passing result; remote enforcement
remains the operator's configuration choice.

## Consequences

Contributions receive measured evidence without claiming every integration
assertion. Releases wait for complete proof, but a merger can bypass a procedure
the repository does not enforce. The decision is wrong if the gate omits timing
evidence or invokes full integration, or a Release Please pull request merges
without a passing release-lane result.
