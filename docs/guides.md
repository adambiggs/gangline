# Guides

Commands here assume the team is running. See the [quickstart](quickstart.md)
to create one.

## Coordinate agents

Launch a worker with an assignment, then add direction from your shell:

```sh
gang hitch worker -c codex -d "$PWD" -r worker \
  -t 'Inspect the tests and recommend a focused check.'
gang send worker --from operator 'Run the focused check and report the result to lead.'
gang queue worker
gang capture worker
```

Inside an agent window, omit `--from`: Gangline reads the sender from the
registered pane. Outside one, supply `--from`; the envelope marks that name
as self-declared. `gang send` prints a message ID and `delivered`, `accepted`,
or `queued`. An `accepted` message belongs to the harness's input queue: do
not send it again. `gang queue` lists messages still pending in Gangline and why each waits.
The receipt tells you where the input is; the pane shows what the worker does.

To stop a current turn and follow it with direction:

```sh
gang interrupt worker -m 'Stop and report the evidence collected so far.'
gang status worker --why
```

The reason is delivered after the native turn stops. Check `status` and the
worker's reply to see whether it stopped and reported back.

## Inspect activity

Find which agent needs attention and what its pane shows:

```sh
gang roster
gang status worker --why
gang capture worker -n 40
gang capture --composer worker
gang context worker
gang log --agent worker
```

`roster` and `status` report observed state; an observation failure is
`unknown`, not proof of an idle or stuck turn. `capture` shows the native
screen, and `--composer` isolates draft input. `context` reports native usage
when available. `log` shows the audit trail. Compare the screen with the
reported activity to decide whether to leave the turn running or intervene.

## Compact and resume

Ask an agent to save working state in a file, then compact at a native idle
boundary and continue from that file:

```sh
gang send worker --from operator 'Save your current findings in notes.md, then tell me when ready.'
```

Wait for the worker's reply and check that the file exists. Then request
compaction:

```sh
gang compact worker --resume 'Read notes.md and continue the assigned work.'
gang status worker --why
```

A busy agent queues the compaction until it reaches a native idle boundary.
The resume note enters the harness's own input queue when compaction starts,
so the harness runs it once compaction ends.
`gang status worker --why` shows whether compaction is queued, completed,
refused, or unconfirmed. If the pane still shows it running, inspect the pane
before `gang compact worker --recover`, which interrupts it and records it as
unconfirmed. Recovery sends nothing when the pane shows an approval, a draft,
or an idle composer. Interrupting can return the queued resume note to the
composer, or the harness can submit it at once; either way the compaction may
not have finished. Clear a restored note, then compact again if needed. A
completed status and the resume note in the worker's pane confirm that work can
continue from the saved file.

## Recover a blocked startup pane

Finish a native login, trust, permission, or choice prompt yourself. Gangline
names the pane and reports that it needs attention. Inspect it:

```sh
gang roster
gang capture worker
gang attach
```

Answer the prompt in the native window, detach with `Ctrl-b d`, then run:

```sh
gang tick
gang status worker --why
```

If the answer ends the native CLI, `gang tick` fails the agent and
`gang roster` shows the CLI's last lines.

If startup input was pasted but could not be verified, run
`gang hitch worker --recover`. It submits the original startup envelope when
its exact text is visible. For a lone collapsed paste, it clears the composer,
checks that it is empty, then repastes the retained envelope. Other drafts are
left untouched. Recovery requires the original submit witness; a refusal
prints the retained message path. Check `gang status worker --why` and the
pane for successful startup.

## Keep an unattended team moving

Run a team recovery pass and inspect its audit events:

```sh
gang tick
gang log --type tick
gang roster
```

On Linux with a systemd user manager or on macOS with launchd, Gangline arms a
transient watchdog that runs whole-team ticks. It needs the host awake and the
user scheduler available. The log's `tick` event identifies a command, hook,
or watchdog source. If the scheduler is unavailable, `gang tick` still does
ordinary work and logs `watchdog_unavailable`; a scheduler failure logs
`watchdog_failed` and returns an error. After fixing the scheduler, run
`gang tick` again; once it arms the timer, an earlier `watchdog_unavailable`
is followed by `watchdog_available`. After the watchdog fires,
`gang log --type tick` should include a record with `source` set to `watchdog`. A command-sourced tick alone
does not verify watchdog operation.

## Stop a team

Keep a copy of the audit log, inspect the roster, and end the selected team:

```sh
gang log > team-log.jsonl
gang roster
gang down
gang teams
```

`down` confirms the selected team and agent count on a terminal. Use
`gang down --yes` in scripts or without a terminal. It stops registered agents
and deletes the team's state and history. From an agent's pane, including any
script that agent starts, only the lead may run it. The team should disappear
from `gang teams`. To stop just one agent, use `gang drop NAME`, then verify
that it is absent from `gang roster`.
