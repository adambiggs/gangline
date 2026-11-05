# Operations

Start with [your first team](quickstart.md). Use the [guides](guides.md) for
daily tasks and [troubleshooting](troubleshooting.md) for observable failures.

## Native session titles

Use `gang title NAME 'TEXT'` at idle to change a native session's display title.
Include a machine name when your teams share names, for example
`gang title worker 'worker@ser5: cache refresh'`. The title does not change
registration identity or mailbox routing.

Gangline does not rename sessions automatically. Remove `--title` from existing
`gang hitch` or `gang up` commands and invoke `gang title` separately when a
rename is wanted. Drafts, native prompts, open turns and compaction prevent
manual submission. Check the native session list to confirm persistence; a
submitted command alone does not prove that the title was saved.

## Clear recovery

A queued `gang clear --note TEXT` holds incoming mail while the old native turn
finishes. Inspect `gang status --why`, `gang queue`, and `gang capture --composer`
for the stated blocker. Resolve a native prompt or draft through its normal
owner; do not discard accepted or uncertain input to force a clear.

Once native clear input is reserved, Gangline does not automatically repeat it.
`submitted` means the command may have run; `unverified` preserves a reported
submission failure. `continuing` waits for the exact new-context continuation
receipt. These states retain the logical agent, children and inbox. Missing
proof is not evidence that nothing happened. Keep the state directory and both
native transcripts when investigating an uncertain transition.

A late exact continuation witness can settle an unverified receipt. Where the
original startup envelope is demonstrably still unsubmitted in the composer,
`gang hitch NAME --recover` uses the existing guarded startup recovery. It does
not replay an admitted continuation, infer a clear from an empty composer, or
restart the harness. Check its reported receipt and clear status afterward.

Install hook support only through the normal launch lifecycle. Do not replace
an executable with an older release while a clear transition is pending.

## Continue through a provider cap

Gangline reads the collar's native five-hour and weekly usage windows. When one of
its usage bands is crossed, it sends one notice for that collar and reset
window to the active lead. A band measures pace: how far the window's use runs
ahead of the share of the window that has elapsed. A notice still waiting for a
lead, or queued behind one, when its window resets is withdrawn rather than
delivered. The default notice contains only the provider, reported usage,
elapsed share, pace, projected exhaustion, and reset time. Bands do
not change agent work. A collar overlay can supply an optional per-band `note`
with operator guidance.
Native
readings can be absent or stale; `gang limits` queries every configured collar
without an agent, `gang limits NAME` shows what an agent has observed, and
`gang limits -c COLLAR` queries one collar.

When a native turn is rejected by an attributable provider cap, Gangline
automatically records a wake for that agent at the observed native reset.
`gang snooze --at TIME --note TEXT` is available as a manual override.
At the due time, Gangline sends the note to the caller to resume work. If that caller is
gone, the active lead receives an overdue wake naming the caller.
`gang snooze --status` shows each pending wake's state and what it needs.

The schedule lives in the team state root, so it survives Gangline process
restarts, compaction, and host reboot. Gangline does not start a team after a
reboot. If the team is running at wake time, its armed watchdog delivers the
wake. When no scheduler can arm a timer, or arming fails, `gang snooze` warns and `gang roster` shows
`[watchdog-unavailable]`; the wake then waits for the next hook tick or
`gang tick` in the team. If the team is not running, the next team startup
delivers the pending wake and marks it overdue.
Keep a durable work note because a wake can reach a different lead after the
original agent disappears.

## Sandboxed development teams

Use a named Codex permission profile owned by the operator. Keep the sandbox
on and select a trusted checkout with `gang hitch NAME -c codex -d CHECKOUT`.
Gangline leaves Git permissions to Codex. Under its default workspace-write
policy, `.git` and a linked worktree's resolved Git directory are read-only,
even within a writable checkout.

For unattended Git writes, configure Codex's
[native automatic approval review](https://learn.chatgpt.com/docs/agent-approvals-security#automatic-approval-reviews)
with `approval_policy = "on-request"` and `approvals_reviewer = "auto_review"`.
Codex can request an approved execution outside the sandbox for commits,
rebases, fetches, pushes and maintenance. The normal sandbox still protects
Git config and hooks when the common Git directory is outside writable roots
or has an explicit read rule. The approved command, including hooks it runs,
executes with host authority; use trusted checkouts and review hooks before running them.
Automatic review can refuse a request, and does not authorize every command.

Review existing Codex exec-policy rules too: an explicit `allow` rule can run
matching commands outside the sandbox without automatic review. Remove standing
Git allow rules if those writes should receive approval for each request.

`approval_policy = "never"` disables escalation; it does not automatically
approve requests. Do not combine that setting with partial Git storage grants:
a commit can survive while a hook fails to write `packed-refs.lock`, leaving
publication blocked.

The following is a starting profile for a dedicated team, using the
[Codex permission profile syntax](https://learn.chatgpt.com/docs/permissions).
Replace the example home and runtime paths with absolute paths on your host,
and merge tables into the existing config rather than duplicating them. The
repository metadata read rule lets Git read shared configuration and packed refs
without making them writable; add an exact rule per repository you use.
`GANG_CODEX_PERMISSION_PROFILE=gangline` selects it for new hitches. Do not
combine named permissions with legacy `sandbox_mode` settings.

```toml
approval_policy = "on-request"
approvals_reviewer = "auto_review"

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
"/home/you/Repos/project/.git" = "read"
"/run/user/1000/gangline" = "write"

[permissions.gangline.filesystem.":workspace_roots"]
"." = "write"

[permissions.gangline.network]
enabled = true
```

This limits unapproved writes to the checkout and team state. Approved
commands can write outside those roots. It allows ordinary temporary files and
direct network access. It is
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

A sandboxed lead can hitch and drop through the team's tmux socket. Creating
or removing linked worktrees needs the same native approval path when it
writes outside the permitted roots, and pushes need working authentication.

`[process-unavailable]` means the caller cannot observe the registered pane's
live host process.
The registered-pane fallback still supports team operations, but detached
children may survive drop; a filesystem grant cannot restore PID namespace
visibility. Full host process cleanup, native trust/login prompts and any
operation refused by the effective policy remain operator responsibilities.

## A team inside Docker Sandboxes

A [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) microVM can hold a
whole team, with the VM as the boundary: agents run there without their own
sandboxes. Network access follows your Docker Sandboxes policy. Each
release publishes the `gangline-team` workload kit as the image
`docker.io/adambiggs/gangline`, carrying that release's `gang` with tmux and
an sshd; Docker's Claude and Codex mixins layer the agents onto it, and each
mixin's credential stays on the host, where the sandbox proxy adds it to
requests. The kit's source is `packaging/sbx`.

Create the sandbox once from an interactive terminal, so `sbx` can ask you to
approve the kit's GitHub credential binding, then start the team inside it:

```sh
sbx run docker.io/adambiggs/gangline:latest --name team -p 127.0.0.1:2722:22 \
  --skills readonly \
  --kit docker.io/sbx/claude-mixin:latest \
  --kit docker.io/sbx/codex-mixin:latest \
  WORKSPACE
gang up
```

The workload kit is the agent argument, and the mixins come as `--kit`.

Network policy and SSH-agent forwarding are optional host settings. Choose a
global network policy with `sbx policy init allow-all` or
`sbx policy init balanced` or `sbx policy init deny-all`; with deny-all,
the kits' allowed hosts define
outbound access. Choose whether to forward your host's SSH agent with
`sbx settings set ssh.agentForwardingEnabled true` or
`sbx settings set ssh.agentForwardingEnabled false`. The daemon reads the
SSH-agent setting when it starts.
To apply a changed setting to a running daemon, use `sbx daemon stop`, then
`sbx run --name team` to restart the sandbox. Stopping the daemon stops all
its sandboxes.

The Claude and Codex mixins read Docker Sandboxes' shared host skills store.
On the host, install your library for both agents and import it into that
store, with `SKILLS_CHECKOUT` set to your library checkout:

```sh
npx --yes skills add "$SKILLS_CHECKOUT" -g -a claude-code codex --skill '*' -y
sbx skills import
sbx skills ls
```

The import copies the host's installed skills; repeat it after updating the
library. Docker links the store into each mixin's skills directory at sandbox
start, so it survives sandbox rebuilds without an install inside each VM.
Keep skills sharing enabled (`--skills readonly` at creation). To expose newly
imported skills to an existing team, save its conversations, then restart the
sandbox from the host and start the team inside again with `gang up`:

```sh
sbx stop team && sbx run --detach --name team
```

To run the kit from its source, for a change to the kit itself, stage it with
a static `gang` for the VM's architecture and pass the staged directory as the
agent argument in place of the image. `SBX_KIT_BUILDER=sandbox` builds it in `sbx`'s own
builder sandbox; a host build needs an OCI-capable buildx driver, which the
release uses to publish both Linux architectures.

```sh
CGO_ENABLED=0 go build -trimpath -o /tmp/gang \
  -ldflags "-X main.version=$(git describe --tags --match 'gangline-v*' | sed 's/^gangline-v//')" \
  ./cmd/gang
packaging/sbx/stage.sh /tmp/gangline-team "$(go env GOARCH)=/tmp/gang"
SBX_KIT_BUILDER=sandbox sbx run --name team -p 127.0.0.1:2722:22 \
  --skills readonly \
  --kit docker.io/sbx/claude-mixin:latest \
  --kit docker.io/sbx/codex-mixin:latest \
  /tmp/gangline-team WORKSPACE
```

The kit path must come before the workspaces. Append further paths, with
`:ro` for read-only mounts. Edit a mounted tree from one side: the VM can
keep reading the old contents of a file the host rewrote in place. Run `sbx secret set openai --oauth` on the host
before the create, and sign Claude in once with `/login` inside the sandbox:
the proxy captures the token and the VM keeps only sentinels. The image's
Claude managed settings select auto mode, so `gang up` meets no permission
prompt, and Codex runs without its own sandbox. The VM has no systemd user
manager, so `gang` arms its process watchdog there.

GitHub goes over https with `sbx secret set github`. The kit declares the
token proxy-managed, so the VM's `GH_TOKEN` holds a sentinel and the proxy
adds the token, and it rewrites ssh remotes to https.

Peer delivery runs over the VM's sshd. On the host, add an ssh alias for the
published port, enroll the VM in `GANG_PEERS`, and authorize the host's key in
the VM with `gang peer-authorize`. In the VM, set `GANG_HOST` and `GANG_PEERS`
in `~/.config/gangline/config`, reach the host as `host.docker.internal`
with `localhost:22` allowed by your network policy, and authorize the VM's
key on the host the same way. `sshd` starts each forced
command without the shell's environment, so a team outside the default state
root needs its `GANG_SESSION`, `GANG_STATE_ROOT` and `GANG_TMUX_SOCKET` set
in the forced command.

Any `sbx` command starts the sandbox daemon when none is running, inside the
caller's cgroup, and stopping the daemon stops its
sandboxes. Restore the sandbox at login, with the daemon inside a unit of its
own, from the unit in `packaging/sbx/systemd`. It expects `sbx` at
`~/.local/share/sbx/bin/sbx`. Stop the daemon the create started first, or the
unit attaches to it and the daemon stays outside the unit:

```sh
sbx daemon stop
cp packaging/sbx/systemd/gangline-sbx@.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now gangline-sbx@team.service
```

Enable lingering for the account so the unit starts at boot. The unit brings
back the VM, its sshd and its ports, not the team. To preserve a stopped team's
hitch identities and queued mail, recover each retained name with
`gang rehitch NAME --stopped --note TEXT`, starting with the lead. The
[recovery prerequisites](reference.md) require the old execution location to be
stopped and the complete state and native session stores to be available.
Use `gang up` for a new team; its stale-claim supersession creates a new identity.

`sbx exec` starts its command from the image environment, so a host terminal
joins a pane with its own terminal type passed along:

```sh
sbx exec -it -e "TERM=$TERM" team -- gang attach
```

`gang down` removes the team directory with its registered senders, so a
service that sends with `--as NAME` registers NAME again after each `gang up`.

## Retain and inspect an implementation's evidence

Use existing native artifacts or verified checkpoint exports and a private
manifest outside the team state directory and public checkout. Restrict the
directory and file permissions to the owning account. Collect only the
explicitly authorized task's material. A private location does not authorize
copying credentials or unrelated sessions. Mark excluded content `withheld`;
use only an authorized redacted derivative where required. Destination privacy
checks do not establish that raw content is safe to disclose.

Keep a snapshot of the exact native bytes, actual structured test result and
output, raw Git commit object, independent review result/report, and hosted CI
jobs response. Author the small manifest from those sources, recording manual
imports honestly. See [private evidence joins](reference.md#private-evidence-joins)
for fields and verdicts. Existing checkpoint IDs may be attached to artifact
records; there is no replacement checkpoint or resume mechanism.

Verify the through-CI chain before claiming that milestone. Inspect `show` to
find incomplete edges and the supporting byte digests. Keep the manifest and
its referenced artifacts together: these commands do not archive them or
protect them from teardown, cleanup or expiry. They also do not establish an
installed deployment or an offsite backup. Preserve delivery receipts and
native identity separately; missing success evidence never authorizes replay.

## Observed idle notifications

After new worker input, a full Gangline tick can send the unique active lead a
`team-idle` notice after repeated observations of all registered workers as
idle span `GANG_TEAM_IDLE_MIN_DURATION` (default `2m`). It includes nested
workers and all collars. The lead's own foreground turn is excluded. Startup
establishes a baseline without an initial alert; an empty team never alerts.

The notice names the workers and the observation window. It always explains that
quiet gaps can look idle, native child or background work may still run, and
assignments are not declared complete. It does not authorize resuming held
work. This is an observation of the roster, not continuous or simultaneous
native idleness. No last-output time is inferred from screen sampling.

Unknown, stale, failed, blocked or transitional workers inhibit the notice,
as do due or uncertain input and known composer drafts. A mailbox-only worker
has no native activity observation and therefore inhibits an all-worker
notice. Contended or incomplete sweeps defer evaluation. Future scheduled
input is included as context and is not delivered early.

The first qualifying sweep starts the delay; even a very small configured
minimum requires another qualifying sweep. Worker activity evidence, new work,
changed membership or duration, and backward time restart it. Evaluated busy,
unknown or blocked state clears it. Consecutive qualifying observations must
be less than 30 seconds apart, so a watchdog outage cannot count as idle time.
An incomplete sweep cannot extend the window. Fresh probe timestamps and lead
activity alone do not reset it. Activity that leaves no retained observation
can still be missed; a longer window does not prove native work stopped.

Set `GANG_TEAM_IDLE_MIN_DURATION=5m`, for example, in the existing config file
or environment. Refresh the watchdog with an unscoped `gang tick` in the team's
runtime context after changing the value; its environment retains the value
from its last refresh and overrides the file. Existing watchdog units without
the key use the default or current config until refreshed.

A new ordinary message publication, including `gang send --at`, can arm the
next notice. The first publication of an explicit snooze wake also counts;
automatic cap retries and observation notices do not. The same work reading
busy and idle again, delayed native callbacks, and the lead handling its notice
do not rearm it. Autonomous work with no new identified Gangline input may
finish without another notice. Registration changes establish a new baseline;
dropping a worker alone does not announce completion.

The optional `idle.json` file and `idle.lock` in the team runtime directory keep
work revisions and one exact notice intent across process restarts and receipt
pruning. Accepted or unverified notice delivery is retained without automatic
replay. A replacement lead does not receive a redirected copy; new work under
the new registration can arm a new notice. Ordinary team teardown removes this
state. Preserve it when investigating uncertain delivery.

The separate `idle-window.json` caches only the current observation window
under `idle.lock`; a missing or unreadable cache starts a fresh window. Existing
publication records do not change. Binaries with the earlier notifier ignore
the window file and can still send messages, but resume single-sweep notices.
Before rolling back, remove `GANG_TEAM_IDLE_MIN_DURATION` from the config file:
older config readers reject unknown keys. Refresh the older watchdog after the
swap. An environment-only value is ignored by the older binary.

Binaries predating the notifier ignore these files but cannot maintain the new publication
revisions or enforce a queued notice's recipient binding. Before downgrading,
resolve any pending `team-idle` input and retain a copy of the stopped team's
state. Do not downgrade with an uncertain or queued notice. Work performed
while downgraded may not produce a notice after upgrade. Existing agent,
message and usage records retain their previous formats.
