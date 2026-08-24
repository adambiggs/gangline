# TD-0001: the queue-flush probe shares the product's composer reader

- **Status:** Resolved 2026-08-24
- **Date:** 2026-08-16
- **Scope:** `test/integration-compose.sh`, `gang composer`, and `cmd_flush`

## Problem

The queue-flush fixture observes the recalled body with `gang composer` after
its ordered input barrier. `gang composer` and `cmd_flush` both obtain that body
through the collar's `collar_input` implementation. The assertion can still
detect an Enter pressed after recall, but its witness is not independent of the
readback under test: a defect in `collar_input` can affect the product decision
and the test observation together.

## Evidence

Adding the erroneous `tmux send-keys Enter` to `cmd_flush`'s failed-readback
branch makes both composer-probe assertions fail, so the guard has a measured
negative. Code inspection shows that `cmd_composer` calls `collar_input`, while
the flush path reaches the same function through `landing_zone`.

## Direction

Keep the ordered barrier, but give the fixture an observation path independent
of `collar_input` before changing the shared reader or relying on this test to
validate it. The replacement must retain the mutation's red result and must not
turn an absent or unreadable composer into a pass.

## Resolution

The fixture now counts prompts in the target shell and snapshots that count
before each rejected flush. Its existing key barrier runs behind every key
`gang flush` sent before reading the count again. An erroneous Enter therefore
executes another command and changes the independent witness; the correct
refusal leaves it unchanged. The collar's composer reader participates in
neither observation, and an absent or malformed counter fails readiness instead
of passing as an empty composer. The original erroneous-Enter mutation still
makes both guards fail.
