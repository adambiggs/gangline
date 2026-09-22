# Operate a team

## Start, leave, and return

Run the supported harness directly once before Gangline so authentication and
repository trust are already explicit. Then start a team in its working tree:

```sh
gang up -c claude-code -m sonnet -e high
```

`gang up` hitches `lead` in the caller's current directory, gives it the
`lead` role brief by default, and attaches when run from a terminal. Detach
with `Ctrl-b d` and return with:

```sh
gang attach
```

If launch stops at a native prompt, Gangline exits with status 4 and names the
pane. Attach, answer the harness-owned prompt, detach, and run `gang tick` to
finish readiness and deliver the startup envelope.

Use an isolated team name and state/socket roots when running more than one
team:

```sh
GANG_SESSION=review GANG_STATE_ROOT="$HOME/.local/state/gangline-review" \
  GANG_TMUX_SOCKET="$HOME/.local/state/gangline-review/tmux.sock" \
  gang up
```

Those values must be present on every command that addresses that team.

`up` and `hitch` wait for a stable native composer before registering the
agent active. If the harness instead shows an operator trust prompt, the hitch
stays booting and the command exits with status 4. Resolve the prompt and run
`gang tick`; a readable composer completes registration even after the boot
deadline, and a still-visible operator prompt remains pending rather than
becoming a terminal failure.

## Add and message agents

Inspect native model names first, then hitch:

```sh
gang models -c codex
gang hitch worker -c codex -d "$PWD" -m MODEL -e EFFORT \
  -r worker -t 'Trace the failure and report the smallest reproduction.'
```

If a native dialog appears after the composer first looked ready, the hitch
keeps the startup assignment queued and retries it after the dialog clears.
The startup delivery has a human-scale deadline; `gang roster` reports the
directly observed idle or blocked pane state while the hitch command waits.

From a registered agent pane:

```sh
printf '%s\n' 'Run the focused test and report the result.' | gang send worker
```

From an operator shell, declare the outside identity:

```sh
printf '%s\n' 'Run the focused test and report the result.' |
  gang send worker --from operator
```

A successful command prints a delivery ID and either `delivered` or `queued`.
Queued work remains in the event log until a safe native boundary releases it.
Status 5 means input may have landed but the native hook did not prove the
attributed envelope; inspect the recipient before retrying.

## Observe and recover

Start with recorded state and the parsed terminal:

```sh
gang roster
gang status worker --why
gang capture worker 40
gang capture --composer worker
gang log
```

`gang tick` is one bounded recovery pass. It retries effects left pending by an
interruption, releases one safe queued delivery per recipient, and records a
wedge only when two qualifying observations support it.

The tmux window name is an immediate projection of that recorded state:
`?name?` needs startup or teardown attention, `~name~` is idle, `-name-` is
working, and `!name!` is blocked, wedged, or failed. The event log remains
authoritative; `gang tick` reconciles a stale window name.

Use `gang interrupt worker -m 'Stop and report current evidence.'` only while
the agent is recorded busy or wedged. Use `gang compact worker --resume 'Read
the saved state and continue.'` for native compaction. If the compaction surface
is stuck, `gang compact worker --recover` applies only the recovery actions
declared by that collar.

`gang collar check NAME` launches disposable private tmux sessions and reports
each native compatibility probe. It observes trust prompts but never answers
them.

## State and replay

Every team has an append-only event log under the v1 state root:

```text
${XDG_STATE_HOME:-~/.local/state}/gangline/v1/TEAM/events.jsonl
```

Replay a copied log without tmux or a native harness:

```sh
gang replay path/to/events.jsonl
```

Snapshots are disposable acceleration; the log is authoritative. Preserve the
team directory when diagnosing a failure.

## Stop a team

Inspect the roster before removing live windows:

```sh
gang roster
gang drop worker
gang down gangline
```

`gang drop` removes one active or failed hitch. A failed hitch keeps its name
reserved until it is dropped. `gang down` requires the exact configured
session name, drops all active and failed hitches, and then removes the team's
v1 state directory, including its event log. Copy evidence first if it must
survive.

Never use an unaimed `tmux kill-server` or `tmux kill-session`; Gangline teams
may share a tmux server with unrelated work.
