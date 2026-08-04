# Gangline

**Many CLI coding agents. One coordinated team.**

Gangline is the substrate under a long-running, multi-harness agent team — not the
orchestration on top of it. Claude Code, Codex, opencode and Pi run as named
windows in one tmux session. `gang` hitches them, *proves* each attributed message
into the target's input box, reports what can and cannot be established about
their state, and carries them across their own context limits. Who does what, and
in what order, is your call — it lives in a markdown brief you can replace with a
directory of your own.

One Bash script over tmux. No daemon, no port, no message bus, no harness plugin,
and `gang` exits after every command. Attach whenever you like and you are inside
the real TUI — the agent's own, at full fidelity, with every keystroke still
yours.

[![A Claude Code lead hitching a Codex worker and an opencode reviewer as tmux windows, sending each an attributed message, and reporting the roster as the work comes back](site/demo.gif)](https://gangline.ai)

<p align="center"><em>One message to a Claude Code lead: it hitches a Codex worker, waits for
the work, then hitches an opencode reviewer to check it. Real harnesses, real
subscriptions, unedited except for cuts through the waiting —
<a href="https://gangline.ai">watch the whole thing at gangline.ai</a>.</em></p>

## What goes wrong without it

Three agents at once is three terminal tabs, and the moment you look away you are
guessing. Is that one working, or waiting on a permission prompt? Did the message
you pasted actually go in, or is it still sitting unsent in the composer with your
next one about to land on top of it? Which one is three thousand tokens from
losing the thread? You become the message bus — copying output from one tab into
another, watching meters, remembering to renew, and discovering an hour later
that the answer you sent never arrived.

Gangline is the substrate that does that job instead: one session, named windows,
messages verified into the box rather than sprayed at it, states that admit what
they could not determine, and context warnings delivered to the agent rather than
to you.

## Start a team

Run this from the repository the team will share:

```sh
cd ~/my-project
gang up                                      # hitch a briefed Claude Code lead and attach
# Detach with Ctrl-b d; reconnect later with: gang attach

gang hitch worker -p codex -r worker         # add a Codex teammate
gang send worker --from operator --stdin <<'MESSAGE'
inspect the failing tests and propose a fix
MESSAGE

gang roster                                  # state and context, with uncertainty visible
# After the worker reports a finished arc:
gang drop worker                             # release one finished teammate
```

The sender label is required. Inside the team it must match the calling window's
name; from an outside operator shell you choose it. The quoted heredoc keeps shell
syntax in message prose literal. `gang down` ends the entire team, so `gang drop`
is the routine teardown for one finished agent. If you use `gang wait`, read the
state it prints: `parked` and `expired` end the wait without establishing idle.

`gang up` is also useful alone. With nobody else hitched, Gangline still measures
context, warns as the window fills, and gives the agent a defined renewal path.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

The installer clones Gangline to `~/.local/share/gangline`, links `gang` into
`~/.local/bin`, and fast-forwards the clone when run again. Set `GANGLINE_HOME`
or `GANGLINE_BIN` to choose other locations. The npm and PyPI registry packages
are not published; `install.sh` is the supported installation route.

Prefer not to pipe into a shell? Read [`install.sh`](install.sh), then clone and
link it yourself:

```sh
git clone https://github.com/adambiggs/gangline.git ~/.local/share/gangline
ln -s ~/.local/share/gangline/bin/gang ~/.local/bin/gang
```

Requirements: Bash, git, tmux 2.6 or newer, the `python3` command, and a supported
CLI harness. [`install.sh`](install.sh) enforces that compatibility floor;
[issue #31](https://github.com/adambiggs/gangline/issues/31) carries the tmux
call-set and paste-behaviour evidence needed to move it.

## What Gangline owns

Gangline owns the common substrate: launching named harness windows, attributed
and verified delivery, conservative state observation, context measurement and
renewal. The workflow above it — task assignment, ordering, retries, and what to
do when work stalls — belongs to the operator and the role briefs. That boundary
is enforced by the [Constitution](CONSTITUTION.md), not added as a positioning
layer after the fact.

This makes Gangline most useful when CLI agents must remain inspectable across
long work: attach and you are inside each harness's native TUI, while `gang`
itself has no server or supervisor left running. Shipped lead, worker, and
reviewer briefs are replaceable examples, not the product.

## When something else fits better

- If every teammate is Claude Code and you want a shared task list, claiming,
  dependencies, and plan approval, start with [Claude Code agent
  teams](https://code.claude.com/docs/en/agent-teams).
- If agents need separate git worktrees, use a tool that owns that isolation,
  such as [claude-squad](https://github.com/smtg-ai/claude-squad). Gangline
  creates no worktree boundary for you.
- If you need a browser dashboard, notifications, durable history, or an
  automatic watchdog, use a control plane built to keep those services running.
- If you need deterministic control flow, retries, structured output, and tests
  around model calls, use an Agent SDK or orchestration framework rather than
  CLI harnesses.

Gangline has no worktree isolation, dashboard, notifications, history to browse
after a window dies, or auto-restart. Per-agent state lives in tmux window options
and dies with the window; a failed harness is one you hitch again. Profile
observations can go stale when a harness changes, and `gang vet` reports that risk
rather than repairing it.
Gangline is Bash on tmux, so Windows means WSL. It also has no task list,
dependency graph, retry policy, or supervisor loop: those are deliberately the
layer above.

## Findings rather than boasts

These are findings rather than boasts: mechanisms not found elsewhere during the
project's comparison, not claims of novelty. If one exists elsewhere, the claim
is wrong and an issue is welcome.

- **Delivery is measured, not fired.** A send must change the target composer and
  then be observed leaving its pasted state. Unreadable and unchanged are loud,
  separate failures; a stranded paste remains visible in status until the box is
  empty. The [reference](docs/reference.md) defines the exact contract.
- **“I cannot tell” remains visible.** `occupied`, `parked`, and `expired` preserve
  findings that cannot safely be collapsed into busy or idle. Status observes a
  TUI; it is not a scheduler guarantee. See [Understanding
  state](docs/operations.md#understanding-state).
- **Context renewal is measure → warn → cycle.** Gangline reads context through
  the active profile, the warning ladder asks the agent to close an arc, and
  `gang cycle` replaces the context instead of summarising it. A cycle carrying
  state accepts only a reviewed continuation package tied to the live team task
  ledger; follow the [operator workflow](docs/operations.md#renewing-context)
  and [`gang cycle` contract](docs/reference.md)
  before running it. Native `gang compact` remains a separate harness operation.
- **Terminal observations are treated as version-fragile.** Profiles declare the
  harness-specific launch and observable surface. `gang vet --probe` can drive a
  real harness on a private tmux socket and reports what it could not probe rather
  than turning missing evidence into a pass. See [Diagnosing profile
  rot](docs/operations.md#diagnosing-profile-rot).

Agents are tmux windows: the window name is identity and command handle, and its
Gangline state disappears when the window does. Profiles and role briefs are
replaceable files through `GANG_PROFILES` and `GANG_ROLES`; exact declarations
and commands belong in the [reference](docs/reference.md).

## Before you leave a team unattended

**Permissions.** Gangline launches each harness with its configured posture. It
does not approve prompts; a harness-owned UI makes the agent `occupied`, and
sends are refused until the operator clears it. Configure unattended permissions
in the harness first. [Operating a team](docs/operations.md#permission-prompts)
has the harness-specific settings and their qualifications.

**Context reading.** A Claude Code window `gang hitch` launches carries
Gangline's statusline beacon on its own launch line — no setup, and `gang
context`, the roster context column, and context patrol read it from the first
turn. Inside that session the beacon is what paints, even if you have wired
your own statusline; your own settings file is untouched and your statusline
runs everywhere else and afterwards. The manual merge below is needed for one
case only — a window `gang adopt` attaches to, which gang did not launch and so
could not carry the beacon in. Merge it into `~/.claude/settings.json` (adjust
the path if `GANGLINE_HOME` differs):

```json
{
  "statusLine": {
    "type": "command",
    "command": "/home/YOU/.local/share/gangline/statusline/claude-code-context.sh"
  }
}
```

`gang vet` names an adopted window running without it, and names the cost: that
agent's context tier is dark.

**Isolation.** Gangline does not isolate agents from one another. They share the
account, checkout, files, network, and tmux server allowed by their harness
configuration. Treat one agent's output as untrusted input to the next. If you
need a security boundary, use a container, VM, or separate account; a different
`$HOME` under the same uid is not one.

## Read next

A gangline is the line down the middle of a dog team. Gangline's mushing terms
map to real roles, states, and commands but never define rules; the [Musher's
Field Guide](docs/field-guide.md) translates them.

- [Operating a team](docs/operations.md) — unattended setup, continuation,
  recovery, and failure paths
- [Command and environment reference](docs/reference.md) — exact syntax,
  output, profile contract, and settings
- [Contributing](CONTRIBUTING.md) — change, measurement, and release practice
- [Architecture decision register](docs/adr/) — the complete record of why
  choices were accepted or refused

The decisions most likely to explain user-visible behaviour are:

- [tmux is the substrate](docs/adr/0001-tmux-is-the-substrate.md)
- [occupancy is not authority](docs/adr/0004-occupancy-is-not-authority.md)
- [server death is a relaunch, not a restore](docs/adr/0007-server-death-is-a-relaunch-not-a-restore.md)
- [a context is renewed by cycling, not by summary](docs/adr/0015-a-context-is-renewed-by-cycling-not-by-summary.md)
- [continuation state is a closed, reviewed set](docs/adr/0018-continuation-state-is-a-closed-reviewed-set.md)

Apache-2.0 — see [`LICENSE`](LICENSE). Copyright 2026 Adam Biggs.
