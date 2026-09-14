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

Accepted ADR-0167 requires full integration in the contribution gate. It may
conflict with the current split, but its status and metadata are outside this
proposal. A separate standing directive authorizes the split. This proposal
binds nothing until accepted.

## Decision

If accepted, the snapshot gate runs lint and smoke and reports queue, total
after lock acquisition, and part timing. Unchanged full integration runs in
`test/release.sh` with lint and smoke on one settled tree. A check stays
mandatory only when healthy under five minutes; otherwise it moves intact. The
`release` job selects a `release-please--` head branch; the merger confirms
origin, approves an `action_required` run, and verifies the release job on the
current head SHA. Remote enforcement remains the operator's choice.

## Consequences

The live tree follows the directive, not this proposal. Before acceptance, no
test cites ADR-0196 and relationships remain empty. Decision-record governance
determines whether the conflict with ADR-0167 warrants a successor. This proposal
is falsified if the gate omits timing evidence or invokes full integration, or a
Release Please pull request merges without a passing release-lane result.
