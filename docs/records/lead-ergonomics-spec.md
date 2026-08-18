# Lead-ergonomics spec

> Status: Landed at `96982ca` on 2026-08-07; retained as a dated
> implementation record. Superseded in part at `fee26a8`: Gangline carries no PII
> scanner, so §5's account of the hook and its fixture step naming
> `tools/pii-scan` describe a file that no longer exists. The body is left as it
> was written; see `docs/DECISIONS.md`, "PII prevention belongs to Snubline".
> Superseded again at `f8410f8`: the "Suite isolation" requirement under
> Cross-cutting requirements asked for a second config-root pin, and an
> assertion to prove it, against a future fixture that would drop the first pin.
> Both are removed. The paragraph says in its own words that the case is a
> future one, and that is not a consumer.
> Superseded again in the "acceptance criterion is scoped" section and its test
> 3: `interrupt`, `flush` and `usage` are listed there as bare-error commands
> that must answer a bare invocation with a synopsis. Each takes one agent
> name, so each shipped with the self-target fallback instead, and the operator
> has ruled that behaviour the design — an agent reading or stopping its own
> state should not have to know its own name. See `docs/DECISIONS.md`, "A
> missing name is a self target".
> Superseded again in 2.0: §7's `gang usage` — the command that drove a
> harness's own usage page through its composer — is deleted, along with the
> `GANG_USAGE_CMD`, `GANG_USAGE_CONFIRM_KEY`, `GANG_USAGE_RENDER` and
> `GANG_USAGE_DISMISS_KEY` collar declarations it consumed. `gang limits` reads
> the same quota from each collar's non-interactive source. The known-dialog
> registry this document specifies is deleted in 2.0 as well; see
> `docs/DECISIONS.md`, "Occupancy is not authority".

Nine operator-directed changes, each born from friction observed in a live
marathon session. This document is the implementation contract: it leaves no
design decisions open. Where a value must come from a live harness capture
rather than from this document, the requirement says so explicitly and gives the
capture procedure.

Read `CONSTITUTION.md`, `AGENTS.md`, and `CONTRIBUTING.md` first. They bind
every change here.

## Verified harness pins

Every claim below about harness behaviour was read out of the installed
binaries, not from memory or vendor prose.

| Harness | Version observed | How it was read |
|---|---|---|
| claude-code | 2.1.224 | hook registry and Notification construction site in the bundled executable; `/usage` driven live in a disposable session |
| codex | 0.145.0 | embedded per-hook JSON schemas and the `HookEventNameWire` enum in the executable; `/usage` driven live in a disposable session |

The live captures were taken in a separately named disposable session
(`ergo-probe-claude`, `ergo-probe-codex`) on an explicit private tmux socket, and
only those exact sessions were deleted afterwards.

**claude-code 2.1.224** exposes a `Notification` hook. Its matcher field is
`notification_type`, whose complete value set is `permission_prompt`,
`idle_prompt`, `auth_success`, `elicitation_dialog`, `elicitation_complete`,
`elicitation_response`, `agent_needs_input`, `agent_completed`. The hook input
carries `hook_event_name: "Notification"` plus `message`, `title`, and
`notification_type`. Output is ignored on exit 0 (`stdout/stderr not shown`), so
a Notification hook cannot return `additionalContext` to its own agent — a note
raised there must travel as an ordinary Gangline message. `idle_prompt` is
raised by a timer whose threshold setting is `messageIdleNotifThresholdMs`, with
the message `Claude is waiting for your input`.

**codex 0.145.0** has no such event. Its complete hook set is `PreToolUse`,
`PermissionRequest`, `PostToolUse`, `PreCompact`, `PostCompact`, `SessionStart`,
`SessionEnd`, `UserPromptSubmit`, `SubagentStart`, `SubagentStop`, `Stop`. The
external notifier program the harness still carries is spelled `legacy_notify`
and fires `agent-turn-complete` with `thread-id`, `turn-id`, `cwd`, `client`,
`input-messages`, `last-assistant-message`. That is a turn-completion event,
which is what Gangline already consumes through the `Stop` hook, so it is not an
awaiting-input witness and nothing here wires it. Codex's only awaiting-input
witness is `PermissionRequest`, which the codex profile already installs.

This corrects the charter's premise: stall lights on codex cover permission
requests and nothing else, because codex 0.145.0 witnesses nothing else. That
gap is a harness fact to be stated, not a hole to be filled with a poller.

---

## 1. Contract truthfulness line

### Problem

The startup contract teaches one half of attribution — that peer messages name
their sender in the gang envelope — and leaves the complement unstated. An agent
that reads unenveloped text in its own pane has no supplied basis for
interpreting it.

### Change

In `bin/gang`, `startup_brief()` (two `printf` branches, doctrine-present and
doctrine-absent), insert one sentence immediately after
`Peer messages name their sender in the gang envelope.` and before
`To message a peer:`.

The sentence, byte-exact:

```
Gangline never delivers a message without one — the only unenveloped text it ever types into a pane is your harness's own compaction command — so any other unenveloped text arrived from the session keyboard, and Gangline cannot attribute it further.
```

Inside the single-quoted `printf` format the apostrophe is written `\047`, as
the surrounding sentences already do:
`your harness\047s own compaction command`.

The compaction clause is load-bearing, not hedging. `bin/gang` has exactly five
`inject` call sites: three carry `$ENVELOPE` (live send, spool drain, the hitch
brief itself) and two carry `$GANG_COMPACT_CMD` unenveloped
(`self_compact_after_stop`, `cmd_compact`). Without that clause the sentence is
false in the compaction case. Do not shorten it.

The sentence states mechanism only. It draws no conclusion about what an agent
should do with unattributed text; that is trust policy and it lives in the
operator's `DOCTRINE.md`, which every hitch already delivers.

### Tests

In `test/integration.sh`, beside the existing startup-contract assertions
(`startup carries the operator-authorized marathon rule`), add:

```sh
contains "startup states the complement of envelope attribution" \
  "$(pane alpha)" \
  "Gangline never delivers a message without one — the only unenveloped text it ever types into a pane is your harness's own compaction command — so any other unenveloped text arrived from the session keyboard, and Gangline cannot attribute it further."
```

Assert the same sentence against an agent hitched under
`GANG_CONFIG_DIR="$CONFIG_CASES/doctrine-present"`, so both `printf` branches are
witnessed. The doctrine-present fixture already exists in the suite.

### Documentation

`docs/reference.md` describes the startup contract; add the complement there in
the same words. No `CHANGELOG.md` edit.

### Commit

`feat(hitch): state the complement of envelope attribution`

---

## 2. Spool by default

### Problem

A dozen-plus live-delivery bounces off busy panes in one session. Each refusal
returns a full copy of the message body to the sender, who must then re-send it
by hand. Parking is already built, already verified, and already opt-in — the
default is simply the wrong way round.

### Flag surface after the change

```
gang send --to <name> [--from <sender>] [--live-only] [--supersede] --stdin
```

- **default** — attempt live delivery; on a refusal, park in the target's spool
  and report it as parked. On a failure after anything was typed, exit loudly
  and park nothing. Unchanged from today's `--spool`.
- **`--live-only`** — attempt live delivery and never park. This is today's
  no-flag behaviour, kept for availability probes.
- **`--supersede`** — retires the same sender's earlier spooled entries for that
  target once this message's own fate is settled, whether it was delivered live
  or parked. Now valid without a companion flag. Refused with `--live-only`,
  which never parks:
  `send: --supersede replaces an older message in the spool, and --live-only never parks one`
- **`--spool`** — accepted, does nothing, and says so on stderr once:
  `send: --spool is the default now and does nothing; drop it, and use --live-only for a delivery that must not park.`
  Exit status is unaffected.

`--spool` is deprecated rather than removed. Gangline is published, the flag is
documented in `docs/reference.md`, and `docs/DECISIONS.md` ("Unpublished renames
are complete") sets the precedent: after publication an external name is
preserved through normal deprecation. Announcing the no-op on stderr keeps it
from being a silent lie about what the flag did.

### The harnesses that cannot drain

A spool drains only on the target's own native `Stop` event. A profile that
declares no `GANG_STOP_HOOK` announces no turn boundary, so a message parked for
it would sit forever. Today `--spool` dies up front for such a target. Under a
default that parks, dying up front would break every ordinary send to
`opencode` and `pi`, whose profiles declare no Stop hook.

Correct behaviour: **for a target that cannot drain, the default degrades to
live-only, and the refusal names why nothing was parked.** The delivery attempt
still happens; only the parking is unavailable. The same applies to a window
carrying no spool identity, which `spool_write` today reaches as a fatal error.

Add beside the other spool helpers in `bin/gang`:

```sh
SPOOL_UNAVAILABLE_WHY=""
spool_available() { # $1 = window id, $2 = target name; 0 = a parked message would drain
  SPOOL_UNAVAILABLE_WHY=""
  if [ "${GANG_STOP_HOOK:-}" != 1 ]; then
    SPOOL_UNAVAILABLE_WHY="profile '$AGENT_PROFILE' declares no GANG_STOP_HOOK, so no native turn boundary reaches Gangline and nothing would ever drain a spool for '$2'"
    return 1
  fi
  if ! spool_existing "$1"; then
    SPOOL_UNAVAILABLE_WHY="the window of '$2' carries no spool identity — it is minted at hitch and adopt, so re-hitch or re-adopt '$2' before a refused message can park"
    return 1
  fi
  return 0
}
```

`spool_existing` still dies on a token Gangline did not mint. That is an
unaccountable directory, not an absent one, and it stays fatal.

### `--supersede` must also fire on a live success

Today supersession lives inside `spool_write`, which `cmd_send` reaches only
after a refusal. Under the old opt-in default that was complete, because
`--spool` implied the sender expected to park. Under a default that delivers
first it is a defect, and a reachable one: a sender parks A while the target is
busy, then sends replacement B with `--supersede` once the target is idle. B
verifies live, `spool_write` is never called, A stays in the spool, and the next
turn boundary delivers A **after** the message that was meant to replace it. The
operator-facing caution below promises the opposite.

Lift supersession out of `spool_write` into its own helper, and call it from
both outcomes:

```sh
spool_supersede() { # $1 = window id, $2 = sender; retire that sender's waiting entries
  local e
  spool_existing "$1" || return 0
  [ -d "$SPOOL_DIR" ] || return 0
  for e in "$SPOOL_DIR"/[0-9]*; do
    [ -f "$e" ] || continue
    spool_read "$e"
    [ "$SPOOL_SENDER" = "$2" ] || continue
    rm -f -- "$e" || die "cannot supersede the spooled message $e"
  done
}
```

`spool_write` loses its fifth parameter and its inline loop. `cmd_send` calls
`spool_supersede "$AGENT_ID" "$SEND_FROM"`:

- on the live-verified path, **after** `send_live` returns 0 and before the
  delivered line is printed;
- on the parked path, immediately before `spool_write`, which is where it
  effectively runs today.

**After, never before.** Retiring a predecessor before the replacement's own fate
is known would destroy A on the strength of a B that then failed hard, leaving
the target with neither. The ordering is the whole safety property.

The glob is `[0-9]*`, so an entry a concurrent drain has already claimed as
`sending-…` is untouched. That is correct: a claimed entry is one whose body may
already have reached the pane, and `docs/DECISIONS.md` forbids treating such a
message as recallable. Supersession retires what is still waiting, not what is
already gone.

### `cmd_send` control flow

Parse `--live-only` and `--supersede`; accept and warn on `--spool`; reject
`--supersede` with `--live-only`. After `resolve "$name"` and `envelope`:

1. If `--live-only`: call `send_live "$name"` at statement level so its refusal
   and exit status propagate unchanged, print the delivered line, return 0.
   `--supersede` cannot reach here; it is refused with `--live-only`.
2. Otherwise capture: `err="$(send_live "$name" 2>&1)" || rc=$?`.
   - `rc` 0 — if `--supersede`, `spool_supersede "$AGENT_ID" "$SEND_FROM"` now;
     then print the delivered line and return 0.
   - `rc` not 0 and not 3 — print `$err` to stderr and `exit "$rc"`. Unchanged.
   - `rc` 3 — continue.
3. On the refusal, evaluate `spool_available "$AGENT_ID" "$name"`. Evaluate it
   here, not earlier: the happy path should not read tmux options it will not
   use.
   - unavailable — print `$err` to stderr, then
     `printf 'send: NOT parked — %s. Send again when '\''%s'\'' is idle.\n' "$SPOOL_UNAVAILABLE_WHY" "$name" >&2`,
     and `exit 3`.
   - available — if `--supersede`, `spool_supersede "$AGENT_ID" "$SEND_FROM"`,
     then `spool_write` (now without its supersede parameter) and print the
     existing `spooled for … NOT delivered` line.

The `GANG_STOP_HOOK` precondition moves out of the `--spool` branch entirely; it
now lives only in `spool_available`.

### Existing guards

Two assertions in `test/integration.sh` encode the opt-in default and must be
rewritten, not deleted. Their underlying claim survives; only the surface that
carries it changed.

- `a harness that announces no turn boundary refuses the spool` (with
  `naming the declaration a drain would need` and
  `and refusing the flag delivers nothing`). The old expectation — that the flag
  is refused before any delivery attempt — is wrong under a default that parks,
  because refusing before the attempt would refuse every ordinary send to a
  no-Stop-hook harness. Rewrite as the NOT-parked path below. The claim it
  guards, that Gangline never holds a message nothing would drain, is preserved
  exactly.
- `superseding without a spool is refused` / `because there is nothing for it to
  replace`. `--supersede` alone is now the ordinary parking form. Rewrite as the
  `--supersede --live-only` refusal, which is the same claim about the same
  emptiness.

### New tests

All fixtures refuse by the mechanism the suite already uses: a human draft typed
into the target composer with `tmux send-keys -l`. Every assertion reads an
artifact — the spool directory, the pane, or the reported count — never an exit
status alone.

1. **Parks with no flag.** Draft into `parker` (the existing `spoolable`
   fixture, `GANG_STOP_HOOK=1`), send with no flag, assert `spooled for parker`
   on stdout and `status parker` reporting one more entry.
2. **`--live-only` never parks.** Record the entry count, draft into `parker`,
   send with `--live-only`, assert it exits non-zero, that `status parker`
   reports the same count, and that the pane never received the body.
3. **A target that cannot drain is not parked, and says why.** Add
   `$RUN_ROOT/profiles/nodrain.sh` sourcing `profiles/bash.sh` with the same
   `GANG_LAUNCH` as `spoolable.sh` and **no** `GANG_STOP_HOOK`. Hitch it, draft
   into it, send with no flag, and assert: exit 3, stderr contains
   `NOT parked` and `GANG_STOP_HOOK`, the pane never received the body, and no
   spool directory exists for that window.
4. **`--spool` is an announced no-op.** Draft into `parker`, send with
   `--spool`, assert stderr contains `is the default now` and that the message
   parked all the same.
5. **`--supersede` stands alone.** Park two bodies from one sender, the second
   with `--supersede` and no other flag, assert `status parker` reports one.
6. **`--supersede --live-only` is refused**, naming `--live-only`.
7. **A live-verified `--supersede` retires the predecessor.** Draft into
   `parker` and park A. Clear the draft so the pane is idle, then send B with
   `--supersede`. Assert B was delivered live (the body reached the pane) and
   that `status parker` now reports **zero** waiting. Then fire a Stop event and
   assert the pane never gains A's body. Without the live-path call this test
   goes red on the last assertion, which is the defect stated exactly: a
   replaced message arriving after its replacement.
8. **A hard failure supersedes nothing.** Same setup, but B fails at a
   non-refusal exit status; assert A is still waiting. This is the guard on the
   ordering — supersession after the outcome, never before it.

### `--supersede` is sender-scoped, not topic-scoped

`--supersede` drops **every** message the same sender has waiting for that
target, not only ones on the same subject. That is what the code does and what
`docs/DECISIONS.md` says; what is missing is the operator-facing caution, and
its absence has already cost a message — an amendment batch was destroyed by a
later `--supersede` from the same sender on an unrelated topic.

Add to `docs/reference.md` §`gang send`, beside the existing `--supersede`
sentence:

> It is scoped to the sender, not to a subject: a sender with two unrelated
> messages waiting for one target loses the first when the second carries the
> flag, whether that second message parks or is delivered live. Pass it only
> when the newer message genuinely replaces everything that sender has parked.

**Do not add topic scoping.** A subject, thread, or topic key would be a
coordination schema, and `docs/DECISIONS.md` §"Gangline is substrate, not
coordination" forbids exactly that: Gangline defines no reporting protocol and
no message taxonomy. The sender knows which of its own messages are still
relevant; the substrate does not and should not learn.

### A held entry is unverified, not undelivered

Observed in the live session: a spool drain pasted a body, watched the composer
change, pressed Enter, and the harness parked the submission in its own input
queue rather than submitting it. `submit_verify`'s post-Enter queue check caught
that correctly, the entry was held rather than re-sent, and `gang flush` later
refused because the parked-body record was gone. The harness then drained its
own queue, so the message **did** reach its target while Gangline was reporting
it as not verified.

Every step of that was right, and the spec changes no mechanism:

- Holding rather than re-sending is the rule (`docs/DECISIONS.md`: Gangline
  never sends a message a second time on the chance the first did not arrive).
  A false negative in this direction is the safe one; a second copy is not.
- `flush` refusing was right too. By the time it ran, the harness had drained
  its own queue, so the recorded body had been retired by `stage_clear` and
  there was nothing to recall. Pressing the recall key on that composer would
  have submitted whatever happened to be there.

What is wrong is only the **words**. "Held" and "not verified" are read by an
operator as "not delivered", and they do not mean that. Fix the reporting:

- `gang status <name>` — the held-entry line reads
  `held (delivery NOT verified — it may still have arrived): <fragment>`
  rather than any phrasing that asserts non-delivery.
- `docs/reference.md` §`gang send` — state that a held entry means Gangline
  could not verify the delivery, not that the message failed to arrive: a
  harness that parks a submission in its own queue may drain it later.
- `docs/operations.md` — the recovery is to read the target before re-sending
  by hand. Gangline will not decide that for the operator, and the reason it
  will not is that it cannot see the harness's internal queue.

**Fixture.** Add a suite case that reproduces the whole sequence against the
existing `spoolable` fixture: arm the fixture to paint queue evidence after
Enter, drive a drain, assert the entry is held and that the reported wording
says unverified rather than undelivered, then clear the queue evidence to
simulate the harness draining itself and fire another Stop event. **Assert that
nothing is re-sent** — the held entry stays held and the target's pane gains no
second copy. That last assertion is the point of the fixture: the arrival must
not become a reason to deliver again.

### Documentation

- `bin/gang` `usage()` — the `gang send` line.
- `docs/reference.md` §`gang send` — rewrite the three `--spool` paragraphs to
  describe parking as the default, `--live-only` as the probe, the deprecated
  `--spool`, and the NOT-parked degradation with its reason. Present tense, no
  account of the change.
- `docs/DECISIONS.md` §"A refused delivery may be spooled, a failed one may not"
  — retitle to **"A refused delivery is parked, a failed one is not"** and edit
  the body so parking is the default and `--live-only` is the explicit probe.
  Delete `Spooling is opt-in per send,` and state instead that a profile whose
  harness announces no turn boundary degrades to live-only and names the missing
  declaration in its refusal. Leave every other sentence of that entry alone.
- `docs/operations.md` §"Sending messages safely" — one sentence that an
  unattended sender no longer needs to re-send a bounced message by hand.

### Commit

`feat(send)!: park a refused message by default`, with a `BREAKING CHANGE:`
footer naming the exit-status change (a refused send to a drainable target now
exits 0 as parked; pass `--live-only` for the old behaviour) and the `--spool`
deprecation.

---

## 3. Stall lights

### Problem

An agent waiting on input is invisible to everyone but whoever attaches. The
harnesses themselves witness it; nothing carries that witness anywhere.

### What may not be built

No patrol, no timer, no watcher (`CONSTITUTION.md` law 7, `docs/DECISIONS.md`
"Evidence is selected per predicate"). The only sources are native hook
deliveries. Gangline translates a fact it was handed and stops.

### There is no lead

`bin/gang` has no lead concept; `lead` is only the default name `gang up` gives
its first agent. Roles are coordination, which Gangline does not do. The notify
target is therefore an **optional operator declaration**, in the exact shape
`gang cutoff` already establishes: one session-scoped value, no default, no
inference, no enforcement.

### `gang notify`

```
gang notify <name>     declare which agent receives stall notes
gang notify clear      remove the declaration
gang notify            report it
```

Stored as `@gl_notify` on the session
(`tmux set-option -t "=$SESSION:" @gl_notify "$name"`). Validate `<name>` with
`valid_name` and refuse otherwise. Do **not** require the window to exist at
declaration time — the declaration and the hitch may arrive in either order,
as with `cutoff`. With nothing declared, `gang notify` prints
`no notify target declared` and exits 0.

Deletion path (law 6): the declaration is a session option and dies with the
session; `gang notify clear` removes it sooner. State that in
`docs/reference.md`.

Add the command to `usage()` and to the `docs/reference.md` command list.

### Profile surface

`profiles/claude-code.sh` — add `Notification` to the hooks JSON built for
`--settings`, beside the existing four:

```
"Notification":[{"hooks":[$_gl_cc_cmd]}]
```

and declare the awaiting-input value set as data:

```sh
# AWAITING INPUT IS THE HARNESS'S OWN WORD. Observed on claude-code 2.1.224:
# the Notification hook matches on notification_type, whose complete value set
# is permission_prompt, idle_prompt, auth_success, elicitation_dialog,
# elicitation_complete, elicitation_response, agent_needs_input,
# agent_completed. These four are the ones that mean a person is being waited
# on; the other four report something that finished. A value not in this list
# is not a stall — a renamed one stops raising notes rather than raising wrong
# ones, and re-verifying this list is what a version bump costs.
GANG_STALL_TYPES="permission_prompt idle_prompt elicitation_dialog agent_needs_input"
```

`profiles/codex.sh` — no wiring change; its `PermissionRequest` hook is already
installed. Add a comment recording the verified pin: codex 0.145.0's hook set
contains no Notification event, and its `legacy_notify` / `agent-turn-complete`
program reports turn completion, which the `Stop` hook already delivers, so it
is not an awaiting-input witness and is deliberately not wired.

`profiles/opencode.sh`, `profiles/pi.sh` — no hooks, so no stall lights. Not a
defect; a harness that reports nothing is reported on by nothing.

### Mechanism

Add to `bin/gang`:

```sh
GANG_STALL_REPEAT=600
```

as a script constant, not a config key. It has no consumer asking to tune it
(law 5), and the suite exercises the repeat path by writing an old stamp into
the window option rather than by waiting.

Why 600 and not less: too short and a single stall fills the target's pane with
duplicates of one fact, which is the friction this feature exists to remove; too
long and a genuinely long stall is announced once and, if missed, waits — and
that miss is recoverable with `gang roster`, while a flooded pane is not.

```sh
stall_note() { # $1 = window id, $2 = kind; never fatal
```

1. Read `@gl_notify` off the session. Empty — return 0. The operator declared no
   target; that is the off state, exactly as context lights are off by default.
2. Resolve the target name to a window. No such window — record
   `@gl_stall_failed` on the raising window with the reason and return 0.
3. Target window is the raising window — return 0, recording nothing. A stall
   note into the stalled pane is noise, and the delivery would be refused anyway.
4. Debounce. Read `@gl_stall` on the raising window as `<kind> <epoch>`. Return 0
   if the recorded kind equals `$2` **and** `now - epoch < GANG_STALL_REPEAT`. A
   malformed value is treated as absent and overwritten; this option is written
   only here.
5. Build the body, one line:
   `stall: <raising agent> is awaiting input (<kind>) — inspect with gang capture <raising agent>`
   Envelope it with the raising agent as sender — the fact originates in that
   window, and `agent_name_of` reads the name off the window, so the attribution
   law holds with no claimed identity anywhere.
6. Send through the ordinary path: live delivery, and on a refusal park it if
   `spool_available` says a parked message would drain. This is the same code
   change 2 installs; a stall note is an ordinary message and gets no private
   transport (law 1). **Synchronously — never backgrounded.** A hook that
   returns before the send's live, parked, or failed outcome has resolved cannot
   truthfully record what happened.
7. **On success** — accepted by live delivery or parked where a drain will reach
   it — write
   `@gl_stall "$2 $(date +%s)"` and unset `@gl_stall_failed`. Return 0.
8. **On failure**, record `@gl_stall_failed` on the raising window with the
   reason, leave `@gl_stall` untouched, and return 0. A hook may not kill its
   harness; this mirrors `spool_drain_dispatch`, which records
   `@gl_spool_failed` the same way.

### The debounce is a record of a note accepted live or parked

Steps 7 and 8 are ordered deliberately, and both halves of the ordering are
load-bearing.

**The stamp is written after acceptance, not before it.** `@gl_stall` exists to
suppress a duplicate of a note the target either saw live or already has parked
for its next drain. A note that failed to be accepted in either place is not a
duplicate of anything, so stamping before the attempt would buy silence for a
fact nobody has and would suppress the retry that repairs it. Written afterwards,
the option means what its name implies: a note of this kind was accepted live or
parked, and another one is noise until the repeat bound elapses. The cost of the
failure path having no stamp is that a stalled agent re-attempts the send on each
native event until one is accepted — attempts that are cheap, bounded by how
often the harness raises the event, and each one a chance for the light to start
working again.

**A failure is retired only by a later note accepted live or parked.** Nothing
else clears `@gl_stall_failed`: not a movement event, not a roster read, not time. A
declaration naming a window that does not exist yet is the ordinary case — the
operator may declare `gang notify lead` before hitching `lead` — and the failure
must stay visible for exactly as long as it is still true. The moment a note
is accepted live or parked, the condition that produced the failure is gone and
the light goes out on the evidence of that outcome, not on a guess. Movement
events clear `@gl_stall` and deliberately leave `@gl_stall_failed` alone: an
agent moving on says nothing about whether its notes can be delivered.

`gang status <name>` and `gang roster` report a set `@gl_stall_failed` beside
the spool-failure reporting they already do. That is where a silently broken
stall light becomes visible (law 8).

### `cmd_hook`

```sh
Notification) stall_note "$TMUX_PANE" "$(notification_type from payload)" ;;
```

guarded so only a `notification_type` present in the profile's
`GANG_STALL_TYPES` raises a note. Parse the field with the same
`python3 -c 'import json,sys; …'` one-liner shape `cmd_hook` already uses for
`hook_event_name`; an unparseable payload raises nothing.

`PermissionRequest` keeps `occupied_raise_write` and additionally calls
`stall_note "$TMUX_PANE" permission_prompt`. On claude-code both events fire for
one prompt; the debounce collapses them because the kind is identical. On codex
this is the only stall source there is.

Clear `@gl_stall` on `UserPromptSubmit`, `PostToolUse`, and `Stop` — each is
proof the agent moved. Clearing on `PostToolUse` is what lets a second
permission prompt inside one turn raise its own note: the tool that ran between
them ended the first stall.

### Tests

Drive `gang hook` with fabricated stdin, as the suite already does for the
malformed-config case. Every assertion reads the target's pane or a window
option; `gang hook` returns only after its live, parked, or failed outcome has
resolved, so no
assertion depends on timing.

1. Declare `gang notify beta`, fire `Notification` / `idle_prompt` in `alpha`'s
   pane, assert `beta`'s pane carries `[gang:alpha#` and
   `alpha is awaiting input (idle_prompt)`.
2. Fire it again immediately; assert `beta`'s pane gained nothing.
3. Fire `Stop` in `alpha`, then `Notification` / `idle_prompt` again; assert a
   second note reached the pane.
4. Rewrite `@gl_stall` on `alpha` with an epoch older than `GANG_STALL_REPEAT`,
   fire the same kind, assert a note reached the pane. This is the repeat path,
   exercised through state and not through wall time.
5. Fire `Notification` / `auth_success`; assert the target pane gained nothing.
6. `gang notify clear`, fire `Notification` / `idle_prompt`, assert nothing
   reached the target pane and the hook exited 0.
7. `gang notify alpha`, fire in `alpha`; assert the target pane gained nothing
   and `@gl_stall_failed` is unset.
8. `gang notify ghost` (no such window), fire in `alpha`; assert
   `@gl_stall_failed` is set and `gang status alpha` reports it.
9. **A failure does not debounce the repair.** Continuing from 8 without
   changing the clock: assert `@gl_stall` is unset, hitch `ghost`, fire the same
   kind again, and assert the note reached `ghost`'s pane, `@gl_stall_failed` is
   now unset, and `gang status alpha` no longer reports it. This is the
   failure→success path, and it fails against a build that stamps before
   delivering (the second fire is swallowed) and against one that never retires
   the failure (status still reports it).
10. **Movement does not retire a failure.** From 8, fire `Stop` in `alpha` and
    assert `@gl_stall_failed` is still set. A light that is still broken keeps
    saying so.
11. `gang notify` with a name `valid_name` rejects is refused.

### Documentation

- `docs/reference.md` — `gang notify` with its three forms, the `@gl_notify`
  deletion path, the note's shape, the debounce and its constant, and a table
  row for `GANG_STALL_TYPES` in the profile contract.
- `docs/operations.md` — stall lights under unattended operation: what raises
  them per harness, and the honest statement that codex 0.145.0 witnesses only
  permission requests.
- `docs/DECISIONS.md` — a new terse entry:

  > ## A stall light is a harness's own witness, forwarded
  >
  > Where a harness itself reports that it is waiting on a person, deliver that
  > fact as an ordinary attributed message to one optional operator-declared
  > target. Nothing polls, nothing infers a stall from a quiet pane, and nothing
  > infers a lead: the target is a declaration in the shape of the team cutoff,
  > and with none declared there are no stall lights. A repeated report of the
  > same kind inside one stall is one note, cleared by the harness's own next
  > move. A harness that reports nothing gets no substitute, and a delivery that
  > fails is recorded on the window for status to surface rather than killing
  > the hook — a record retired only by a later note accepted live or parked,
  > because a light that is still broken has to keep saying so.

### Commit

Two: `feat(hook): forward native awaiting-input events as stall notes` and
`feat(profiles): wire the claude-code Notification hook`.

---

## 4. Known-dialog triage

### Problem

Codex 0.145.0 raises a transient menu when a response is slow. Verified strings
from the installed binary, internal id `safety-buffering-prompt`:

- `Our systems are thinking a bit more about this request before responding.`
- `Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.`
- option: `Retry with a faster model`
- option: `Dismiss and keep waiting`
- footer: `No action is required. Codex will keep waiting, and this menu will close when the response is ready.`

It occupies the composer, so `GANG_OCCUPIED_REGEX` matches and every send to
that agent is refused as `occupied (authority unknown)` until a person
dismisses it — a menu whose own footer says no action is required.

### The three buckets

Operator doctrine, recorded here because it decides what may ever be
registered:

- **AUTO-ANSWER** — benign transients that carry no authority. The proven case
  is the menu above; the safe option is `Dismiss and keep waiting`, and the
  operator's standing rule is always that option.
- **STALL-LIGHT** — unknown dialogs. Behaviour is unchanged: occupied and
  refused. A stall note follows only where the harness itself raised one of the
  events change 3 forwards, which is not the same set: on claude-code a
  permission or elicitation dialog raises `Notification`, but **the codex menu
  above raises nothing**. It is not a `PermissionRequest`, and codex 0.145.0 has
  no other awaiting-input event, so an unrecognized codex dialog produces
  occupancy and a refusal and no light at all. Stating that is the point of the
  bucket; promising a note here would be the same false claim change 3 was
  written to avoid.
- **LOUD-MANUAL** — trust prompts, permission prompts, approval dialogs, and
  anything that grants or widens access. **Never** auto-answered. These are the
  prompt-injection boundary: a dialog's text can be written by whatever the
  agent just read, so a component that answers dialogs by matching their text
  must never be able to answer one that decides access.
  `docs/operations.md` already holds this stance and keeps it.

### Registry

Harness knowledge, so it lives in the profile as data (law 4). A dialog is
declared in two parts: a record naming how it behaves, and a block holding
exactly what it says.

```sh
GANG_DIALOGS='<id>|<marker>|<safe>|<move>|<confirm>'      # one record per line
GANG_DIALOG_LINES_<id> ='<every line of the dialog, in painted order>'
```

- `id` — a short slug, `[a-z0-9-]+`, used in messages and in the block's
  variable name with `-` written `_`. Read back with `${!var}`, which bash 3.2
  supports.
- `marker` — an ERE, anchored at line start, matching the prefix the harness
  paints on the **selected** row and on no other. Codex 0.145.0: `^› [0-9]+\. `.
  It is the same shape as `GANG_OCCUPIED_REGEX`, which a profile already writes.
- `safe` — the label of the option that carries no authority. Must be one of the
  block's lines.
- `move` — space-separated `tmux send-keys` key names that move the selection to
  the safe row. May be empty when nothing needs moving; it is never trusted, only
  checked (below).
- `confirm` — the single key name that commits the selection.

The dialog's own text goes in the block rather than in a record field because
its lines contain commas, and would eventually contain whatever in-field
separator was chosen instead. A newline-separated block needs no escaping and
reads in the profile as the thing it is.

**The block holds every line, not only the selectable ones.** A dialog is
explanatory sentences plus rows plus a footer, and all of it is fingerprint: the
first line bounds the region above, the last bounds it below, and the lines
between are what proves the dialog is still the one that was enumerated.

**A row count would not work, and observation is why.** In the live codex
0.145.0 capture below, the selection marker *and* the row number are painted
only on the currently-selected row; every other row is blank-prefixed:

```
› 1. Show usage                View recent account token usage.
     Redeem usage limit reset  No usage limit resets available.
```

Counting lines that match `GANG_OCCUPIED_REGEX` therefore returns 1 for a
two-row dialog and would return 1 for a dialog that grew a third option — the
exact change the fingerprint exists to catch. Matching the whole block as an
ordered run is what actually detects a dialog that is no longer the one the
profile enumerated, whether it was reworded, reordered, or grew an option.

### Matching is width-tolerant by construction

Before comparing, both the capture and the declared block are normalized: each
line has runs of whitespace collapsed to one space and its ends trimmed, and
blank lines are dropped. `capture_joined` already passes `-J`, which rejoins
lines the terminal soft-wrapped. Between them, the two things that vary with
pane width — soft wrapping and the padding of a two-column row — are removed
before any comparison, so a block captured at one width matches at another. This
matters directly: the operator reads this tool over phone SSH, and a fingerprint
that only matched at 80 columns would silently stop recognizing dialogs at 48.

### `load_profile` refusals

A malformed registry is refused at load: a record without exactly five fields, an
empty `id`, `marker`, `safe`, or `confirm` (only `move` may be empty), an `id`
outside `[a-z0-9-]+` or repeated across records, a missing or empty
`GANG_DIALOG_LINES_<id>`, or a `safe` that is not one of that block's lines
after normalization.

**`load_profile` also refuses any registry whose `id`, `safe`, or *any line of
whose block* matches**
`(trust|permission|approve|allow|full access|sandbox|credential|token|secret)`
**case-insensitively.** This is the LOUD-MANUAL bucket made mechanical. The scan
covers the whole block rather than the labels alone because the sentence that
makes a dialog dangerous is usually not its button text — "Do you want to allow
this tool to run?" carries its authority in the question, not in `Yes`. Refusing
too much is the safe direction and is loud and fixable; refusing too little
auto-answers a permission prompt.

It runs at `load_profile` rather than in `test/lint.sh` because it must also bind
operator-supplied profiles under `GANG_PROFILES`, which lint never reads, and
because it should fail at the moment a dangerous registry would be consulted.

### Where triage runs

Only where Gangline is already about to write to that pane:

- `send_live`, immediately before its occupancy refusal.
- `wait_ready`, in the hitch readiness loop.

**Not** in `occupied()` itself. `gang status` and `gang roster` call `occupied()`
and must stay read-only; a read command that presses keys is a surprise nobody
asked for. Those commands instead gain naming: a recognized dialog is reported
as `occupied (known transient: <id>)` rather than the bare
`occupied (authority unknown)`, so the operator gets the diagnosis without the
keystroke.

This narrows the charter's "send/status/hitch" to "send/hitch, with status
naming only". Recorded here so the reviewer sees it as a decision rather than an
omission.

### Algorithm

```sh
dialog_triage() { # $1 = window id; 0 = a known dialog was answered and cleared
```

1. `[ -n "${GANG_DIALOGS:-}" ]` or return 1.
2. Capture the pane once (`capture_joined`) and normalize it as above.
3. For each record, match the normalized block as a **contiguous ordered run**
   in the normalized capture. Every line must be equal, in order, except that
   exactly one capture line in the run may carry the selected-row prefix: strip
   `marker` from it before comparing. No match if the run is absent, if any line
   differs, or if the number of run lines matching `marker` is not exactly one —
   zero or two means Gangline cannot tell what is selected, and it does not press
   a key on a dialog it cannot read.
4. More than one record matching is ambiguity — `die`, naming both ids.
5. `lock_pane` is already held by the caller on the `send_live` path; on the
   `wait_ready` path take it. Never press a key on an unlocked pane.
6. Send each key name in `move` with `tmux send-keys -t "$1"`, one call per name.
   Nothing is committed by this; `move` only changes which row is highlighted.
7. **Prove the selection before committing it.** Re-capture and normalize. The
   block must still match by the rule in step 3, and the single line carrying
   `marker` must, with the marker stripped, equal `safe`. If either fails, `die`
   naming the id, the keys sent, and `gang attach`. **Do not press `confirm`.**
8. Send `confirm`.
9. **Verify it cleared.** Re-read the pane: the block's first line must be gone
   **and** `profile_input` must read a composer. If either fails, `die` naming
   the id, the keys sent, and `gang attach` — the window is now in a state
   Gangline caused and cannot account for, which is the loudest case there is.
10. On success, print one line to stderr:
    `answered the known transient '<id>' in '<name>' with its safe option (<safe>)`,
    so the answer is never invisible, and return 0. The caller re-reads occupancy
    and proceeds.

### Why the selection is read rather than assumed

Steps 6–8 are split — move, read, then confirm — and the split is the security
property of this whole item.

Pinning the initial selection in the record and trusting `move` to walk from it
would make the answer only as good as an assumption written from a single
capture. A menu that reopens with a different row highlighted, or whose options
are reordered in a later harness version, satisfies every static predicate while
`move` lands somewhere else: with `Down Enter` pinned against a two-row menu that
happens to open on row 2, the `Down` wraps to row 1 and the `Enter` confirms
`Retry with a faster model` — the option that is not the safe one. The pane is
the only thing that knows what is actually highlighted, so the pane is asked,
after the move and before the commit.

Step 9 cannot substitute for this. Confirming the *wrong* option also clears the
dialog and also restores a composer, so a post-confirm check reports success
either way. **The only place a wrong answer can be caught is before the confirm
key**, which is why the confirm key is reached exclusively through step 7.

The same split removes the need to reason about wrap-around, key repeat, or
whether a `Down` at the bottom stops or cycles. Those become observations
instead of assumptions: a move that lands wrong is a loud refusal, not a
keystroke.

### The codex entry — capture required before landing

The block and the `marker`, `move`, and `confirm` fields must be pinned to an
observed rendering of **this** dialog. Its strings are verified above; its
painted layout is not, and this document will not invent it.
`CONTRIBUTING.md` already requires this: "Marker changes must name the harness
version that was observed and add it to the profile's verified pins."

Corroborating evidence from a different codex 0.145.0 dialog, captured live (the
`/usage` selection menu, reproduced in item 7): the selected row is painted
`› <n>. <label>` and the footer reads
`Press enter to confirm or esc to go back`. If the slow-response menu paints the
same way, `marker` is `^› [0-9]+\. `, `move` is `Down`, and `confirm` is `Enter`.
Expect that; verify it anyway, because a different dialog is corroboration and
not observation.

Procedure:

1. Start a separately named disposable Gangline session on an explicit private
   tmux socket, per `AGENTS.md`. Never the live `gangline` session and never the
   development agent.
2. Hitch a codex agent and drive it until the menu appears (a long,
   heavily-reasoning request is the reliable trigger).
3. `tmux capture-pane -pJ -e` the pane and record it verbatim in the commit body.
4. From that capture, read: every line of the dialog in painted order, the
   prefix the harness paints on the selected row, and which key or keys move the
   selection to `Dismiss and keep waiting`.
5. Write the record and its block, expected shape:

   ```sh
   # Verified against codex 0.145.0 — capture in the commit body.
   GANG_DIALOGS='safety-buffering|^› [0-9]+\. |Dismiss and keep waiting|<move>|<confirm>'
   GANG_DIALOG_LINES_safety_buffering='Our systems are thinking a bit more about this request before responding.
   Hang tight or retry with a faster model for a quicker response, though it may be less capable of handling complex requests.
   Retry with a faster model
   Dismiss and keep waiting
   <footer line as painted>'
   ```

   Written with no leading indentation in the profile; the indentation above is
   this document's.

6. Confirm the answer clears the menu in that same disposable session before
   landing.

Checked against the forbidden-word scan: none of `safety-buffering`,
`Dismiss and keep waiting`, or the five block lines above contains
`trust`, `permission`, `approve`, `allow`, `full access`, `sandbox`,
`credential`, `token`, or `secret`, so this entry loads. If the observed footer
differs from the one recorded above and introduces one of those words, the scan
refuses the profile and the entry does not land — that is the scan working, and
it is not to be relaxed to accommodate a specific dialog.

### The capture is a precondition, not an extra

**If the menu cannot be reproduced, item 4 does not land.** Not the mechanism,
not the `load_profile` refusals, not the status naming.

Landing the machinery with `GANG_DIALOGS` unset in every shipped profile would
put a keystroke-pressing component into `bin/gang` that nothing shipped can
reach, which is exactly what law 5 forbids, and the suite fixtures do not rescue
it: a fixture profile is a test of a consumer, not a consumer. The live consumer
for this item is the codex registry entry, because the friction this item exists
to remove is a real menu blocking real sends on the operator's real harness.
Without that entry there is no consumer at all.

The menu is not rare — it is why this item is in the charter — so the ordinary
outcome is that the capture is taken during the arc. If it cannot be, item 4
defers to the next release and the other eight land without it. That costs one
feature; landing inert machinery costs the law that keeps `bin/gang` small, and
it costs it permanently, because nothing ever deletes a surface that was already
merged.

### Tests

Fixture profiles under `$RUN_ROOT/profiles`, sourcing `profiles/bash.sh`, with a
pane painted to look like a numbered menu. No real harness.

1. **A known dialog is answered and the send proceeds.** Paint the fixture
   dialog with the marker on the unsafe row, send, assert the body reached the
   pane and stderr named the id.
2. **A dialog that grew a row is not answered.** The same block plus one
   undeclared non-blank line inside it; assert the send refuses as occupied and
   that the pane still shows the dialog — the artifact, not the exit status.
   This is the assertion the row-count design would have passed while pressing a
   key, and the explanatory lines of the block are what it walks over.
3. **A reordered dialog is not answered.** The same lines, two of them swapped;
   assert the same. Ordered-run matching is what this proves, and a set-membership
   match would pass it.
4. **A dialog whose safe label is absent is not answered**, same assertions.
5. **The selection is read before the confirm key.** Fixture whose `move` key
   moves the marker to the *wrong* row. Assert the command dies naming the id and
   `gang attach`, that the dialog is still on screen, and — the load-bearing
   assertion — that **the confirm key was never pressed**, witnessed by a fixture
   that records each key it receives to a file. This is the finding-1 exploit as
   a guard: it must go red against a build that pins the initial selection and
   trusts `move`.
6. **Two markers, or none, is not a match.** Two fixtures, one painting the
   marker on both rows and one on neither; assert both refuse as occupied with no
   key pressed.
7. **Confirm keys that do not clear the dialog fail loud.** A fixture whose
   `confirm` does nothing; assert the command dies naming the id and
   `gang attach`.
8. **The match survives a width change.** Paint the fixture dialog in a pane
   resized narrow enough to soft-wrap its longest line and to change the padding
   of its two-column row; assert it is still answered. This is the normalization
   rule, and it goes red against a build that compares raw capture lines.
9. **`load_profile` refuses a registry naming a security surface.** Two fixtures:
   one with a forbidden word in `safe`, one where the only occurrence is in an
   explanatory line of the block (`Do you want to allow this tool to run?` with
   `Yes`/`No` rows). Assert `gang roster` fails naming the profile file and the
   forbidden word. The second fixture is the one that proves the scan covers the
   block and not just the labels.
10. **`load_profile` refuses a malformed record** — four fields; an `id`
    outside `[a-z0-9-]+`; a `safe` absent from the block; a record whose
    `GANG_DIALOG_LINES_<id>` is unset.
11. **Two matching records are ambiguity**; assert the die names both ids.
12. **`gang status` names a known dialog and presses nothing.** Paint the
    dialog, run `status`, assert `known transient` in the output and that the
    pane is byte-identical afterwards.

Guard-order requirement (`docs/DECISIONS.md`, "A guard witnesses the artifact"):
tests 2, 3, 5, 8, and 12 must be shown to go red against a build that has the
defect each one names — for 5, a build that confirms without re-reading the
marker — and not merely against the pre-feature build. Record that in the commit
body.

### Documentation

- `docs/reference.md` — the `GANG_DIALOGS` record format and its
  `GANG_DIALOG_LINES_<id>` block, the `load_profile` refusals, where triage runs,
  and both verifications: the selection read before the confirm key, and the
  cleared dialog read after it.
- `docs/operations.md` — replace `Gangline never answers permission dialogs.`
  with the precise current rule: Gangline answers only dialogs a profile
  enumerates as carrying no authority, verifies the answer cleared them, and
  never answers a permission, trust, or approval surface — those are refused
  into `occupied (authority unknown)`. A stall note follows only when a native
  harness hook witnesses an event change 3 forwards; coverage is not universal.
  Claude Code can raise notes for declared `Notification` kinds and permission
  requests. Codex has no `Notification` hook, so only its permission requests
  are witnessed; an unknown codex dialog can be refused with no stall note at
  all. Keep the prompt-injection reasoning and state this coverage limit next to
  the refusal rule.
- `docs/DECISIONS.md` §"Occupancy is not authority" — the sentence "Gangline does
  not autonomously answer native dialogs" is no longer true and must be
  rewritten, not annotated. Replacement for that sentence:

  > A profile may enumerate transient dialogs that carry no authority at all,
  > naming each one's whole painted shape and the keystrokes that pick its safe
  > option; Gangline answers such a dialog only where it is already about to
  > write to that pane, only on a whole-shape match, only once the pane itself
  > shows the safe option selected, and only if the pane afterwards proves the
  > dialog gone and a composer present. A dialog that grants,
  > widens, or trusts is never enumerable — a registry matched against on-screen
  > text must not be able to answer the one dialog whose text an agent's own
  > reading can influence — and an unrecognized dialog stays occupied, refused,
  > and reported.

### Commit

One: `feat(gang): answer enumerated benign dialogs where gang already writes`,
carrying both the mechanism and the codex registry entry, with the live pane
capture in its body. They do not split: the mechanism without the entry is a
surface with no consumer, and the entry without the mechanism does nothing.

---

## 5. Issue #106 — pre-push lints a tree with no `.git`

### Problem

`.githooks/pre-push` exports the pushed commit with `git archive | tar` into a
temporary directory and runs `test/lint.sh` there. That tree has no `.git`, so
any lint check that shells out to git — `git ls-files` being the natural one —
dies with `fatal: not a git repository` and refuses every clean push. Latent
only because `test/lint.sh` does not currently invoke git.

### Change

Replace the archive export with a detached worktree of the pushed SHA.

Verified in this checkout: `git worktree add --detach` works even from a linked
worktree, where `.git` is a file pointing into the main repository's
`.git/worktrees/` directory rather than a directory of its own; the resulting
tree resolves `git ls-files`, and `git worktree remove --force` cleans it up
with no stale administrative entry. The repository
has no `.gitattributes`, so no `export-ignore` or `export-subst` behaviour is
lost with the archive.

In the per-ref loop, replacing the `work=…; git archive … | tar -x -C "$work"`
block:

```sh
n_ref=$((n_ref + 1))
work="$tmp/tree-$n_ref"
( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
  git worktree add --detach --quiet "$work" "$local_sha" ) || {
  echo "pre-push: cannot check out $local_sha into a worktree, so nothing was linted" >&2
  fail=1; continue
}
```

with `n_ref=0` initialized beside `fail=0`. The counter, not the SHA, names the
directory: two refs can push the same commit.

**The environment is the whole point of the fix.** Git exports `GIT_DIR` to its
hooks. A `git ls-files` running inside `$work` with `GIT_DIR` still pointing at
the main repository would read the *main* tree and report a pass over ground
nobody covered — the archive bug with a green face. Both the worktree creation
and the lint run must clear it:

```sh
if [ -x "$work/test/lint.sh" ]; then
  ( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
    cd "$work" && ./test/lint.sh && ./test/integration.sh ) || fail=1
else
  echo "pre-push: $local_sha carries no test/lint.sh, so nothing was linted" >&2
  fail=1
fi
( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
  git worktree remove --force "$work" ) 2>/dev/null || true
```

The unsets are scoped to subshells so the hook's own later `git rev-list`,
`git log`, `git cat-file`, and `git show` calls — which must read the main
repository — are untouched.

Backstop for an interrupted run, replacing the existing trap:

```sh
trap '( unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
        for w in "$tmp"/tree-*; do
          [ -d "$w" ] && git worktree remove --force "$w" 2>/dev/null
        done ) || true
      rm -rf "$tmp"' EXIT
```

The `[ -d "$w" ]` guard absorbs the unmatched glob.

**No `git worktree prune`.** The trap removes the exact directories this hook
created, by name, which is the whole of its own mess. `prune` is
repository-wide: it deregisters *any* worktree whose directory it cannot see,
including one belonging to a different checkout that happens to be on an
unmounted volume, a detached external drive, or a path temporarily unavailable
for any other reason. This repository is developed across linked worktrees, so
that is a live hazard and not a theoretical one — a push from one worktree
would quietly deregister another. Exact creation gets exact removal, and a
worktree this hook could not remove stays registered and visible rather than
being swept up along with someone else's.

Everything else in the hook is unchanged: the missing-`lint.sh` refusal, the
conventional-commits range, and the PII scan extracted with `git show` all keep
their current form and their current reasoning.

### Regression test

In `test/integration.sh`, under `$RUN_ROOT`:

1. `git init` a throwaway repository with a local `user.name` and `user.email`
   and no remotes.
2. Copy in `.githooks/pre-push`, `.githooks/commit-msg`, and `tools/pii-scan`
   from `$ROOT`, preserving the executable bit.
3. Plant `test/lint.sh` as an executable stub with this exact body, including a
   required read of `PROBE_DIR` and `git ls-files >/dev/null`, the shape issue
   #106 names:

   ```sh
   #!/bin/sh
   # SPDX-License-Identifier: Apache-2.0
   set -eu
   : "${PROBE_DIR:?}"
   git ls-files >/dev/null
   git rev-parse --absolute-git-dir > "$PROBE_DIR/gitdir"
   git ls-files main-index-only > "$PROBE_DIR/index"
   ```

   Plant `test/integration.sh` as an executable stub that exits 0. `PROBE_DIR`
   sits outside the worktree, so the evidence survives the worktree's removal.
4. Commit with a conforming Conventional Commits message; record the SHA.
5. **Diverge the main index from the pushed commit.** Create `main-index-only`
   and `git add` it without committing. It is now in the main repository's index
   and absent from the pushed tree, so it is a single file whose visibility
   answers which index was read.
6. **Run the hook with the environment git actually gives a hook.** Export
   `GIT_DIR` pointing at the throwaway repository's `.git` for the invocation:

   ```sh
   ( cd "$repo" &&
     printf 'refs/heads/main %s refs/heads/main %s\n' "$sha" "$zero" |
       env GIT_DIR="$repo/.git" PROBE_DIR="$probe" ./.githooks/pre-push )
   ```

   A hand invocation without `GIT_DIR` tests a friendlier environment than the
   real one, and that difference is not cosmetic: it is the difference between
   the old archive hook failing red because there is no `.git` at all, and a
   worktree hook silently reading the main repository's index and reporting a
   pass. The hostile environment is the environment under test.

Assertions:

- The hook exits 0. This is issue #106 proper.
- `$PROBE_DIR/gitdir` ends in `.git/worktrees/…`, and is **not** the main
  repository's `.git`.
- `$PROBE_DIR/index` is empty — `main-index-only` was invisible to the lint run.

The named old-code mutant replaces the hook under test with the archive-export
implementation while leaving this right-hand-side environment intact. The old
hook itself exits 0 under the real leaked `GIT_DIR`, but the regression test goes
red: `gitdir` names the main repository's `.git`, not `.git/worktrees/…`, and
`index` contains `main-index-only`. Both assertions are required, so the test
cannot pass merely because the hook command returned success.

**Not `git rev-parse --show-toplevel`.** Verified in a scratch repository: with
`GIT_DIR` leaked to the main repository from inside a detached worktree,
`--show-toplevel` still prints the worktree, so an assertion on it passes in
exactly the broken case it was meant to catch. `--absolute-git-dir` prints the
main `.git` when leaked and `.git/worktrees/<name>` when clean, and
`git ls-files main-index-only` prints the entry when leaked and nothing when
clean. Both discriminate; the toplevel does not.

`test/lint.sh` bans wall-time constructs in `test/*.sh`; the stub is written to
a temporary directory and never matches that glob, so no exemption is needed.

Per `docs/DECISIONS.md` ("A guard witnesses the artifact, and witnesses it in
order"), the test is not proven until its provenance assertions have been seen
to fail against the unmodified `git archive` hook. That hook false-greens on the
exit status (`rc=0`) under the real leaked `GIT_DIR`, while `gitdir` names the
main repository's `.git` and `index` contains `main-index-only`; those two failed
assertions are the witnessed red. A worktree hook written **without** the
environment unsets is the additional plausible half-fix: it likewise goes green
on the exit status and red on both provenance assertions.

Record the actual old-mutant evidence — `rc=0`, main-tree `gitdir`, and main-index
content — in the commit body. A check that passes both ways is a guard, not
evidence.

### Documentation

`CONTRIBUTING.md` mentions `.githooks/pre-push` as the authoritative gate list;
no wording changes. No `docs/reference.md` change.

### Commit

`fix(githooks): lint the pushed commit in a worktree, not a git-less export`,
with `Closes #106` in the footer and the red-first evidence in the body.

---

## 6. On-demand context query

### Problem

Gangline already computes per-agent context for `GANG_CONTEXT_LIGHTS` and the
band ladder, but that computation is reachable only when a threshold crosses.
An agent deciding whether to compact, or an operator sizing a team's remaining
room, has no way to ask.

### Surface: `gang context [name]`, and no roster column

Both halves are decided on mechanism.

**The query is a command.** `context_now()` already is the one computation; the
threshold path (`context_light_read`) and the query become its two readers.
Nothing is measured twice, cached, or reconciled.

**There is no roster column, and `profile_context` is why.** That function is
built to die when it cannot read its native source: `claude-code` dies with
`no ctx beacon in pane` for any window whose lights were not enabled at hitch
and for every adopted window; `codex` dies without the `@gl_key` that only a
hitch mints, which adopted windows never have. `cmd_roster` fails loudly when a
row cannot be observed — the suite guards that with
`roster fails when an agent row cannot be observed`. A context column would
therefore kill `gang roster` for ordinary, correctly-configured teams, and
`roster` is the one command that has to answer when everything else is
uncertain. It stays as it is.

### `cmd_context`

1. `resolve "$name"`, then `context_now "$AGENT_ID"`, then print its output
   raw — the profile's own `<used>k/<window>k (<pct>%)` format, unparsed.
2. **Do not route through `context_light_read`.** That function returns early
   when `GANG_CONTEXT_LIGHTS` is off, and a query that only answers when
   notifications are enabled is not an on-demand query. Lights are the
   notification; this is the read.
3. A profile declaring no `profile_context` refuses, naming the profile.
4. `profile_context`'s own `die` message stands and the command exits non-zero.
   Absent evidence is reported as absent; no value is fabricated and no fallback
   is invented.

### One hitch-time gate has to move, and it is named

`cmd_hitch` mints the startup-envelope nonce under a compound condition:

```sh
if [ -n "$lights" ] && [ "${GANG_SESSION_KEY:-}" = 1 ]; then
  tmux set-option -w -t "$id" @gl_key "$ENVELOPE_NONCE"
fi
```

Codex's `profile_context` dies without `@gl_key`. So a codex agent hitched with
lights **off** cannot answer `gang context` at all — the query would inherit the
notification gate through the back door, at hitch time, which is precisely the
gate this item exists to remove.

**Drop the `[ -n "$lights" ]` half of that condition.** Mint `@gl_key` whenever
the profile declares `GANG_SESSION_KEY=1`. The nonce is not a lights artifact: it
is the startup envelope's own nonce, already written into the pane by the brief
that was just delivered, and recording it on the window states which conversation
this window is. That is a fact about the window, true whether or not anyone asked
for notifications.

Audit of the existing semantics, since broadening an option's meaning is the
risky half of this: `@gl_key` is written at exactly this one site and read at
exactly one — `codex_session_for` inside `profiles/codex.sh`'s
`profile_context`. Nothing branches on its absence except that function's own
`die`, and nothing infers "lights are on" from its presence. Widening it from
"this window was hitched with lights" to "this window was hitched" therefore
changes one thing only: codex context becomes readable on demand without lights,
which is the change being asked for. Adopted windows still have no key, still
have no startup envelope to take one from, and their refusal is unchanged.

### The availability is not uniform, and that is stated

With lights off and the gate above removed, `gang context` answers on `codex`
(its source is the rollout file, needing only the hitch-time `@gl_key`), on
`opencode`, and on `pi` (both read the pane). It cannot answer on `claude-code`,
whose source is a statusline beacon that only an enabled-lights hitch wires —
that one is a launch flag, not a gate that can be lifted here, and no amount of
option-minting substitutes for a statusline that was never installed. The
existing refusal already says exactly that, and it is left to say it rather than
being softened. It also cannot answer for any adopted window on either harness.
`docs/reference.md` records the per-profile availability so an operator is not
surprised by a refusal that is really a launch choice.

### Self-targeting

Bare `gang context` targets the window it runs in, per item 8. This is the
canonical use — an agent asking its own remaining room before deciding to
compact — so it joins that item's table.

### Tests

1. A fixture profile with a `profile_context` returning a known reading: bare
   and named invocations both print it, byte-equal.
2. A fixture whose `profile_context` dies: the command exits non-zero and the
   profile's own message reaches stderr, with no value on stdout.
3. A fixture declaring no `profile_context`: refused, naming the profile.
4. **Lights off still answers.** Same fixture with `GANG_CONTEXT_LIGHTS` unset;
   assert the reading is printed. This is the assertion that `cmd_context` did
   not route through `context_light_read`.
5. **The hitch-time gate is gone, proven at the real site.** Hitch a fixture
   profile that declares `GANG_SESSION_KEY=1` with `GANG_CONTEXT_LIGHTS` unset,
   and assert the window option `@gl_key` is non-empty afterwards. This exercises
   `cmd_hitch`'s own condition rather than a synthetic `profile_context`, which
   is the difference that matters: test 4 alone passes against a build that
   still gates the mint, because a fixture profile that never reads `@gl_key`
   cannot notice it is missing. This one goes red against that build.
6. `gang roster` output contains no context column — a guard against a future
   change reintroducing the failure mode above.

### Documentation

`docs/reference.md` — the command, the raw-output guarantee, per-profile
availability, why roster carries no column, and `@gl_key` described as minted by
any hitch of a profile declaring `GANG_SESSION_KEY`. `docs/DECISIONS.md` §"Context
lights are optional and minimal" gains one sentence: the same computation is
exposed on demand as a query, which reads whether or not lights are enabled,
because signalling and asking are different acts.

### Commit

`feat(gang): expose the context computation as an on-demand query`

---

## 7. `gang usage <name>`

### Problem

Plan and quota usage sits behind an interactive command in each harness. Reading
it means attaching to a window, typing, reading, and dismissing — per agent, by
hand.

### What was observed

Both harnesses were driven live in a disposable session. The two renderings are
not the same shape, and the difference is the whole design.

**claude-code 2.1.224 — `/usage` is a full-screen modal.** Submitting it
replaces the pane with a tabbed page (`Settings  Status  Config  Usage  Stats`)
carrying session cost, session and weekly limit bars with reset times, and a
contributing-factors section. No composer is on screen. `Escape` dismisses it,
and the composer returns reading empty through the profile's own `profile_input`
parser — verified by running that parser over the post-dismissal capture. The
page is scrollable: the capture ends with a `↓` marker and more content below.

**codex 0.145.0 — `/usage` is a two-step, and it renders inline.** Submitting it
opens a selection menu:

```
  Usage
  View account usage or redeem an earned reset.

› 1. Show usage                View recent account token usage.
     Redeem usage limit reset  No usage limit resets available.

  Press enter to confirm or esc to go back
```

`Show usage` is preselected, so a further `Enter` confirms it. The content then
lands **in the transcript**, not in an overlay, and the composer is already
restored with no key pressed. There is nothing to dismiss.

The command name is verified: typing `/us` in codex 0.145.0 matches exactly one
command, `/usage  view account usage or use a usage limit reset`.

This refutes the charter's "dismiss the UI (profile-declared key, e.g. Esc)" as a
universal step. It is right for claude-code and wrong for codex, so dismissal is
per-profile data that may legitimately be empty.

### Profile surface

```sh
GANG_USAGE_CMD          # the command to type, e.g. /usage
GANG_USAGE_CONFIRM_KEY  # keys pressed after submit to reach the content; may be empty
GANG_USAGE_RENDER       # modal | inline
GANG_USAGE_DISMISS_KEY  # key that closes it; empty when the harness self-restores
```

`profiles/claude-code.sh`:

```sh
# Verified on claude-code 2.1.224: /usage opens a full-screen tabbed modal with
# no composer, and Escape restores an empty composer. The page scrolls; gang
# returns the visible screen and does not drive the scrollbar.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY=""
GANG_USAGE_RENDER="modal"
GANG_USAGE_DISMISS_KEY="Escape"
```

`profiles/codex.sh`:

```sh
# Verified on codex 0.145.0: /usage opens a selection menu with "Show usage"
# preselected; one Enter confirms it and the content is appended to the
# transcript with the composer already restored, so nothing dismisses it.
GANG_USAGE_CMD="/usage"
GANG_USAGE_CONFIRM_KEY="Enter"
GANG_USAGE_RENDER="inline"
GANG_USAGE_DISMISS_KEY=""
```

`profiles/opencode.sh`, `profiles/pi.sh`, `profiles/bash.sh` declare none.
`gang usage` refuses for them, naming the missing declaration — the same shape
`gang flush` and `gang interrupt` already use for an undeclared profile.

### `cmd_usage`

1. `resolve "$name"`. Refuse if `GANG_USAGE_CMD` is empty:
   `usage: profile '<p>' declares no GANG_USAGE_CMD, so gang does not know this harness's usage command`.
   Refuse an unknown `GANG_USAGE_RENDER`.
2. `lock_pane`. Every keystroke below is under the pane lock.
3. **Same predicates as a send, and no weaker.** `occupied` refuses.
   `busy` refuses on 0 and on could-not-determine — note that this is *stricter*
   than `send_live`, which falls through could-not-determine to a provably empty
   box. Typing a UI command mid-turn cannot be verified the way a message
   delivery can: there is no envelope to read back, so the fall-through's
   residual-risk argument does not carry here. `composer_settled` refuses.
   `input_clear` must be true.
4. Capture `before` as the **whole buffer**, history included:
   `tmux capture-pane -pJ -S - -t "$AGENT_ID"`.
5. `inject "$AGENT_ID" "$GANG_USAGE_CMD" head` — the same path `gang compact`
   uses to submit a native command.
6. Press each key in `GANG_USAGE_CONFIRM_KEY`, if any.
7. **Prove the screen changed.** Bounded look loop, the shape `cmd_flush`
   already uses for its recall readback: capture the whole buffer, compare with
   `before`, break on difference, and after the bound refuse with
   `usage: the screen of '<name>' never changed after <cmd>, so gang has no usage content to report and pressed nothing further`.
   No new timing primitive; `cmd_flush`'s loop is the precedent and `bin/gang`
   is not bound by the suite's no-sleep rule.
8. Extract the content:
   - `modal` — the whole after-capture's **visible pane**
     (`tmux capture-pane -pJ`, no `-S`), trailing blank rows trimmed, exactly as
     `cmd_capture` trims. A modal replaces the screen; the screen is the answer.
   - `inline` — the difference between the two whole-buffer captures: strip
     their longest common leading run of lines, then their longest common
     trailing run, and print what remains of `after`.
9. Dismiss: press `GANG_USAGE_DISMISS_KEY` if non-empty.
10. **Restore is part of the contract.** Re-read: `profile_input` must read a
    box, and `input_clear` must be true. If either fails, `die` naming the
    agent, the key pressed, and `gang attach` — and print the captured content
    anyway on stdout before dying, because the content was already read and
    withholding it helps nobody. Exit non-zero.
11. `lock_release`, print the content raw to stdout, nothing else on stdout.

### Why `inline` is a buffer difference and not row arithmetic

An inline TUI does not append at the bottom of the screen. It inserts above a
composer box that stays pinned to the last rows, and once the transcript is
taller than the pane it also pushes lines off the top into tmux's history. Both
of those break any rule phrased in visible-row positions. Measured live on codex
0.145.0 at 80×24: submitting `/usage` moved the pane's last non-blank transcript
row and left the usage block spanning most of the screen with the composer below
it, while earlier content scrolled into history. "Everything after the old last
line" returns the tail of the answer and drops its header and totals — the
extraction loses most of what the operator asked for, and loses it silently.

The difference is taken over the **whole buffer** because that is the one
coordinate system nothing shifts. tmux history is append-only and indexed from
its own start, so a line that scrolls off the visible pane keeps its position in
`-S -` output; capturing a fixed window like `-S -50` would slide by however many
lines scrolled and destroy the alignment the comparison depends on.

The trailing-run trim is what removes the redrawn composer: at rest, before and
after, the box is byte-identical, so it falls inside the common suffix and never
reaches stdout. If the harness redraws it differently — a hint line, a changed
placeholder — the trim stops earlier and a row or two of chrome is printed. That
is the right direction to fail: chrome is noise the operator can ignore, and
missing content is an answer they cannot recover.

Store the two whole-buffer captures in shell variables `before` and `after`.
Feed their contents to awk with Bash 3.2 process substitution; the names are not
paths and no capture file or cleanup lifecycle is implied. Pure awk, mawk-safe
(no `length()` over multibyte text):

```sh
awk 'FILENAME==ARGV[1] { b[FNR]=$0; nb=FNR; next }
     { a[FNR]=$0; na=FNR }
     END {
       p=0; while (p<nb && p<na && b[p+1]==a[p+1]) p++
       s=0; while (s<nb-p && s<na-p && b[nb-s]==a[na-s]) s++
       for (i=p+1; i<=na-s; i++) print a[i]
     }' <(printf '%s\n' "$before") <(printf '%s\n' "$after")
```

**One bound.** If `before` was non-empty and the common prefix is zero lines, the
history evicted its oldest lines between the two captures and the buffers no
longer share an origin. Refuse with
`usage: the scrollback of '<name>' rolled over while gang was reading it, so gang cannot tell the usage content from the transcript around it`
rather than printing an unbounded diff. Loud and rare beats plausible and wrong.

The content is the harness's own UI text, unparsed. Gangline does not summarize
it, extract numbers from it, or decide anything from it: parsing and pacing
policy are the operator's.

Known limitation, stated rather than engineered around and specific to `modal`:
a full-screen page that scrolls **within itself** is returned as its visible
screen only. Its overflow is not in tmux's history — the modal painted over the
pane rather than scrolling through it — so there is nothing captured to
reassemble, and driving a scrollbar to make some would be a second product. The
operator can attach. `inline` has no such limit: content that scrolls off the
pane is in history, and the extraction reads history. `docs/reference.md` says
both.

### Tests

Fixture profile over `profiles/bash.sh` whose `GANG_USAGE_CMD` is a shell line
that paints a known block, exercising both `modal` and `inline` extraction with
two fixtures. No real harness.

1. **Content is returned raw**, byte-equal to the painted block.
2. **`inline` returns only the appended lines**, not the pre-existing transcript.
   Paint a distinctive marker line into the transcript before running, and assert
   it is absent from the output while every line of the block is present in order.
3. **`inline` survives a block taller than the pane.** The geometry case, and
   the one the row-arithmetic design fails. Size the fixture window small
   (`tmux resize-window`), paint a numbered block of more lines than the pane has
   rows so that the earlier ones are pushed into history, and assert **every**
   line of the block is returned, first line included. A build that extracts
   "lines beyond the last non-blank line of before" returns only the tail and
   goes red here; so does one that captures a fixed `-S -N` window.
4. **A rolled-over scrollback refuses.** Same fixture with `history-limit` set
   small enough that the block evicts the whole prior buffer; assert the refusal
   naming the rollover, and that nothing was printed to stdout.
5. **Restoration is verified.** After a successful run the fixture's composer
   reads empty through `profile_input`.
6. **A mutant that skips dismissal fails.** Fixture whose declared dismiss key
   does nothing; assert the command exits non-zero, names `gang attach`, and
   **still printed the content**. This is the test the amendment names
   explicitly, and it is the one that proves restoration is a contract rather
   than a hope.
7. **A screen that never changes refuses**, naming the command, with nothing
   further pressed and the composer left empty.
8. **A busy target refuses**, and a could-not-determine target refuses — the
   assertion that `cmd_usage` is stricter than `send_live` and did not
   accidentally inherit the fall-through.
9. **An occupied target refuses.**
10. **A profile declaring no `GANG_USAGE_CMD` refuses**, naming the variable.
11. **Bare `gang usage` inside an agent window prints its own help**, presses
    nothing, and leaves the composer empty — see below.

### `gang usage` has no self form

Step 3 refuses a busy target, and an agent invoking `gang usage` with no name is
running a turn — its own composer is occupied by the shell command asking the
question. Bare self `gang usage` would therefore refuse every time it was used
for its stated purpose, and the only way to write a passing test for it would be
a fixture with no turn in flight, which is not the live consumer.

Weakening step 3 for the self case is not available: the predicates exist because
typing a UI command into a busy composer cannot be verified, and self is the one
target guaranteed to be busy.

Deferring it the way `gang compact` defers self-compaction does not work either,
and for a reason particular to this command rather than a matter of effort.
`GANG_SELF_COMPACT=deferred` records a *request* and lets a one-shot worker
submit it after the Stop hook, which is enough because compaction's result is a
state change in the harness. `gang usage`'s entire result is text on the
caller's stdout, and the caller's stdout is gone by the time the turn ends. A
deferred usage run would read a page and have nowhere to put it.

So `gang usage` is a `help` verdict in item 8's table, with the same shape as
`interrupt` and `flush`: incoherent for self, printing its own synopsis rather
than an error. An agent reading its own plan usage asks a peer to run
`gang usage <its own name>`, or the operator does.

### Documentation

`docs/reference.md` — the command, the four profile variables in the profile
contract table, the predicates, the raw-output guarantee, and the scrolling
limitation. `docs/operations.md` — reading usage across a team without
attaching.

### Commit

`feat(gang): report a harness's own usage page without attaching`, with both
live captures in the body.

---

## 8. Self-targeting defaults

### Problem

An agent asking about itself must know and type its own name, and a bare
invocation answers with a naked error instead of help.

### Rule

**Bare invocation only.** A command invoked with zero arguments, where the only
missing argument is a target name and self-targeting is coherent, targets the
window it runs in. Any argument at all selects the existing signature unchanged.

The bare-only rule exists to kill an ambiguity rather than to be terse.
`gang capture <name> [lines]` with one numeric argument could mean a window
named `40` or forty lines of self; `valid_name` permits a numeric name, so
inferring would be a silent fallback and law 8 forbids it. Zero arguments is the
one form that cannot be misread.

Self is resolved exactly as `send_sender` resolves sender identity: `self_window`
then `agent_name_of`. Outside a Gangline window there is no self, and the
command prints its own help with one line saying so — not an error.

### Verdicts

| Command | Bare | Reason |
|---|---|---|
| `gang status` | self | An agent reading its own state is the common case. |
| `gang capture` | self | Reading your own pane; default line count applies. |
| `gang composer` | self | Reading your own input box. |
| `gang compact` | self | Self-compaction is already a first-class path, with its own deferred handling per profile. |
| `gang context` | self | An agent asking its own remaining room before compacting is the canonical use, and it reads a file or a beacon rather than typing. |
| `gang usage` | help | Incoherent: it types into the composer that is running the command, so its own predicates refuse, and its result is stdout the turn no longer has. |
| `gang interrupt` | help | Incoherent: it would run inside the turn it drops, and the bracket it edits is your own. |
| `gang flush` | help | Incoherent: recovering your own parked queue needs your harness idle, which it is not while you are running. |
| `gang drop` | help | Destructive, and self-targeting makes it destructive *by omission*. |
| `gang hitch` | help | The name argument is a new agent's, so there is no self to default to. |
| `gang adopt` | help | Names a window that is not yet an agent; self already is one. |
| `gang send` | help | `--to` is the message's destination; sending to yourself is incoherent. |
| `gang up`, `roster`, `attach`, `profiles`, `config`, `cutoff`, `notify`, `down` | unchanged | Already meaningful bare; none of them is a bare-error case today. |

`gang down` ends the whole team and takes no target, so it is not a
self-targeting question. It keeps its current behaviour.

### Implementation

One helper:

```sh
self_name() { # -> this window's agent name; nonzero outside a Gangline window
```

built on `self_window` and `agent_name_of`. Each self-targeting command, on zero
arguments, calls it and falls back to `cmd_help <command>` plus the line
`(no target given, and this shell is not a Gangline agent window)` when it fails.

`usage_die` is replaced at every bare-invocation site by `cmd_help <command>` and
exit 1. **No command may answer a bare invocation with a naked error.**

### The acceptance criterion is scoped, because the sweep is not universal

The commands divide in two, and only one half is swept.

**Bare-error commands** — those that reach `usage_die` today, or would once
self-targeting is added and self is unavailable: `hitch`, `adopt`, `send`,
`flush`, `interrupt`, `compact`, `status`, `capture`, `composer`, `context`,
`usage`, `drop`. Each must answer a bare invocation with its own synopsis and a
non-zero exit. This list lives in the suite as an explicit array.

**Meaningful-bare commands** — `up`, `roster`, `attach`, `profiles`, `config`,
`cutoff`, `notify`, `down`. A bare invocation is their ordinary form and does
work; they must **not** print help, and asserting a synopsis for them would be
asserting a regression. They are excluded from the sweep by name, and `attach`
and `down` are excluded from being invoked by it at all: `attach` replaces the
process, and `down` ends the entire team, including the fixture the rest of the
suite is running in. Their bare behaviour is already covered by the tests that
exist for them.

Sweeping "every command in `usage()`" would therefore both fail on the second
group and destroy the run while failing. The two groups are named separately
because they are different claims.

### Tests

1. Bare `gang status`, `capture`, `composer`, `context` inside a fixture agent
   window each report on that window. Drive them with `TMUX_PANE` set to the
   fixture's pane, as the suite already does for `gang hook`.
2. Bare `gang compact` inside a fixture window compacts that window.
3. Bare `gang interrupt`, `flush`, `usage`, `drop`, `hitch`, `adopt`, `send`
   each print their own synopsis and exit non-zero, and — the real assertion —
   leave the fixture window untouched: same pane bytes, same composer.
4. Bare `gang status` outside any Gangline window prints the synopsis and the
   not-an-agent line.
5. **Sweep of the bare-error list.** Every command in that array, invoked bare,
   produces output containing its own synopsis line and exits non-zero. This is
   the no-naked-error rule as one assertion.
6. **The meaningful-bare list does not print help.** For `roster`, `profiles`,
   `config`, `cutoff`, and `notify`, assert a bare invocation exits 0 and does
   **not** contain the top-level synopsis. This is the guard that a future
   refactor does not "fix" them into printing help. `up`, `attach`, and `down`
   are not invoked here; they are covered where they already are.
7. **Every dispatched command is classified in one list or the other.** Extract
   the dispatcher's case-arm names from `bin/gang`. Sort and deduplicate the
   union of the explicit bare-error and meaningful-bare arrays, then assert that
   union equals the dispatcher names once this literal allowlist is subtracted:
   `hook`, which is the harness callback and deliberately undocumented, `spawn`,
   an alias of `hitch`, and `-h`/`--help`/`help`. Separately assert that the help
   inventory names the same classified union. A new dispatch-and-help entry
   omitted from both behavioural arrays must therefore fail the dispatcher-to-
   union comparison; deriving the expected set from help alone would let it
   dodge classification. The allowlist is a literal in the test, so adding to
   it is a visible edit.

### Documentation

`docs/reference.md` — a short section stating the bare-only rule, the table
above, and that self resolves the way a sender does.

### Commit

`feat(gang): default a bare command to the window it runs in`

---

## 9. Narrow-terminal help

### Problem

The operator reads help over SSH on a phone. Wide single-line usage blocks wrap
mid-token and the output becomes unreadable.

### Width budget: 48 columns

Argued, not asserted. Phone-SSH portrait sits near 40 columns on common
terminals, and 80 is the floor everywhere else. The two directions of error are
not symmetric: too wide turns a synopsis into hash across a wrap boundary, which
is unrecoverable by the reader; too narrow only costs vertical lines, which
scroll. So the budget is set by the narrow case with a small margin — 48 — which
soft-wraps at most one line at 40 while keeping each synopsis fragment long
enough to still read as a command.

The budget is set by the reader, and the content is then made to fit it — not
the other way round. Today's longest help line,
`  gang hitch <name> [-p <profile>] [-d <dir>] [-m <model>] [-e <effort>] [--resume]`,
is far past any phone width, which is the complaint. Under the stacked shape
below it is not a line at all: it decomposes into a base form and one optional
group per line, and the widest fragment that cannot be broken further is a single
flag group like `    [-p <profile>]` or a base form like
`  gang send --to <name> --stdin`. Both sit well inside 48, so the budget
constrains the prose written around them rather than forcing the syntax to be
abbreviated. Any future line that cannot fit is a line that should have been
split.

### Shape

`gang --help` lists commands one per line, name and one-line purpose, no
column-aligned table:

```
gang — drive native CLI agents in tmux

  up        start a team and join it
  hitch     add an agent
  send      deliver a message to an agent
  ...

gang <command> --help for one command.
```

Per-command help stacks its synopsis, one form per line, optional groups on
their own lines:

```
gang send
  Deliver a message to one agent.

  gang send --to <name> --stdin
    [--from <sender>]
    [--live-only]
    [--supersede]

  Parks on refusal. --live-only never parks.
```

Every subcommand gets `--help`, routed through the same `cmd_help <command>`
that item 8 reuses for bare invocations with no self default. `cmd_help` with no
argument prints the top-level list.

Help text lives in one place — a single `case` in `cmd_help` — so a command
cannot gain a synopsis in one place and not the other.

### Test

Assert that no line of `gang --help`, and no line of `gang <cmd> --help` for
every command in the list, exceeds 48 columns.

**Measure in characters, not bytes.** Local `awk` is mawk and CI's is gawk, and
`length($0)` on a line containing `—` or `❯` differs between them: mawk counts
bytes, gawk counts characters. A byte-counting assertion would pass locally and
fail in CI, or worse, pass in both while measuring the wrong thing. Measure with
`python3 -c 'import sys; …len(line)…'`, which the suite already depends on
elsewhere, and have the failure print the offending command, line, and its
width.

Enumerate the commands for this sweep the way item 8's test 7 does — from the
dispatcher's case arms, minus the documented allowlist — not from the help list.
Measuring only the help text that exists cannot notice help that is missing, and
the two acceptance criteria (every command has help; no help line is too wide)
are then checked over the same, complete enumeration.

### Documentation

`docs/reference.md` keeps the full syntax; help stays a pointer to it, and the
budget is recorded beside the help section so a future edit knows the constraint
is deliberate.

### Commit

`feat(gang): make help legible at phone-SSH widths`

---

## Cross-cutting requirements

### Portability

Pure bash 3.2: no associative arrays, no `${var^^}`, no `mapfile`, no `&>`. All
new registry and list data is newline- or space-separated strings parsed with
`case`, `read`, or `awk`. `test/lint.sh` runs `bash -n` and
`shellcheck -S warning` over `bin/gang`, `collars/*.sh`, and `.githooks/*`, and
must stay clean.

### Suite isolation

`test/integration.sh` already exports `GANG_CONFIG_DIR="$RUN_ROOT/config"`, which
outranks `XDG_CONFIG_HOME` and `$HOME` in `config_load`, so the operator's real
`DOCTRINE.md` is unreachable from fixtures today. Harden it against a future
case that runs `env -u GANG_CONFIG_DIR`: add, beside the existing exports,

```sh
export XDG_CONFIG_HOME="$RUN_ROOT/xdg"
```

so the second precedence tier also lands inside the run root. Add one assertion
that `env -u GANG_CONFIG_DIR "$GANG" config` reports a config root under
`$RUN_ROOT` — the belt proven, not assumed.

### Timing

No new test may sleep, poll, or assert on elapsed time. The stall-light repeat
bound is exercised by writing an old epoch into `@gl_stall`; dialog triage and
delivery are synchronous, so their assertions follow the command that caused the
effect. The suite's fake `sleep` on `PATH` stays as it is.

### Docs discipline

No counts, versions, sizes, or tallies in standing documentation, except the
verified harness pins in this spec and in profile comments, which are dated
evidence a decision rests on. Edits describe the current system; no sentence
narrates the change that produced it.

### Order

1 and 5 are independent and can land first. 2 must land before 3, which reuses
`spool_available` for a refused stall note. 4 is independent of all of them and
carries no ordering claim against 3: on codex, the harness that has the dialog,
an unrecognized one raises no native event and therefore has no stall light to
fall back to. 4 lands only with its live capture, and if that capture cannot be
taken it defers without blocking anything else. 6 and 7 are independent, except
that 6 edits `cmd_hitch`'s key-minting condition and should be checkpointed
against the full suite for that reason. 9 must land before 8, which routes bare
invocations
through `cmd_help`; landing 8 first would mean writing that router twice. Land
each as its own commit with the suite green at every checkpoint.

---

## Release

Release Please owns versioning; nothing here bumps a version by hand.

The flow, confirmed from repository history and `.github/workflows/release.yml`:
`release-please-action@v4` runs on every push to `main`, reads
`release-please-config.json` and `.release-please-manifest.json`, and maintains
the branch `release-please--branches--main--components--gangline` carrying a
`chore(main): release gangline <version>` commit. That commit updates
`CHANGELOG.md`, `.release-please-manifest.json`, `version.txt`,
`packaging/npm/package.json`, and `packaging/pypi/pyproject.toml`. Merging its
pull request is what tags and cuts the GitHub Release. Every prior release in
this repository followed exactly that path.

Steps for the implementer, after all nine changes are merged to `main`:

1. Confirm the release pull request updated itself to include this work.
   `bump-minor-pre-major` is set and change 2 carries a `BREAKING CHANGE:`
   footer, so pre-1.0 that is a minor bump.
2. Read the generated `CHANGELOG.md` diff. Do not hand-edit it, and do not add
   changelog entries to the feature pull requests.
3. Confirm the packaging updaters fired — `packaging/npm/package.json` and
   `packaging/pypi/pyproject.toml` must carry the new version, and
   `version.txt` and `.release-please-manifest.json` must agree with it. These
   updaters fail silently when a path or JSON pointer drifts, which is why this
   is a read and not an assumption.
4. Merge the release pull request. The tag and GitHub Release follow from the
   merge.
5. Smoke-test the released tree in a separately named disposable Gangline
   session on an explicit private tmux socket: `gang up`, `gang notify`, a
   refused `gang send` that parks and then drains at the next turn boundary, and
   `gang roster`. Delete only that exact session afterwards.
