# Gangline

Gangline is a local substrate for long-running teams of native CLI coding
agents. A team is a tmux session; each agent is a named window running Claude
Code, Codex, OpenCode, or Pi with its normal terminal, tools, permissions,
subscription, and context.

Gangline supplies only the shared primitives:

- start, attach, observe, and stop native harnesses;
- send attributed messages through their terminal and verify delivery;
- expose conservative busy, idle, occupied, and indeterminate state;
- invoke each harness's native compaction, including Codex self-compaction; and
- optionally give agents yellow and red context or team-time lights.

It does not assign work, manage roles, enforce deadlines, patrol agents, or run a
supervisor. Coordination remains visible in the native harnesses and under the
operator's control.

## Install

Gangline requires Bash, tmux, Python 3, and at least one supported harness.

```sh
curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
```

Or run [`bin/gang`](bin/gang) directly from a clone. `gang profiles` lists the
available harness profiles.

## Start a team

From the repository the agents should work in:

```sh
gang up
```

This hitches `lead` with `GANG_PROFILE` (Claude Code by default) and attaches to
it. Detach from tmux with `Ctrl-b d`.

Add another native harness and send it work:

```sh
gang hitch worker -p codex -d "$PWD"

gang send --to worker --stdin <<'TASK'
Inspect the failing parser tests, fix the root cause, and report the proof.
TASK
```

Every delivered message names its sender in a nonce-bound envelope. Inside the
team, Gangline reads the sender from the calling window; outside callers name
themselves with `--from`. Delivery succeeds only after the target composer
visibly accepts the paste and submission.

Observe and control the team without replacing the harness interface:

```sh
gang roster
gang status worker
gang capture worker
gang wait worker
gang attach
gang drop worker
gang down
```

## Long sessions

Agents receive one short startup contract: their Gangline name, how to send,
and how to request native compaction. Goals, roles, and working agreements stay
as ordinary prose in the native harnesses and messages.

At a natural checkpoint an agent runs:

```sh
gang compact worker
```

Gangline submits the profile's native compaction command. Codex cannot submit
`/compact` while its own turn is active, so a self-request is recorded and the
native Stop hook submits it once at the turn boundary. Failure remains visible
in `gang status` and `gang roster`.

Context lights are optional and off by default. Enable them when hitching with
absolute yellow and red token thresholds:

```sh
GANG_CONTEXT_LIGHTS=120000,200000 gang hitch worker -p codex
```

The native hook advises once when usage crosses yellow and once when it crosses
red. Dropping below yellow starts a new context epoch. Lights are guidance only;
the agent chooses the natural checkpoint.

An operator may also declare one optional cutoff for the whole team:

```sh
gang cutoff 2h
gang cutoff 17:30
```

Yellow appears halfway through the declared span and red after four-fifths.
Hook-enabled agents receive each edge once. `gang cutoff` shows the declaration
and `gang cutoff clear` removes it. Nothing stops automatically.

## Safety model

Gangline is single-tenant and provides attribution, not authentication. It
refuses ambiguous tmux targets, occupied native dialogs, non-empty composers,
and state it cannot determine. It never answers permission prompts or weakens a
harness sandbox.

State lives in tmux options and dies with its window or team. There is no daemon,
database, cloud service, background coordinator, or private agent protocol.

## Documentation

- [`docs/reference.md`](docs/reference.md) — exact commands, environment, and
  profile contract
- [`docs/operations.md`](docs/operations.md) — unattended operation and recovery
- [`CONSTITUTION.md`](CONSTITUTION.md) — binding project laws
- [`docs/DECISIONS.md`](docs/DECISIONS.md) — terse durable decisions
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — repository gates and contribution policy

Gangline is licensed under Apache-2.0.
