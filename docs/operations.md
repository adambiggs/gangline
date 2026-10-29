# Operations

Start with [your first team](quickstart.md). Use the [guides](guides.md) for
daily tasks and [troubleshooting](troubleshooting.md) for observable failures.

## Native session titles

Gangline titles Claude Code and Codex sessions at the first idle tick with a
free composer after startup. Titles identify the agent, team and the task's
first line so native session lists distinguish them. Drafts, native prompts,
open turns and compaction defer the attempt.

Automatic title outcomes appear in `gang log` with an `automatic title:`
reason. A failed or unconfirmed attempt leaves the agent running and is not
retried automatically. Check the native session list for persistence; use
`gang title NAME 'TEXT'` at idle to retry or choose another title.

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
sandboxes, and the network reaches only the hosts the kit allows. Each
release publishes the `gangline-team` workload kit as the image
`docker.io/adambiggs/gangline`, carrying that release's `gang` with tmux and
an sshd; Docker's Claude and Codex mixins layer the agents onto it, and each
mixin's credential stays on the host, where the sandbox proxy adds it to
requests. The kit's source is `packaging/sbx`.

The kit's allows are the only egress when the global policy denies the rest,
and `sbx` forwards the host's ssh agent into sandboxes by default. Before
the first sandbox, set both; the daemon reads the setting when it starts:

```sh
sbx policy init deny-all
sbx settings set ssh.agentForwardingEnabled false
sbx daemon stop
```

Create the sandbox once from an interactive terminal, so `sbx` can ask you to
approve the kit's GitHub credential binding, then start the team inside it:

```sh
sbx run docker.io/adambiggs/gangline:latest --name team -p 127.0.0.1:2722:22 \
  --kit docker.io/sbx/claude-mixin:latest \
  --kit docker.io/sbx/codex-mixin:latest \
  WORKSPACE
gang up
```

The workload kit is the agent argument, and the mixins come as `--kit`.

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
adds the token, and it rewrites ssh remotes to https. It denies GitHub's ssh
port, which the Codex mixin's GitHub allows open, so no git traffic goes
around the proxy's token. The kit also allows the Go, npm and PyPI registries
and GitHub's download hosts, so builds fetch their dependencies.

Peer delivery runs over the VM's sshd. On the host, add an ssh alias for the
published port, enroll the VM in `GANG_PEERS`, and authorize the host's key in
the VM with `gang peer-authorize`. In the VM, set `GANG_HOST` and `GANG_PEERS`
in `~/.config/gangline/config`, reach the host as `host.docker.internal`, and
authorize the VM's key on the host the same way. `sshd` starts each forced
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
back the VM, its sshd and its ports, not the team: start the team inside with
`gang up`, as on a host after a reboot, and resume an agent's conversation with
`--resume SESSION`.

`sbx exec` starts its command from the image environment, so a host terminal
joins a pane with its own terminal type passed along:

```sh
sbx exec -it -e "TERM=$TERM" team -- gang attach
```

`gang down` removes the team directory with its registered senders, so a
service that sends with `--as NAME` registers NAME again after each `gang up`.
