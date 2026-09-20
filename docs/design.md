# Design decisions

Use this page before changing delivery, agent lifecycle, runtime safety, or the
test harness. Each decision records a tradeoff that still shapes the code. The
[constitution](../CONSTITUTION.md) sets the broader constraints.

## Delivery and identity

### Attribute every message and verify delivery

A message crosses from one harness pane into another harness's composer. Being
typed, accepted, and submitted are different facts, so Gangline reports
delivery only after the target composer visibly accepts the message.

When Gangline can observe the calling window, it reads the sender from that
window and refuses a claimed name. Otherwise it marks the supplied name as
`self-declared:`. This is attribution, not authentication: Gangline is a
single-tenant tool and does not treat the operator's shell as hostile. A send
whose keystrokes landed but cannot be verified exits with status 5 and is not
retried.

### Park a refusal, not an unknown outcome

A refusal happens before any keystroke, so the sender still owns the body.
Gangline parks that body in the target spool unless `--live-only` is set.

A failure after a paste has an unknown outcome. Sending another copy could
duplicate a message that arrived. Gangline keeps that record but never sends it
again automatically. Spool identity is minted before another delivery can race
the same window.

### Archive mail before teardown

A queued message may be the only copy of what its sender wrote. `gang drop` and
`gang down` archive every waiting or held entry before deleting its spool, and
they refuse teardown if the archive cannot be written. `gang queue` archives an
agent's own entries before printing them, so a downstream shell filter cannot
erase the only copy. Gangline never deletes these archives automatically; each
read names the command that removes them.

### Require a name for team teardown

`gang down` requires the exact session it will end and refuses to run from
inside that session. A bare invocation stays informational, and teardown remains
a deliberate, named operation. `gang teams` prints the required name.

## Native harness lifecycle

### Leave native prompts with the harness

Gangline does not answer a harness-owned prompt or dialog. A collar may
recognize an occupied composer with `GANG_OCCUPIED_REGEX`; ordinary input is
then refused and a person can answer through `gang attach`. An unrecognized
surface fails closed. Security, trust, and permission choices therefore stay
in the native harness.

### Continue after compaction

Compaction can leave a native session at an empty composer with no next turn.
Every compaction submitted by Gangline gets a continuation that asks the agent
to reload its brief and saved state. `gang compact --resume` replaces that
text. The recovery instruction remains operator prose, not hidden runtime
state.

### Put standing terms in the system prompt when available

The standing terms live in `CONTRACT.md`, resolved operator-first and validated
before a window opens. A missing or unreadable contract refuses the hitch.
Where a collar declares `GANG_ROLE_PROMPT_OPT`, Gangline attaches the contract
and role brief to the harness system prompt. Otherwise the startup message
points the agent to the files. A running agent keeps the copy it launched with.

### Resume a failed native stream only from two witnesses

Claude Code can return to an idle prompt without emitting a Stop event after a
provider stream failure. With `GANG_AUTO_RESUME`, Gangline requires both a later
`idle_prompt` notification bound to the transcript and the newest top-level
assistant record carrying a structured API error and identifier. It submits at
most one continuation for that failure. Collars without equivalent native
evidence declare no automatic resume behavior.

### Keep a scoped hitch identity immutable

Renaming an agent does not restart its pane. Each scoped hitch therefore gets
an immutable identity recorded in `@gl_scope`. Rename leaves the systemd unit
alone, so the displayed name can be reused without moving a live process
between cgroups.

## Runtime safety

### Parse configuration instead of sourcing it

The config file mirrors environment names as strict scalar assignments.
Gangline refuses unknown or duplicate keys and gives an explicit environment
value precedence. Operator text is never executed as shell code, and a
misspelled setting fails instead of being ignored.

### Isolate scoped agents in separate cgroups

A tmux server otherwise places every agent under the same inherited cgroup, so
one memory-pressure decision can end the whole team. With `GANG_SCOPE=on`, each
hitch launches in its own transient systemd user scope. If Gangline cannot
create that scope, the hitch refuses instead of running unscoped. The operator
still owns the host's memory-pressure policy.

### Preserve unknown option state

A failed tmux option read is not the same as an unset option. Every read failure
propagates to its caller; no reader clears, omits, or renders the value as
absent. Status and roster show unreadable values as unknown. A new failure
status requires auditing both pipelines and command substitutions because
neither reliably trips `set -e`.

### Follow the tmux server a command can reach

Inside a pane, `$TMUX` outranks `TMUX_TMPDIR`; some tmux versions also fall back
to a default socket when a configured directory is absent. The tmux guard
resolves the socket a teardown command would reach and refuses `kill-server`
and `kill-session` on protected team sockets. Callers can add refusals but
cannot add authorizations. The guard is a safety rail, not a security boundary.

### Serialize tick launches and passes separately

Each team has a queue lock and a run lock. A launch tries the queue lock without
blocking. The queued tick then waits on the run lock and releases the queue lock
before its pass begins, so work arriving during a pass schedules one later
pass. Kernel locks disappear with their holder. A partial pass leaves a cursor
for the next launch.

## Test behavior

### Bound the mandatory gate's memory

ShellCheck's peak memory grows sharply when it receives the whole repository at
once. `test/lint.sh` invokes it once per file. A test file too large for the
gate's memory budget is split into sourced parts that share one fixture and
counter set.

### Keep mandatory tests immediate

Mandatory tests do not use sleeps, polling, or elapsed wall time as evidence.
They assert state established by a command through immediate reads, event
barriers, or fake clocks. A timeout fixture may scale a fake clock, but it must
record its measured margin. Real harness turns belong only in the opt-in
end-to-end test.

### Bound blocking barriers

`tmux wait-for` has no timeout, so an unanswered barrier can hold the shared
gate indefinitely. The suite puts a tmux shim first on `PATH`; it bounds
blocking waits and names the barrier. Lint rejects calls that bypass the shim.
Nonblocking signals pass through.
