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

Accepted ADR-0167 runs full integration in the contribution gate. Serial
integration cannot meet the five-minute policy; terminating it would refuse
contributions and deleting assertions would misreport coverage. This proposed
successor keeps the complete release proof.

## Decision

Gangline runs lint and smoke in the snapshot contribution gate and prints queue
time, total wall time after heavy-lock acquisition, and per-part timing at its
end. The unchanged full integration suite runs in `test/release.sh` with lint
and smoke on one settled tree under the same lock. A check stays mandatory only
when its healthy aggregate is observed under five minutes; otherwise it moves
intact to this pre-release lane. The `release` job selects a
`release-please--` head branch; the merger confirms Release Please origin,
approves an `action_required` run, and verifies the release job on the current
head SHA. Remote enforcement remains the operator's configuration choice.

## Consequences

Contributions receive measured evidence without claiming every integration
assertion. Releases wait for complete proof, but a merger can bypass a procedure
the repository does not enforce. The decision is wrong if the gate omits timing
evidence or invokes full integration, or a Release Please pull request merges
without a passing release-lane result.
