---
id: 0036
status: accepted
date: 2026-08-24
supersedes: []
superseded-by: []
tags: [architecture]
---

# ADR-0036: An automatic resume is an arming threshold, not a cap detector

## Context

Gangline needs one stable rule across concurrent state changes, incomplete evidence, and
supported harnesses.

## Decision

Let an operator declare one provider-used percentage that arms the existing reset wake
from the agent's own turn hook, separately from the warning thresholds.

## Consequences

Gangline cannot witness the provider's refusal: the only in-band evidence is pane prose,
already refused as a data contract, and a refused agent takes no further turns, so no
hook fires after the cap. Arming therefore happens at the last percentage observable
while the agent still runs, and the resulting continuation may reach an agent that never
capped — accepted, because it rides the ordinary spool and the alternatives are a
threshold that can never fire and a scrape. Arm once per provider window, keyed on the
reset that was decided for, so a cleared wake is not re-armed over the operator and a
refusal is not retried. One shared transaction serves the manual command and the hook:
it accepts the already-validated sample instead of taking a second native reading that
could cross a provider reset, and overlapping hooks serialize the marker check with the
arm. An existing manual wake is the operator's decision and wins over that automatic
transaction: mark the sampled window handled without cancelling its timer or replacing
its optional continuation. An overdue declaration, or a future one whose timer is
provably dead, is residue instead: replace it, and disclose when that replacement
discards a custom continuation.

Founding-import placeholder: no concrete falsifier was independently derived for this
record; any successor must supply one.
