# Operations

Start with [your first team](quickstart.md). Use the [guides](guides.md) for
daily tasks and [troubleshooting](troubleshooting.md) for observable failures.

## Continue through a provider cap

Gangline reads the collar's native five-hour and weekly usage windows. When a
configured usage band is crossed, it sends one notice for that collar and reset
window to the active lead. A notice still waiting for a lead, or queued behind
one, when its window resets is withdrawn rather than delivered. The default
notice contains only the provider, reported usage, and reset time. Bands do
not change agent work. A collar overlay can supply an optional per-band `note`
with operator guidance.
Native
readings can be absent or stale; `gang limits NAME` shows what an agent has
observed, and `gang limits -c COLLAR` queries a collar that supports a
standalone native limits query.

When a native turn is rejected by an attributable provider cap, Gangline
automatically records a wake for that agent at the observed native reset.
If the reset is not yet known, the park remains pending until a fresh reading
supplies it. The agent's native conversation and the durable wake remain in
place. `gang snooze --at TIME --note TEXT` is available as a manual override.
At the due time, Gangline sends the note to the caller to resume work. If that caller is
gone, the active lead receives an overdue wake naming the caller. A wake
remains pending until the matching native turn finishes successfully. If
input is blocked before submission, ordinary ticks keep trying the queued
wake; `gang snooze --status` shows it as queued, and `gang snooze --clear` or a
new `gang snooze --at` withdraws it from the recipient's inbox. A submitted
prompt remains pending until its matching turn finishes successfully. When
native input is accepted or its outcome cannot be verified, Gangline records
the submission and does not type it again. The caller can
inspect an uncertain wake with `gang snooze --status` and clear it after
checking the native turn. The lead sees uncertain notices and wakes routed
from absent callers in the same status output, and can clear or withdraw one
with `gang snooze --clear ID`. Clearing cannot retract native input.
If an automatic wake is rejected by another attributable cap, Gangline
schedules another wake at the next observed reset. A manually scheduled wake
is re-armed once when a fresh reading confirms the capped window.
Failed turns and unknown completion stay visible in `gang snooze --status`
until the agent clears or replaces the wake.
An unconfirmed generic rate-limit error remains visible in
`gang snooze --status` while a fresh capped-window reading could confirm it.
Without that confirmation, Gangline does not park the agent. A failed manual
wake remains available for the agent to inspect or clear.

The schedule lives in the team state root, so it survives Gangline process
restarts, compaction, and host reboot. Gangline does not start a team after a
reboot. If the team is running at wake time, its watchdog delivers the wake;
otherwise the next team startup delivers the pending wake and marks it overdue.
Keep a durable work note because a wake can reach a different lead after the
original agent disappears. If the host stops after submitting a wake but before
recording successful completion, the next team startup treats that work as
unconfirmed and routes it to the lead if the caller is gone.
