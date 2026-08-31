# Blocked state — known gaps and what closes each

> Status: Open 2026-08-30. Each entry names the specimen or change that closes
> it. An entry leaves this file when its closing condition is met, not when
> someone becomes confident.

Companion to [`blocked-state-spec.md`](blocked-state-spec.md), which describes
what `!blocked!` does cover.

## A dead process inside a live pane is not a transcript question

**Closing condition: none available from a transcript. This needs a different
signal, not a better reader.**

A codex harness panicked with `OS can't spawn worker thread: Resource
temporarily unavailable` while its window went on reading `~idle~`. Its rollout
ends mid-turn: one turn left open, the last record an ordinary tool call, and no
error-typed record anywhere. The rollout simply stops.

A process that dies does not get to record that it died. Both shipped readers
work only because the harness survives its own failure long enough to write
something down.

The unclosed bracket that remains is diagnostic only *after the fact*, once a
session is known to be over. It is not diagnostic while the window is live,
which is the only situation state sensing works in: a turn still running and a
process that died mid-turn produce an identical file, and gang has to answer
before it can know which it has. Both readers therefore treat an unfinished
bracket as absent, always, and report nothing for a panicked harness. That is
correct rather than a shortfall.

Telling the two apart needs evidence from outside the transcript — whether the
process is still there. `!dead!` answers the half where the pane has exited.
Nothing answers the half seen here, where the pane survived and the process
inside it did not. That case is owned as a liveness question and must not be
folded into `!blocked!`; buying coverage of the panic by loosening this reader
would report every working agent as blocked.

One further caution for whoever builds it: a positive liveness witness answers
"the process is gone" and not "the process is there and cannot work". The
specimen's harness failed to spawn a thread, which does not necessarily kill it,
and nobody looked before the window was dropped. Treating liveness as health
would recreate the original defect one layer down.

## A hollow codex completion followed by an input-less turn

**Closing condition: a live-side witness that a window is at rest, or a
measurement showing the exposure is empty in practice.**

The codex reader retires a hollow completion when a newer terminator exists. It
does not ask what opened the newer turn, because it cannot: roughly an eighth of
codex turns begin with no input record at all, and pure auto-compaction turns
are the clean case — the harness opens those itself.

So a hollow completion followed by an input-less turn flips the verdict to
absent with nobody having intervened. On a genuinely stuck window the exposure
is small: no turns are running, so context is not growing and auto-compaction is
unlikely to fire. It is not zero, and it is stated rather than argued away.

## Three of four declared stall types have no state consequence

**Closing condition: one captured frame per type, each showing what the window
can and cannot do while that notification stands.**

`collars/claude-code.sh` declares:

```
GANG_STALL_TYPES="permission_prompt idle_prompt elicitation_dialog agent_needs_input"
```

`permission_prompt` reaches a state: the `PermissionRequest` hook latches
`@gl_occupied` and the window reads `!occupied!`. The other three raise a
courtesy note and nothing else.

| type | the question a specimen answers |
|---|---|
| `elicitation_dialog` | Does the dialog own the input box? If it does, the composer read may already reach `!occupied!` without help — or may not, and the window reads idle while a dialog holds it. |
| `agent_needs_input` | The same question, plus whether it fires for a state that is not a drawn dialog at all. |
| `idle_prompt` | Fires both after a turn dies and on ordinary idleness. A specimen of each is needed to know whether they are separable from the event alone, or only by the state read that follows. |

Until each has a frame, none is wired. Guessing which of them gates progress is
the error the counter-specimen in the spec exists to prevent.

## `@gl_stall` latches on the note's delivery, not on the event

**Closing condition: latch the event, and let delivery record its own outcome
separately.**

`stall_note_claimed` writes `@gl_stall` only after `stall_deliver` succeeds. A
note that failed to deliver leaves no record that the notification arrived, so
the debounce that record exists to provide is absent exactly when notes are
already going wrong, and the same stall can re-notify on every later event.

The fact and its delivery are different things and only one is the window's
state. Nothing in `!blocked!` reads `@gl_stall`, which is why it was not changed
alongside it.

## Standing hazards in this code, and what each would cost

Three properties of the code below are load-bearing, and each has a guard
against it because the failure it permits is silent. What follows states the
defect and its price, not its history.

**The newest-terminator rule needs its weaker justification, not its stronger
one.** Two arguments support it. The strong one — a codex turn only ever begins
from an input, so a newer `task_started` proves something was sent — is false:
about an eighth of codex turns begin with no input record at all, and pure
auto-compaction turns are the clean case, since the harness opens those itself.
The weak one is unconditionally true: a newer terminator means the record can no
longer speak to the older turn, whatever opened it. The reader is correct under
the second and unsupported under the first. Cost of resting it on the strong
claim: a spec falsifiable in one command over the rollouts, and a maintainer who
checks it correctly concluding the reader is unsound when it is not.

**Delivery decides in two tiers and `send_live` decides first.** A state guard
present in `inject` alone does not run before `send_live` has already answered.
A blocked window still carrying busy paint from its finished turn then refuses
as "mid-turn" — the wrong reason, taken from a screen the ended turn left
behind — and a blocked window with a quiet screen is typed into and reported
delivered. That is the original defect reproduced inside its own fix, so both
tiers ask, in the state read's order, and `test/integration-hooks.sh` asserts
the refusal does not say "mid-turn" while that paint is on screen.

**`collars/codex.sh` defines `newest_lines(path)` twice.** `codex_action_read`
and `codex_blocked_read` each carry their own copy. Any edit, patch, or tool
that anchors on that signature resolves to the first occurrence, which is not
the reader a change to the blocked path means to reach; an edit bounded by it
duplicates the function instead of modifying it. The duplicate parses, the
collar sources without error, and the failure appears only when the stale copy
runs. The property is in the file today and applies to the next edit as much as
the last.

The first and third are the same failure wearing different clothes, and naming
the pair is the durable part. A suffix test on `_call` reads as a rule about
tool calls and is a rule about the last five characters of a name — which is why
it admitted `tool_search_output` and `mcp_tool_call_end`. A slice anchored on a
function signature reads as a rule about one function and is a rule about the
first match in a file. Both look specific and are general, and both hold right
up until something in the general set turns up that the specific reading never
imagined. **When a rule is written as a pattern, ask what else matches the
pattern rather than what the pattern was for.**

This is also why the guards for all three are executable rather than
review-time. Anything that slices code, generates code, or classifies a
vocabulary fails in ways reading does not look for: a duplicated function reads
as correct, a vocabulary gap reads as absent, and a justification that is merely
usually true reads as true. `test/probes/codex-hollow-turns.py` and the fixtures
in `test/integration-cli.sh` exist so those three claims are checked by running
rather than by inspection.
