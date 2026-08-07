# Command and environment reference

`gang --help` is the authoritative command synopsis.

## Lifecycle

### `gang up [name] [hitch flags]`

Hitches one agent, named `lead` when omitted, then attaches or switches the
current tmux client to it. `GANG_PROFILE` selects the harness.

### `gang hitch <name> [-p profile] [-d dir] [-m model] [-e effort] [--resume]`

Starts a native harness in a named tmux window and delivers one startup contract.
The launch environment carries the exact `GANG_SESSION` plus any custom profile
and lock paths, so harness commands cannot drift to another session on the same
tmux server.

If a first-run prompt owns the screen before the composer appears, `hitch`
directs the operator to `gang attach` once the profile's occupied pattern
provides positive pane evidence, then keeps waiting within the original
`GANG_BOOT_TIMEOUT`. Clearing the prompt lets that same hitch report the
recovered input box and deliver its startup contract. If the bound expires
first, the window is left for inspection and the error gives the attach, drop,
and re-hitch recovery.

- `-p` selects a profile.
- `-d` selects the harness working directory.
- `-m` passes a harness-native model choice through the profile's model option.
- `-e` passes a harness-native reasoning-effort level through the profile's
  effort option, joined with no space, on the plain and `--resume` launch forms
  alike. The profile prints its level vocabulary and a level outside it is
  refused before any window opens. A profile that declares no effort spelling
  refuses the flag, and a vocabulary that cannot be determined is refused as a
  broken declaration rather than a bad value.
- `--resume` uses the profile's directory-scoped native resume command. Profiles
  without a safe resume command refuse it.

Names use letters, digits, dot, dash, and underscore, may not begin with dot or
dash, and must be unique in the team. `hitch` is reserved as the startup-envelope
sender.

When context lights are enabled, their thresholds are copied to the new window.
Codex also binds that window to the useful startup envelope's nonce so later
token events can be found without a marker turn.
Operators normally place both thresholds high in the native window so lights do
not impose an artificial context disadvantage, but below the harness's observed
automatic-compaction boundary so the lights remain reachable.

### `gang adopt <name> -p <profile>`

Registers an existing window in `GANG_SESSION`. Adoption does not inject startup
text or retroactively add launch-time native hooks. A profile whose context
source requires hitch-time identity may therefore report context unavailable.

Both hitch and adopt stamp `@gl_binary_id` on the window. The identity is
`cksum:` plus the POSIX checksum and byte size of the invoked `bin/gang`, in a
checkout or an installed tree alike. It therefore witnesses the executable
bytes rather than repository metadata. If the checksum cannot be read, the
stamp is `unavailable`; identity evidence never blocks lifecycle commands.

### `gang attach`

Attaches to `GANG_SESSION`.

### `gang drop <name>`

Kills the exact agent window. Its tmux-owned state and its spool die with it.

### `gang down`

Kills the exact team session and every window in it, deleting each window's
spool first.

## Delivery and compaction

### `gang send --to <name> [--from <sender>] [--spool [--supersede]] --stdin`

Reads the full message body from standard input. Inside the team, Gangline derives
the sender from the calling window and refuses `--from`. Calls from outside the
team must supply `--from`. Gangline wraps the body in a nonce-bound envelope,
serializes writers per pane, verifies the paste changed the target composer,
submits it, and reports success only after verification.

Gangline refuses a missing or occupied composer, a human draft, indeterminate
state, and unsafe mid-turn input. A profile may declare that its native harness
accepts ordinary mid-turn input.

A refusal on a box that is not provably empty classifies what Gangline read, in
one word with the look or recovery that settles it: `draft` (a human line
Gangline did not write), `staged` (Gangline's own undelivered body, byte-identical
to the rendering it recorded), `unattributed` (Gangline recorded an undelivered
paste here but cannot match it to this box, so the author is exactly what is
unknown), `parked` (the harness's declared queue evidence), `whole-pane` (the
profile declares no input reader, so the reading is the pane rather than a box),
or `unreadable`. A suggested-prompt placeholder is not among
them: the profile's styled reading strips it, so it never reaches a refusal —
`gang capture` is where it looks like a draft, and `gang composer` is the reading
that settles it.

An expired busy witness alone does not veto delivery: could-not-determine
falls through to direct box evidence, a provably empty composer proceeds
under the full submission verification, and anything less refuses naming both
the expired witness and the box state. No reader writes turn state — not
delivery, not status: the bracket is written only by the native hooks that
own it, because tmux offers no atomic compare-and-delete and a reader's
unset can erase a fresh hook stamp landing between the read and the unset.
A malformed value is reported as unreadable, never repaired, and eligibility
is re-derived from live evidence on every send.

The frozen-paint demotion requires an expired bracket, so a window with no
native hooks and no mid-turn-input declaration whose pane keeps a matching
busy marker stays refused until the marker scrolls off or the agent is
dropped. That refusal is fail-closed and deliberate.

Queued is not delivered: where a profile declares queue evidence, a harness
that parks the submission in its own input queue is reported as a failed
delivery naming the `gang flush` recovery, both before pasting and after the Enter —
never as a success. An unreadable verification capture after the Enter is
ambiguity and fails the same way.

This detection is scoped to verified harness renderings: the claude-code pin
is the composer hint observed on 2.1.223, and a harness version whose
rendering has not been observed narrows the guarantee back to box-change
verification without refusing sends. A reworded hint is the event that
reopens the decision between stronger session-record verification and a
version gate.

A cleared staged record means the obstruction is gone — it is never
retroactive proof that the recorded body was delivered.

`--spool` changes what a refusal does, never what a delivery proves. The live
delivery is attempted first; if it is refused, the nonce-bound envelope is
written to a per-target spool and reported as parked, not delivered. A failure
after anything was typed is never spooled, because that body's fate is unknown
and a second copy would be a second message.

The target's own native Stop event drains its spool, oldest first, through the
verified delivery path. Each entry is claimed before it is delivered, so no
later drain can send a body this one may already have landed. A refused drain
returns its entry to the spool for the next turn boundary. A drain that cannot
verify, or one that dies between the submission and the entry's retirement,
leaves that entry held: `status` and `roster` report how many are held, the
bodies stay readable under `GANG_LOCK_DIR`, and Gangline never sends them again.
`--supersede` drops the same sender's earlier spooled messages before parking
this one, and applies to that sender only.

A profile that declares no `GANG_STOP_HOOK` refuses `--spool`: nothing else
drains a spool.

Spool entries live under `GANG_LOCK_DIR`, keyed to an identity minted into the
target's window options at `hitch` or `adopt` — never later, so that senders
arriving together cannot mint competing ones. A window without that identity
refuses `--spool`. Entries die with the window: `gang drop` and `gang down`
delete them.

### `gang flush <name>`

Recovers a message the harness parked in its own input queue, as a verified
operation. Gangline presses the profile's declared recall key, reads the loaded
composer back against the body it recorded when it watched the harness park the
message, submits it, and verifies the submission the way any delivery is
verified.

It refuses before pressing anything when the profile declares no queue evidence
or no recall key, when the composer shows no parked-queue evidence, and when
Gangline holds no record of the parked body. It refuses after the recall key
when that key loaded nothing, or when the readback is not the recorded message;
the Enter is not pressed in either case.

### `gang interrupt <name>`

Sends the keystroke the profile declares as its harness's turn-stop key and
drops Gangline's turn bracket, so the interrupted turn neither leaves a bracket
answering busy until its bound expires nor a written one answering idle. State
then comes from the pane: a harness that stopped goes idle, and one that
ignored the key stays busy and unreachable. Whether the harness stops remains
the harness's verdict. A profile that declares no interrupt key refuses the
command, and an occupied composer refuses it too — that keystroke is often what
a native dialog reads as an answer.

### `gang compact <name>`

Submits the profile's native compaction command through the same verified input
path. An external request refuses a busy or indeterminate target.

When a Codex agent requests its own compaction, Gangline records a one-shot
request. Its native Stop hook submits `/compact` after the active turn releases
the composer. `status` and `roster` expose pending or failed self-compaction.

### `gang cutoff [<duration|HH:MM>|clear]`

Declares, shows, or clears one optional wall-clock cutoff for the team. Durations
carry their units, such as `90m`, `2h`, or `1h30m`; a bare number is refused.
`HH:MM` means the next occurrence of that local 24-hour clock time.

The declaration creates two relative advisory edges: yellow halfway through its
span and red after four-fifths. Native prompt and tool hooks expose each edge once
to hook-enabled agents. There is no default, per-agent budget, patrol, or automatic
action. Declaring again replaces and restarts the span; `clear` removes it.

## Observation

### `gang status <name>`

Prints one current state:

- `busy (tight tug)` — a native event, terminal activity, or profile marker
  witnesses active work;
- `idle (slack tug)` — the evidence positively witnesses readiness;
- `occupied (authority unknown)` — a native UI owns the composer;
- `expired (...)` — the available evidence can no longer determine the answer.

A turn bracket left open by an interruption the harness never reported decays
once it passes `GANG_TURN_LIMIT`: an expired bracket over a quiet, stable pane
whose input box is on screen and provably empty reads `idle`, because that is
the same positive evidence idle means everywhere else. A drafted box, a frozen
busy marker, or a pty whose quietness cannot be measured keeps it `expired`, and
an unreadable or future-stamped bracket is unknown rather than abandoned. This
narrows no delivery guard: within its bound the bracket still outranks the tiers
beneath it, and a provably empty box already accepted delivery while the state
read `expired`.

An undelivered-input report is followed by a `box:` line classifying what is in
that box now, in the same vocabulary a refusal uses. The record says what
Gangline did; the `box:` line says what is there at reading time.

It also reports staged input, pending or failed self-compaction, and binary skew
when the window has no hitch/adopt stamp or its executable-byte witness differs
from the invoked `gang` binary. An unavailable witness is reported explicitly
instead of treated as either match or mismatch.
It also reports staged input, pending or failed self-compaction, how many
messages are spooled for that target, and a spool drain that could not be
verified.

### `gang roster`

Prints every session window with its profile and current state. Unadopted windows
are shown but not treated as agents. Context usage belongs to each agent and is
not reported to the lead or operator. Each row compares the window's binary
stamp with the invoked `gang` binary and visibly marks skew; the comparison runs
only for this snapshot.

### `gang capture <name> [lines]`

Prints the tail of the target pane after trimming trailing blank terminal rows.

### `gang composer <name>`

Prints what a human actually typed into the agent's input box, via the
profile's styled reading. `capture` shows the raw pane, where a harness's dim
suggested-prompt placeholder is indistinguishable from a real draft; `composer`
strips styling, so placeholder text vanishes. Empty output means an empty box —
a whitespace-only reading (prompt padding) counts as empty, the same rule
delivery uses. Fails loudly when no input box is on screen or the profile
declares no `profile_input`.

## Discovery and hooks

### `gang profiles`

Lists shipped and custom harness profiles. The Bash substrate fixture is hidden
unless `GANG_TEST_PROFILES=1`.

### `gang hook`

Internal endpoint for native harness events. It reads one JSON payload from
standard input. Prompt/tool events open the turn fact, Stop closes it and may
dispatch deferred self-compaction and a spool drain, and permission requests
raise occupancy.
Hooks are silent unless an enabled context light or declared team-time light
crosses an edge. Context-source warm-up is silent until the first native turn
completes; an unreadable source after that boundary fails visibly.

## Environment

| Variable | Meaning |
|---|---|
| `GANG_PROFILE` | default profile for `up` and `hitch` |
| `GANG_SESSION` | exact tmux session Gangline addresses |
| `GANG_CONTEXT_LIGHTS` | `off`, or absolute `yellow,red` token thresholds |
| `GANG_PROFILES` | custom profile directory searched before shipped profiles |
| `GANG_BOOT_TIMEOUT` | harness startup readiness bound |
| `GANG_LOCK_DIR` | shared per-pane delivery-lock directory; per-target spools live under it |

Operational evidence bounds also use `GANG_CHURN_WAIT`,
`GANG_ACTIVITY_WINDOW`, `GANG_ACTIVITY_LIMIT`, `GANG_TURN_LIMIT`,
and `GANG_OCCUPIED_LIMIT`. Their defaults live once in
`bin/gang`; change them only with evidence about the native harness surface.

Every process addressing one team must agree on `GANG_SESSION`, `GANG_PROFILES`,
and `GANG_LOCK_DIR`.

## Profile contract

A profile is a Bash file sourced by `bin/gang`. Harness-specific behavior belongs
there, never in a harness-name branch in the core script.

| Declaration | Purpose |
|---|---|
| `GANG_LAUNCH` | required native launch command |
| `GANG_RESUME_LAUNCH` | optional safe native resume command |
| `GANG_MODEL_OPT` | optional native model flag |
| `GANG_EFFORT_OPT` | optional native effort option, declared whole with its separator; the level joins with no space |
| `GANG_EFFORT_CMD` | prints the effort vocabulary, one level per line, given `GANG_MODEL`; empty output means could-not-determine |
| `GANG_BUSY_REGEX` | pane evidence of an active turn |
| `GANG_OCCUPIED_REGEX` | pane evidence that a native UI owns input |
| `GANG_QUEUED_REGEX` | input-box evidence that the harness parked input in a native queue instead of submitting |
| `GANG_QUEUE_RECALL_KEY` | tmux key name that loads the parked message back into the composer, used by `flush` |
| `GANG_INTERRUPT_KEY` | tmux key name that stops an active turn, used by `interrupt` |
| `GANG_STOP_HOOK=1` | the launch command installs a native Stop hook reaching `gang hook`, so this harness can drain a spool |
| `GANG_QUIET_AT_REST=1` | harness terminal becomes quiet when idle |
| `GANG_MIDTURN_INPUT=1` | ordinary text may safely enter during a turn |
| `GANG_COMPACT_CMD` | native compaction command |
| `GANG_SELF_COMPACT=deferred` | self-compaction must wait for Stop |
| `GANG_SESSION_KEY=1` | context lookup needs the startup-envelope nonce |
| `profile_input target` | print human-authored composer contents, or fail if absent |
| `profile_context target` | print `usedk/windowk (percent%)`, or fail loudly |

Profiles may install native event hooks by composing them into their launch
command. They must not weaken sandboxing, approvals, or operator permissions.
