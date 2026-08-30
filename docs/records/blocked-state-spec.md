# Blocked state — design

> Status: Implemented 2026-08-30; retained as a dated implementation record.

## The defect

A window whose last input produced no work, and for which no work is coming
without intervention, reports `~idle~`. `gang send` types into it and reports
delivered. `gang wait --until idle` returns satisfied. The lead learns nothing.

Specimens in `docs/records/specimens/`:

- `blocked-turn-pane.txt` — the pane. A completed turn (`Cooked for 23s · done`)
  whose whole body is `API Error: <model>'s safeguards flagged this message …
  Claude Code can't respond to this message with <model>.` Then an empty composer.
- `blocked-turn-explain.txt` — `gang explain` on that window: `state: ~idle~`,
  with `GANG_OCCUPIED_REGEX: did not match`, `collar_bricked: did not match`,
  `GANG_BUSY_REGEX: did not match`.
- `blocked-turn-transcript.txt` — the transcript tail the fatal reader consults.

The message that window was given was never acted on and never will be. Every
rule gang has
says the window is fine.

## What blocked means

**A window is blocked when the last input it accepted produced no work and none
is coming without intervention.**

That is wider than "a dialogue is up". Two instances:

- **A UI owns the input box.** A permission prompt, an elicitation dialog, a
  model picker. Delivery is impossible: keystrokes go to the dialog.
- **A turn ended without producing work.** A refused or dead turn. Delivery is
  *possible* — the composer is free — and that is what makes it dangerous, because
  every guard gang owns is a guard on the composer.

The specimen is the second kind, and the second kind is the one gang cannot see.

## The counter-specimen: menu-shaped and not blocked

`docs/records/specimens/not-blocked-menu-pane.txt`. A numbered menu offering
"Retry with a faster model / Dismiss and keep waiting / Learn more", above the
harness's own sentence: *"No action is required. Codex will keep waiting, and
this menu will close when the response is ready."* The pane above it shows the
agent still working.

It reads `!occupied! (authority unknown)` and a send to it is queued. Both are
right. Under the definition above this window is **not** blocked: work is coming.

The two specimens bound the problem from opposite sides. The first looks idle
and is blocked. The second looks blocked and is fine. Any rule that keys on what
the screen looks like gets one of them wrong, and a rule tuned to get both right
by their appearance is tuned to two frames rather than to the thing itself.

### The criterion is not "a menu is on screen"

It is whether the harness has recorded that the turn **ended**. That is not a
property of the screen and cannot be forged by one.

- Specimen 1: the transcript holds a turn-ending error record, followed only by
  `turn_duration`. The turn is over. No further work is coming.
- Specimen 2: the turn is in flight. Codex writes `task_started` and
  `task_complete` around each turn, and the second has not been written. The
  menu decorates that wait; it does not gate it.

So the discriminator between a prompt that gates progress and one that decorates
it is the harness's own turn bracket, read from its own records. A gating prompt
sits after a turn that ended; a decorating one sits inside a turn that has not.
This is the same reason the reader keys on `isApiErrorMessage` rather than on
the error's prose: both are the harness's account of its own state, and neither
is a description of pixels.

### When the harness gives no way to tell

Then gang does not tell. The collar declares no blocked reader, `explain`
reports `not declared`, and the window is described by what can be seen — a UI
owns the box, so `!occupied!` — with no claim about whether progress is coming.

Nothing is inferred from the absence of a reader. A collar that cannot answer
must not be read as answering "healthy"; that inversion is the original defect
wearing a different hat. This is why codex declares nothing here despite having
a usable turn bracket: the bracket alone says a turn is or is not running, and
turning "no turn running" into "blocked" would report every idle codex window as
blocked. The missing half is a record of a turn that ended *without producing
work*, and no codex specimen of one exists yet.

### The queue is where the cost lands

Queueing behind specimen 2 is correct and needs no operator: the window
completes a turn and the queue drains at it. Queueing behind specimen 1 is a
message parked forever, because no turn boundary is coming unattended.

The mechanism does not change — the distinction does, and it is the state that
carries it. `gang roster` already prints each window's queue depth and the age
of its oldest entry beside its state, so with `!blocked!` present a queue behind
a dead turn is visibly a queue behind a dead turn rather than an ordinary wait.

The send notice is corrected to match. The standing text promises the message
"enters the session when '<name>' next completes a turn, and nothing further is
needed from you" — true for every window that will take another turn, and for a
blocked one the same false reassurance as reporting it idle. A send that parks
against a blocked window now says so, says the queue will not drain on its own,
and says repair is needed. The question is asked of the collar, not inferred
from the refusal's wording.

## Native or regex, per collar

Regex on pane text is the fallback. The question was asked correctly: does the
signal fire on both entry and exit, survive a restart, attribute to a pane, and
work mid-turn. Findings, with evidence:

### claude-code — native, already, for the first instance

`bin/gang:9203` consumes the harness's `PermissionRequest` hook and calls
`occupied_raise_write`, latching `@gl_occupied`. `state_now` checks
`occupied_witness` first, so a permission dialog reads `!occupied!` from a native
event, not from paint.

The exit problem is already solved, and solved the right way. There is no native
dialog-exit event — a turn a person ends by declining a permission dialog
announces nothing at all (`bin/gang:8107`). So the latch is not retired by a
second event; it is retired by independent evidence that the window moved on:
`occupied_witness` clears it when `composer_live` shows the box came back, and
an unreadable pane retires nothing. `GANG_OCCUPIED_REGEX` sits underneath as a
second tier for dialogs that raise no event.

**This is already the pattern the requirement asks for.** A one-way native
signal, latched, retired by evidence rather than by a matching event. The work
below extends it; it does not replace it.

Also already native: `GANG_STALL_TYPES="permission_prompt idle_prompt
elicitation_dialog agent_needs_input"` (`collars/claude-code.sh:95`) — typed
`Notification` events for the whole awaiting-input family. Gang currently spends
them on a courtesy note to the notify target (`stall_note`) and on nothing else.
`@gl_stall` is set only when that note is *delivered*; a note that failed to
deliver leaves no record that the event happened.

### claude-code — native, available, unread, for the second instance

The specimen's transcript
(the window's `@gl_session`-bound transcript) ends:

```
{"type":"user",      …}
{"type":"attachment",…}
{"type":"system",    "subtype":"model_refusal_no_fallback", …}
{"type":"assistant", "isApiErrorMessage":true, "error":"invalid_request", …}
{"type":"system",    "subtype":"turn_duration", …}
```

Structured, deterministic, per-pane (bound through `@gl_session`), survives
restart (it is a file), and readable mid-turn. Two independent structural
witnesses: a `system` record whose `subtype` names the class exactly, and the
`isApiErrorMessage` assistant record.

`collar_bricked` reads this exact file and reaches this exact record. It reports
"did not match" for one reason: its fatal reader accepts `error == "server_error"`
and `error == "model_not_found"` and returns *absent* for everything else
(`collars/claude-code.sh`, `claude_record_read` mode `fatal`).

**The reader enumerates error values by allowlist, and every class outside the
allowlist reads as a healthy idle window.** That is the defect, and it is a
guard that rots silently: each new error class the harness ships is a fresh
false-idle, arriving without a signal.

The declarative shape, stated once: **the shape is `isApiErrorMessage`, not the
error vocabulary.** The newest top-level semantic assistant record carrying
`isApiErrorMessage: true` *is* a turn that ended without producing work. The
`error` value supplies the reason and nothing else. An unrecognised value yields
**blocked with an unnamed reason**, never absent.

Recorded for the reader: the specimen's refusal is a per-model safeguard
response. The same prompt on another model does not reproduce it, and the
sentence varies. Detection keys on record structure, never on that prose.

### codex — native material, no evidence to write a reader from

`collars/codex.sh` declares `GANG_STOP_HOOK=1` and binds a rollout jsonl per
window (`codex_session_file`). It declares no `collar_bricked` and no
`GANG_STALL_TYPES`.

A codex rollout carries top-level `response_item`, `event_msg`, `turn_context`,
`world_state`, `session_meta` and `compacted` records; its `event_msg` payloads
include `task_started` and `task_complete`, a clean native turn bracket.

No turn-ending error class appears in any rollout available here, because no
session on hand was blocked. So a codex blocked reader **cannot be written from evidence
yet**, and will not be guessed at. Codex declares nothing here until a codex
blocked frame is driven and captured.

### opencode, pi, bash

Declare nothing. They stay declaring nothing.

### The resulting split

| collar | first instance (UI owns box) | second instance (turn produced nothing) |
|---|---|---|
| claude-code | native `PermissionRequest` latch, regex second tier | native transcript read |
| codex | none | none — no specimen yet |
| opencode, pi, bash | regex where declared | none |

Native everywhere it exists. Regex survives only as the second tier under a
native witness. Nothing is invented for a collar with no specimen.

## Judgment calls

### One state, carrying a reason

`!blocked! (<reason>)`, one state, reason always present.

It is **not** folded into `!bricked!`. The two demand different repairs and the
state word is what the lead acts on. `!bricked!` says the session cannot work —
drop it and re-hitch; `wait` rightly dies on it (`bin/gang:8485`). `!blocked!`
says *this input* got no work — intervene and re-drive; a differently worded
message, or `/model`, revives the same window. Calling a content refusal
"bricked" would tell the lead to destroy a window that one new message repairs.

### Precedence

Between occupied and bricked:

```
!session-lost!  →  !occupied!  →  !bricked!  →  !blocked!  →  -busy-  →  ~wait~ / ~idle~
```

- Below `!occupied!`: a dialog owning the box is the more specific fact and the
  more urgent repair.
- Below `!bricked!`: unrecoverable outranks recoverable. A window that is both
  should be reported as the one that cannot be repaired in place.
- Above `-busy-`: a turn-ending error record means the turn is over. A busy
  marker still painted above it is retained paint, exactly as `collar_bricked`
  already outranks the retained 529 line (`collars/claude-code.sh:520`).
- `?unknown?` keeps its existing meaning and is never collapsed into blocked.
  A reader that cannot tell says so.

### What the surfaces render

`roster` and `status` print `!blocked! (<reason>)`, glyph `!`, colour `C_BAD`,
window name `!name!` — the same treatment `!occupied!` and `!bricked!` get.
`gang explain` gains a `blocked:` row beside `collar_bricked:`, reporting
`matched` with its reason, `did not match`, `could not determine` with its
cause, or `not declared` for a collar with no reader. `--porcelain` prints
`blocked`.

### `gang send`

Refusing beats a false delivered. This is the one genuinely new coupling: state
and delivery are decided independently today — `state_now` has no caller in the
send path, and `!occupied!` stops delivery only because `composer_live` fails.
A blocked window's composer is free, so nothing currently stops the paste.

`gang send` to a blocked window **refuses, names the reason, and spools**. It
does not type. The message stays recoverable and the sender is told why, in the
same shape as the existing composer refusals. A spooled message drains on the
next real turn boundary, which is exactly the evidence that the window is no
longer blocked — so the repair path needs no new command.

Gang does not answer the dialog, retry the turn, switch the model, or drop the
window. Surface only; the lead decides. The one existing exception stays as it
is: auto-resume, where the operator has already opted in.

### `gang wait`

`--until idle` and `--until done` must never report satisfied on a blocked
window. They die with the state and its reason, as they already do for
`!bricked!` (`bin/gang:8485`, `:8503`, `:8511`), including the after-boundary
arm — a turn that ends *into* a blocked state has not reached the boundary the
caller asked for.

### Bounding false positives

The hazard is a rule matching dialogue-shaped text that is merely quoted — a
transcript discussing a permission prompt, an agent pasting an error message it
is fixing. Four bounds, in order of strength:

1. **Read structure, not prose.** The claude-code reader keys on
   `isApiErrorMessage`, a field the harness sets, on a record it wrote. Quoted
   text cannot forge it. This is why the second instance is transcript-read and
   not pane-read, and it removes the hazard rather than bounding it.
2. **Newest semantic record only.** `semantic()` already skips sidechains, meta
   records and tool-result-only turns, and a later real user turn outranks any
   terminal record. An error the agent has since worked past is not blocked.
3. **The regex tier never stands alone.** It survives only underneath a native
   witness and above a composer check, as `occupied` already arranges it: a
   match that a live composer contradicts is discarded.
4. **Unknown stays unknown.** An unreadable transcript, a malformed record, or
   a recognised error class with an unrecognisable message shape yields
   `?unknown?` with its cause — never blocked, and never idle.

## Work

1. Generalise the claude-code fatal reader from an error-value allowlist to the
   `isApiErrorMessage` structure, with the error value as reason and an
   unrecognised value reported honestly. `collar_blocked` withholds only what
   the fatal reader positively claims, so a `server_error` carrying a status
   that reader declines is reported here rather than falling through to idle.
2. Add `collar_blocked` to the collar contract and `docs/reference.md`: print
   reason; 0 blocked, 1 absent, 2 unknown. Optional; absence means the collar
   cannot answer, which `explain` reports as `not declared`.
3. Add the state to `state_now` at the stated precedence, plus `glyph_write`,
   the roster/status colouring, the porcelain word, and the `explain` row.
4. Refuse and spool in the send path on blocked, naming the reason. Delivery
   keeps two guard tiers — `send_live`'s and `inject`'s — and both ask, in the
   same order the state read uses. `send_live` decides first, so leaving the
   question to `inject` alone answers a blocked window from whatever its screen
   still shows: mid-turn, off busy paint the finished turn left behind.
5. Fail `wait --until idle|done` on blocked, all three arms.

### Deliberately not done

- **`@gl_stall` is left as it is.** Latching it on the `Notification` event
  rather than on the courtesy note's delivery is a real defect — a note that
  failed to deliver leaves no debounce record — but nothing in this state reads
  it, so fixing it here would only widen the diff. It belongs in its own change.
- **The unwired stall types stay unwired.** `GANG_STALL_TYPES` names
  `elicitation_dialog`, `agent_needs_input` and `idle_prompt` alongside
  `permission_prompt`, and only the last has a state consequence. Whether the
  others leave a window unable to act is not knowable without a specimen of
  each, and guessing is what this design exists to stop.
- **Delivery does not refuse an unknown reading.** The state surface already
  reports `?unknown?`; refusing delivery on it as well would let one corrupt
  transcript line strand sends to a window whose composer read back clean,
  which is a worse failure than the one being fixed.

## Tests

Five of the six reader assertions below are regressions: each one fails against
the unfixed reader. The sixth — that a blocked turn is *not* reported as a fatal
one — is green with or without the fix and is a preservation guard on the fatal
reader's scope, not evidence of this change.

Reader (`test/integration-cli.sh`), against transcript fixtures:

- the specimen's shape reports blocked, naming `invalid_request`;
- an `isApiErrorMessage` record with no error name is blocked with an unnamed
  reason — the guard against the allowlist growing back;
- a class the fatal reader owns is declined;
- a real newer user turn clears older blocking evidence;
- a complete malformed record is unknown with its cause, not blocked;
- the fatal reader does not start claiming the blocked class.

State and surfaces (`test/integration-hooks.sh`), driven against a live pane on
a private tmux socket through the existing fixture collar:

- blocked outranks a busy marker repainted immediately before the read;
- `status`, `roster`, `roster --porcelain` and `explain` each surface it;
- `wait --until done` and `--until idle` both refuse rather than satisfy;
- `send --live-only` refuses before any paste, names the reason, and does not
  answer from the busy paint still on that screen;
- fatal evidence outranks it, and `explain` then reports the blocked reader as
  `not evaluated`;
- a verdict with no reason, a reason smuggled onto an absent verdict, an
  unknown with no cause, and an undeclared verdict are each refused by name;
- an unreadable source is `?unknown?` with its cause.
