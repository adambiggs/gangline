# TD-0002: a launch that dies after the preflight is reported as a slow boot

- **Status:** Resolved 2026-08-17 (7f9c2ed, b883253, 053e6a1)
- **Date:** 2026-08-17
- **Scope:** `bin/gang` — `wait_ready` and the hitch boot loop

This is a correctness defect rather than ordinary debt: the refusal names a
state that does not hold. It is filed here because this repository keeps one
register for open work, and the fix is blocked on machinery the boot loop does
not have rather than on a decision.

## Problem

`wait_ready` never reads the pane's death. When a harness launch fails after the
preflight, the wait runs to the end of the boot budget and the hitch refuses
with

    'probe1' is up but is showing something other than its input box

— a live-agent recovery instruction for a process that is not running. The
operator is told to inspect a window whose command already exited, and pays the
full boot timeout to be told it.

A launch that dies *instantly* takes a different path: the window is lost before
registration and the failure surfaces as a raw `no such window: @1` with exit 1.
Neither path names the command that died.

## Evidence

Reproduced against the committed binary during arc 43, using a `systemd-run`
that fails while holding the pane. Found in independent review
(2026-08-17).

## Direction

Ending the wait on an absent or dead pane, and naming the command that died, is
about four lines, and those four lines were written and deliberately not landed.
They cannot be guarded: the mandatory suite's clock is stopped, so `wait_ready`'s
whole wait is one instant and there is no deterministic way to make a pane die
*inside* it without a busy-wait or a real sleep, both of which the suite bans.
Landing an unguarded change on a binary attached agents execute on save was
judged the worse trade.

Paying this down means giving the boot loop an event it can wait on — a pane-exit
barrier the fixture can fire — so the death is observable in the suite without a
timing dependency. The four-line fix follows from that; it is not the work.

## Acceptance

A hitch whose launch dies after the preflight refuses promptly, names the command
that died, and does not describe the agent as up. The guarding fixture drives a
real pane death inside `wait_ready`'s wait with no sleep, no polling, and no
scaled clock, and fails against the current binary.

## Resolution

Met. The wait ends on the death, and the refusal names the launch command, plus
the exit status and the pane's last line wherever a corpse was held. Every window
registration a hitch performs reads the same fact, so the instant-death path no
longer surfaces as a raw tmux error either.

The direction stated above was wrong about the cost, and cheaply: the boot loop
already had the event hook it was said to lack. A collar is sourced into gang's
own process and `wait_ready` calls the collar's `collar_input` once per pass, so
a fixture collar stages a real pane death at an exact point inside the wait with
no clock, no polling and no sleep. tmux's own `pane-died` hook is the barrier
that makes the death settled before the next read; a signal from the dying
shell's EXIT trap fires too early.

The guards fail against the parent binary — the reds are the two symptoms above
verbatim, plus the burned boot budget measured as the number of pane reads. One
defect was found in independent review and fixed before the fix landed: the
reading asked tmux for the window's *active* pane, so a split window holding a
corpse in front of a live shell read as empty, and the refusal that produces
names `gang drop`, which would have killed the live pane.
