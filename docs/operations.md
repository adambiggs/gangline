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
checking the native turn. The lead's `gang snooze --status` lists every
agent's wake with its due time and note, and uncertain usage notices; the
lead can clear a notice or withdraw a wake routed from an absent caller with
`gang snooze --clear ID`. `gang log --agent NAME` shows the wakes NAME
scheduled, cleared, completed, or failed; their delivery is logged under the
recipient. Clearing cannot retract native input.
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
reboot. If the team is running at wake time, its armed watchdog delivers the
wake. When no scheduler can arm a timer, or arming fails, `gang snooze` warns and `gang roster` shows
`[watchdog-unavailable]`; the wake then waits for the next hook tick or
`gang tick` in the team. If the team is not running, the next team startup
delivers the pending wake and marks it overdue.
Keep a durable work note because a wake can reach a different lead after the
original agent disappears. If the host stops after submitting a wake but before
recording successful completion, the next team startup treats that work as
unconfirmed and routes it to the lead if the caller is gone.

## Sandboxed development teams

Use a named Codex permission profile owned by the operator. Keep the sandbox
on and select a trusted checkout with `gang hitch NAME -c codex -d CHECKOUT`.
Gangline grants a linked worktree's private Git directory and existing common
`objects`, `refs` and `logs` at launch; see the [launch contract](reference.md).
It deliberately leaves shared config, hooks and other common root files out:
those can control code executed by later host Git commands. Shared refs and
objects still let workers move any branch, including one checked out in the
canonical checkout. Private metadata and the writable `.git` pointer can also
redirect later host Git commands to executable configuration. Use mutually
trusted teammates and review changes before executing or publishing them;
these grants do not make hostile repository content safe for host Git.

With the scoped `gangline` profile, operations that rewrite common-root
`packed-refs` require host-side execution. These include `git branch -d` for
packed branches, `git pack-refs`, `git gc`, and large fetches that trigger ref
packing or garbage collection.

The following is a starting profile for a dedicated team, using the
[Codex permission profile syntax](https://learn.chatgpt.com/docs/permissions).
Replace the example home and runtime paths with absolute paths on your host,
and merge tables into the existing config rather than duplicating them. The
repository metadata read rule lets Git read shared configuration and packed refs
without making them writable; add an exact rule per repository you use.
`GANG_CODEX_PERMISSION_PROFILE=gangline` selects it for new hitches. Do not
combine named permissions with legacy `sandbox_mode` settings.

```toml
[permissions.gangline]
description = "Trusted checkout development and a dedicated team state root"

[permissions.gangline.filesystem]
":minimal" = "read"
":tmpdir" = "write"
":slash_tmp" = "write"
"/home/you/.local/state/gangline" = "write"
"/home/you/.local/share/gangline" = "read"
"/home/you/.local/bin/gang" = "read"
"/home/you/.config/gangline" = "read"
"/home/you/team-briefs" = "write"
"/home/you/Repos/project/.git" = "read"
"/run/user/1000/gangline" = "write"

[permissions.gangline.filesystem.":workspace_roots"]
"." = "write"

[permissions.gangline.network]
enabled = true
```

This limits permanent writes to the checkout, selected Git storage, team state and
briefs. It allows ordinary temporary files and direct network access. It is
not a network destination allowlist: profile domain rules require Codex's
network proxy. If using that proxy, allow only the team's exact tmux socket
through its Unix socket allowlist; do not enable all Unix sockets. Prepare the
socket directory before starting the team. Use a dedicated server and the
[isolated-team procedure](reference.md#configuration) for tests.
The tmux socket and shared team state confer control of that team; they are
not an isolation boundary between mutually untrusted teammates.

Keep secrets outside readable workspace roots. Unlike `:root = "read"`, the
minimal baseline does not deliberately open the entire home directory. Add
only the toolchain paths that commands actually need to read and individual
cache directories that they need to write. Do not grant SSH keys, cloud
credentials, the entire harness profile, the home directory or all repositories.
This example needs validation against your installed toolchains and any
administrator-managed policy; a syntactically valid profile is not proof that
a command can run. Restart affected agents after changing grants.

A sandboxed lead can hitch and drop through the team's tmux socket and use
operator-prepared worktrees. Letting it create linked worktrees also requires
writes to shared storage, the common Git `worktrees` registration directory
and the chosen source-worktree parent. Registration access lets it rewrite
all sibling private metadata, including pointers that can redirect later host
Git commands. That broader authority is an explicit operator decision, not
part of the profile above. Otherwise the operator creates/removes worktrees
and performs maintenance needing common root files. Primary checkouts get no
automatic metadata grant, so canonical merges need separately reviewed policy
or a host handoff. Do not grant the whole repositories parent. A push from an
arc branch can avoid canonical source writes, but still needs independently
working authentication. Leave pushes and releases with the host operator when
credentials are unavailable; do not introduce an automatic host command proxy
or forward a signing agent to work around the sandbox.

A "host-shell" role is an ordinary sandboxed worker with a specific task
checkout, not an escape hatch. It can apply reviewed patches, run local test
runners and commit scripts inside its grants. Hardcoded external test scratch
needs an exact directory grant or a supported runner override. A deployment tree
containing code used by host services is not ordinary scratch: keep patch
application there with the host operator unless that execution authority is
explicitly intended. Reading a selected transcript does not require write
access to a real harness profile; prefer a sanitized export for investigation.
Native reproduction requiring that profile to write remains a host task.

`[process-unavailable]` means the caller cannot verify host process ancestry.
The registered-pane fallback still supports team operations, but detached
children may survive drop; a filesystem grant cannot restore PID namespace
visibility. Do not remove the marker or weaken identity checks to make a team
look healthy. Full host process cleanup, native trust/login prompts and any
operation refused by the effective policy remain operator responsibilities.
