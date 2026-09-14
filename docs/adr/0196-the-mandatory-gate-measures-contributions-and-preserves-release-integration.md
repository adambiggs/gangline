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

The contribution gate must be observed finishing in five minutes. Full serial
integration takes much longer, so terminating it at a deadline would refuse
valid contributions. Removing assertions would misreport coverage. Releases
still require the complete proof.

## Decision

Gangline runs lint and smoke in the snapshot contribution gate and prints total
and per-part timing evidence at its end. The unchanged full integration suite
runs in `test/release.sh` with lint and smoke on one settled tree under the same
heavy-test lock. A check stays mandatory only when its healthy aggregate path
is observed below five minutes; otherwise it moves intact to this pre-release
lane. The `release` job runs it for Release Please pull requests. A merger must
confirm its passing result; remote enforcement remains the operator's
configuration choice.

## Consequences

Contributions receive measured evidence without pretending to prove every
integration assertion. Releases wait for complete proof, but a merger can
bypass a procedure the repository does not enforce. The decision is wrong if
the gate omits timing evidence or invokes full integration, or a Release Please
pull request merges without a passing release-lane result.
