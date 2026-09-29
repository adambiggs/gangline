# Operations

Start with [your first team](quickstart.md). Use the [guides](guides.md) for
daily tasks and [troubleshooting](troubleshooting.md) for observable failures.

## Continue through a provider cap

Gangline reads the collar's native five-hour and weekly usage windows. When a
configured usage band is crossed, it sends one notice for that collar and reset
window to the active lead. The notice includes the observed reset time. Native
readings can be absent or stale; `gang limits NAME` shows what an agent has
observed, and `gang limits -c COLLAR` queries a collar that supports a
standalone native limits query.

At a checkpoint, the lead can ask agents to save their work and run
`gang snooze --note 'Resume from STATE_FILE'`. An agent can use
`gang snooze --at TIME --note TEXT` when the native reset is unavailable or
another time is appropriate. Agents then wait in their windows. At the due
time, Gangline sends the note to the caller to resume work. If that caller is
gone, the active lead receives an overdue wake naming the caller. A wake
remains pending until the matching native turn finishes successfully. If
input is blocked before submission, ordinary ticks keep trying the queued
wake. A submitted prompt remains pending until its matching turn finishes
successfully. When native input is accepted or its outcome cannot be verified,
Gangline records the submission and does not type it again. The status
command shows the uncertainty so the agent can inspect and clear the wake.
If Claude Code reports an attributable usage-cap refusal and a fresh native
reading confirms a capped window with a future reset, Gangline records it and
schedules one replacement wake at that reset.
A later refusal from that replacement is logged for the lead to handle.
Failed turns and unknown completion stay visible in `gang snooze --status`
until the agent clears or replaces the wake.

The schedule lives in the team state root, so it survives Gangline process
restarts, compaction, and host reboot. Gangline does not start a team after a
reboot. If the team is running at wake time, its watchdog delivers the wake;
otherwise the next team startup delivers the pending wake and marks it overdue.
Keep a durable work note because a wake can reach a different lead after the
original agent disappears. If the host stops after submitting a wake but before
recording successful completion, the next team startup treats that work as
unconfirmed and routes it to the lead if the caller is gone.
