# Codex Stop hook report-before-idle — implementation record

> Status: In progress for issue #176 on 2026-08-31. The paired
> `bin/gang reported-to-hitcher` query and collar/helper are one landing unit.
> The direct-command guard is required for enforcement; without it, a fake
> query fixture cannot establish that the shipped command exists. Current
> behavior is defined by the code and tests, not this historical note.

The paired query has one wire rule: every answered verdict exits zero and
prints exactly `status<TAB>destination<TAB>cause<LF>`. A nonzero exit is no
answer, never an unreported verdict. This keeps Gangline's ordinary `die`
status structurally outside the native blocking path.

The Stop helper privately persists an **idle notice not accepted** state on the
child through the tmux option the normal Gangline process shares. Its settled representation is a newline-separated, occurrence-ordered list of
`failed-turn-id<TAB>destination<TAB>one-line-actionable-failure` rows in
`@gl_idle_notice_failed`; there are no blank rows. A nonaccepted ordinary
attributed `gang send` appends, never overwrites. An accepted verified delivery,
or a parked delivery that will drain without operator repair, removes every
well-formed row for that destination regardless of the later reporting turn;
other destinations stay in order. A malformed stored row is never discarded or
called repaired. `cmd_hook` must never clear it merely because a native event
parsed. Notify owns its readers: `status` renders every outstanding row and
`roster` adds `idle-notice-failed`.

Malformed is terminal for that option: every physical row must have exactly
three nonempty tab-separated fields, with no blank row or C0/DEL control byte.
Once any row violates that shape, the helper refuses both later appends and
later repairs, so a subsequent nonaccepted notice is only printed on stderr
and is not accumulated in the option. A person must inspect and preserve the
bad evidence, then explicitly clear `@gl_idle_notice_failed` on the child
before normal accumulation or repair can resume.

`@gl_state_note_failed` is deliberately not reused: it belongs to state
transitions, clears at a new prompt, and can name a different receiver.

Teardown preserves rather than repairs this state. While the child exists,
roster carries `idle-notice-failed` and status prints every exact record. On
either `drop` or `down`, copy the exact option value as
`idle-notice-failed` under the child name in the ordinary teardown archive,
print its warning and archive path before ending the window, and leave no ghost
roster row after the child is gone. `down` must preflight every such record
before moving any team's mail or ending any window; a material notice failure
may therefore create an otherwise-empty archive.

## Native Stop contract

`codex-cli 0.151.0` emits Stop payloads with `hook_event_name: "Stop"`,
`stop_hook_active`, and `last_assistant_message`. With
`stop_hook_active: false`, a block decision renders *Stop hook (blocked)* and
continues the turn. With `stop_hook_active: true`, an empty JSON object allows
the idle. Its embedded Stop schema declares these fields required.

That is the contract used by the collar helper: one block, then an allowing
escape that lets Gangline—not the hook—tell the relevant teammate.

## Native hook-execution failures

`codex-cli 0.151.0` allows all six non-block hook outcomes: valid `{}`, exit
1, exit 127 from a missing command, malformed stdout, empty stdout, and native
timeout. Exit 1, exit 127, malformed stdout, and timeout render `Stop hook
(failed)` with their cause in the TUI; empty stdout allows without a failure
marker. Native timeout terminates the hook at its configured bound, so the
collar's `timeout = 15` is an enforced fuse, not a merely declarative setting.

The helper returns zero on its own failure paths. The only Stop result that
blocks is its valid block JSON; all query, send, bookkeeping, and parsing
failures are bounded then return `{}`. Codex handles these adjacent break cases
as specified rather than inferring behavior from hook exit status alone.

## Codex hook trust and recovery

Codex's native trust is keyed to the hook event and command string, not to the
contents of the executable that command names. The Stop command names
`collars/plugins/codex-stop-hook.py`; after a person trusts that command once,
later in-place edits to the helper run without a new trust prompt. Changing the
configured command or path mints a new hash and needs re-trust.

This is trust-on-first-use over a name, not code attestation. Anyone who can
write that helper can change what runs at every Codex turn end on this machine
without a native prompt. Gangline does not present this as an extra capability:
the same writer can already alter the checkout and its collars.

When Codex reports untrusted hooks, the collar preflight refuses and holds the
window rather than leaving an agent at a dialog. Its final refusal line is:

```
gang: N codex hook(s) are untrusted here — run the codex line above in CWD once, answer 'Trust all and continue', then re-hitch (this held window carries the full list: gang capture <name> 40).
```

The full held-window diagnostic prints the exact `codex` command above that
line. The operator must, from a terminal in that same working directory, run
the printed command, choose Codex's native **Trust all and continue**, then
run `gang hitch` again. Gangline cannot answer the menu or grant trust. There
is intentionally no pre-hitch provisioning surface; the first loud refusal is
the once-per-machine recovery path. A changed configured command/path means
every later Codex hitch on that machine refuses until a person completes it.

Two alternatives were considered and not added. A changing command-version
token would make every helper edit mint a new hash and impose a recurring
native re-trust tax, encouraging avoidance of safety-helper maintenance. A
Gangline-side content hash checked by the preflight was not rejected on merit,
but has no live consumer; it is the option to build if this documented boundary
must become an enforced control.

## Stop-boundary cost

Before the paired query existed, the old native `gang hook` path measured
**424 ms per turn end** and the helper path **1280 ms**:
an additional **856 ms**. Python startup accounted for **47 ms**; the rest was
an additional full `bin/gang` startup. This is a floor, not a production cost:
the query did no useful work on that worktree. Re-measure and record the same
paths once `reported-to-hitcher` is real.

The query could send the rare post-cap/unknown alert itself and save one
`bin/gang` startup on that alert path. It cannot also merge with generic
`gang hook` without a policy-specific core surface or a Codex branch in
`bin/gang`, both contrary to the collar boundary. The rare-path optimisation is
therefore rejected; the sub-second ordinary Stop cost is accepted for the
operator-visible report-before-idle guarantee.

## N=1 is a policy choice, not a native constraint

The earlier N=1 rationale treated `stop_hook_active`, a boolean without a
counter, as the complete native counter surface. That was an inference from an
incomplete payload surface, not a native limitation, and is now refuted.

The populated `transcript_path` records an injected continuation as a
first-class `HookPrompt` with a `hookRunId` and the native `turn_id`. That same
`turn_id` is carried by both the blocked Stop and the following
`stop_hook_active` Stop. A stateless N=2 is therefore available:
read the transcript, count `HookPrompt` records whose `hookRunId` identifies
this Stop hook and whose `turn_id` matches the payload. It uses Codex-owned
state, not Gangline state or the reporting-send path, and naturally degrades to
N=1 if the nullable transcript path cannot be read.

Gangline nevertheless chooses N=1: one block followed by Gangline's attributed
alert attempt to the hitcher is enough pressure for the failure actually
observed. A broken delivery path remains visible and cannot be promised as an
accepted alert, so this record does not call that contingent transport result
guaranteed. A counter has no live consumer until real sessions show what agents
do after that first block. Revisit N only when those first-block outcomes have
been observed; the technical counter is already known.

## Writable `CODEX_HOME` corroboration

Codex needs a writable `CODEX_HOME` for its local state database; its auth file
may remain read-only. This same in-process state-database write also prevents
the snubline scanner from running with `--ephemeral` under a read-only home.

These independent paths establish the same operational prerequisite. The
scanner path does not establish the Stop-hook contract, nor does the Stop-hook
path establish scanner behavior.
