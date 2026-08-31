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

**A window is blocked when a delivery gang reported as made produced no work,
and none is coming until something else is sent.**

The first clause matters as much as the second. A turn a person interrupts also
ends without producing work, and it is *not* blocked: the interruption was the
intervention, performed knowingly, and the window takes the next turn normally.
What makes this a defect is that a message gang *said it delivered* died
silently and nobody was told.

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

### codex — native, on a conjunction rather than an error class

Codex has no error-typed terminator at all. There is no `"type":"error"` record,
no `stream_error`, no `task_failed`. What it does have is a complete turn
bracket: `task_started` is closed by exactly one of `task_complete` (carrying
`last_agent_message`, `time_to_first_token_ms`, `duration_ms`) or `turn_aborted`
(carrying `reason`). So "the turn ended" is a record here too, and the criterion
transfers; what does not transfer is a record saying it ended *badly*.

`turn_aborted` looks like the shape to key on and is not. Every abort observed
carries `reason: "interrupted"` — a person pressing Esc. That turn ended and
produced nothing, and the window takes the next turn normally, because the
interruption **was** the intervention. Keying on the abort would mark every
interrupted window blocked.

The signal is a conjunction on an ordinary completion: no `last_agent_message`,
**and** no `time_to_first_token_ms`, **and** a turn body holding nothing but the
input that opened it. No leg carries it alone — turns that ran to dozens of tool
calls routinely end with no closing message, and a compaction turn is otherwise
byte-identical to a blocked one until you look inside it.

Reading the body means classifying every payload type in it, and the default is
**unknown by name** rather than "not work". A suffix test on `_call` was tried
first and was the wrong shape: this vocabulary carries `_call`, `_call_output`,
`_call_end` and bare `_output`, and `custom_tool_call_output` had to be listed
explicitly precisely because the suffix never reached it. Three real payload
types slipped through that test and each turned a turn that had worked into a
blocked window. Extending the suffix list buys the next name and not the one
after it, so the inversion is the fix: a name nobody has classified costs an
honest unknown, never a false verdict, and a record whose payload carries no
type of its own is named by its record instead so nothing goes unclassified by
accident. That also turns the rollout corpus into a real negative control —
every name in it is classified today, so a future unknown is a genuinely new
name rather than a silent pass.

**Duration is rejected as a signal.** The observed blocked population runs from
949ms to 83s, overlapping healthy turns at both ends. It separates nothing, and
recording that it was tried is worth as much as recording what was chosen.

Only the newest turn counts, and the reason is weaker than it first looks. It is
tempting to argue that a codex turn only ever begins from an input, so a newer
`task_started` proves something was **sent**. That is false: measured over the
rollouts, about an eighth of all turns begin with no input record at all, and
pure auto-compaction turns — `context_compacted` and a token count, nothing else
— are the clean counter-example. Codex opens those itself.

What the rule actually rests on is that a newer terminator means the record can
no longer speak to the older turn, whatever opened the newer one. That is true
unconditionally, and it is all the reader needs: it retires spent evidence
exactly as a newer user turn retires it on the claude-code side.

The intervention is nonetheless in the corpus verbatim where it happened. One
window took three deliveries that produced nothing, and the turn after them
opens with a message saying the agent had been relaunched on a different
model. Three messages lost in silence, and the recovery in the record is a person
working out what was wrong.

**Residual exposure, stated rather than argued away:** a hollow completion
followed by an input-less turn flips the verdict to absent with nobody having
intervened. On a genuinely stuck window the exposure is small — no turns are
running, so context is not growing and auto-compaction is unlikely to fire — but
it is not zero.

### The typeless-payload trap, worked

Inverting the default is right, and the whole risk of doing it sits on the
*benign* side of the line rather than the work side. A work record wrongly
called bookkeeping is the same defect the inversion was built to remove, just
moved; and a benign record left unclassified turns a true positive into an
unknown.

The specimen proves the second half by itself. Its turn body is not only the
input — it also carries a `world_state` record, a `turn_context` record, and a
`message` with role `developer` alongside the user one. The first two have **no
`type` field in their payload at all**.

So a reader that classifies by payload type alone leaves them unnamed, and under
an inverted default an unnamed record is an unknown. The frame this whole reader
exists for would report `?unknown?` instead of `!blocked!` — the inversion would
have removed the false positives and destroyed the one true positive with them.

The fix is one line of care: a payload carrying no type of its own is named by
its **record** type instead, so `turn_context` and `world_state` are classified
rather than skipped. `test/integration-cli.sh` pins this with a fixture built
from the specimen's own shape — typeless records and a developer-role message —
asserting it still reads blocked.

Anyone extending this vocabulary should assume the same trap is waiting. The
question to ask of a new name is not "is this work?" but "if I get this wrong in
the benign direction, which frame stops being detected?"

### `newest_lines` is defined twice in this collar

`codex_action_read` and `codex_blocked_read` each carry their own copy of
`def newest_lines(path):`. Any edit, patch, or tool that anchors on that
signature resolves to the first occurrence, which is inside the action reader
rather than this one. An edit bounded by it duplicates the function it meant to
change; the duplicate parses, the collar sources without error, and the failure
surfaces only when the stale copy runs.

That hazard and the suffix test this section replaced are the same failure. A
suffix test on `_call` reads as a rule about tool calls and is a rule about the
last five characters of a name, which is why it admitted `tool_search_output`
and `mcp_tool_call_end`. An anchor on a function signature reads as a rule about
one function and is a rule about the first match in a file. Both look specific,
both are general, and both hold until something in the general set turns up that
the specific reading never imagined. **When a rule is written as a pattern, ask
what else matches the pattern rather than what the pattern was for.**

It is also why the guards here are executable rather than review-time. A
duplicated function reads as correct and a vocabulary gap reads as absent, so
neither is visible to inspection; `test/probes/codex-hollow-turns.py` and the
fixtures in `test/integration-cli.sh` check both by running.

### What this reader does NOT cover, and must not grow to

A harness process that dies mid-turn reports **nothing** from the codex reader,
and that is correct rather than a shortfall. Its rollout ends with a
`task_started` nothing closes — and a turn still running looks exactly the same,
because a process that dies does not get to record that it died. The reader
therefore treats an unfinished bracket as absent, always.

A window whose harness panicked is a **liveness** question about the process, not
a state question about the record, and it is answered elsewhere. It is not
`!blocked!` and must not be folded into it. Blurring the two would buy coverage
of the panic by reporting every working codex agent as blocked, which is the
inversion this whole design exists to refuse.

### The corpus this was measured against

Measured across a corpus spanning every rollout shape the reader can meet, it
reports blocked on the sessions whose newest turn is hollow — the specimen among
them — and stays silent on every other file, with no unknown verdicts. Silence
across the four healthy populations is the claim: ordinary completions,
turns that worked and closed without a message, compaction turns, and
interrupted aborts.

The counts are deliberately not written down here, because they change whenever
the corpus does. `test/probes/codex-hollow-turns.py` reproduces the measurement
and prints the current population sizes, so the numbers are checkable rather
than believed.

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
