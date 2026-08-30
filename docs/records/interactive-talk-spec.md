# Interactive talk — implementation spec

> Status: Implemented 2026-08-30.

## Purpose

An operator recording or running a team should be able to write a message at a
terminal without constructing a pipe, while agents retain the scriptable
`gang send --to <name> --stdin` surface unchanged. The live consumer is the
operator-directed re-recorded demo.

The prior-art search found the styled composer reader in closed PR #103 and the
existing send/refusal work. It found no earlier interactive authoring command.
The reader is observation only; delivery remains `send`'s job.

## Surface

`gang talk <name> [--from <sender>] [--live-only] [--supersede]` opens
`${VISUAL:-${EDITOR:-vi}}` on a private temporary draft. Closing the editor
sends its non-empty body through the same `cmd_send` path as
`gang send --to <name> --stdin`, carrying those flags unchanged. A local shell
outside a Gangline window defaults its claimed sender to `operator`; a Gangline
agent retains the ordinary observed-sender rule, and an explicit `--from` keeps
`send`'s validation.

The command is deliberately a terminal convenience, not an alternative agent
protocol: it requires stdin, stdout, and stderr to be terminals and tells a
non-terminal caller to use `send --stdin`. `send` neither gains a TTY check nor
changes its options, diagnostics, or exit meanings. A successful `talk` prints
only `send`'s verified-delivery line, keeping the recorded happy path clean.

An editor that exits non-zero sends nothing and its status is returned. Closing
an empty draft sends nothing, announces a cancellation, and exits zero. A
non-empty draft is unlinked before the `send` call so every delivery exit path
has the same deletion path; editor and cancellation paths remove it directly,
and signal handling removes an interrupted editor's draft.

## Proof

Integration coverage uses a disposable tmux terminal and a fixture editor to
prove that `talk` sends a multi-line body with the default external attribution,
passes deliberate send flags without warnings, and leaves the target composer
submitted. It also proves the no-terminal refusal happens before any editor
launch. Help and reference documentation state the editor, cancellation, TTY,
and agent-safe stdin rules.
