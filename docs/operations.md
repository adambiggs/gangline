# Operating a team

Gangline keeps control in native harnesses and tmux. There is no background
coordinator to recover work, approve a dialog, or decide what an agent should do.
An unattended session succeeds when the harnesses have the access they need, the
operator can observe them, and agents compact at natural checkpoints.

## Before leaving a team unattended

Check the direct surfaces:

```sh
gang config
gang roster
gang status lead
gang capture lead
```

Confirm that each harness has already completed first-run setup, authentication,
and repository trust prompts. Gangline answers no native dialog at all. A screen
that owns the input box reads `!occupied! (authority unknown)` until the operator
resolves it in `gang attach`. A stall note follows only when a
native harness hook witnesses a configured event: Claude Code can report its
declared notifications and permission requests, while Codex has no Notification
hook and reports only permission requests. An unknown Codex dialog can therefore
refuse delivery without raising a stall note.

When another action should start only after an agent settles, use an explicit
barrier instead of repeatedly sampling status:

```sh
gang wait worker --until idle
gang wait worker --until done
```

`idle` waits through native Stop boundaries until a live read is idle. `done`
accepts the next Stop, including the active turn's completion when the agent was
already working; it does not assign or track a turn. Unknown evidence and a
vanished or replaced target fail loudly. A hookless collar cannot supply the
boundary and is refused before the command blocks. Each boundary also has a
fail-loud foreground deadline: pass `--timeout <seconds>` for a caller-specific
bound, or omit it to use `GANG_TURN_LIMIT`.

During hitch, a first-run prompt is reported as soon as Gangline has positive
pane evidence rather than a merely blank startup screen. At that point it parks
the startup contract instead of dropping it. Leave a direct `hitch` running and
use `gang attach` to answer the native prompt. `gang up` exposes the gated agent
in its tmux client automatically, so answer the prompt there. The same
invocation delivers once the composer appears, even if the native choice
disables hooks; a transient draft or modal makes it wait and retry without
typing. Once positive prompt evidence commits the contract, that foreground
wait has no time bound: `GANG_BOOT_TIMEOUT` becomes the length of each
observation slice, not a deadline that drops ownership. Run a hitch that may
meet first-run authority from an operator terminal; an agent that invokes it
remains occupied until the operator answers, by design, because Gangline starts
no background watcher.
Do not send a second copy by hand. `gang roster` and `gang status <name>` show
the contract queued until the verified drain completes. Interrupting hitch
leaves that attributed envelope in the spool for inspection rather than
silently discarding it, but also leaves no process that will drain it before a
turn exists. Recover an interrupted gated hitch by dropping that exact window
and re-hitching after resolving the native gate. Use `--resume` only when drop
prints a stamped native-session line; a pre-turn gate normally has no native
identity to resume. Do not send a replacement contract by hand.

Native sandboxes must be able to reach the tmux server and the `gang` executable.
For Codex, this commonly means the tmux socket and Gangline checkout need to be
inside paths its sandbox permits. Fix that in the operator's Codex configuration;
shipped collars never disable sandboxing or bypass approvals.

Use a stable `GANG_SESSION`, `GANG_COLLARS`, `GANG_LOCK_DIR`, and absolute
`GANG_CONFIG_DIR` for every shell that addresses the team. A hitch pins the
resolved config root into its agent so nested hitches read the same file and
doctrine.

## Forwarding native stall witnesses

Declare one optional receiver with `gang notify <name>`. Gangline forwards only
events the native harness itself reports as awaiting a person; it never infers a
stall from a quiet pane and runs no patrol. Claude Code can witness its declared
`Notification` kinds and permission requests. Codex has no `Notification` hook,
so its shipped collar witnesses only permission requests. opencode and Pi
declare no stall source, so they raise no notes.

A note accepted live or parked is debounced until the raising harness reports
movement or the repeat bound expires. `gang status` and `gang roster` expose a
delivery failure until a later note is accepted. Use `gang notify clear` to turn
the forwarding off; the declaration also dies with the team session.

## Sending messages safely

Pass message prose on standard input so shell syntax remains data:

```sh
gang send --to worker --stdin <<'TASK'
Run the parser test whose fixture contains $(literal shell text).
TASK
```

A quoted heredoc delimiter prevents the caller's shell from expanding the body.
Gangline then verifies the target composer changed before submitting it.

Delivery refuses when:

- a native dialog or picker owns input;
- a person has a draft in the composer;
- tmux copy-mode or another tmux mode owns the pane;
- the target cannot safely accept mid-turn input;
- the state evidence is unknown; or
- another delivery owns that pane's lock.

A lock-owner refusal happens before Gangline accepts the body; retry that same
send after the named delivery finishes. This is distinct from a command that
reports `queued`, which has accepted the body and needs no retry.

A failure after a paste may leave staged text in the composer. `gang status` and
`gang roster` report it. Inspect with `gang capture` or `gang attach`; the next
safe delivery clears only a staged rendering that still exactly matches what
Gangline recorded.

## Parking a message a busy target refused

No extra flag is needed when a message should wait for a drainable target:

```sh
gang send --to worker --stdin <<'TASK'
When you surface, the parser fix needs a second reviewer.
TASK
```

After the command owns the pane delivery lock, live delivery is tried first. If
it is refused, the message is parked and reported as parked. Stop drains every
collar through the same verified path; a `steer` collar also tries at
PostToolUse while the turn remains open, but only after a free composer can take
the already-attributed claim. A live compaction remains closed to peer
steering; PostCompact is its delivery opportunity. If the harness parks an
accepted steering Enter in its native queue, Gangline keeps the exact composer
record so `gang status` accounts for it and `gang flush` can recover it. A draft
or tmux copy-mode leaves the entry live. Add `--supersede` when the newer
message should replace the sender's earlier waiting ones. An unattended sender
does not have to re-send a message Gangline reports as queued. Pass
`--live-only` only when a caller needs a refusal to come back rather than park.

Do not write a retry loop around `gang send`. Spooling exists so that loop does
not have to, and a loop that re-sends after a failure — as opposed to a refusal
— can deliver a message twice.

`gang status` and `gang roster` report how many messages are waiting for a
target and how many are held. A message is held when its delivery could not be
verified, or when the drain died between submitting it and retiring it. Either
way its fate is unknown, so Gangline stops acting on it: the body stays readable
under `GANG_LOCK_DIR`, and it is never sent again. A harness may have accepted
it into an internal queue and drained that queue later, so read the target before
re-sending it by hand. Spools die with their window: `gang drop` and `gang down`
archive any waiting or held entries under `GANG_ARCHIVE_DIR`, then forget the
spool.

An addressee's own `gang mail` read also archives every entry before printing
it. These read archives are durable operator recovery state under
`GANG_ARCHIVE_DIR`; Gangline never expires or reaps them, and a home-directory
backup may retain them too. Each read prints its exact archive root and a quoted
`rm -rf -- <root>` command on stderr. The archive dies when the operator has
finished any recovery or audit that required it and runs that command. Periodic
operator housekeeping should remove read and teardown archives that no longer
have a recovery purpose.

For a harness whose native Stop event does not reach Gangline, an ordinary send
still tries live delivery. A refusal is not parked, and names the missing
`GANG_STOP_HOOK`, because nothing would drain the message.

## Working in the shared checkout

Teammates hitched to the same directory share one working tree. Two habits
keep that safe:

**Commit as you go, atomically.** One logical change per commit, split by
default — never a pile at the end of an arc. When a change is coherent, land
it.

**Stage by explicit path, never by sweep.** `git add -A`, `git add .`, and
`git commit -a` stage whatever the tree holds — including a teammate's
half-finished work sitting beside yours. Name every file you stage. A path you
did not touch showing up in `git status` is someone else's arc in flight:
leave it alone.

## Compaction

The contract tells every agent to use native compaction at natural
checkpoints. Bare, it targets the calling window, so an agent does not name
itself:

```sh
gang compact
```

Do this after finishing a coherent arc and putting unfinished work in repository
files that teammates can read, not in the middle of a half-applied edit. Native
harness compaction owns the summary and context transition.

Codex self-compaction is deferred to its Stop event because its active turn
cannot submit `/compact` into its own composer. The request is one-shot. If the
native command cannot be submitted, `status` and `roster` retain the failure
instead of claiming success.

## Reading provider limits without attaching

Ask for each agent by name:

```sh
gang limits worker-a
gang limits worker-b
```

The result names each native usage window, its percentage used, reset time, and
sample age. Claude Code uses its headless usage command. Codex uses the target
session rollout's newest native rate-limit event, so its sample age is the time
since that agent's last API turn. An old Codex event remains printable evidence,
but after five minutes it is too stale to drive a light; the next API turn
refreshes it. Its absolute future reset remains sufficient to arm a wake without
spending quota on a refresh. A collar with neither source reports the capability
unavailable. The command may target an agent mid-turn, because it never drives
that agent's composer.

`gang usage`, which drove the harness's own usage page through the composer, was
retired in 2.0. Attach if you want the full native page.

## Waiting for a provider reset

An agent can end its current work and arrange one continuation at the reset of
its most constrained native window:

```sh
gang wait-limit worker
gang wait-limit worker --resume "Re-read the assigned arc and continue."
gang wait-limit worker --clear
```

The most constrained window is the highest percentage used; the later reset
breaks a tie. Gangline creates one transient systemd user timer. No Gangline
process remains running, the timer invokes ordinary attributed delivery once,
and the unit is collected after it runs. `gang status` and `gang roster` show a
pending or failed wake. Clearing it stops the exact timer; dropping an agent or
ending its team also attempts that cleanup. If the team or agent has already
disappeared when a timer fires, the stale timer does nothing and is collected.

The user manager must be available for scheduling. A failure to create or clear
the timer is loud and is never reported as a scheduled wake.

## Optional context lights

Context lights are disabled unless the operator supplies thresholds at hitch
time. Percentages serve mixed-window teams; absolute tokens remain available
for one observed harness window. Set them intentionally high, but below the
observed automatic-compaction boundary so the agent can self-compact first:

```sh
GANG_CONTEXT_LIGHTS="50%,80%" gang hitch worker -c codex
```

Yellow asks the agent to compact at its next natural checkpoint. Red asks it to
finish the current arc and compact now. A light is emitted once per context
epoch; usage falling below yellow resets the epoch. These are advisory native
hook messages, not patrols or automatic actions.

If an enabled source fails, the affected agent receives one unavailable notice.
Disabled lights perform no context read and add no prompt or roster noise.
An absolute red threshold above the window reports one invalid notice as soon as
the native source makes that window readable; it never remains silently armed.

Claude Code reads its hook configuration once, at process startup, and
re-executes the statusline script from disk on every repaint. A change on the
statusline side therefore reaches running harnesses at the next repaint, while
a change to hook wiring reaches only processes started after it — re-hitch to
pick it up.

## Optional provider-usage lights

Provider-usage lights are disabled unless the operator supplies used-percentage
thresholds at hitch or adopt time:

```sh
GANG_USAGE_LIGHTS="90%,95%" gang hitch worker -c codex
```

Yellow asks the agent to finish at its next natural checkpoint. Red asks it to
finish the current arc and stop before the provider limit. Each edge is emitted
once until usage falls below yellow. The last native reading or loud
unavailability remains visible in `gang status`; `gang roster` carries yellow,
red, and unavailable states without performing a new read.

Claude's headless read launches the harness behind a portable 50-second process
bound, so hook-driven lights reuse a sample for one minute instead of launching
it after every tool call. Explicit `gang limits` and `gang wait-limit` commands
always take a fresh reading. A missing or incompatible `timeout` command is
named as that dependency failure, not misreported as a native harness failure.

These lights use the same collar correctness source as `gang limits`. They do
not type `/usage`, scrape a pane, poll in the background, or infer a limit from
quiet terminal activity.

## Optional automatic resume at a provider reset

Auto-resume is disabled unless the operator declares a used percentage at hitch
or adopt time:

```sh
GANG_AUTO_RESUME="97%" gang hitch worker -c claude-code
```

At or above that percentage the agent's own turn hook arms the same transient
timer `gang wait-limit` arms, once per provider window, and says so in that
turn. At the reset the agent receives the ordinary continuation asking it to
re-read its assignment and continue only if work remains. Nothing polls and no
process stays running between the arming and the reset.

For claude-code, the same declaration also covers a provider stream that dies
mid-turn. The native `idle_prompt` notification says the harness is waiting and
binds its transcript; Gangline resumes only when that transcript's newest
top-level assistant record is structurally an API error. Error prose is never
matched. One error UUID receives one attributed continuation. Its visible
Gangline envelope is also the ownership marker: if that continuation dies,
there is no second hop. An unreadable transcript, unverified marker, refused
delivery, or exhausted hop is fail-closed and appears in `gang status`; roster
adds `auto-resume-failed`. A later ordinary prompt starts a new episode without
erasing that refusal record; a later positively owned automatic continuation or
dropping the window retires it.

Claude Code is the live consumer for stream-failure recovery. Codex exposes no
native failed-turn record that can distinguish dead, running, and killed work,
so Gangline does not infer parity. The transcript binding, handled-error UUID,
one-hop marker, and any refusal live on the tmux window and die when that window
is dropped.

It is independent of `GANG_USAGE_LIGHTS`; declare both when the team should be
warned before it is resumed. `gang status` shows the pending wake and, while the
reset is still ahead, whether the transient unit that keeps it still exists.
`gang wait-limit <name> --clear` cancels a wake and is not overridden: that
provider window will not be armed again. An existing manual wake also wins over
automatic arming. Gangline records the sampled provider window as handled but
does not stop its timer or replace an optional `--resume` turn.

Two limits are worth knowing before relying on it unattended. A provider window
that is exhausted between two samples is never seen at the threshold and arms
nothing, so choose the percentage with the sampling interval in mind. And a wake
survives exactly what the team survives — the declaration dies with the tmux
window and the timer dies with the user manager, and a host restart takes both
along with the team the resume was for.

## Optional team curfew

Declare one wall-clock endpoint when a team is timeboxed:

```sh
gang curfew 2h
gang curfew 17:30
gang curfew
gang curfew clear
```

Gangline gives hook-enabled agents one yellow time light halfway through the
declared span and one red light after four-fifths. Each reports the elapsed
fraction and remaining time. These are native hook messages, not a patrol. The
curfew never stops an agent, and no declaration means no time guidance.

## Understanding observation

Gangline selects evidence for each predicate rather than averaging it. Fresh
native events outrank owned file state, which outranks pane scraping. Missing,
expired, or contradictory evidence produces an explicit unknown state.

`-busy-` means Gangline has positive evidence of work. `~idle~` means it has positive
evidence the composer is ready. `!occupied!` means a native UI owns the composer.
`?unknown?` means the available evidence can no longer answer truthfully.

For the window-name glyph, its staleness, bare addressing, and tmux's appended
flags, see [Observation](reference.md#observation); `gang roster` remains the
live-computed truth.

An abandoned turn — one stopped by keys typed straight into the pane, which no
harness reports — decays rather than standing unknown forever: once its bracket
passes `GANG_TURN_LIMIT`, the state reads `~idle~` only when every quieter tier
positively witnesses readiness, and stays `?unknown?` otherwise. The exact
conjunction is in [`gang status`](reference.md#gang-status-name).

`binary-skew` means the window was hitched or adopted from different `bin/gang`
bytes than the observation command. Finish or checkpoint work using the agent's
existing context, then drop and re-hitch it with the intended binary; changing a
live checkout does not retrofit hooks or collars already loaded into a running
window. `binary-identity unavailable` means the snapshot could not checksum one
side; repair the invoked Gangline installation before using the witness.

Collars contain the only harness-specific regexes and parsers. A native TUI
update can invalidate them. The intended repair loop is direct: reproduce the
wrong observation in a disposable team, run `gang explain NAME` to see the
exact rule and pane fragment used by that state read, inspect the full pane when
needed, update the collar, and run the repository gate. Gangline records no
diagnostic history and ships no strategy-rot machinery.

## Disposable real-harness smoke tests

Never test Gangline against the development agent or the live `gangline` team.
Use an explicitly named, disposable session:

```sh
GANG_SESSION=gangline-smoke-codex gang hitch probe -c codex -d "$PWD"
GANG_SESSION=gangline-smoke-codex gang capture probe
GANG_SESSION=gangline-smoke-codex gang down gangline-smoke-codex
```

Drive only the native behavior under test, inspect the pane and tmux options, and
delete the exact smoke session afterward. Mandatory tests use a private tmux
server and stand-in collar; they do not spend real harness turns.

## Recovery

### A malformed occupancy witness refuses observation

Gangline retains malformed `@gl_occupied` evidence rather than demoting an
unknown permission state to absence. The refusal names the affected agent and
the exact tmux window target. Inspect that window and its option, then remove
only the malformed witness:

```sh
tmux show-options -wv -t '<window-id>' @gl_occupied
tmux set-option -uw -t '<window-id>' @gl_occupied
```

Re-run `gang status`. Do not clear a valid `open <unix-seconds>` witness; direct
observation of the restored composer clears valid evidence itself.

### Every command and native hook refuses after a config edit

Gangline parses the config before dispatch. An unknown or duplicated key, empty
value, malformed line, or forbidden byte therefore refuses every command,
including `gang hook`; agents can surface hook errors and temporarily lose turn
facts, Stop-driven work, occupancy events, and lights. This is a single loud
failure, not a fallback to defaults.

Read the error: it names the file, line, offending key, and settable keys, then
edit that line. To remove the file layer immediately at the default or selected
config root, move it aside and confirm the remaining layers:

```sh
config_dir="${GANG_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/gangline}"
mv "$config_dir/config" "$config_dir/config.disabled"
gang config
```

The environment remains authoritative, so `gang config` is also the check for
an override that still masks the repaired file.

### A larger startup contract cannot be delivered to the target pane

Operator doctrine can make the startup contract larger than the target
composer can render in the current pane. Hitch reports that the startup contract
was not delivered and leaves the exact window for inspection; byte count is not
used as a proxy for this geometry-dependent boundary.

Use `gang capture NAME` or `gang attach` to inspect it. Then run `gang roster`
before the destructive step, `gang drop NAME`, shorten the doctrine or enlarge
the target pane, and hitch the agent again.

### A harness is blocked on a dialog

Run `gang status`. Any dialog that owns the input box reads
`!occupied! (authority unknown)`; run `gang attach`, answer it in the native TUI,
then re-run status. Do not send prose into a dialog: a keystroke there answers
the dialog rather than reaching the agent.

### Codex will not start until its hooks are reviewed

Codex reviews new or changed hooks before it will run them, and Gangline wires
hooks at launch. The window reads `!occupied! (authority unknown)`, hitch
reports that the startup contract was not delivered, and the refusal points at
`gang attach`. Attach and the screen names itself.

Gangline will never answer this one. It answers no dialog, and this one is an
authority decision besides. Answer it yourself, in the native TUI, choosing to
review the hooks.

Two things a capture will not tell you. Answering once in a checkout persists
that trust in codex's own configuration, per wired event, under a hash of the
hook command — which embeds this checkout's path. A fresh clone, a moved
checkout, a container image, or any change to what the collar composes raises
it again, so pre-answer it once per environment rather than expecting it never
to return.

And continuing WITHOUT trusting is the option to avoid. It leaves codex running
with its hooks inert, so that agent has no `Stop` hook: no spool drain and no
deferred self-compaction. Gangline waits on turn boundaries that never arrive,
and the roster shows a teammate that looks alive.

### The harness parked a message in its own input queue

Delivery reports this instead of claiming success. Recover it:

```sh
gang flush worker
```

Gangline presses the collar's recall key, reads the loaded composer back
against the message it recorded as parked, and submits it only if they match.
It refuses when the composer no longer shows queue evidence, when it holds no
record of the parked body, or when the readback does not match — without
pressing Enter. If the flush reports that the harness parked the message again,
the queue is hard-stuck: `gang drop` that agent, copy its printed explicit-id
relaunch line, run that line, and re-send what the queue swallowed.

### Something is in the input box and it is not clear what

Do not diagnose this from `gang capture`. The raw pane renders a harness's dim
suggested-prompt placeholder identically to a half-written human line, and that
reading has been got wrong in public. Refusals and the `box:` line under an
undelivered-input report already classify it — `draft`, `staged`, `unattributed`,
`parked`, `whole-pane`, `cleared`, `unreadable` — and each names the look or
recovery that settles it. `unattributed` means Gangline pasted something here and
never saw where it went; `gang status` prints the record of what it pasted.
`cleared` means the box emptied while Gangline was naming what it refused on:
the refusal stands, and the send will go through now.
To see the box directly:

```sh
gang composer worker
```

This prints the collar's styled reading: what a human typed, and nothing else.
Empty output is an empty box, so text on a `gang capture` that `gang composer`
does not show is the harness's own placeholder, not a stuck draft. A placeholder
never blocks delivery for the same reason.

### A turn has to be stopped

```sh
gang interrupt worker
```

This sends the collar's declared turn-stop keystroke and drops Gangline's turn
bracket, so the stopped turn neither keeps reading as busy nor is declared idle
on Gangline's say-so. `gang status` then answers from the pane itself: a harness
that stopped reads idle, and one that ignored the key stays busy and refuses
delivery. Gangline refuses to interrupt an occupied composer, because that
keystroke is often what a native dialog reads as an answer.

### The whole team vanished at once

Every window gone with no `gang down` and no stall can be `systemd-oomd` rather
than anything in Gangline. Unless the team was hitched with `GANG_SCOPE=on`, the
tmux server and every agent in it share one cgroup — the login session scope
that started the server — and `oomd` kills a cgroup whole, so one decision takes
everything in that scope, with no partial survival and no signal an agent could
catch. A kernel OOM kill, a killed tmux
server, or a reboot erase the same windows, so identify the killer from evidence
before treating it as this one:

```sh
journalctl --no-pager | grep 'systemd-oomd.*Killed'   # add -b -1 after a reboot
oomctl
```

`oomctl` prints the live policy and the thresholds it is holding. Where
`/user.slice` is a swap-monitored cgroup, `oomd` acts only once system memory
*and* system swap are both over that limit, and it then kills eligible
descendants starting from the one holding the most swap. Read both halves: a full
swap alone is not the trigger, and swap fills early and stays full, so the memory
half is usually what decides when a team dies. The victim pool is narrower than
the trigger, so a team can be selected while a larger share of the swap sits
outside `/user.slice`, where nothing is eligible to be chosen.

`ManagedOOMPreference=avoid` and `=omit` do not rescue a team, and they fail
quietly. `systemd-oomd` honors those attributes only for a root-owned cgroup, and
a `--user` unit's is not; a user manager may not write the attribute at all while
still reporting the property as set. Presence of the attribute is not proof it
will be honored either — read the cgroup's ownership alongside it:

```sh
stat -c '%U %n' /sys/fs/cgroup/<unit-cgroup-path>
getfattr -n user.oomd_omit /sys/fs/cgroup/<unit-cgroup-path>
```

What is eligible is narrower still: descending a monitored tree, `oomd` can only
select a cgroup that has no subgroups of its own, or one whose
`memory.oom.group` is 1. A tmux server creates no subgroup, so a team is one
leaf and one candidate. `GANG_SCOPE=on` is the lever for that half — each
hitched harness launches in its own transient user scope, so each agent is its
own leaf, holds its own swap, and is named in the kill message. The rest of the
levers are the operator's: the policy on `/user.slice`, and caps on whatever
drives the machine to the memory threshold. Read the placement either way,
because a path naming a session scope means the team can die with a login
session it has usually already outlived:

```sh
cat /proc/$(tmux display-message -p '#{pid}')/cgroup
```

With scopes in place a kill takes one agent and names it, so recovery is the
ordinary one — read the dead window's identity with `gang whoami` and bring it
back with the `gang hitch <name> --resume <session-id>` line. If the scope
outlived the pane because the agent left a detached child, the next hitch of
that name refuses and prints the unit to stop.

### The tmux server was lost

Gangline persists no roster. Recreate only the agents the operator chooses:

```sh
gang hitch lead -c claude-code -d "$PWD" --resume <lead-session-id>
gang hitch worker -c codex -d "$PWD" --resume <worker-session-id>
```

Use ids copied from each agent's earlier `gang drop` output or native transcript.
`--resume` substitutes the exact id into the collar's native command and
refuses collars that cannot do so. There is no latest-session fallback. This is
relaunch, not a claim that Gangline reconstructed the old team.

### A collar no longer recognizes its TUI

Capture the real pane, update only that harness's collar, and require loud
failure for shapes the parser cannot identify.

## tmux scope

`gang down` destroys every window in `GANG_SESSION`; `gang drop` destroys one
named window. Always run `gang roster` first when other people or agents may be
using the same session.

For experimental tmux work, use an explicit private socket (`tmux -L NAME ...` or
`tmux -S PATH ...`). Never run an unaimed `tmux kill-server` or `kill-session` on
a shared server.
