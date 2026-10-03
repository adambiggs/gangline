# Troubleshooting

Start with `gang status NAME --why` and `gang capture NAME`. Keep the error
output and the team directory if you need to report a failure. `gang down`
deletes the team's history.

## The shell cannot find `gang`

The installer uses `~/.local/bin` unless you set `GANGLINE_BIN`. Add that
directory to your shell's `PATH`, reload the shell, and run `gang --version`.
Use `command -v gang` to check which executable your shell selected.

## Startup says the harness needs attention

Attach with `gang attach`, switch to the named pane, and answer its native
prompt. Detach and run `gang tick`, then `gang status NAME --why`. Gangline
leaves login, trust, permission, and choice prompts to you.

A hitch that fails with `native startup was not observable` leaves the
startup queued; the pane may show a prompt Gangline does not recognize. If
`gang status NAME --why` then shows the agent failed with `boot deadline
elapsed`, answer the prompt and run `gang hitch NAME --recover` instead;
`gang tick` does not resume a failed agent.

If the error says startup input is unverified, follow the next entry.

## Startup input is unverified

Inspect the pane and resolve any native prompt, then run:

```sh
gang hitch NAME --recover
```

Recovery submits the retained startup draft, or restores it to an empty idle
composer when its receipt proves that submission was never attempted and the
native submit witness has not changed. A matching native witness verifies
startup without sending it again. Otherwise recovery refuses and prints the
retained message path. Keep that file for diagnosis. An ordinary
`gang send` does not replace the startup contract.

## A message stays queued

Run `gang queue NAME`. Each message is `ready`, `scheduled` with the time it
becomes due, `blocked` with the reason, or `unknown` when Gangline cannot read
the recipient. A blocked message names what holds it, such as the recipient's
status, a native prompt, a draft in its composer, a mid-turn harness that
takes input only when idle, an unverified startup, a compaction resume ahead
of it, or expiry. A ready message behind another names the first message in
line. Check a draft with `gang capture --composer NAME`, resolve native prompts
yourself, then run `gang tick`. Observation does not submit or discard a
draft.

`accepted` means the native queue already owns the input. Do not resend it.
If the result is `unverified`, inspect the recipient before deciding how to
recover: the text may already have reached the harness.

## Status is unknown or reports a failed turn

`unknown` means Gangline could not establish the state. Read the reason in
`gang status NAME --why` and compare it with `gang capture NAME`. A failed
probe does not prove that a turn is stuck.

A native turn failure retains its reason in activity evidence. For a provider
error that ended a turn, `gang tick` can send continuations within the
configured `GANG_CAPACITY_TIMEOUT` budget. Check `gang config` and
`gang limits NAME`; handle any native choice menu in the pane.

## Compaction has not resumed the work

Run `gang status NAME --why` and inspect the pane. A request can be waiting for
idle, refused by the harness, or unconfirmed. An admitted resume note does not
confirm completion. When the pane still shows the compaction running, run
`gang compact NAME --recover` after inspection. It interrupts only a busy pane
with an empty composer and refuses an approval, a draft, an idle composer, or
an unrecognized screen without sending a key. It runs once per compaction and
refuses once the harness has taken the resume note, since a busy pane is then
later work; use `gang interrupt NAME` for that. Interrupting can leave the
resume note in the composer; clear it if the compaction did not finish. Do not
treat missing resume text as proof that compaction finished.

## A hook or tick failed

Native hooks start ticks that run detached from any terminal, so their errors
appear only in the team log. Run `gang log --type tick_failed` and
`gang log --type hook_failed`; each record carries the error and names the
agent where one applies. Repair the reported cause and run `gang tick`.

## The watchdog is not running

Check `gang log --type watchdog_unavailable` and
`gang log --type watchdog_failed`. The watchdog needs a systemd user manager
on Linux or launchd on macOS, plus an awake host. For unattended operation,
keep the user manager running across logout; Gangline does not configure
login lingering. Repair the reported scheduler problem and run `gang tick`.

## The roster lists agents after a reboot

A team does not survive a host reboot. Its agents fail with `the team did not
survive a host reboot`. `gang down` clears its records without signalling any
process, since a process ID recorded under the earlier boot can now name another
process, and prints each agent's native session to resume.

## Drop or shutdown refuses

Gangline refuses teardown when it cannot safely identify the registered
processes. Keep the reported evidence and inspect the pane. Do not kill an
unrelated process to clear a lock. If timer cleanup reports contention,
retry `gang down` after the other operation finishes.

## An upgrade refuses

`gang upgrade` expects an installer-managed release checkout without local
changes. Follow its error message if the checkout is dirty or on a source
branch. Without a terminal on stdin it refuses before installing; pass `--yes`
to install non-interactively. Declining the confirmation installs nothing.
