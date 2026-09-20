# Design

These decisions explain behavior that would otherwise be easy to simplify in
the wrong direction.

## Messages are attributed and delivery is verified

A message crosses from one harness's pane into another's composer, where being
typed, accepted, and submitted are different facts. Gangline therefore reports
delivery only after the target composer visibly accepts and submits the
message.

When Gangline can observe the calling window, it reads the sender from that
window and refuses a claimed name. Otherwise the supplied name is marked
`self-declared:`. This is attribution, not authentication: Gangline is a
single-tenant tool and does not treat an operator's shell as hostile. A
delivery it cannot verify exits 5 and is not retried.

## Configuration is parsed, never sourced

The config file mirrors environment names as strict scalar assignments.
Gangline parses it, refuses unknown or duplicated keys, and gives an explicit
environment value precedence. Operator text is never executed as shell code,
and a misspelled setting fails visibly instead of being ignored.

## The irreversible verb demands a name

`gang down` requires the exact session it will end and refuses to run from
inside that session. This keeps a bare invocation informational and makes team
teardown a deliberate, named operation. `gang teams` prints the required name.

## Teardown archives mail before deleting its spool

A queued message may be the only copy of what its sender wrote. `gang drop` and
`gang down` archive every waiting or held entry before deleting its spool and
refuse teardown if the archive cannot be written. `gang queue` archives entries
before printing them, so a downstream shell filter cannot erase the only copy.
Gangline never deletes these recovery archives automatically; each read names
the command that will remove them.

## Native continuation owns compaction recovery

Compaction can leave a native session at an empty composer with no next turn.
Every compaction submitted by Gangline is followed by a continuation that asks
the agent to re-read its brief and saved state. `gang compact --resume` replaces
that continuation text. The recovery instruction remains operator prose rather
than hidden Gangline state.

## The contract uses the system prompt where available

The standing terms live in `CONTRACT.md`, resolved operator-first and validated
before a window opens. A missing or unreadable contract refuses the hitch.
Where a collar declares `GANG_ROLE_PROMPT_OPT`, the contract and role brief are
attached to the harness's system prompt; otherwise the startup message points
the agent at the files. System-prompt attachment keeps the terms across native
compaction without depending on pane geometry. A running agent keeps the copy
it launched with.

## Occupancy is not authority

Gangline does not answer a harness-owned prompt or dialog. A collar may
recognize that the composer is occupied with `GANG_OCCUPIED_REGEX`; ordinary
input is then refused and a person can answer through `gang attach`. An
unrecognized surface fails closed. This keeps security, trust, and permission
choices in the native harness instead of encoding version-specific dialog text
and keystrokes in Gangline.

## The mandatory gate has a memory ceiling

ShellCheck's peak memory grows sharply when it receives the whole repository at
once. `test/lint.sh` invokes it once per file, and a test file too large for the
gate's memory budget is split into sourced parts. The parts share one fixture
and counter set; variables crossing a part boundary carry a ShellCheck directive
naming the corresponding file.

## A dead Claude stream is resumed from two native witnesses

Claude Code may return to an idle prompt without emitting a Stop event when a
provider stream fails. Under `GANG_AUTO_RESUME`, Gangline requires both a later
`idle_prompt` notification bound to the transcript and a newest top-level
assistant record carrying a structured API error and UUID. It submits at most
one continuation per error UUID. If ownership cannot be proved, Gangline
refuses and exposes the reason in status and roster. Collars without equivalent
native evidence declare no auto-resume behavior.

## Scoped agents need separate cgroups

A tmux server otherwise places every agent under the same inherited cgroup,
allowing one memory-pressure decision to end the whole team. With
`GANG_SCOPE=on`, each hitch launches in its own transient systemd user scope.
If that scope cannot be created, the hitch is refused rather than silently run
unscoped. The operator still owns the host's memory-pressure thresholds.

## Mandatory tests are immediate

Mandatory tests do not use sleeps, polling, or elapsed wall time as evidence.
They assert state already established by a command through immediate reads,
event barriers, or fake clocks. A timeout fixture may scale a fake clock, but
must record its measured margin. Real harness turns belong only in the opt-in
end-to-end test.

## Mandatory barriers stay inside the wait ceiling

`tmux wait-for` has no timeout, so an unanswered barrier can hold the shared
gate indefinitely without output. The suite puts a tmux shim first on `PATH`
that bounds blocking waits and names the barrier. Lint rejects blocking
`wait-for` calls that bypass the shim. Nonblocking `-S` signals pass through.

## Teardown authority follows the tmux server reached

Inside a pane, `$TMUX` outranks `TMUX_TMPDIR`; some tmux versions also fall back
to the default socket when a configured directory is absent. The tmux guard
resolves the socket a teardown command would actually reach and refuses
`kill-server` and `kill-session` on the team's launch socket, the host's
`default` and `gangline` servers, and an empty `-S`. Callers may add refusals but
cannot add authorizations. Ordinary tmux commands pass through without a probe.
The guard is a guardrail, not a security boundary.

## Option readers preserve unknown state

A failed tmux option read is not the same as an unset option. Every read failure
propagates to its caller, and no reader clears, omits, or renders the value as
absent. Status and roster show unreadable values as unknown. Readers returning a
new failure status require auditing both pipes and command substitutions because
neither reliably trips `set -e`.

## A scoped hitch has an immutable identity

Renaming an agent does not restart its pane. Each scoped hitch therefore mints
an immutable identity and launches in a unit named from the session, original
hitch name, and identity, recorded in `@gl_scope`. Rename leaves the unit alone,
allowing the displayed name to be reused later without moving a live process
between cgroups.

## A refused delivery is parked; a failed one is not

A refusal happens before any keystroke, so the sender still owns the body. A
failure after a paste has an unknown outcome, and sending another copy could
duplicate a message that arrived.

Refused messages are parked in the target spool unless `--live-only` is used.
The drain claims each entry before delivery. An entry whose delivery cannot be
verified, or whose drain ends mid-flight, is held and never sent again
automatically. Spool identity is minted at hitch and adopt, before another
delivery can race it.

## Tick passes use two kernel locks

Each team has a queue flock and a run flock. A launch tries the queue lock
without blocking; if another launch owns it, no process is started. The queued
tick waits on the run lock and releases the queue lock before its pass begins,
so work arriving during a pass schedules one later pass. The kernel releases
both locks with their holder, removing any recovery protocol for lock ownership.
A partial pass leaves its cursor for the next launch.
