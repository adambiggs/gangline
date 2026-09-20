# Operate a team

Use this guide to run a Gangline team, inspect what it is doing, and recover it
when native state needs attention. Start with `gang roster`: it gives you the
smallest useful snapshot without entering an agent's terminal.

```sh
gang roster
```

If a row is not clearly idle or busy, inspect that agent before you act:

```sh
gang status NAME
gang capture NAME
```

## Start, leave, and return

Start the team in the repository it should work on:

```sh
cd ~/src/my-project
gang up -c claude-code -m sonnet -e high
```

The command creates the configured tmux session, hitches the lead, and attaches
your terminal. Use `Ctrl-b d` to detach without stopping anything. From an
outside shell, list or rejoin the team:

```sh
gang roster
gang attach
```

If you run more than one team, `gang teams` lists their names and sockets. Set
`GANG_SESSION` when you want a shell to address a non-default team.

## Add an agent

Choose the harness first. These commands show the available collars, models,
and role briefs:

```sh
gang collars
gang models -c claude-code
gang roles
```

Then hitch a named agent:

```sh
gang hitch worker -c claude-code -d "$PWD" -m sonnet -e high -r worker \
  -t 'Trace the parser failure and report the smallest reproduction.'
```

To use Codex instead, run `gang models -c codex` and pass one listed model and
effort to `gang hitch`.

The working directory, model, effort, and role are launch choices. To change
one, drop the window and hitch a new agent with the intended values. `gang drop`
prints a resume command when the collar can recover the native session.

Use `gang adopt` only for a harness already running in a window of this tmux
team. Adoption does not deliver the startup contract or install native hooks,
so context readings and turn-boundary delivery may be unavailable.

## Send work

Pipe a message to a named agent. From outside the team, identify the sender:

```sh
printf '%s\n' 'Run the focused test and report the exact failure.' |
  gang send worker --from operator
```

From an agent window, omit `--from`. Gangline reads the window's registered
name and refuses a conflicting claim.

A successful send has two possible outcomes:

- `delivered` means Gangline saw the native composer accept the message.
- `queued` means Gangline owns the body and will retry at a safe native turn
  boundary. `gang roster` shows `spooled=N` until it moves.

Read stderr before retrying a failed send. If Gangline reports an unverified
delivery, the body may already be in the target's composer. Inspect the target
with `gang status NAME` and `gang capture NAME`; do not create a second copy
until you know the first one is absent.

Use a timed message when the work should not enter the session yet:

```sh
printf '%s\n' 'Resume the release check.' |
  gang send worker --from operator --at 45m
```

## Read team state

Use the narrowest command that answers your question:

| Question | Command |
| --- | --- |
| What is every agent doing? | `gang roster` |
| Why is one agent in this state? | `gang status NAME --why` |
| What is on its terminal? | `gang capture NAME` |
| What is in its composer? | `gang capture --composer NAME` |
| What durable events were recorded? | `gang log NAME` |
| How full is its context? | `gang context NAME` |
| What provider limits are visible? | `gang limits NAME` |
| How much local token use is attributed? | `gang usage` |

Treat `?unknown?` as missing evidence, not as idle. Treat `!occupied!` as a
native UI that needs a person. The full state vocabulary is in the
[reference](reference.md#states-and-exit-status).

## Answer native prompts

Authentication, repository access, permissions, and trust stay in the native
harness. Gangline does not choose an answer. Inspect and enter the window:

```sh
gang capture NAME
gang attach
```

Answer the prompt in the harness, detach, then run `gang status NAME` again.
Queued work can move once the native composer becomes safe.

## Manage context and provider limits

Read native context use before asking for compaction:

```sh
gang context worker
gang compact worker --resume 'Continue from the saved checkpoint.'
```

When an agent asks to compact itself, Gangline waits for the end of its current
turn. Every compaction gets a continuation turn so the native session does not
stop at an empty composer.

Use these commands for capacity decisions:

```sh
gang limits worker
gang usage --daily
gang cap
```

`gang limits` reads a provider's published usage window through the collar.
`gang usage` attributes local tokens when `ccusage` is installed. `gang cap`
retains account-window samples. These are different measurements; Gangline does
not infer account quota from token totals.

Set a team curfew when agents need a visible deadline:

```sh
gang curfew 2h
gang curfew
```

The curfew produces advisory edges for hook-enabled agents. It does not stop the
team at the deadline.

## Troubleshoot and recover

Start from live evidence:

```sh
gang roster
gang status NAME
gang capture NAME
```

Then follow the state you can prove:

- For `!occupied!`, answer the native dialog in `gang attach`.
- For `?unknown?`, restore or inspect the native condition. Do not assume the
  composer is safe.
- For a queued Gangline message, wait for the next native boundary. Use
  `gang queue NAME` to read another agent's queue without consuming it.
- Use `gang flush NAME` only when the session transcript or current context
  proves that the harness itself parked the recorded body.
- Use `gang interrupt NAME -m 'reason' --from operator` to stop a live turn and
  deliver a reason when the composer returns.
- If the window is unusable, capture anything you need, then run
  `gang drop NAME`. Use the printed resume command when one is available.

`gang queue` is destructive when an agent reads its own queue: printed entries
move to an archive and will not be delivered later. Do not pipe that read
through `head` or `tail`.

## Run a host command for an agent

From an agent window, `gang run` starts a command in a transient host service
and sends the result back when it exits:

```sh
gang run -- make test
gang run --active
```

The command inherits the agent's working directory and environment, but not its
sandbox. Combined output is stored in Gangline's durable state directory. Only
the requesting agent can list or cancel its runs.

## Stop work

Remove one agent with an exact name:

```sh
gang drop worker
```

Before ending the entire team, read the roster and list the exact session name:

```sh
gang roster
gang teams
gang down SESSION
```

`gang down` refuses an omitted session name. Both teardown commands archive
waiting messages before removing their target. When an archive is created, the
command prints its recovery location.
