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
and repository trust prompts. Gangline answers only dialogs a collar enumerates
as carrying no authority, with the selected safe row verified before confirm and
the restored composer verified afterwards. It never answers permission, trust,
approval, or access surfaces; those remain `!occupied! (authority unknown)` until
the operator resolves them in `gang attach`. A stall note follows only when a
native harness hook witnesses a configured event: Claude Code can report its
declared notifications and permission requests, while Codex has no Notification
hook and reports only permission requests. An unknown Codex dialog can therefore
refuse delivery without raising a stall note.

During hitch, a first-run prompt is reported as soon as Gangline has positive
pane evidence rather than a merely blank startup screen. Leave hitch running,
use `gang attach` to answer the native prompt, and the same hitch will report
when the input box appears and deliver the startup contract if that happens
before `GANG_BOOT_TIMEOUT` expires.

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
- the target cannot safely accept mid-turn input;
- the state evidence is unknown; or
- another delivery owns that pane's lock.

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

The live delivery is tried first. If it is refused, the message is parked and
reported as parked; the target's own native turn boundary drains it through the
same verified path. Add `--supersede` when the newer message should replace the
sender's earlier waiting ones. An unattended sender does not have to re-send a
bounced message by hand. Pass `--live-only` only when a caller needs a refusal
to come back rather than park.

Do not write a retry loop around `gang send`. Spooling exists so that loop does
not have to, and a loop that re-sends after a failure — as opposed to a refusal
— can deliver a message twice.

`gang status` and `gang roster` report how many messages are waiting for a
target and how many are held. A message is held when its delivery could not be
verified, or when the drain died between submitting it and retiring it. Either
way its fate is unknown, so Gangline stops acting on it: the body stays readable
under `GANG_LOCK_DIR`, and it is never sent again. A harness may have accepted
it into an internal queue and drained that queue later, so read the target before
re-sending it by hand.
Spools die with their window, so `gang drop` and `gang down` remove them.

For a harness whose native Stop event does not reach Gangline, an ordinary send
still tries live delivery. A refusal is not parked, and names the missing
`GANG_STOP_HOOK`, because nothing would drain the message.

## Working in the shared checkout

Teammates hitched to the same directory share one working tree. Two habits
keep that safe:

**Commit as you go, atomically.** One logical change per commit, split by
default — never a pile at the end of an arc. A window can end at any moment
(teardown, compaction, a crash), and work that reached a commit survives it;
work in your head or your uncommitted tree may not. When a change is coherent,
land it.

**Stage by explicit path, never by sweep.** `git add -A`, `git add .`, and
`git commit -a` stage whatever the tree holds — including a teammate's
half-finished work sitting beside yours. Name every file you stage. A path you
did not touch showing up in `git status` is someone else's arc in flight:
leave it alone.

## Compaction

The startup contract tells every agent to use native compaction at natural
checkpoints:

```sh
gang compact NAME
```

Do this after finishing a coherent arc and recording durable state in the
repository, not in the middle of a half-applied edit. Native harness compaction
owns the summary and context transition.

Codex self-compaction is deferred to its Stop event because its active turn
cannot submit `/compact` into its own composer. The request is one-shot. If the
native command cannot be submitted, `status` and `roster` retain the failure
instead of claiming success.

## Reading harness usage without attaching

Ask for each idle agent by name:

```sh
gang usage worker-a
gang usage worker-b
```

The output is the harness's own plan or quota page, not a Gangline summary.
Gangline will not open it over a running turn, draft, unknown state, or native
dialog. A full-screen page is limited to what is visibly painted; attach when
that modal has internal scrollback. Inline output includes content that moved
into tmux history. If the command reports that the composer was not restored,
the content already printed is still useful, but inspect that agent with
`gang attach` before sending it anything else.

## Optional context lights

Context lights are disabled unless the operator supplies absolute thresholds at
hitch time. Set them intentionally high so the harness retains most of its
effective native window, but below its observed automatic-compaction boundary
so the agent can choose self-compaction first. Set `YELLOW_TOKENS` and
`RED_TOKENS` from a current observation of that harness:

```sh
GANG_CONTEXT_LIGHTS="${YELLOW_TOKENS},${RED_TOKENS}" gang hitch worker -c codex
```

Yellow asks the agent to compact at its next natural checkpoint. Red asks it to
finish the current arc and compact now. A light is emitted once per context
epoch; usage falling below yellow resets the epoch. These are advisory native
hook messages, not patrols or automatic actions.

If an enabled source fails, the affected agent receives one unavailable notice.
Disabled lights perform no context read and add no prompt or roster noise.

Claude Code reads its hook configuration once, at process startup, and
re-executes the statusline script from disk on every repaint. A change on the
statusline side therefore reaches running harnesses at the next repaint, while
a change to hook wiring reaches only processes started after it — re-hitch to
pick it up.

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

The glyph wrapped around a gang-managed tmux window name is the state Gangline
last witnessed at an existing observation point or native hook event. It can be
stale; `gang roster` remains the live-computed truth. Commands always address
the bare agent name, so `gang send --to pii-impl` does not change as the glyph
changes. tmux appends its own flags after the name, which makes a last-active
busy window render as `3:-name--`; the trailing pair is tmux's, and Gangline
does not replace the operator's status format.

An abandoned turn decays rather than standing unknown forever. A turn
stopped by keys typed straight into the pane is one no harness reports, and
`gang interrupt` — the only command that edits the fact, by dropping it — was
never involved, so its turn bracket stays open and only grows older. Once that bracket passes `GANG_TURN_LIMIT`,
the tiers under the expired event decide: with nothing painting a turn, the pty
past its quiet window, a stable pane, the harness's own input box on screen and
empty, and neither the pty clock nor the screen having moved while Gangline read
those tiers, the state is `~idle~`. Any one of those missing — a draft in the box,
a frozen busy marker, a pty whose quietness cannot be measured, a harness that
wrote mid-decision — and the answer stays `?unknown?`. A collar that does not
declare `GANG_QUIET_AT_REST` never decays, because its harness writes at rest
and its quiet is an abstention rather than an observation. A bracket that is
unreadable or stamped in the future is unknown, not abandoned, and never decays. Within its bound the bracket still
outranks every quiet tier beneath it, so a working-but-silent harness keeps its
`-busy-` verdict.

`binary-skew` means the window was hitched or adopted from different `bin/gang`
bytes than the observation command. Finish or checkpoint work using the agent's
existing context, then drop and re-hitch it with the intended binary; changing a
live checkout does not retrofit hooks or collars already loaded into a running
window. `binary-identity unavailable` means the snapshot could not checksum one
side; repair the invoked Gangline installation before using the witness.

Collars contain the only harness-specific regexes and parsers. A native TUI
update can invalidate them. The intended repair loop is direct: reproduce the
wrong observation in a disposable team, inspect the actual pane, update the
collar, and run the fast repository gates. Gangline does not ship a diagnostic
agent or automated strategy-rot machinery.

## Disposable real-harness smoke tests

Never test Gangline against the development agent or the live `gangline` team.
Use an explicitly named, disposable session:

```sh
GANG_SESSION=gangline-smoke-codex gang hitch probe -c codex -d "$PWD"
GANG_SESSION=gangline-smoke-codex gang capture probe
GANG_SESSION=gangline-smoke-codex gang down
```

Drive only the native behavior under test, inspect the pane and tmux options, and
delete the exact smoke session afterward. Mandatory tests use a private tmux
server and stand-in collar; they do not spend real harness turns.

## Recovery

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
composer can render in the current pane. The doctrine byte ceiling catches a
category error, not this geometry-dependent bound. Hitch reports that the
startup contract was not delivered and leaves the exact window for inspection.

Use `gang capture NAME` or `gang attach` to inspect it. Then run `gang roster`
before the destructive step, `gang drop NAME`, shorten the doctrine or enlarge
the target pane, and hitch the agent again. Do not treat the pre-launch doctrine
ceiling as evidence that a target can render the body.

### A harness is blocked on a dialog

Run `gang status`. A collar-enumerated benign transient is named and will be
answered only when a send or hitch already needs to write there. Any unknown or
authority-bearing dialog remains `!occupied! (authority unknown)`; run
`gang attach`, answer it in the native TUI, then re-run status. Do not send prose
into an unknown dialog or widen the registry to include an access decision.

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
failure for shapes the parser cannot identify. A degraded fallback that reports
healthy is worse than an explicit break.

## tmux scope

`gang down` destroys every window in `GANG_SESSION`; `gang drop` destroys one
named window. Always run `gang roster` first when other people or agents may be
using the same session.

For experimental tmux work, use an explicit private socket (`tmux -L NAME ...` or
`tmux -S PATH ...`). Never run an unaimed `tmux kill-server` or `kill-session` on
a shared server.
