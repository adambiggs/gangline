# Operating a team

This guide covers the setup and failure paths that matter once `gang up` works.
For syntax, see the [command reference](reference.md).

## Permission prompts

Gangline launches each harness with the profile's `GANG_LAUNCH`. It does not add
a general permission bypass and never answers a modal. A prompt removes or takes
over the composer, so Gangline reports the agent as `gated (hook set)` and refuses
all delivery until the operator clears it with `gang attach`.

Choose the unattended permission posture in the harness's persistent
configuration, where it remains visible and applies consistently:

- **Claude Code:** set the desired `permissions.defaultMode` in
  `~/.claude/settings.json`. The shipped profile adds no permission flag.
- **Codex:** set `approval_policy` in `~/.codex/config.toml`. The shipped profile
  disables only the startup update prompt for its process; it does not change
  approvals or sandbox mode.
- **opencode:** vanilla opencode allows tools unless your `opencode.json` has a
  permission rule set to `ask`. The shipped profile does not add `--auto`.
- **Pi:** core Pi has no tool-approval system. An extension can add one; if it
  changes modal chrome, shadow `pi.sh` and pin the extension's version as part of
  `GANG_VERSION_CMD`.

There is one narrow profile-owned grant. A role brief lives outside the project
when Gangline is installed elsewhere, and opencode otherwise asks before reading
that external directory. The opencode profile merges allow rules for the active
`GANG_ROLES` and shipped `roles/` directories into `OPENCODE_CONFIG_CONTENT` for
the launched process. It does not replace your other opencode configuration.

A role hitch checks for a gate shortly after delivering the brief and exits
nonzero if one appears. This is a race that catches an early gate, not proof that
no later prompt can occur.

## Codex must be able to reach tmux

Gangline's control path is the tmux Unix socket. Under Codex's
`workspace-write` sandbox, network denial also denies `connect()` to Unix
sockets. Incoming messages can still arrive because an outside Gangline process
writes into the pane, but the Codex agent cannot run `gang send`, `roster`,
`status`, or self-compaction back through that socket.

Keep the filesystem sandbox while allowing the socket by choosing this in
`~/.codex/config.toml`:

```toml
sandbox_mode = "workspace-write"

[sandbox_workspace_write]
network_access = true
```

This enables ordinary network access too; Codex provides no Unix-socket-only
allowlist. Decide whether that trade is acceptable for the work.

Delivery also needs one lock directory shared by every Gangline process that can
write to a pane. Its default is
`${XDG_RUNTIME_DIR:-/tmp}/gangline-<uid>`. A systemd environment normally sets
`XDG_RUNTIME_DIR=/run/user/<uid>`, which Codex may mount read-only. In that case,
choose a path all outside and sandboxed callers can write **before** hitching the
team, and preserve it in every shell and cron job that runs `gang`:

```sh
export GANG_LOCK_DIR="/tmp/gangline-$(id -u)"
```

Do not give different writers different lock directories: they would stop
serialising against one another. Gangline fails loudly rather than silently
falling back when the configured lock path cannot be created.

## Context readouts and warnings

Every context consumer goes through the active profile's `profile_context`:
`gang context`, the roster column, patrol, compaction proof, and the optional
in-turn context hook.

The shipped profiles obtain the value differently:

- **Claude Code:** `statusline/claude-code-context.sh` formats the harness's own
  statusline payload into a `ctx <used>k/<window>k <percent>%` pane beacon. Wire
  it into `settings.json` as shown in the README.
- **Codex:** a hitch-time random marker links the window to exactly one rollout
  JSONL file. The profile reads its last `token_count` event. An adopted Codex
  window has no marker and cannot provide context.
- **opencode:** the pane provides used tokens, rounded percent, and model badge;
  the profile joins the window size from opencode's models cache and requires the
  percent to agree with the join.
- **Pi:** the profile reads Pi's native `<percent>%/<window>k` status-bar value.

A missing readout is not hidden: the dedicated command fails, roster displays
`-`, and patrol reports the agent as not patrolled.

### Ambient patrol

`gang patrol` is a one-shot sweep, suitable for cron. Create the log directory
first because the shell opens a redirect before it starts `gang`:

```sh
mkdir -p "$HOME/.local/state/gangline"
```

```cron
*/2 * * * * $HOME/.local/bin/gang patrol 2>&1 | grep -v ' steady ' >> $HOME/.local/state/gangline/patrol.log || true
```

The negative filter keeps unknown future errors while removing routine steady
rows. A patrol only sweeps `GANG_SESSION`; use one cron entry per session. Carry
`GANG_PROFILES`, `GANG_CONTEXT_BANDS`, and `GANG_LOCK_DIR` into cron too when the
team overrides them.

Bare `GANG_CONTEXT_BANDS` entries are absolute tokens; `%` entries are a
percentage of that agent's context window. The default ladder is entirely
absolute. The last warned band is a tmux window option, shared with
`context-hook`, and re-arms when usage falls after compaction.

### In-turn hook

`gang context-hook` consumes a harness hook event on stdin and can return an
`additionalContext` warning during the agent's own turn. It is designed for
Claude Code's `UserPromptSubmit` and `PostToolUse` events. Configure those events
to run:

```text
gang context-hook
```

The hook is deliberately silent on missing or malformed input so context
telemetry cannot block work. Patrol remains the harness-independent warning leg.

## Compaction and handoff

The robust self-compaction form is:

```sh
gang compact <self> --from <self> --resume "checkpoint and next action"
```

The calling agent can be busy: its compaction command queues behind the current
turn. A different busy agent is refused because forced compaction would discard
its live work.

The resume is not typed immediately after the slash command. Harness queues can
hand ordinary text to the turn still running while keeping a slash command for
the boundary, causing the resume to overtake and be consumed before compaction.
Gangline instead starts a detached waiter.

The waiter uses the first available safe signal:

1. a declared live compaction marker (none of the shipped profiles currently
   declares one);
2. for a gang-issued compaction with a readable baseline, context falling below
   half that baseline;
3. after the compaction grace or without context introspection, a bounded series
   of non-busy and stable-screen observations;
4. at the overall timeout, an attempted delivery through the ordinary verified
   injection path.

The context-drop threshold deliberately favours a late resume: a compaction that
reclaims less waits for the fallback instead of declaring success early. If the
detached delivery fails, the error lives in `@gl_resume_failed` and is reported
by status and patrol.

## Understanding state

Gangline observes a terminal rather than receiving a harness lifecycle API.
State therefore combines several independent signals:

- a profile's busy regex against the pane;
- tmux's window-activity timestamp, only for profiles verified quiet at rest;
- pane content changing across two captures;
- a profile's modal regex, confirmed by the composer not being live;
- the conservative fallback that a missing expected composer is gated when no
  painted busy marker explains it.

Recent pty activity covers full-screen redraws that move the cursor without
changing pane cells. Pane churn remains as a fallback and also catches working
screens with no marker. A marker is the fast path, not the only busy signal.

`pane_stable` proves only that captured cells did not change during the sample.
A silent tool call or compaction can hold still while work continues. Patrol
therefore also checks Gangline-owned compaction state and the input box before
injecting. The system prefers a delayed nudge or send over keystrokes aimed at an
unknown widget.

An agent blocked in its own `gang wait` is special. Gangline stores the waiter's
PID in `@gl_waiting`; if that agent's profile accepts mid-turn input, status can
show it available and a teammate can send to it. A dead waiter PID is reclaimed
when observed.

## Undelivered pastes

Delivery reads the composer before paste, after paste, and repeatedly after its
separate Enter. Failures after paste can leave text in the composer or make its
location unknowable. Blindly sending a clear key is unsafe because a modal may
now own that key.

Gangline records the incident in window options. It clears only when all of these
hold:

- the modal is gone;
- the composer is live and not changing;
- its exact rendering matches what Gangline recorded;
- repeated `Ctrl-u` presses visibly shrink it to empty.

Otherwise status prints a red `undelivered paste in the input box` detail,
roster adds `undelivered paste`, and patrol reports it. The record disappears
when the composer reads empty, even if a person cleared it.

## Diagnosing profile rot

Run plain vet first:

```sh
gang vet
```

It checks installed version words against profile pins, runs profile-owned file
format gates, and verifies that a UTF-8 locale is available. It never drives a
marker. Thus an all-OK result means the versions and declared file schemas still
match known observations; it does not prove today's TUI still paints the same
chrome. Themes, statuslines, and TUI extensions can move chrome without changing
the harness version.

Drive the actual marker when symptoms remain:

```sh
gang vet --probe codex
```

The probe uses its own explicit `tmux -L gangvet-<pid>` socket, launches the
harness exactly as its profile declares, waits for the composer, sends a real
prompt, and checks a transition: marker absent at rest, present during work,
absent after the pane settles. It then calls the profile's context reader. The
probe costs tokens and can take several minutes at its documented worst-case
bounds.

A fresh temporary directory may trigger a harness trust dialog. Gangline never
answers it. Set `GANG_PROBE_DIR` to an **empty directory already trusted by that
harness**; do not use Gangline's own checkout, whose profile source contains the
marker strings being tested.

Interpret results narrowly:

- `MARKER DEAD`, `MARKER STUCK`, or a missing context readout is drift and exits
  nonzero;
- not installed, no marker declaration, a gate, or no observed turn is **not
  probed**, not a pass;
- a zero exit covers only markers actually fired;
- gated and compacting states are not exercised;
- one ordinary turn cannot exercise every alternate branch in a busy regex.

After live re-verification, shadow the profile through `GANG_PROFILES` if you
need a local repair. `gang vet` prints the custom file path when it is the one
loaded. `gang vet --file-issue` can file a deduplicated issue for version-pin rot
when the GitHub CLI is installed and authenticated.

## tmux socket safety

From inside a tmux pane, setting `TMUX_TMPDIR` does not redirect bare `tmux`:
tmux follows `$TMUX`. A diagnostic that owns a server must use an explicit
`-L` or `-S` on every operation, especially `kill-server`.

Gangline's probe does this and places a shim first on `PATH` so profile functions
that call bare `tmux` are directed to the same private socket. Teardown calls
`tmux -L <owned-name> kill-server` and removes only the socket name it minted.

For your own probes, use the same discipline:

```sh
tmux -L my-probe new-session -d -s test bash
tmux -L my-probe kill-server
rm -f "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/my-probe"
```
