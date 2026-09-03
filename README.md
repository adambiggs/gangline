# Gangline

Gangline runs Claude Code, Codex, OpenCode, and Pi as named windows in one tmux
session, so they can hand each other work by name and you can watch the whole
thing happen.

[![An operator briefs a Claude Code lead with gang talk, watches it and a Codex worker build an animated terminal show through gang attach, then runs their colorful result](site/demo.gif)](https://gangline.ai/#demo)

<p align="center"><em>Compose with <code>gang talk</code>, watch the real
harnesses build with <code>gang attach</code>, then run their result. Nothing
here is re-enacted.</em></p>

A team is a tmux session, and each agent is one window running its harness's own
CLI with the terminal, tools, permissions, subscription, and context an ordinary
session of that harness has. Gangline supplies only the shared primitives:

- start, attach, observe, and stop native harnesses;
- send attributed messages through their terminal and verify delivery;
- expose conservative -busy-, ~wait~, ~idle~, !occupied!, and ?unknown? state;
- invoke each harness's native compaction, including Codex self-compaction;
- optionally give agents yellow and red context, provider-usage, or team-time
  lights; and
- park an agent until a native provider-limit reset with one transient timer.

It does not assign work, manage roles, enforce deadlines, patrol agents, or run a
supervisor. Coordination remains visible in the native harnesses and under the
operator's control.

## Install

Gangline requires Bash, tmux, Python 3, a UTF-8 locale, and at least one
supported harness.

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

The bootstrap script installs the latest stable `gangline-v*` GitHub release,
not the current `main` branch. Later, `gang upgrade --check` checks explicitly
for a newer release and `gang upgrade` installs it. Other commands do not check
the network. A branch checkout stays developer-owned: update it with Git rather
than `gang upgrade`.

Or run [`bin/gang`](bin/gang) directly from a clone. `gang collars` lists the
available harness collars and marks which of them an agent can be resumed onto.

## Start a team

Before starting a team, run each native harness directly to finish its ordinary
onboarding, authentication, and repository-trust prompts. Gangline never answers
those security choices. The first Codex hitch through Gangline can raise an
additional hook-review prompt because Gangline supplies the hook commands only at
launch; running Codex by itself cannot pre-approve them, and a changed hook path
or command can raise the review again. If a hitch reports a first-run prompt, run
`gang attach` from another operator terminal, select that agent's named window,
and answer it there. The original hitch will then finish delivering its queued
contract.

From the repository the agents should work in:

```sh
gang up -c claude-code -m sonnet -e high
```

This hitches `lead` with Claude Code, attaches the shipped `lead` role brief,
and joins its window. Omit `-c` to use `GANG_COLLAR` instead, and pass `-r` to
select another role. Detach from tmux with `Ctrl-b d`.

Claude Code caps consecutive Stop-hook blocks and ends the turn past the cap.
Gangline leaves that cap in force. Its Stop adapter refuses idle once per turn
while a verified peer reply is owed, then releases the turn with the debt still
recorded: `gang status` and `roster` keep showing it, the notify target or the
lead is told, and the next delivery raises it again. Nothing has to be set in
the hitching shell.

Pass a model and effort at every hitch whose collar takes them; Gangline warns
when a supported choice is omitted and lets the collar pick. Not every collar
takes both — one that declares no effort spelling refuses `-e` rather than
guessing — and `gang models` lists what a collar accepts. Claude Code's aliases
are stable enough to name directly, as above. Ask the harness for Codex's rather
than copying an id that changes:

```sh
gang models -c codex
read -r -p "Codex model: " CODEX_MODEL
read -r -p "Codex effort: " CODEX_EFFORT
```

Every Codex example below reuses those two variables. Add the agent, then send
it work from this detached operator shell:

```sh
gang hitch worker -c codex -d "$PWD" -m "$CODEX_MODEL" -e "$CODEX_EFFORT"

gang send --to worker --from operator --stdin <<'TASK'
Inspect the failing parser tests, fix the root cause, and report the proof.
TASK
```

Every delivered message names its sender in a nonce-bound envelope. Inside the
team, Gangline reads the sender from the calling window; outside callers name
themselves with `--from`. Delivery succeeds only after the target composer
visibly accepts the paste and submission.

A target that cannot take input right now gets the message parked by default:
it waits in the target's spool. Native delivery opportunities and the
cooperative tick carried by every later Gangline invocation retry it through
the same verified path. A collar may declare that a free composer accepts
native mid-turn steering; attribution still lands in the spool before the first
keystroke. Drafts and tmux copy-mode remain parked, then drain as soon as their
gates clear and any team window invokes Gangline. `--live-only` refuses instead
of parking when a message is only worth sending now.

Observe and control the team without replacing the harness interface:

```sh
gang roster
gang status worker
gang alerts
gang explain worker
gang wait worker --until idle
gang capture worker
gang attach
gang interrupt worker
gang flush worker
gang drop worker
gang down gangline
```

`gang interrupt` sends the keystroke the harness's collar declares for stopping
a turn. `gang flush` recovers a message a harness parked in its own input queue,
reading the reloaded composer back against what Gangline recorded before
submitting it. Cooperative-tick failures appear as a compact active count in
the existing tmux status line. Unseen alerts are visually distinct; Prefix+A
opens the full list in an on-demand popup when that key was free, without
creating or selecting another tmux window. Reading the list never resolves a
condition—recovery does.

## Gangline and native subagents

Where a harness spawns subagents of its own, Gangline does not replace them; it
operates one level up. A subagent is created and owned by a single harness
session, and that ownership is what makes it a subagent. Gangline works between
sessions instead: every agent in a team is a separate top-level process running
the harness's ordinary CLI in its own tmux window, and one team can put Claude
Code, Codex, OpenCode, and Pi side by side.

That is a difference in architecture, not a list of promises. What each command
does, what it refuses, and under which conditions belong to
[`docs/reference.md`](docs/reference.md), which is the specification.

## Ontology

A **dog** — a model instance — wears a **harness**, its native CLI runtime
(Claude Code, Codex, OpenCode, or Pi). Gangline fits the dog to the
**gangline**, the tmux session, by its **collar**: the per-harness adapter in
`collars/` that carries every piece of harness-specific knowledge. The
**musher** — you — drives. `gang up` creates the **lead**, names its window
`lead` by default, and attaches the shipped lead role unless the musher selects
another. That role is launch prose; Gangline neither records nor manages it
after delivery.

*Harness* names only the runtime, never the instance running it.

## Long sessions

Agents receive one short startup contract: their Gangline name, how to send,
and how to request native compaction. Goals, roles, and working agreements stay
as ordinary prose in the native harnesses and messages.

At a natural checkpoint an agent runs:

```sh
gang compact worker
```

Gangline submits the collar's native compaction command. Codex cannot submit
`/compact` while its own turn is active, so a self-request is recorded and the
native Stop hook submits it once at the turn boundary. Failure remains visible
in `gang status` and `gang roster`.

Context lights default to the collar's own thresholds for the model being
hitched, so a team mixing harnesses gets working lights on every agent with
nothing configured. Override them for one agent, or for the whole team, with
percentages that serve mixed windows or absolute tokens for a single observed
one. Keep both edges high, but below the harness's observed automatic-compaction
boundary:

```sh
gang hitch worker -c codex -m "$CODEX_MODEL" -e "$CODEX_EFFORT"
gang hitch narrow -c codex -m "$CODEX_MODEL" -e "$CODEX_EFFORT" --lights 50%,80%
gang hitch quiet  -c codex -m "$CODEX_MODEL" -e "$CODEX_EFFORT" --lights off
```

The native hook advises once when usage crosses yellow and once when it crosses
red. Dropping below yellow starts a new context epoch. Lights are guidance only;
the agent chooses the natural checkpoint. An absolute red threshold above the
native window reports itself as invalid when that window is first readable.

An operator may also declare one optional curfew for the whole team:

```sh
gang curfew 2h
gang curfew 17:30
```

Yellow appears halfway through the declared span and red after four-fifths.
Hook-enabled agents receive each edge once. `gang curfew` shows the declaration
and `gang curfew clear` removes it. Nothing stops automatically.

Provider-usage lights use non-interactive sources declared by each collar, not
screen scraping. They are optional too:

```sh
GANG_USAGE_LIGHTS="90%,95%" gang hitch worker -c codex \
  -m "$CODEX_MODEL" -e "$CODEX_EFFORT"
gang limits worker
gang wait-limit worker
```

`gang usage` reports what each agent consumed: Gangline's launch record joined
to ccusage's per-session token counts when that tool is installed, and the
same table with its gaps named when it is not.

`gang limits` reports native usage windows, reset times, and sample age. A
provider-reset wait is one transient systemd user timer; it delivers an
attributed continuation through the ordinary Gangline path and then disappears.

`GANG_AUTO_RESUME="97%"` arms that wait automatically, once per provider window,
from the agent's own turn — so a team keeps working across provider windows with
nobody at the keyboard:

```sh
GANG_AUTO_RESUME="97%" gang hitch worker -c claude-code -m sonnet -e high
```

On claude-code the same opt-in also resumes one turn whose provider stream ends
with a native API-error record. If that continuation fails too, Gangline stops
after that one hop and leaves the refusal in `status` rather than retrying.

## Safety model

Gangline is single-tenant and provides attribution, not authentication. It
refuses ambiguous tmux targets, occupied native dialogs, non-empty composers,
and state it cannot determine. It never answers permission prompts or weakens a
harness sandbox.

State lives in tmux options and dies with its window or team. There is no
resident daemon, database, cloud service, or private agent protocol. Each
Gangline invocation may leave one detached tick process behind only for the
bounded pass it was born to finish; a singleton lock, dirty rerun edge, and hard
deadline keep it ephemeral.

## Documentation

- [`docs/reference.md`](docs/reference.md) — exact commands, environment, and
  collar contract
- [`docs/operations.md`](docs/operations.md) — unattended operation and recovery
- [`docs/benchmarks.md`](docs/benchmarks.md) — benchmark selection and validity
  gates
- [`CONSTITUTION.md`](CONSTITUTION.md) — binding project laws
- [`docs/DECISIONS.md`](docs/DECISIONS.md) — terse durable decisions
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — repository gates and contribution policy

Gangline is licensed under Apache-2.0.
