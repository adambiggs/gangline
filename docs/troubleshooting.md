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

If the error says startup input is unverified, follow the next entry.

## Startup input is unverified

Inspect the pane and resolve any native prompt, then run:

```sh
gang hitch NAME --recover
```

Recovery submits the original startup message only when its exact text is
still in the composer. If Gangline cannot identify it, recovery refuses and
prints the retained message path. Keep that file for diagnosis. An ordinary
`gang send` does not replace the startup contract.

## A message stays queued

Run `gang queue NAME` and `gang capture --composer NAME`. The recipient may
be at a prompt, have unsubmitted draft input, or be waiting for a scheduled
delivery time. Resolve native prompts yourself, then run `gang tick`.
Observation does not submit or discard a draft.

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
idle, refused by the harness, or unconfirmed. The resume note is withheld
until completion is confirmed. For a stuck request, run
`gang compact NAME --recover` after inspection. Do not treat missing resume
text as proof that compaction finished.

## The watchdog is not running

Check `gang log --type watchdog_unavailable` and
`gang log --type watchdog_failed`. The watchdog needs a systemd user manager
on Linux or launchd on macOS, plus an awake host. For unattended operation,
keep the user manager running across logout; Gangline does not configure
login lingering. Repair the reported scheduler problem and run `gang tick`.

## Drop or shutdown refuses

Gangline refuses teardown when it cannot safely identify the registered
processes. Keep the reported evidence and inspect the pane. Do not kill an
unrelated process to clear a lock. If timer cleanup reports contention,
retry `gang down SESSION` after the other operation finishes.

## An upgrade refuses

`gang upgrade` expects an installer-managed release checkout without local
changes. Follow its error message if the checkout is dirty or on a source
branch. `gang upgrade --check` inspects release availability without installing.
