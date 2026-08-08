# Command and configuration reference

`gang --help` is the authoritative command inventory, and
`gang <command> --help` prints that command's synopsis. Help has a deliberate
48-character line budget so it remains legible in narrow phone-SSH terminals;
this reference carries the complete command contract.

When a command's only missing argument is an agent name, a bare invocation from
inside a Gangline window targets that window. This applies only with zero
arguments, so a numeric argument to `capture`, for example, remains an agent
name rather than becoming a self-targeted line count. Self is resolved from the
calling tmux pane in the same way as a message sender.

| Bare command | Result |
|---|---|
| `status`, `capture`, `composer`, `compact`, `context` | Target the calling agent. |
| `usage`, `interrupt`, `flush` | Print help; self-use is incoherent while its turn is running. |
| `drop`, `down` | Print help; destructive commands never target by omission. |
| `hitch`, `adopt`, `send` | Print help; the missing name is not a self target. |
| `up`, `roster`, `spool`, `attach`, `collars`, `roles`, `config`, `curfew`, `notify` | Keep their ordinary bare meaning. |

Outside a Gangline window, a bare self-targeting command prints its synopsis and
states that no target or Gangline agent window was available.

## Lifecycle

### `gang up [name] [hitch flags]`

Hitches one agent, named `lead` when omitted, then attaches or switches the
current tmux client to it. `GANG_COLLAR` selects the harness.

### `gang hitch <name> [-c collar] [-d dir] [-m model] [-e effort] [-r|--role role] [--resume [session-id]]`

Starts a native harness in a named tmux window and delivers one startup contract.
That contract tells the agent to choose a model and reasoning effort deliberately
when hitching teammates. Peer messages name their sender in the gang envelope.
Gangline never delivers a message without one — the only unenveloped text it
ever types into a pane is your harness's own compaction command — so any other
unenveloped text arrived from the session keyboard, and Gangline cannot
attribute it further. If `$GANG_CONFIG_DIR/DOCTRINE.md` is present, readable,
valid UTF-8 prose within its category-error ceiling, the contract attributes and
appends it byte-exactly. Every hitch carries doctrine; Gangline cannot infer
which caller is the operator. `adopt` still injects no startup text.

The launch environment carries the exact `GANG_SESSION`, the absolute resolved
`GANG_CONFIG_DIR`, and any custom collar and lock paths, so harness commands
cannot drift to another session or configuration layer on the same tmux server.
File-layer settings therefore reach nested hitches. Other per-invocation
environment overrides do not become sticky inside the agent.

If a first-run prompt owns the screen before the composer appears, `hitch`
directs the operator to `gang attach` once the collar's occupied pattern
provides positive pane evidence, then keeps waiting within the original
`GANG_BOOT_TIMEOUT`. Clearing the prompt lets that same hitch report the
recovered input box and deliver its startup contract. If the bound expires
first, the window is left for inspection and the error gives the attach, drop,
and re-hitch recovery.

- `-c` selects a collar.
- `-d` selects the harness working directory.
- `-m` passes a harness-native model choice through the collar's model option.
- `-e` passes a harness-native reasoning-effort level through the collar's
  effort option, joined with no space, on the plain and `--resume` launch forms
  alike. The collar prints its level vocabulary and a level outside it is
  refused before any window opens. A collar that declares no effort spelling
  refuses the flag, and a vocabulary that cannot be determined is refused as a
  broken declaration rather than a bad value.
- `-r`, `--role` attaches the named role brief to this hitch only. There is no
  default or inference from the agent name. Role names use letters, digits,
  dot, dash, and underscore and may not begin with dot or dash.
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
Operators normally place both thresholds high in the native window so lights do
not impose an artificial context disadvantage, but below the harness's observed
automatic-compaction boundary so the lights remain reachable.

### `gang adopt <name> -c <collar>`

Registers an existing window in `GANG_SESSION`. Adoption does not inject startup
text or retroactively add launch-time native hooks. A collar whose context
source requires hitch-time identity may therefore report context unavailable.

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

Takes the target's delivery lock and reports its spool before any other output.
Every waiting entry is printed with its sender, fragment, and full stored body,
then removed because the window it addressed is ending. Held entries, unaccepted
fragments, and unaccounted children are named and preserved; the spool directory
is removed only when it is empty. A missing reservation is reported and does not
prevent recovery.

It then prints the window's stamped native session id and the exact
`gang hitch <name> --resume <session-id>` relaunch command, and kills the exact
agent window. If the collar has not supplied a stamp, it says so instead.

### `gang down <session>`

Takes every agent window's delivery lock before changing any spool or window. If
one lock is held, it refuses without pruning any spool or killing any window.
Under those locks it gives every window the same report-and-preserve treatment
as `drop`, prefixing each report line with the agent name, then kills the exact
team session.
The session name is required and must match the team this shell is pointed at;
`down` refuses a mismatch and refuses outright from a pane inside that session.
There is no override: an agent must not be able to end the team it is running in.

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

Gangline refuses a missing or occupied composer, a human draft, unknown
state, and unsafe mid-turn input. A collar may declare that its native harness
accepts ordinary mid-turn input.

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
own it, because tmux offers no atomic compare-and-delete and a reader's
unset can erase a fresh hook stamp landing between the read and the unset.
A malformed value is reported as unreadable, never repaired, and eligibility
is re-derived from live evidence on every send.

The frozen-paint demotion requires an expired bracket, so a window with no
native hooks and no mid-turn-input declaration whose pane keeps a matching
busy marker stays refused until the marker scrolls off or the agent is
dropped. That refusal is fail-closed and deliberate.

Queued is not delivered: where a collar declares queue evidence, a harness
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

A refused delivery parks by default. The live delivery is attempted first; if
it is refused, the nonce-bound envelope is written to a per-target spool and
reported as parked, not delivered. A failure after anything was typed is never
parked, because that body's fate is unknown and a second copy would be a second
message. Pass `--live-only` for an availability probe that must return a refusal
to its caller instead of parking it. The deprecated `--spool` flag is accepted
as an announced no-op.

The target's own native Stop event drains its spool, oldest first, through the
verified delivery path. Each entry is claimed before it is delivered, so no
later drain can send a body this one may already have landed. A refused drain
returns its entry to the spool for the next turn boundary. A drain that cannot
verify, or one that dies between the submission and the entry's retirement,
leaves that entry held: `status` says its delivery was not verified and may
still have arrived, `status` and `roster` report how many are held, the bodies
stay readable under `GANG_LOCK_DIR`, and Gangline never sends them again. Held
entries outlive the target window: routine teardown cannot erase the evidence
behind a recovery pointer. Retirement is a separate, deliberate operator act. A
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
arriving together cannot mint competing ones. Mint atomically creates the token
directory before publishing the identity; that directory is the token's durable
reservation, even while empty. A fresh mint therefore never reuses a surviving
directory. Parking asserts that the reservation still exists and never recreates
a missing one: after a refusal in that state, recover with `gang drop <name>` and
the new `gang hitch ... --resume ...` command it prints. Re-adopting preserves the
broken identity and is not a repair.

### `gang spool`

Lists the whole spool root without creating or changing it. An absent root says
nothing has ever been parked or reserved. An unsafe root is refused without
following it. Every directory is classified against every window on the
reachable tmux server, across all sessions, as owned, contested, unattributed,
or unaccountable; root debris is named separately. Counts include waiting,
held, and unaccepted fragments, and empty unattributed reservations are listed
rather than collected. If no tmux server is reachable there are no local
carriers, so directories are unattributed. Every ownership report explicitly
says that its scope is this tmux server. The command never deletes anything.

### `gang spool retire <name|token> [--assume-unowned]`

The live-agent form takes that window's delivery lock, prints each held entry
with its sender, fragment, and full body, removes it only after the report write
succeeds, and reconciles the held ledger. Waiting entries remain deliverable.
Unaccounted children are named and preserved. It refuses when nothing is held,
so an agent typo cannot look like successful recovery.

The token form acts only on a directory unattributed on this tmux server, and
requires `--assume-unowned` because another tmux server may still carry it. It
refuses owned, contested, or unaccountable directories. With the flag, every
accounted entry and unaccepted fragment is printed before removal, then the now
empty reservation is removed. An empty directory still requires the flag: its
reservation is the state being deliberately retired.


### `gang flush <name>`

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

### `gang interrupt <name> [-m "reason"] [--from <sender>]`

Sends the keystroke the collar declares as its harness's turn-stop key and
drops Gangline's turn bracket, so the interrupted turn neither leaves a bracket
answering busy until its bound expires nor a written one answering idle. State
then comes from the pane: a harness that stopped goes idle, and one that
ignored the key stays busy and unreachable. Whether the harness stops remains
the harness's verdict. A collar that declares no interrupt key refuses the
command, and an occupied composer refuses it too — that keystroke is often what
a native dialog reads as an answer.

With `-m`, Gangline attributes the reason like an ordinary message, holds one
pane lock across the stop key and the boundary it creates, then delivers the
reason before releasing that lock. The reason is never spooled: a queued stop
would arrive after the work it was meant to stop. If the composer does not
return or delivery cannot be verified, the reason is printed back to the caller
in full and reported as not delivered and not parked. Existing backlog remains
untouched for the next native turn boundary.

### `gang compact <name>`

Submits the collar's native compaction command through the same verified input
path. An external request refuses a busy or unknown target.

When a hooked Codex or claude-code agent requests its own compaction, Gangline
records a one-shot request. Its native Stop hook submits `/compact` after the
active turn releases the composer. `status` and `roster` expose pending or failed
self-compaction. A guarded claude-code launch that cannot install hooks declares
neither Stop support nor deferred self-compaction.

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

### `gang mail <name>`

Prints every message waiting in that agent's spool, oldest first, then every
held entry, each with its sender and its entry filename, each body exactly as it
would go onto the wire. It reads: it delivers nothing, creates nothing, removes
nothing, and takes no delivery lock. It needs no loadable collar, because a
queue is files on disk and the harness may be the reason you are reading it.

### `gang status <name>`

Prints one current state:

- `-busy-` — a native event, terminal activity, or collar marker
  witnesses active work;
- `~idle~` — the evidence positively witnesses readiness;
- `!occupied! (authority unknown)` — a native UI owns the composer;
- `!occupied! (known transient: <id>)` — a collar-enumerated benign dialog is
  visible; observation names it but presses no key;
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

### `gang usage <name>`

Drives the usage command declared by the target's harness collar and prints the
harness's own page raw. The pane is locked for the entire operation. Gangline
refuses a busy, unknown, occupied, changing, or non-empty composer; it
types only after all five predicates pass. A bare `gang usage` prints help
because the calling agent is necessarily running in the composer it would have
to drive.

Modal pages are returned as the visible screen, trailing blank terminal rows
removed, and then dismissed with the collar's key. A modal that scrolls within
itself may contain more than Gangline can capture; attach to read that overflow.
Inline pages are extracted from the difference between the whole scrollback
before and after the native command, so content taller than the pane is retained.
If tmux history rolls over and the two captures lose their common origin,
Gangline refuses instead of printing a plausible but unbounded transcript diff.

After reading either shape, Gangline verifies that the collar can read an empty
composer again. If restoration fails, the captured content is still printed,
the command exits non-zero, and `gang attach` is required before more input is
safe. Collars without a verified usage declaration refuse the command.

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
With no running session it prints no rows and exits successfully.

After the human-readable rows, a non-empty unattributed spool, a contested token,
or an unaccountable directory adds one footer pointing to `gang spool`. Roster
scans and removes nothing. The same footer scan runs after the human no-team line
when the configured session is absent, because surviving directories are most
relevant then. An unsafe spool root is named without hiding otherwise readable
rows or changing roster's exit status. Porcelain output remains TSV rows only.

### `gang capture <name> [lines]`

Prints the tail of the target pane after trimming trailing blank terminal rows.

### `gang composer <name>`

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
standard input. Prompt/tool events open the turn fact, Stop closes it and may
dispatch deferred self-compaction and a spool drain, and permission requests
raise occupancy.
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
| `GANG_CONTEXT_LIGHTS` | `off` | `off`, or absolute `yellow,red` token thresholds |
| `GANG_BOOT_TIMEOUT` | `30` | harness startup readiness bound in seconds |
| `GANG_CHURN_WAIT` | `0.5` | stable-pane observation interval |
| `GANG_ACTIVITY_WINDOW` | `5` | recent terminal-activity window |
| `GANG_ACTIVITY_LIMIT` | `300` | activity-only evidence bound |
| `GANG_TURN_LIMIT` | `300` | native turn-fact bound |
| `GANG_OCCUPIED_LIMIT` | `900` | native occupancy-fact bound |
| `GANG_CLEAR_PRESSES` | `40` | maximum verified composer-clear attempts |

Collar declarations are refused because `load_collar` clears them before
sourcing the selected collar; put those values in a custom collar and point
`GANG_COLLARS` at it. `GANG_TEST_COLLARS` is suite-only,
`GANG_CONFIG_DIR` cannot set the file that is already being read, and internal
variables are refused. Any malformed file refuses every command, including
native hooks; recovery is in `docs/operations.md`.

Doctrine is never written by Gangline. It must be a readable regular file with
no NUL, no controls other than tab and newline, valid UTF-8, and no more than
8192 bytes. That ceiling catches a log, binary, or document tree placed in the
prose slot; it is not a deliverability bound. Actual delivery is bounded by the
target composer's rendering and pane geometry and remains verified at hitch.
Delete `DOCTRINE.md` to remove the slot; a hitched copy dies with its window.

Role briefs use the same prose validation. Resolution checks
`$GANG_CONFIG_DIR/roles/<name>.md` before the shipped `roles/<name>.md`; an
operator file replaces the shipped file whole, and an unusable override refuses
instead of falling back. A named role must be nonempty. Where the selected
collar declares `GANG_ROLE_PROMPT_OPT`, Gangline passes the validated bytes by
value through that launch option and puts only an attribution pointer in the
startup contract. Otherwise it puts the body in the startup contract before
operator doctrine. The system-prompt form includes Gangline's own preamble
stating that later operator doctrine governs any disagreement.

Every process addressing one team must agree on `GANG_SESSION`, `GANG_COLLARS`,
`GANG_LOCK_DIR`, and the resolved `GANG_CONFIG_DIR`.

### Deprecated names

The 1.0 rename keeps every old name working through 1.x. Each announces itself
on stderr and is removed in 2.0. A setting Gangline reads once per command
announces once; a collar declaration read for each window announces for each
window. Setting one setting under both names — in the file, in the environment,
or one in each — is refused rather than silently resolved.

| Accepted through 1.x | 1.0 name |
|---|---|
| `gang profiles` | `gang collars` |
| `gang cutoff` | `gang curfew` |
| `-p`, `--profile` | `-c`, `--collar` |
| `GANG_PROFILE` | `GANG_COLLAR` |
| `GANG_PROFILES` | `GANG_COLLARS` |
| `GANG_TEST_PROFILES` | `GANG_TEST_COLLARS` |
| `profile_input` | `collar_input` |
| `profile_context` | `collar_context` |
| `profile_session_id` | `collar_session_id` |

The window option `@gl_profile` and the session option `@gl_cutoff` are not
aliases: Gangline migrates them in place the first time it reads a window or
team carrying them, so a fleet running when the rename lands keeps working.

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
| `GANG_BUSY_REGEX` | pane evidence of an active turn |
| `GANG_OCCUPIED_REGEX` | pane evidence that a native UI owns input |
| `GANG_QUEUED_REGEX` | input-box evidence that the harness parked input in a native queue instead of submitting |
| `GANG_QUEUE_RECALL_KEY` | tmux key name that loads the parked message back into the composer, used by `flush` |
| `GANG_INTERRUPT_KEY` | tmux key name that stops an active turn, used by `interrupt` |
| `GANG_STOP_HOOK=1` | the launch command installs a native Stop hook reaching `gang hook`, so this harness can drain a spool |
| `GANG_STALL_TYPES` | space-separated native `Notification` kinds that mean the harness is awaiting a person |
| `GANG_DIALOGS` | newline-separated `id|marker|safe|move|confirm` known-transient records |
| `GANG_DIALOG_LINES_<id>` | every painted line of one dialog, in order; dashes in `id` become underscores |
| `GANG_DIALOG_HITCH_DIR_TRUST` | optional one dialog id whose directory trust was already chosen by `hitch -d` |
| `GANG_QUIET_AT_REST=1` | harness terminal becomes quiet when idle |
| `GANG_MIDTURN_INPUT=1` | ordinary text may safely enter during a turn |
| `GANG_COMPACT_CMD` | native compaction command |
| `GANG_SELF_COMPACT=deferred` | self-compaction must wait for Stop |
| `GANG_USAGE_CMD` | native command that opens the harness's usage page |
| `GANG_USAGE_CONFIRM_KEY` | optional space-separated tmux keys that reach the usage content after submit |
| `GANG_USAGE_RENDER` | usage page shape: `modal` or `inline` |
| `GANG_USAGE_DISMISS_KEY` | optional tmux key that closes the page; empty when the harness restores itself |
| `collar_input target` | print human-authored composer contents, or fail if absent |
| `collar_context target` | print `usedk/windowk (percent%)`, or fail loudly |
| `collar_session_id target payload` | print the exact native session id witnessed by a hook, or fail without fabricating one |

Collars may install native event hooks by composing them into their launch
command. They must not weaken sandboxing, approvals, or operator permissions.

Each dialog record has five fields. `id` is `[a-z0-9-]+`; `marker` is a
line-start-anchored ERE for the selected numeric row; `safe` is one declared
option label; `move` is zero or more tmux key names; and `confirm` is one key.
The associated block contains the whole dialog, not a subset. Load refuses
malformed or duplicate records, absent blocks, safe labels absent from their
block, invalid key names, and any id, safe label, or block line containing
authority language such as permission, trust, approval, authorization, access,
elevation, grants, administration, denial, bypass, credentials, tokens, secrets,
privileges, or sandboxing. A collar may name one record in
`GANG_DIALOG_HITCH_DIR_TRUST`; that record alone may carry directory-trust and
explanatory allow language because `hitch -d` already named the directory.
Every other authority family remains forbidden in that record.

Before a send, or while hitch is already waiting for a composer, Gangline may
answer one unambiguous full-block match. Whitespace and soft wraps are
normalized; numeric row prefixes move with the selection and are excluded from
the fingerprint, while every label, body line, footer, and their order must
match. Gangline re-captures after every movement key, verifies the selected
label is `safe` immediately before confirmation, and then verifies both that
the dialog disappeared and a composer returned. Any failed proof refuses and
does not confirm. `status` and `roster` only name a recognized transient and
never press a key.
