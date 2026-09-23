# Operate a team

## Start, leave, and return

Run each harness by itself once first, so its sign-in and repository trust
prompts are out of the way. Then start a team in its working tree:

```sh
gang up -c claude-code -m sonnet -e high
```

This hitches `lead` in the current directory with the `lead` role and
attaches. Detach with `Ctrl-b d` and come back with:

```sh
gang attach
```

If the harness stops at a prompt during startup, gang exits with status 4 and
names the pane. Attach, answer the prompt, detach, and run `gang tick` to
finish startup. A recognized trust prompt stays blocked without a boot deadline;
the queued message retains the contract and assignment.

If input became unverified after paste, resolve the native prompt and run
`gang hitch NAME --recover`. Recovery submits the original envelope only when
its exact text is still in the composer, without pasting again. If it cannot
identify the original text, it refuses and prints the retained message path.
Do not replace a lost startup message with ordinary `gang send`: that omits
the original contract. A later matching native submit hook can also reconcile
an uncertain receipt without resending it.

To run more than one team, give each its own name, state root, and socket, and
set them on every command for that team:

```sh
GANG_SESSION=review GANG_STATE_ROOT="$HOME/.local/state/gangline-review" \
  GANG_TMUX_SOCKET="$HOME/.local/state/gangline-review/tmux.sock" \
  gang up
```

## Add and message agents

List a harness's models, then hitch:

```sh
gang models -c codex
gang hitch worker -c codex -d "$PWD" -m MODEL -e EFFORT \
  -r worker -t 'Trace the failure and report the smallest reproduction.'
```

From an agent's window:

```sh
printf '%s\n' 'Run the focused test and report the result.' | gang send worker
```

From your own shell, say who you are:

```sh
printf '%s\n' 'Run the focused test and report the result.' |
  gang send worker --from operator
```

`gang send` prints a message ID and `delivered`, `accepted`, or `queued`.
An accepted message belongs to the native input queue; do not send it again.
A queued message waits until the agent takes it or is dropped. Status 5 means the text may have
been typed but the harness never confirmed it; look at the agent before
sending again.

## Observe and recover

```sh
gang roster
gang status worker --why
gang capture worker 40
gang capture --composer worker
gang log
```

`gang tick` makes one recovery pass: it retries work an interruption left
pending, sends queued messages that can go now, marks agents that look stuck,
and resumes agents whose turn ended on a provider error.

To stop a turn, `gang interrupt worker -m 'Stop and report current evidence.'`.
To compact, `gang compact worker --resume 'Read the saved state and continue.'`.
A busy agent queues the request until a native idle seam. Submitting the command
is not completion: gang reports an unconfirmed result and withholds the resume
until a newer native completion hook or transcript event confirms compaction.
A native refusal is reported as failure. The follow-up then uses normal verified
delivery. Use `gang status worker --why` to inspect the compaction outcome.
If a compaction gets stuck, `gang compact worker --recover` runs the collar's
recovery steps.

`gang collar check NAME` tests a harness in throwaway tmux sessions.

To look at a team's history without tmux, copy its audit log and run
`gang log path/to/log.jsonl`. Keep the team directory when diagnosing a
failure.

## Stop a team

Check the roster first:

```sh
gang roster
gang drop worker
gang down gangline
```

`gang drop` stops one agent and the processes it started. `gang down` stops
every agent and deletes the team's state, including its history, so copy
anything you need first. Both refuse if the system can't identify the agent's
processes safely.

Never run `tmux kill-server` or `tmux kill-session` without an exact target;
other work may share the tmux server.

A send that reports `accepted` belongs to the native input queue. Do not send it
again. The retained receipt can become `delivered` if an exact native submit
hook arrives later. An incomplete queue preview remains unverified; inspect the
recipient before deciding how to recover.
