# Gangline

Gangline is a local substrate for long-running teams of native CLI coding
agents. A team is a tmux session; each agent is a named window running Claude
Code, Codex, OpenCode, or Pi with its normal terminal, tools, permissions,
subscription, and context.

[![Two native Codex sessions using Gangline to deliver attributed work, verify the result, and return idle](site/demo.gif)](https://gangline.ai/#demo)

<p align="center"><em>One goal, two native Codex sessions. The worker's
attributed report starts the lead's next turn; delivery, pane output, and final
state are real.</em></p>

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

Gangline supplies only the shared primitives:

- start, attach, observe, and stop native harnesses;
- send attributed messages through their terminal and verify delivery;
- expose conservative -busy-, ~idle~, !occupied!, and ?unknown? state;
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
available harness collars.

## Start a team

From the repository the agents should work in:

```sh
gang up
```

This hitches `lead` with `GANG_COLLAR` (Claude Code by default), attaches the
shipped `lead` role brief, and joins its window. Pass `-r` to select another
role. Detach from tmux with `Ctrl-b d`.

Add another native harness and send it work:

```sh
gang hitch worker -c codex -d "$PWD"

gang send --to worker --stdin <<'TASK'
Inspect the failing parser tests, fix the root cause, and report the proof.
TASK
```

Every delivered message names its sender in a nonce-bound envelope. Inside the
team, Gangline reads the sender from the calling window; outside callers name
themselves with `--from`. Delivery succeeds only after the target composer
visibly accepts the paste and submission.

A target that cannot take input right now gets the message parked by default:
it waits in the target's spool, and the target's own native delivery
opportunities carry it through the same verified path. A collar may declare
that a free composer accepts native mid-turn steering; attribution still lands
in the spool before the first keystroke. Drafts and tmux copy-mode remain
parked. `--live-only` refuses instead of parking when a message is only worth
sending now.

Observe and control the team without replacing the harness interface:

```sh
gang roster
gang status worker
gang explain worker
gang wait worker --until idle
gang capture worker
gang attach
gang interrupt worker
gang flush worker
gang drop worker
gang down
```

`gang interrupt` sends the keystroke the harness's collar declares for stopping
a turn. `gang flush` recovers a message a harness parked in its own input queue,
reading the reloaded composer back against what Gangline recorded before
submitting it.

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

Context lights are optional and off by default. Percentage thresholds let one
team setting serve harnesses with different context windows; absolute token
thresholds remain available for a single observed window. Keep both edges high,
but below the harness's observed automatic-compaction boundary:

```sh
GANG_CONTEXT_LIGHTS="50%,80%" gang hitch worker -c codex
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
GANG_USAGE_LIGHTS="90%,95%" gang hitch worker -c codex
gang limits worker
gang wait-limit worker
```

`gang limits` reports native usage windows, reset times, and sample age. A
provider-reset wait is one transient systemd user timer; it delivers an
attributed continuation through the ordinary Gangline path and then disappears.

`GANG_AUTO_RESUME="97%"` arms that wait automatically, once per provider window,
from the agent's own turn — so a team keeps working across provider windows with
nobody at the keyboard:

```sh
GANG_AUTO_RESUME="97%" gang hitch worker -c claude-code
```

## Safety model

Gangline is single-tenant and provides attribution, not authentication. It
refuses ambiguous tmux targets, occupied native dialogs, non-empty composers,
and state it cannot determine. It never answers permission prompts or weakens a
harness sandbox.

State lives in tmux options and dies with its window or team. There is no daemon,
database, cloud service, or private agent protocol.

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
