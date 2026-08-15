# Command and configuration reference

`gang --help` is the authoritative command inventory, and
`gang <command> --help` prints that command's synopsis. Help has a deliberate
48-character line budget so it remains legible in narrow phone-SSH terminals;
this reference carries the complete command contract.

`gang` with no arguments prints a getting-started page written for an agent
rather than an inventory: what it is, how to send, that reading its own mail
consumes it, what each exit status means, and where the inventory is. It exits
0 on stdout, so "run gang" is a complete onboarding instruction for a
freshly hitched agent of any harness. The executable-skew warning is emitted
before it, since this is the first command an agent runs.

When a command's only missing argument is an agent name, an invocation from
inside a Gangline window that omits that name targets that window. An agent
reading or stopping its own state does not have to know its own name. What is
omitted is the name, not every argument: a leading flag is not a name, so
`gang interrupt -m "reason"` is a self-targeted stop carrying its reason, while
a positional argument is always read as the name — a numeric argument to
`capture` remains an agent name rather than becoming a self-targeted line count.
Self is resolved from the calling tmux pane in the same way as a message sender.

| Bare command | Result |
|---|---|
| `status`, `capture`, `composer`, `compact`, `context`, `mail`, `limits`, `wait-limit`, `interrupt`, `flush` | Target the calling agent. |
| `drop` | Print help; destructive commands never target by omission. |
| `hitch`, `adopt`, `send`, `wait`, `explain`, `down` | Print help; the missing name is not a self target. |
| `up`, `roster`, `attach`, `collars`, `roles`, `config`, `curfew`, `notify`, `upgrade` | Keep their ordinary bare meaning. |

Resolving self is not a promise that the command proceeds. `gang compact`
self-targets and is then refused, or deferred, on that same agent's own turn
evidence. Who the target is and what its state allows are separate answers.

Outside a Gangline window, a bare self-targeting command prints its synopsis and
states that no target or Gangline agent window was available.

Every command names an argument it does not consume, and refuses. Nothing is
accepted and discarded: a word Gangline drops silently has told its caller that
the word was understood, and the reading that comes back is then of something
nobody asked for. `gang hook` names it and declines the event instead of dying,
because its caller is a harness configuration rather than a person; see below.

## Installation

The bootstrap command in `README.md` downloads `install.sh` from `main`, but the
installer resolves the newest stable `gangline-vMAJOR.MINOR.PATCH` tag and puts
that release in the install tree. An existing install is moved directly from
its current release to that tag; local changes in the install tree are refused.

`gang upgrade --check` contacts the same repository and compares the installed
`version.txt` with its latest stable release. `gang upgrade` installs that
release over the tree and command link currently running. Both are explicit
network operations. Gangline performs no automatic release check during other
commands. `GANGLINE_REPO` continues to select a different tagged source for a
custom install. `gang upgrade` refuses a branch checkout and points to
`git pull --ff-only`; only the detached release tree produced by the installer
is replaced.

## Lifecycle

### `gang up [name] [hitch flags]`

Hitches the team's lead, names its window `lead` when omitted, then attaches or
switches the current tmux client to it. It attaches the shipped `lead` role by
default; an explicit `-r` or `--role` selects another role. `GANG_COLLAR`
selects the harness. If a native first-run gate appears, `up` exposes it before
waiting for startup-contract delivery, so answering that prompt remains the
only operator step.

### `gang hitch <name> [-c harness] [-d dir] [-m model] [-e effort] [-r|--role role] [--resume [session-id]]`

Starts a native harness in a named tmux window and delivers one startup contract.
That contract names the agent and carries `CONTRACT.md`, which holds the
standing terms. Any supported `-m` or `-e` choice omitted at hitch warns before
the collar silently chooses that default.

How it reaches the agent depends on the collar. Where one declares
`GANG_ROLE_PROMPT_OPT`, the contract's bytes are passed through that launch
option into the harness's own system prompt, so every agent holds it
unconditionally and still holds it after a compaction. The startup contract does
not restate what that prompt already says. It carries the one thing the prompt
cannot: where to look, and an instruction to report it, if the attachment is not
there — Gangline cannot verify that a launch option reached the model. Where a collar declares no such
option, the startup contract points at the path and tells the agent to read it
first and again after a compaction, and to say so and stop rather than improvise
if it cannot. Either way the bytes are never pasted into a composer, so the
contract's length is bounded by nothing the pane can render, and Gangline
resolves and validates the file before opening a window.
If `$GANG_CONFIG_DIR/DOCTRINE.md` is present, readable,
valid UTF-8 prose, the contract attributes and
appends it byte-exactly. Every hitch carries doctrine; Gangline cannot infer
which caller is the operator. `adopt` still injects no startup text.

The launch environment carries the exact `GANG_SESSION`, the absolute resolved
`GANG_CONFIG_DIR`, and any custom collar and lock paths, so harness commands
cannot drift to another session or configuration layer on the same tmux server.
File-layer settings therefore reach nested hitches. Other per-invocation
environment overrides do not become sticky inside the agent.

If a first-run prompt owns the screen before the composer appears, `hitch`
directs the operator to `gang attach` as soon as the collar's occupied pattern
provides positive pane evidence. It immediately parks the attributed contract
in the window's ordinary spool: answering the native prompt is the only manual
step. Direct `hitch` stays in the foreground; `gang up` exposes that gated
window in its tmux client while the same invocation observes beside it. The
verified drain runs when the composer appears and retries pre-keystroke
refusals, so delivery does not depend on a native hook the operator may decline.
After positive gate evidence, the foreground wait is deliberately unbounded;
`GANG_BOOT_TIMEOUT` is then one observation slice. Gangline starts no watcher,
so interrupting that hitch leaves the attributed entry inspectable but not
owned by a future drain before the target has a turn. Drop and re-hitch (or use
the printed `--resume` form only if a native session was stamped) to recover
that interrupted case. A pre-turn gate normally has no resumable identity.
`gang up` needs a terminal for the tmux client that exposes the prompt; without
one it refuses and leaves the contract queued rather than pretending the prompt
was exposed.
If hitch has no positive prompt evidence, it fails with the manual attach/send
recovery because it cannot truthfully call an unknown stable screen a startup
gate.

- `-c` selects the harness, by the name of the collar that drives it.
- `-d` selects the harness working directory.
- `-m` passes a harness-native model choice through the collar's model option.
- `-e` passes a harness-native reasoning-effort level through the collar's
  effort option, joined with no space, on the plain and `--resume` launch forms
  alike. The collar prints its level vocabulary and a level outside it is
  refused before any window opens. A collar that declares no effort spelling
  refuses the flag, and a vocabulary that cannot be determined is refused as a
  broken declaration rather than a bad value.
- `-r`, `--role` attaches the named role brief to this hitch only. `gang hitch`
  has no role default and never infers one from the agent name. Role names use
  letters, digits, dot, dash, and underscore and may not begin with dot or dash.
- `--resume <session-id>` substitutes that exact native session identity into
  the collar's resume template. Bare `--resume` is only valid when a dead,
  still-existing window registered to the same agent carries `@gl_session_id`.
  A missing id refuses with `gang whoami` and the explicit-id remedy; there is
  no latest/continue fallback. Collars without an explicit-id resume template
  refuse either form.

Names use letters, digits, dot, dash, and underscore, may not begin with dot or
dash, and must be unique in the team. `hitch` is reserved as the startup-envelope
sender.

When context lights are enabled, their thresholds are copied to the new window.
Percentages are relative to each native window and serve mixed-window teams.
Absolute tokens remain supported for an observed fixed window. Place both high
enough not to impose an artificial context disadvantage, but below the harness's
observed automatic-compaction boundary. An absolute red threshold above the
reported native window produces an invalid-light notice rather than staying
silently unreachable.

### `gang adopt <name> -c <harness>`

Registers an existing window in `GANG_SESSION`. `-c` names the harness already
running there, by the name of the collar that drives it. Adoption does not inject
startup text or retroactively add launch-time native hooks. A collar whose
context source requires hitch-time identity may therefore report context
unavailable.

Both hitch and adopt stamp the agent name in `@gl_agent` and the executable
identity in `@gl_binary_id`. Reusing a window whose recorded identity names
another agent refuses. The binary identity is
`cksum:` plus the POSIX checksum and byte size of the invoked `bin/gang`, in a
checkout or an installed tree alike. It therefore witnesses the executable
bytes rather than repository metadata. If the checksum cannot be read, the
stamp is `unavailable`; identity evidence never blocks lifecycle commands.
When the invoked `bin/gang` is tracked in a git checkout but differs from HEAD,
every operator command prints one stderr warning naming the path and HEAD. The
native hook endpoint stays silent except for a crossed light. Restoring the file
to HEAD removes the warning; the live symlink/install path is unchanged.

### `gang attach`

Attaches to `GANG_SESSION`.

### `gang drop <name>`

Prints the window's stamped native session id and the exact
`gang hitch <name> --resume <session-id>` relaunch command, then kills the exact
agent window. If the collar has not supplied a stamp, it says so instead. Its
tmux-owned state and spool die with it after any pending messages are archived.

### `gang down <session>`

Kills the exact team session and every window in it, archiving each window's
pending spool first. The session name is required and must match the team this
shell is pointed at; `down` refuses a name that does not match, and refuses
outright when it is run from a pane inside that session. There is no override:
an agent must not be able to end the team it is running in.

## Delivery and compaction

### `gang send --to <name> [--from <sender>] [--live-only] [--supersede] --stdin`

Reads the full message body from standard input. Inside the team, Gangline derives
the sender from the calling window and refuses `--from`, but only when the pane
carries matching `@gl_agent` and collar registration and no recorded native
session mismatch. An unadopted window name is not an identity. Self-send to the
same pane also refuses. Calls from outside the team must supply `--from`.
Gangline wraps the body in a nonce-bound envelope,
serializes writers per pane, verifies the paste changed the target composer,
submits it, and reports success only after verification.

Gangline refuses a missing or occupied composer, a human draft, tmux copy-mode,
unknown state, and unsafe mid-turn input. Copy-mode is operator-owned: Gangline
does not cancel it, and checks `#{pane_in_mode}` before every paste, submit,
queue-recall, clear, and interrupt key. A collar may declare that its native
harness accepts ordinary mid-turn input, or that a free mid-turn composer
accepts attributed spool entries as native steering.

A refusal on a box that is not provably empty classifies what Gangline read, in
one word with the look or recovery that settles it: `draft` (a human line
Gangline did not write), `staged` (Gangline's own undelivered body, byte-identical
to the rendering it recorded), `unattributed` (Gangline recorded an undelivered
paste here but cannot match it to this box, so the author is exactly what is
unknown), `parked` (the harness's declared queue evidence), `whole-pane` (the
collar declares no input reader, so the reading is the pane rather than a box),
`cleared` (the box read empty when Gangline looked again, so the obstruction
left between the decision and its naming), or `unreadable`. The classifying
look is taken after the decision it names and changes nothing about it; a
refusal stands even when the box clears underneath it. A suggested-prompt placeholder is not among
them: the collar's styled reading strips it, so it never reaches a refusal —
`gang capture` is where it looks like a draft, and `gang composer` is the reading
that settles it.

An expired busy witness alone does not veto delivery: could-not-determine
falls through to direct box evidence, a provably empty composer proceeds
under the full submission verification, and anything less refuses naming both
the expired witness and the box state. No reader writes turn state — not
delivery, not status: the bracket is written only by the native hooks that
own it. A malformed value is reported as unreadable, never repaired, and eligibility
is re-derived from live evidence on every send.

The frozen-paint demotion requires an expired bracket, so a window with no
native hooks and no mid-turn-input declaration whose pane keeps a matching
busy marker stays refused until the marker scrolls off or the agent is
dropped.

An unexpected queue is not delivered: where a collar declares queue evidence,
an ordinary submission the harness parks is reported as a failed delivery
naming the `gang flush` recovery, both before pasting and after Enter. The
exception is a claim that arrived through a collar's explicit `steer` path;
there, native mid-turn queueing is the declared destination. Gangline records
the exact pre-Enter composer even though the attributed spool claim retires, so
status can expose the parked landing and `gang flush` can verify a later recall.
An unreadable verification capture after Enter remains ambiguity and fails
closed.

This detection is scoped to verified harness renderings: the claude-code pin
is the composer hint observed on 2.1.223, and a harness version whose
rendering has not been observed narrows the guarantee back to box-change
verification without refusing sends. A reworded hint is the event that
reopens the decision between stronger session-record verification and a
version gate.

A cleared staged record means the obstruction is gone — it is never
retroactive proof that the recorded body was delivered.

A refused delivery parks by default. The live delivery is attempted first; if
it is refused, the nonce-bound envelope is written to a per-target spool and
reported as parked, not delivered. A `steer` collar then tries the whole queue
under that same pane lock: only a free composer may take its already-attributed
claim. A failure after anything was typed is never parked, because that body's
fate is unknown and a second copy would be a second message. Pass `--live-only`
for an availability probe that must return a refusal to its caller instead of
parking it; without the spool, it cannot use steering.

The target's own native Stop event drains ordinary mail. On a `steer` collar,
PostToolUse also tries the spool while the turn remains open; a free composer
accepts its claim as native steering, while an occupied composer defers without
typing. An open compaction is not a steering composer for peer mail: its native
queue belongs only to its attributed continuation, and PostCompact drains peer
entries after that bracket closes. Hitch itself drains a startup contract
parked before any turn existed once it observes the composer. Every route uses
the same oldest-first verified delivery path. The pane delivery lock is taken
before the first entry is
claimed and held through delivery and claim retirement, so crossed native
workers cannot split or reorder the queue. Copy-mode and other pre-keystroke
refusals leave entries live and unclaimed for the next native opportunity. A
drain that cannot read a composer after an idle native boundary leaves its
entries waiting and records a visible drain failure; it never types through
that uncertainty. A
drain that cannot verify, or one that dies between the submission and the
entry's retirement, leaves that entry held: `status` says its delivery was not
verified and may still have arrived, `status` and `roster` report how many are
held, the bodies
stay readable under `GANG_LOCK_DIR`, and Gangline never sends them again. A
harness may accept a submission into its own queue and drain it later; read the
target before re-sending by hand.

`--supersede` retires the same sender's earlier waiting messages after the newer
message is accepted, whether the newer one parks or is delivered live. It is
scoped to the sender, not to a subject: a sender with two unrelated messages
waiting for one target loses the first when the second carries the flag, whether
that second message parks or is delivered live. Pass it only when the newer
message genuinely replaces everything that sender has parked.

A collar that declares no `GANG_STOP_HOOK` still receives the ordinary live
attempt. If that attempt is refused, Gangline exits with the refusal, says the
message was not parked, and names the missing declaration because nothing else
would drain its spool.

Spool entries live under `GANG_LOCK_DIR`, keyed to an identity minted into the
target's window options at `hitch` or `adopt` — never later, so that senders
arriving together cannot mint competing ones. After a live refusal, a window
without that identity says the message was not parked and names re-hitch or
re-adopt as the repair. When a window dies, `gang drop` and `gang down` move
waiting and held entries under `GANG_ARCHIVE_DIR`, grouped by teardown and
agent, before deleting the spool. Empty queues create no archive directory.

### `gang flush [name]`

Recovers a message the harness parked in its own input queue, as a verified
operation. Gangline presses the collar's declared recall key, reads the loaded
composer back against the body it recorded when it watched the harness park the
message, submits it, and verifies the submission the way any delivery is
verified.

The readback is byte-for-byte against the whole recorded reading, with nothing
normalized away: every normalization discards content that some body means, and
trailing-space trimming is line-oriented in every tool that offers it, so it
cannot tell a line ending in two spaces from one that does not.

It refuses before pressing anything when the collar declares no queue evidence
or no recall key, when the composer shows no parked-queue evidence, and when
Gangline holds no record of the parked body. It refuses after the recall key
when that key loaded nothing, or when the readback is not exactly the recorded
message; the Enter is not pressed in either case.

### `gang interrupt [name] [-m "reason"] [--from <sender>]`

Sends the keystroke the collar declares as its harness's turn-stop key and
drops Gangline's turn bracket, so the interrupted turn neither leaves a bracket
answering busy until its bound expires nor a written one answering idle. State
then comes from the pane: a harness that stopped goes idle, and one that
ignored the key stays busy and unreachable. Whether the harness stops remains
the harness's verdict. A collar that declares no interrupt key refuses the
command, and an occupied composer or tmux mode refuses it too — that keystroke
is often what a native dialog or copy-mode reads as an answer.

With `-m`, Gangline attributes the reason like an ordinary message, holds one
pane lock across the stop key and the boundary it creates, then delivers the
reason before releasing that lock. The reason is never spooled: a queued stop
would arrive after the work it was meant to stop. If the composer does not
return or delivery cannot be verified, the reason is printed back to the caller
in full and reported as not delivered and not parked. Existing backlog remains
untouched for the next native delivery opportunity.

Omitting the name stops the calling window's own turn, with or without a
reason. A reason that returns to its own author is refused on `send` and
intended here: it is written to be read after the turn it ended. Gangline
cannot promise that delivery, because the caller runs inside the turn it is
stopping — a harness that ends that turn by killing the tool call takes the
sending process with it before the reason is typed. Whether it survives is the
same harness verdict the stop itself is.

### `gang compact [<name>] [--resume <turn>]`

Submits the collar's native compaction command through the same verified input
path. An external request refuses a busy or unknown target.

A continuation turn is typed immediately behind the compaction command, enveloped
from the reserved sender `gangline`. A harness that is compacting parks it and
submits it when the compaction ends; a harness that refused the compaction takes
it at once. `--resume` supplies that turn, and a default fires when the agent
supplies none.

When a hooked Codex or claude-code agent requests its own compaction, Gangline
records a one-shot request. Its native Stop hook submits `/compact` after the
active turn releases the composer, then the continuation behind it. `status` and
`roster` expose pending or failed self-compaction. A guarded claude-code launch
that cannot install hooks declares neither Stop support nor deferred
self-compaction.

### `gang notify [<name>|clear]`

Declares, shows, or clears the optional agent that receives stall notes. The
target is stored as the session option `@gl_notify`; it need not exist when
declared, and no target is inferred. `gang notify clear` removes the declaration
sooner, and ending the tmux session deletes it with the rest of the team state.

When a collar's native hook witnesses an awaiting-input event, Gangline sends
one ordinary attributed message:
`stall: <agent> is awaiting input (<kind>) — inspect with gang capture <agent>`.
Repeated events of the same kind within 600 seconds are one note. A native
prompt, tool, or Stop event clears that debounce because it proves movement; an
older stamp permits a fresh note without any patrol or timer. A delivery failure
is recorded on the raising window for `status` and `roster`, and is retired only
after a later note is accepted live or parked.

### `gang curfew [<duration|HH:MM>|clear]`

Declares, shows, or clears one optional wall-clock curfew for the team. Durations
carry their units, such as `90m`, `2h`, or `1h30m`; a bare number is refused.
`HH:MM` means the next occurrence of that local 24-hour clock time.

The declaration creates two relative advisory edges: yellow halfway through its
span and red after four-fifths. Native prompt and tool hooks expose each edge once
to hook-enabled agents. There is no default, per-agent budget, patrol, or automatic
action. Declaring again replaces and restarts the span; `clear` removes it.

## Observation

Gang-managed tmux windows wrap the bare agent name in the glyph of the state
Gangline last witnessed: `-name-`, `~name~`, `!name!`, or `?name?`. This is an
at-a-glance hint and can be stale between existing observation points and native
hook events; `gang roster` remains the live-computed truth. Addressing always
uses the bare name, so `gang send --to pii-impl` never changes. tmux appends its
own flags after the name: a last-active busy window renders as `3:-name--`, where
the trailing pair is tmux's flag rather than part of the agent name. Gangline
does not set the operator's tmux status formats.

### `gang mail [name]`

Prints every message waiting in that agent's spool, oldest first, then every
held entry, each with its sender and its entry filename, each body exactly as it
would go onto the wire. Another agent's or the operator's read is inspection and
touches nothing. The addressee's own read is delivery: it consumes each waiting
entry so a later native delivery opportunity cannot deliver the same message again. Before
printing an entry, it moves it into a human-readable directory under
`GANG_ARCHIVE_DIR`; the path and its explicit deletion command go to stderr, so
an ordinary stdout filter cannot destroy the only copy or hide its recovery
location. Held entries are never consumed. Mail takes no delivery lock and needs
no loadable collar, because a queue is files on disk and the harness may be the
reason you are reading it.

Read archives are durable and have no automatic expiry. Their stderr notice
names the read-scoped root and the exact command that deletes it. Delete that
root after its recovery or audit purpose ends; see
[Operations](operations.md#sending-messages-safely) for retention.

### `gang wait <name> --until idle|done [--timeout <seconds>]`

Blocks the caller on a native target boundary without polling `status`.
An already-idle target returns immediately for either condition. Otherwise the
target collar must declare `GANG_STOP_HOOK=1`, and the window must already carry
readable native turn evidence rather than relying on that declaration alone.
Each caller owns one temporary, uniquely named tmux `wait-for` channel and a
sparse caller-owned hook key. The target's next native `Stop` signals it after
closing the turn bracket. `idle` then re-reads live state and arms another
boundary while positive busy or occupied evidence remains; `done` returns after
that first Stop.

`done` does not track turns. If the target is already working, that active
turn's completion may match. A wait is an explicit barrier chosen by its caller,
not a supervisor: Gangline starts no daemon and records no durable state. Each
boundary has a foreground fail-loud deadline. `--timeout` chooses its positive
whole number of seconds; without the flag, `GANG_TURN_LIMIT` supplies the bound.

The registration pins both the target's window id and active pane id. A missing,
replaced, ambiguous, or pane-switched target fails loudly rather than transferring
the wait. `?unknown?` fails instead of hanging. Natural pane-process exit plus
Gangline's `drop` and `down` release temporary registrations, so those teardown
paths are observed and reported as a vanished target. Direct `tmux kill-pane`
and `tmux kill-window` do not emit the supported exit hook on tmux 3.2a and
bypass Gangline's compensating release. The foreground deadline then refuses
loudly and removes the temporary hook; use `gang drop` or `gang down` to observe
teardown immediately.

The implementation uses only `wait-for -S`; tmux channel locks are not used
because a dead lock holder can leave them locked indefinitely. A successful
wait consumes its native signal exactly once; cleanup never signals and waits
on the channel again, because that can race the returning waiter and block
forever. Tmux has no channel-delete operation, so a caller killed or bounded at
the same instant as a native signal can leave an unreachable nonce-named latch
in tmux memory until that server exits. No wait option, file, or daemon is
created; the deadline process lives only as long as the foreground command.

### `gang explain <name>`

Runs the ordinary live state classification once and prints the agent, pinned
active pane, collar, and resulting state. It then reports both collar-owned
state rules, `GANG_OCCUPIED_REGEX` and `GANG_BUSY_REGEX`, as matched, did not
match, not declared, or not evaluated because higher-priority evidence settled
that part of the classification first. A matched rule includes its exact ERE and
the first pane line that matched.

The diagnostic instruments the regex evaluations inside that same state read.
It does not recapture the pane afterward, so a moving TUI cannot make the
explanation describe a screen different from the one classified. Regex capture
targets the pinned pane rather than following a later active-pane selection.
The command writes no diagnostic option or file; ordinary state observation may
still refresh the window glyph and the transient evidence that `status` itself
maintains.

### `gang status [name]`

Prints one current state:

- `-busy-` — a native event, terminal activity, or collar marker
  witnesses active work;
- `~idle~` — the evidence positively witnesses readiness;
- `!occupied! (authority unknown)` — a native UI owns the composer;
- `?unknown? (...)` — the available evidence can no longer determine the answer.

A turn bracket left open by an interruption the harness never reported decays
once it passes `GANG_TURN_LIMIT`: an expired bracket over a quiet, stable pane
whose input box is on screen and provably empty reads `~idle~`, because that is
the same positive evidence idle means everywhere else. A drafted box, a frozen
busy marker, or a pty whose quietness cannot be measured keeps it `?unknown?`, and
an unreadable or future-stamped bracket is unknown rather than abandoned. Within
its bound the bracket still outranks the tiers beneath it, and a provably empty
box already accepted delivery while the state read `?unknown?`.

A decay refused because the pane was being written to during the decision reads
`?unknown? (the pane was written to while gang was deciding)`, and that reason
refuses delivery rather than falling through to the empty-box check. A harness
can paint the opening of a turn with its composer still empty, so an empty box
read out of a moving screen does not witness an idle target.

An undelivered-input report is followed by a `box:` line classifying what is in
that box now, in the same vocabulary a refusal uses. The record says what
Gangline did; the `box:` line says what is there at reading time.

It also reports staged input and the current box reading, pending or failed
self-compaction, the number of messages spooled for that target and how long
the oldest has waited, held-message
details and their directory, a spool drain that could not be verified, a stall
note that could not be accepted, native-session identity mismatch, and binary
skew when the window has no hitch/adopt stamp or its executable-byte witness
differs from the invoked `gang` binary. An unavailable witness is reported
explicitly instead of treated as either match or mismatch.

### `gang whoami`

From inside an agent pane, prints the Gangline agent name, pane id, collar,
stamped harness session id, latest live hook-payload id, team session, and any
recorded identity mismatch. A native hook stamps `@gl_session_id` on first
sighting and compares every later live id against it. A mismatch is visible in
status and roster and blocks sends from that pane until a matching hook repairs
the record.

### `gang context [name]`

Prints the target collar's native context reading raw, in its own
`usedk/windowk (percent%)` format. The query reads the collar source whether or
not context lights are enabled; asking and edge-triggered signalling are
separate acts. A missing or unreadable source fails loudly and no value is
fabricated.

Codex context becomes available when a native hook payload binds its
`transcript_path`; this works equally for hitched and adopted windows once a
hook has fired.
opencode and Pi read their panes and can answer with lights off when their native
readout is visible. claude-code can answer only when lights were enabled at
hitch, because that launch choice installs its statusline beacon. Adopted
windows answer only when their required native source is independently present.

### `gang limits [name]`

Reads the non-interactive provider-limit source declared by the target's collar.
It prints one native window per line with percent used, the reset in local time,
and the age of the native sample. The command may target an agent mid-turn
because it never drives that agent's composer.

Claude Code's collar runs the harness's headless usage command behind a
fail-loud process bound, so a wedged reader makes the source unavailable rather
than holding a native hook indefinitely. Codex's collar
reads the exact target rollout stamped by its native hook and selects the newest
rate-limit event. That event's timestamp is the sample clock, so an idle Codex
reading truthfully grows stale until its next API turn. It remains visible to
this reporting command even after it is too old to drive a warning or wake. A missing source, failed
reader, malformed row, missing reset, or missing observation time fails loudly.

A successful read records the most constrained native window on the target's
tmux window. `status` and `roster` report that ephemeral evidence without
sampling again; it dies with the agent window.

### `gang wait-limit [name] [--resume <turn>]` / `gang wait-limit [name] --clear`

Reads the same non-interactive provider limits as `gang limits`, chooses the
highest percentage used (later reset on a tie), and schedules one transient
systemd user timer for that reset. The optional turn is delivered after reset;
otherwise the continuation only reports that the window reset and asks the
agent to re-read its assignment.

A collar may declare a maximum actionable age for warning evidence. Codex
marks an event more than five minutes old stale until its next API turn, but a
future absolute reset remains sufficient to arm a wake without spending quota
on a refresh. Claude's headless read is fresh on demand.

The timer invokes `gang wait-limit <name> --fire <reset> --unit <unit>` with the
team's pinned session and configuration environment. Firing uses ordinary
attributed, verified delivery, including the ordinary spool when a target is
temporarily busy. A firing does nothing unless the agent still exists and its
stored declaration names the same reset and the same unit. The unit name is
derived from the session, the window id and the reset, so that comparison
refuses a timer that outlived its tmux server whenever any of the three
differs — it does not refuse one whose team was recreated holding all three.
The declaration is stored before `systemd-run` arms the timer, after the
transient unit derived from that name is stopped and proven gone. An older
callback therefore cannot consume the new declaration, while a reset arriving
during arming finds and delivers it. A failed declaration write leaves nothing
armed; a failed timer arm clears both declaration options and refuses.

The unit is collected after it runs. `--clear` stops the pending timer and the
service that timer may already have started, and it removes the declaration
whether or not those stops succeeded: a unit that already fired and was
collected is not loaded, and `systemctl stop` reports that as an error. A unit
still active after the stop is reported as an error naming the exact
`systemctl --user stop` to run, with the declaration already cleared. Drop and
down attempt the same cleanup. There is no watcher process.

The pending reset, transient unit, and optional continuation live as tmux window
options and die with the window. Status and roster expose pending, overdue,
unreadable, and failed wake states. A user manager or timer creation failure is
an error and leaves no wake declaration.

`status` reads the declared unit as well as the declaration whenever the reset
is still in the future. The declaration lives on the tmux window and the timer
lives in the user manager, so a manager restarted under a surviving tmux server
leaves a declaration promising a wake nothing will deliver. A unit that reports
inactive or failed is named as armed nowhere; a state that cannot be read at all
is reported unverified rather than assumed. A reset that has already passed is
not checked, because a fired unit is expected to be gone.

### `GANG_AUTO_RESUME`

An operator-declared provider-used percentage arms the wake above without anyone
typing `wait-limit`. It is off unless declared, is stamped on the window at
hitch or adopt time like the usage lights, and is a separate declaration from
them: warning an agent and arming its return are different decisions at
different percentages, and an agent may run auto-resume with the lights off.

At the first sample at or above the threshold whose reset is still in the
future, the agent's own turn hook passes that validated sample through the same
arming transaction as `gang wait-limit` and reports the result in the same
turn. It takes no second native reading while arming. Arming happens once per provider window:
the reset that was decided for is recorded on the window, so later samples in
the same window arm nothing, and a wake cleared with `--clear` is not armed
again over the operator. A refused arm is reported in the turn, recorded where
status reads it, and not retried for that window.

A reading too old to act on arms nothing. A collar may cap the age of
actionable evidence, and Codex's percentage comes from a session event that goes
stale while the agent is idle. A stale percentage is poor evidence for a
threshold decision even though a still-future absolute reset remains sufficient
to arm against once that decision has been made, so the sample is refused and
the agent is told, rather than arming from a number that may no longer hold.

On claude-code this declaration also opts into one-hop recovery from a provider
stream failure. `idle_prompt` supplies the native idle witness and transcript
path; the collar selects the newest non-sidechain assistant record and returns
its UUID only when `error` is nonempty and `isApiErrorMessage` is true. Gangline
then closes the missing turn bracket and submits one continuation for that UUID.
The exact attributed envelope is recorded before submission and must match the
next native prompt byte-for-byte. If that owned continuation fails, or ownership
cannot be established, no next continuation is sent; `status` reports
`automatic resume refused` and roster reports `auto-resume-failed`. A later
ordinary native prompt clears the one-hop episode. Collars without both native
readers do not attempt stream-failure recovery.

This is deliberately an over-approximation of the provider limit. Gangline
cannot observe the harness's own refusal — the only in-band evidence is pane
prose, which is not a data contract — and an agent that has been refused takes
no further turns, so the last moment Gangline can arm from the agent's own hook
is before the cap rather than at it. A continuation may therefore arrive at a
reset for an agent that never capped; it carries the ordinary continuation turn
and lands on the ordinary spool. If a provider window goes from below the
threshold to exhausted between two samples, nothing is armed, and status shows
no pending wake; the threshold is the operator's lever, as it is for the lights.

### `gang roster`

Prints every session window with its collar and current state. Unadopted windows
are shown but not treated as agents. Context is deliberately not a roster
column: ordinary adopted windows and lights-off claude-code windows have no
readable source, and one absent value must not make the team inventory fail.
Use `gang context <name>` when a reading is wanted. Each row compares the
window's binary stamp with the invoked `gang` binary and visibly marks skew; the
comparison runs only for this snapshot. A non-empty queue reports both its depth
and the age of its oldest waiting entry.

`gang roster --porcelain` is the scripting interface. It prints one unpadded,
uncoloured TSV row per window with these columns in order: `name`, `collar`,
`state`, `spooled`, `oldest_age_s`, and `session_id`. State is one lowercase
word: `busy`, `idle`, `occupied`, or `unknown` for the four human glyph states;
unadopted windows and missing collars read `unadopted` and `collar-missing`.
`spooled` is an integer. `oldest_age_s` is integer seconds or `-` for an empty
queue or an unreadable age. `session_id` is the exact stamp or `UNSTAMPED`.
With no running session it prints no rows and exits successfully, like the human
roster.

### `gang capture [name] [lines]`

Prints the tail of the target pane after trimming trailing blank terminal rows.

### `gang composer [name]`

Prints what a human actually typed into the agent's input box, via the
collar's styled reading. `capture` shows the raw pane, where a harness's dim
suggested-prompt placeholder is indistinguishable from a real draft; `composer`
strips styling, so placeholder text vanishes. Empty output means an empty box —
a whitespace-only reading (prompt padding) counts as empty, the same rule
delivery uses. Fails loudly when no input box is on screen or the collar
declares no `collar_input`.

## Discovery and hooks

### `gang collars`

Lists shipped and custom harness collars. The Bash substrate fixture is hidden
unless `GANG_TEST_COLLARS=1`.

### `gang roles`

Lists every discovered Markdown role file as three tab-separated columns:
name, origin, and status. Operator roles show their terminal-safe path; shipped
roles use the stable origin `shipped`. A winning file is `ok` only when it is a
nonempty usable role at that instant. Invalid names and unusable files remain
visible with their specific defect, and only `ok` names appear in an unknown
role refusal's accepted-value list.

### `gang config`

Prints every effective operator setting with its origin: built-in default,
config file and line, or environment, including when the environment overrides
a file line. It also reports whether the doctrine file and operator roles
directory are present, with the terminal-safe path to each slot. Dynamic text
is terminal-safe: control bytes are rendered visibly rather than written raw.
The command takes no arguments and needs no tmux server.

### `gang hook`

Internal endpoint for native harness events. It reads one JSON payload from
standard input and takes no arguments. An invocation carrying one is an
invocation Gangline cannot read: the argument is named on stderr and recorded on
the window for `status` and `roster`, and the event is not acted on. It is the
one command that does not die on unconsumed arity, because a hook must not be
fatal to the harness that fired it; declining the event is the refusal.
Prompt/tool events open the turn fact, Stop closes it and may
dispatch deferred self-compaction and a spool drain, and permission requests
raise occupancy. PreCompact opens the compaction bracket and PostCompact closes
it and drains. A harness that refuses a compaction raises the opening event and
never the closing one, so any turn event also settles a bracket left open.
Permission occupancy has no timed decay: a readable composer clears it, and a
malformed witness refuses rather than becoming ordinary absence.
Awaiting-input events listed by `GANG_STALL_TYPES`, plus permission requests,
raise a stall note only when `gang notify` has declared a target.
Hooks are silent unless an enabled context light or declared team-time light
crosses an edge. Context-source warm-up is silent until the first native turn
completes; an unreadable source after that boundary fails visibly.

## Configuration

The precedence is environment variable, then config file, then built-in default.
An environment variable that is set but empty still outranks the file. Values
read from the file are shell variables, not exported into harnesses or helper
commands.

`GANG_CONFIG_DIR` is environment-only and defaults to
`${XDG_CONFIG_HOME:-$HOME/.config}/gangline`. The resolved directory must be
absolute. It contains:

- `config`, the optional settings file;
- `DOCTRINE.md`, optional operator prose delivered in every hitch contract;
- `CONTRACT.md`, an optional operator contract that replaces the shipped one;
- `roles/`, optional operator role briefs that replace shipped roles
  file-for-file by name.

The settings file is parsed, never sourced. Blank lines and lines whose first
non-blank character is `#` are ignored. Every other line is `NAME=VALUE`: leading
blanks are ignored, the first `=` separates the exact key, and trailing spaces
and tabs are removed from the literal value. Quotes, `#`, backticks, `$`, and
further `=` characters have no special meaning. Empty values, duplicate or
unknown keys, blanks in a key, NUL bytes, and control characters other than tab
and newline are fatal. A missing final newline is accepted.

Exactly these keys are settable:

| Key | Built-in default | Meaning |
|---|---|---|
| `GANG_COLLAR` | `claude-code` | default collar for `up` and `hitch` |
| `GANG_SESSION` | `gangline` | exact tmux session Gangline addresses |
| `GANG_COLLARS` | unset | custom collar directory searched before shipped collars |
| `GANG_LOCK_DIR` | `/tmp/gangline-$(id -u)` | shared delivery locks and per-target spools |
| `GANG_ARCHIVE_DIR` | `${XDG_STATE_HOME:-$HOME/.local/state}/gangline/archive` | pending-message archive written before windows die |
| `GANG_CONTEXT_LIGHTS` | `off` | `off`, `yellow,red` token thresholds, or `yellow%,red%` relative thresholds |
| `GANG_USAGE_LIGHTS` | `off` | `off` or increasing provider-used thresholds such as `90%,95%` |
| `GANG_AUTO_RESUME` | `off` | `off` or one provider-used percentage such as `97%` at which a reset wake is armed automatically |
| `GANG_BOOT_TIMEOUT` | `30` | initial startup readiness bound; after a positively identified gate, one foreground observation slice in seconds |
| `GANG_CHURN_WAIT` | `0.5` | stable-pane observation interval |
| `GANG_ACTIVITY_WINDOW` | `5` | recent terminal-activity window |
| `GANG_TURN_LIMIT` | `300` | native turn-fact bound and default `gang wait` boundary timeout |

Collar declarations are refused because `load_collar` clears them before
sourcing the selected collar; put those values in a custom collar and point
`GANG_COLLARS` at it. `GANG_TEST_COLLARS` is suite-only,
`GANG_CONFIG_DIR` cannot set the file that is already being read, and internal
variables are refused. Any malformed file refuses every command, including
native hooks; recovery is in `docs/operations.md`.

Doctrine is never written by Gangline. It must be a readable regular file with
no NUL, no controls other than tab and newline, and valid UTF-8. Byte count does
not predict deliverability: a doctrine the target cannot accept fails loudly at
hitch through the verified paste, while system-prompt prose meets the harness,
operating-system, and model-context boundaries that actually consume it. Delete
`DOCTRINE.md` to remove the slot; a hitched copy dies with its window.

The contract uses the same prose validation and the same operator-first
resolution: `$GANG_CONFIG_DIR/CONTRACT.md` before the shipped `CONTRACT.md`. It
is the one prose slot that is not optional — a root without one refuses every
hitch, naming both paths. Its bytes are never pasted: they ride the system
prompt where the collar declares that option, and where it does not, only the
path is sent. A role brief joins them in that prompt on the same collars.
Nothing downstream reads those bytes back, so Gangline applies only the prose
shape checks it can prove before launch. Their consumers apply the real size
bounds; Gangline does not substitute an unrelated byte threshold for those
interfaces.

A hitch may be role-less, but the contract is always present, so a role-less
hitch still carries one into the system prompt where the collar has that option.
The contract and the role brief share a single option rather than passing it
twice: a collar declares one spelling, and repeating it would be a guess about
whether that harness concatenates the values or keeps the last. The composite
is Gangline's preamble, then the contract, harness-specific guidance where the
collar declares it, and the role brief where one was named.

Role briefs use the same prose validation. Resolution checks
`$GANG_CONFIG_DIR/roles/<name>.md` before the shipped `roles/<name>.md`; an
operator file replaces the shipped file whole, and an unusable override refuses
instead of falling back. A named role must be nonempty. Where the selected
collar declares `GANG_ROLE_PROMPT_OPT`, Gangline passes the validated bytes by
value through that launch option, after the contract and under the same
preamble, and puts only an attribution pointer in the startup contract.
Otherwise it puts the body in the startup contract before operator doctrine.
The system-prompt form includes Gangline's own preamble stating that later
operator doctrine governs any disagreement.

Every process addressing one team must agree on `GANG_SESSION`, `GANG_COLLARS`,
`GANG_LOCK_DIR`, and the resolved `GANG_CONFIG_DIR`.

### Removed names

The 1.0 rename kept the pre-rename spellings working through 1.x. 2.0 removes
them: `gang profiles`, `gang cutoff`, `-p`/`--profile`, `send --spool`,
`GANG_PROFILE`, `GANG_PROFILES`, `GANG_TEST_PROFILES`, the `profile_*` collar
contract functions, the undocumented `spawn` alias for `hitch`, and in-place
migration of the `@gl_profile` window option and the `@gl_cutoff` session
option. A config file naming a removed key is refused as an unknown key; a
removed flag or command name is refused as unknown.

## Collar contract

A collar is a Bash file sourced by `bin/gang`. Harness-specific behavior belongs
there, never in a harness-name branch in the core script.

| Declaration | Purpose |
|---|---|
| `GANG_LAUNCH` | required native launch command |
| `GANG_RESUME_LAUNCH` | optional explicit native resume template containing exactly one `{{session_id}}` slot |
| `GANG_MODEL_OPT` | optional native model flag |
| `GANG_EFFORT_OPT` | optional native effort option, declared whole with its separator; the level joins with no space |
| `GANG_EFFORT_CMD` | prints the effort vocabulary, one level per line, given `GANG_MODEL`; empty output means could-not-determine |
| `GANG_ROLE_PROMPT_OPT` | optional native option whose next argument is a system-prompt addition passed by value |
| `GANG_HARNESS_PROMPT` | optional harness-specific prose included in that system-prompt addition; requires `GANG_ROLE_PROMPT_OPT` |
| `GANG_BUSY_REGEX` | pane evidence of an active turn |
| `GANG_OCCUPIED_REGEX` | pane evidence that a native UI owns input |
| `GANG_QUEUED_REGEX` | input-box evidence that the harness parked input in a native queue instead of submitting |
| `GANG_QUEUE_RECALL_KEY` | tmux key name that loads the parked message back into the composer, used by `flush` |
| `GANG_INTERRUPT_KEY` | tmux key name that stops an active turn, used by `interrupt` |
| `GANG_STOP_HOOK=1` | the launch command installs a native Stop hook reaching `gang hook`, so this harness can drain a spool |
| `GANG_STALL_TYPES` | space-separated native `Notification` kinds that mean the harness is awaiting a person |
| `GANG_QUIET_AT_REST=1` | harness terminal becomes quiet when idle |
| `GANG_MIDTURN_INPUT=1` | ordinary text may safely enter during a turn |
| `GANG_MIDTURN_INPUT=park` | declares that mid-turn composer input can only stage or queue; it never authorizes a composer keystroke |
| `GANG_MIDTURN_INPUT=steer` | commit to the attributed spool first, then allow a free composer to accept its claim as native mid-turn steering; PostToolUse supplies later opportunities |
| `GANG_COMPACT_CMD` | native compaction command |
| `GANG_SELF_COMPACT=deferred` | self-compaction must wait for Stop |
| `collar_usage_limits target` | print `label<TAB>percent-used<TAB>reset-epoch<TAB>observed-epoch` rows from a non-interactive native source; absence declares provider-limit awareness unavailable |
| `GANG_USAGE_LIGHT_INTERVAL` | minimum seconds between hook-driven native usage reads; zero disables reuse, while explicit commands remain fresh |
| `GANG_USAGE_LIMIT_MAX_AGE` | maximum seconds a native sample may drive a light; zero accepts any age before its reset |
| `collar_input target` | print human-authored composer contents, or fail if absent |
| `collar_context target` | print `usedk/windowk (percent%)`, or fail loudly |
| `collar_session_id target payload` | print the exact native session id witnessed by a hook, or fail without fabricating one |
| `collar_auto_resume_record target notification-kind` | optional native failed-turn discriminator; print one stable error-record identity, return 1 for an ordinary idle turn, or return 2 when the native record cannot be read |
| `collar_auto_resume_prompt target payload` | print the exact native prompt from a prompt-submission event so Gangline can prove whether its marked continuation owns that turn |

Collars may install native event hooks by composing them into their launch
command. They must not weaken sandboxing, approvals, or operator permissions.

Gangline does not answer native dialogs. A screen matching
`GANG_OCCUPIED_REGEX` is occupied whoever drew it: `status` and `roster` report
`!occupied! (authority unknown)`, delivery refuses and parks, and `hitch` keeps
waiting and names the prompt for the operator. Answering one is `gang attach`.
