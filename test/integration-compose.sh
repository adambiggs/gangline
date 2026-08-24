# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Composition: the claude-code collar end to end, a harness that parks Enter in its own queue, compaction brackets, and interrupt.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# The real claude-code collar driven end-to-end through core: the exact -m
# binding and the joined -e must reach the launch line of a window built from
# the REAL collar's declarations — a core that stopped passing either would
# stay green against fixture collars alone. The harness is a stub on PATH,
# reached two ways: through hitch's own environment for the vocabulary check,
# and through the tmux global environment for the pane. GANG_BOOT_TIMEOUT=0
# keeps the world clock-free: the stub never paints a claude composer, so
# hitch dies AFTER the launch facts this world asserts are already
# established in tmux, and the world reads them from the surviving window.
cat > "$CLAUDE_STUB/bin/claude" <<'SH'
#!/bin/sh
if [ "${1:-}" = "--help" ]; then
  printf '  --effort <level>                      Effort level for the current session\n                                        (low, medium, high, xhigh, max)\n'
  exit 0
fi
if [ "${1:-}" = "--model" ] && [ "${3:-}" = "-p" ]; then
  printf 'Error: Input must be provided either through stdin or as a prompt argument when using --print\n'
  exit 1
fi
PS1='stub ' exec bash --norc
SH
if tmux_path="$(tmux show-environment -g PATH 2>/dev/null)"; then
  tmux_path="${tmux_path#PATH=}"
else
  tmux_path="$PATH"
fi
tmux set-environment -g PATH "$CLAUDE_STUB/bin:$tmux_path"
if PATH="$CLAUDE_STUB/bin:$PATH" GANG_BOOT_TIMEOUT=0 \
  "$GANG" hitch realmodel -c claude-code -d /tmp -m claude-opus-5 -e xhigh \
  >/dev/null 2> "$RUN_ROOT/realmodel.err"; then
  fail "a stub that never paints a composer cannot complete a hitch" \
    "hitch reported success"
else
  pass "a stub that never paints a composer cannot complete a hitch"
fi
excludes "a hitch with both choices emits no missing-default warning" \
  "$(<"$RUN_ROOT/realmodel.err")" "hitching 'realmodel' without"
tmux set-environment -g PATH "$tmux_path"
real_launch="$(tmux display-message -p -t "$(window_id realmodel)" '#{pane_start_command}')"
contains "the real collar's launch command is the one that ran" \
  "$real_launch" "claude --settings"
contains "the exact model binds into the real launch line" \
  "$real_launch" "--model claude-opus-5"
contains "and the joined effort rides beside it" \
  "$real_launch" "--effort=xhigh"
"$GANG" drop realmodel >/dev/null

"$HITCH" 1 -c bash -d /tmp >/dev/null
printf 'MARK_NUMERIC' | "$GANG" send --to 1 --from tester --stdin >/dev/null
contains "a numeric name reaches its exact window" "$(pane 1)" "MARK_NUMERIC"
excludes "numeric addressing does not fall through to another window" \
  "$(pane alpha)" "MARK_NUMERIC"

self_sent="test-self-attributed-send-$$"
printf -v self_send_command \
  'printf MARK_SELF_ATTRIBUTED | %q send --to 1 --stdin >/dev/null; tmux wait-for -S %q' \
  "$GANG" "$self_sent"
tmux send-keys -l -t "$(window_id alpha)" "$self_send_command"
tmux send-keys -t "$(window_id alpha)" Enter
tmux wait-for "$self_sent"
contains "a Gangline window derives its own sender identity" \
  "$(pane 1)" "[gang:alpha#"
contains "a self-attributed send reaches the intended peer" \
  "$(pane 1)" "MARK_SELF_ATTRIBUTED"

claimed_sent="test-claimed-inside-send-$$"
printf -v claimed_send_command \
  'printf MARK_FALSE_CLAIM | %q send --to 1 --from impostor --stdin >/dev/null 2>&1; tmux set-option -w @test_sender_rc "$?"; tmux wait-for -S %q' \
  "$GANG" "$claimed_sent"
tmux send-keys -l -t "$(window_id alpha)" "$claimed_send_command"
tmux send-keys -t "$(window_id alpha)" Enter
tmux wait-for "$claimed_sent"
equal "a Gangline window cannot override its observable identity" "1" \
  "$(tmux show-options -wqv -t "$(window_id alpha)" @test_sender_rc)"
excludes "a refused claimed identity delivers nothing" "$(pane 1)" "MARK_FALSE_CLAIM"

if printf 'MARK_UNSIGNED' | "$GANG" send --to 1 --stdin >/dev/null 2>&1; then
  fail "an outside caller must provide its identity" "send exited successfully"
else
  pass "an outside caller must provide its identity"
fi

alpha_delivery_id="$(window_id alpha)"
alpha_delivery_lock="$GANG_LOCK_DIR/$(printf '%s' "$alpha_delivery_id" | tr -c 'A-Za-z0-9' '_').lock"
mkdir -p "$GANG_LOCK_DIR"
ln -s "$$" "$alpha_delivery_lock"
if live_lock_refusal="$(printf 'MARK_LIVE_LOCK' |
  GANG_LOCK_WAIT=not-a-number "$GANG" send --to alpha --from tester --stdin 2>&1)"; then
  live_lock_rc=0
else
  live_lock_rc=$?
fi
equal "a live delivery lock refuses immediately" "3" "$live_lock_rc"
contains "a live delivery lock explains the contention" \
  "$live_lock_refusal" "another Gangline process is delivering"
excludes "a live delivery lock prevents the paste" "$(pane alpha)" "MARK_LIVE_LOCK"
rm -f -- "$alpha_delivery_lock"

ln -s 99999999 "$alpha_delivery_lock"
printf 'MARK_STALE_LOCK' |
  "$GANG" send --to alpha --from tester --stdin >/dev/null
contains "a stale delivery lock is recovered exactly once" \
  "$(pane alpha)" "MARK_STALE_LOCK"
[ ! -e "$alpha_delivery_lock" ] \
  && pass "a recovered delivery releases its lock" \
  || fail "a recovered delivery releases its lock" "$alpha_delivery_lock remains"

tmux send-keys -l -t "$(window_id 1)" HUMAN_DRAFT
draft_refusal=""
if draft_refusal="$(printf 'MARK_DRAFT' |
  "$GANG" send --to 1 --from tester --stdin 2>&1)"; then
  fail "delivery refuses a human draft" "send exited successfully"
else
  pass "delivery refuses a human draft"
fi
# The command a refusal names is the one that answers the question it raises.
# That was `gang capture`, whose raw pane renders a dim suggested-prompt
# placeholder identically to a half-written line — the reading that produced a
# public misdiagnosis. `gang composer` is the collar's styled reading, so what
# it prints is what a human typed; the next check spends it on this very box.
contains "a delivery refusal names a runnable inspection command" \
  "$draft_refusal" "gang composer 1"
if "$GANG" composer 1 >/dev/null; then
  pass "the inspection command named by the refusal runs"
else
  fail "the inspection command named by the refusal runs" "gang composer 1 failed"
fi
contains "and classifies what it refused on" "$draft_refusal" "[draft:"
# the refusal above is the barrier proving the draft is on screen
contains "composer prints what a human typed" \
  "$("$GANG" composer 1)" "HUMAN_DRAFT"
tmux send-keys -t "$(window_id 1)" C-u

# A harness may park the Enter in its own input queue: the fixture's composer
# flips to the queue hint once the strand flag exists, exactly as claude
# 2.1.223 leaves its box reading "Press up to edit queued messages" while the
# parked preview in the transcript looks like a submitted prompt. The box
# changing is therefore not proof of entry, and delivery must say so instead
# of reporting success.
cat > "$RUN_ROOT/queue-rc" <<'RC'
PS1='❯ '
PROMPT_COMMAND='[ -f "$QUEUE_STRAND" ] && PS1="❯ Press up to edit queued messages"'
RC
mkdir -p "$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/queueing.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'QUEUE_STRAND=$RUN_ROOT/queue-strand ENV=$RUN_ROOT/queue-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
SH
export GANG_COLLARS="$RUN_ROOT/collars"
"$HITCH" strand -c queueing -d /tmp >/dev/null
touch "$RUN_ROOT/queue-strand"
if strand_out="$(printf 'MARK_QUEUED' | "$GANG" send --to strand --from tester --stdin 2>&1)"; then
  fail "a submission the harness parks in its queue is not a delivery" \
    "send reported success"
else
  pass "a submission the harness parks in its queue is not a delivery"
fi
contains "the failure names the parked queue" \
  "$strand_out" "parked it in its own input queue"
contains "and hands over the automated recovery" "$strand_out" "gang flush strand"
contains "the parked message is recorded against the window" \
  "$(tmux show-options -wqv -t "$(window_id strand)" @gl_staged)" "queue"
# The record says what gang did; the status line says what is in the box now.
# An operator reading a raw capture here sees the harness's queue hint and has
# to know what it means — the classification says it, with the verb that fixes
# it, so a parked queue is never diagnosed as somebody's half-written line.
strand_status="$("$GANG" status strand)"
contains "status classifies the parked box, not just the record" \
  "$strand_status" "box: parked:"
contains "and points at the recovery for that class" \
  "$strand_status" "gang flush strand"
# With the queue still parked, the NEXT delivery is refused before anything is
# typed, named as the queue rather than blamed on a half-written draft.
if strand_more="$(printf 'MARK_SECOND' | "$GANG" send --to strand --from tester --stdin 2>&1)"; then
  strand_more_rc=0
else
  strand_more_rc=$?
fi
equal "a parked queue refuses the next delivery without typing" "3" "$strand_more_rc"
contains "naming the queue it is waiting on" \
  "$strand_more" "parked earlier input in its own queue"
# Recovery is the collar's word too. This collar declares the evidence and no
# recall key, so gang knows the composer is parked and does not know which
# keystroke loads the body back — which is a refusal, never a guessed keypress.
if norecall_out="$("$GANG" flush strand 2>&1)"; then
  fail "a collar with no declared recall key refuses to flush" "flush reported success"
else
  pass "a collar with no declared recall key refuses to flush"
fi
contains "and the refusal names the missing declaration" \
  "$norecall_out" "GANG_QUEUE_RECALL_KEY"
"$GANG" drop strand >/dev/null

# THE PARKED QUEUE, RECOVERED RATHER THAN DESCRIBED. The fixture's composer
# reads as the queue hint while its strand file exists, and the body the
# harness "parked" is the last command in the pane's own history — so the
# collar's declared recall key genuinely loads that body back into the box,
# the way Up does in claude's composer. The drain flag is what a queue entering
# the session looks like from outside: the next prompt after it appears clears
# the strand.
# The queue appears the way a real one does — as a consequence of a submission
# the harness swallowed, not as scenery arranged beforehand. Arming the fixture
# makes the next prompt raise the strand; the composer then reads as the hint
# while it is empty, and as its own contents once the recall key loads them.
# That distinction is the whole subject here, so the hint lives in the collar's
# reader rather than in the prompt string, where it would concatenate with the
# recalled body and make every readback look altered.
cat > "$RUN_ROOT/flush-rc" <<'RC'
PS1='❯ '
HISTCONTROL=ignorespace
PROMPT_COMMAND='_flush_prompt_count=$((_flush_prompt_count + 1))
printf "%s" "$_flush_prompt_count" > "$FLUSH_PROMPT_COUNT"
if [ -f "$FLUSH_DRAIN" ]; then rm -f "$FLUSH_STRAND" "$FLUSH_DRAIN"; fi
if [ -f "$FLUSH_ARM" ]; then rm -f "$FLUSH_ARM"; : > "$FLUSH_STRAND"; fi
if [ -s "$FLUSH_SIGNAL" ]; then _flush_chan="$(cat "$FLUSH_SIGNAL")"; : > "$FLUSH_SIGNAL"
  tmux wait-for -S "$_flush_chan"; fi'
_flush_probe() {   # ordered behind every key flush sent
  tmux wait-for -S "$(cat "$FLUSH_PROBE_CHAN")"
}
bind -x '"\C-t": _flush_probe'
RC
cat > "$RUN_ROOT/collars/flushable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'FLUSH_STRAND=$RUN_ROOT/flush-strand FLUSH_DRAIN=$RUN_ROOT/flush-drain FLUSH_ARM=$RUN_ROOT/flush-arm FLUSH_SIGNAL=$RUN_ROOT/flush-signal FLUSH_PROMPT_COUNT=$RUN_ROOT/flush-prompt-count FLUSH_PROBE_CHAN=$RUN_ROOT/flush-probe-chan ENV=$RUN_ROOT/flush-rc exec bash --posix' fixture"
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*\$'
GANG_QUEUE_RECALL_KEY='Up'
collar_input() { # a composer that spans lines, and reads as the hint when empty
  local box n
  box="\$(tmux capture-pane -pJ -t "\$1" | awk '
    { line[NR] = \$0
      if (index(\$0, "❯")) start = NR
      if (\$0 != "") last = NR }
    END {
      if (!start) exit 1
      s = line[start]; sub(/^.*❯/, "", s); print s
      for (i = start + 1; i <= last; i++) print line[i]
    }' | sed 's/[[:space:]]*\$//' \\
       | sed '/\[TS\]/{ s/\[TS\]//; s/\$/ /; }')" || return 1
  if [ -f "$RUN_ROOT/flush-strand" ] && ! printf '%s' "\$box" | grep -q '[^[:space:]]'; then
    printf 'Press up to edit queued messages'
    return 0
  fi
  # A Claude redraw can expose an absent frame and then its old empty box before
  # the completed multi-line composer. Tickets make those frames deterministic:
  # no fixture loop or clock decides when the final reading appears.
  if [ -f "$RUN_ROOT/redraw-arm" ] && printf '%s' "\$box" | grep -q MARK_REDRAW; then
    n="\$(cat "$RUN_ROOT/redraw-tickets")"
    if [ "\$n" -gt 0 ]; then
      printf '%s' "\$((n - 1))" > "$RUN_ROOT/redraw-tickets"
      case "\$n" in
        3|1) return 1 ;;
        2) printf ''; return 0 ;;
      esac
    fi
  fi
  printf '%s' "\$box" | tr -d '\302\240'
}
SH
: > "$RUN_ROOT/flush-signal"
"$HITCH" parked -c flushable -d /tmp >/dev/null
parked_id="$(window_id parked)"
flush_prompt_count() {
  local count
  [ -r "$RUN_ROOT/flush-prompt-count" ] || return 1
  count="$(<"$RUN_ROOT/flush-prompt-count")"
  case "$count" in ''|*[!0-9]*) return 1 ;; esac
  printf '%s' "$count"
}
if flush_prompt_count >/dev/null; then
  pass "the flush fixture exposes its executed-command witness"
else
  fail "the flush fixture exposes its executed-command witness" \
    "the prompt counter is absent or malformed"
fi

# POST-PASTE READ-BACK SURVIVES TRANSIENT REDRAW FRAMES. The body is long and
# multi-line so this takes the same collar path as the three live strands that
# motivated the guard. The collar above returns unreadable, unchanged, then
# unreadable again after it first sees the paste; only its fourth reading is the
# completed composer. The test drives no polling and advances no clock.
printf '3' > "$RUN_ROOT/redraw-tickets"
: > "$RUN_ROOT/redraw-arm"
if redraw_out="$(printf '%s\n' \
  'MARK_REDRAW head' 'line 02' 'line 03' 'line 04' 'line 05' 'line 06' \
  'line 07' 'line 08' 'line 09' 'line 10' 'line 11' 'MARK_REDRAW tail' |
  "$GANG" send --to parked --from tester --stdin 2>&1)"; then
  pass "a long multi-line paste survives transient Claude redraw frames"
else
  fail "a long multi-line paste survives transient Claude redraw frames" "$redraw_out"
fi
equal "all three false redraw frames were consumed before submission" "0" \
  "$(<"$RUN_ROOT/redraw-tickets")"
equal "the recovered read-back leaves no staged uncertainty" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)"
equal "the recovered multi-line composer was submitted" "" \
  "$("$GANG" composer parked)"
rm -f "$RUN_ROOT/redraw-arm"

# The fixture raises and lowers its strand from a prompt hook, so a world that
# arranges one has to know that hook has finished before it looks. The barrier
# is an event through the pane, not a wait, and it is armed by the settling
# command ITSELF: a hook still pending from an earlier command finds the channel
# file empty and cannot fire it early, so the wait returns after the settling
# command's own hook and nothing is left in flight to type into the composer
# later. Its leading space keeps it out of the history the recall key reads.
# Whether the target executed another command, observed in ORDER rather than at
# a moment. Reading the prompt count straight after flush returns is timing
# luck: tmux send-keys returns once the key is enqueued, so a mutant Enter may
# not have been consumed yet. The probe key travels the same input path behind
# anything flush sent. By the time it runs, that Enter has either produced
# another prompt or never existed; no collar composer reader participates.
flush_probed=0
flush_prompt_probe() {
  flush_probed=$((flush_probed + 1))
  local chan="test-flush-probe-$flush_probed-$$"
  printf '%s' "$chan" > "$RUN_ROOT/flush-probe-chan"
  tmux wait-for "$chan" &
  local waiter=$!
  tmux send-keys -t "$parked_id" C-t
  wait "$waiter"
  flush_prompt_count
}

flush_settled=0
flush_settle() {
  flush_settled=$((flush_settled + 1))
  local chan="test-flush-$flush_settled-$$"
  tmux send-keys -l -t "$parked_id" " printf %s $chan > $RUN_ROOT/flush-signal"
  tmux send-keys -t "$parked_id" Enter
  tmux wait-for "$chan"
}

# THE RECORD IS THE WHOLE COMPOSER, NOT ITS FIRST LINE. A record that stops at
# the first line is satisfied by a recalled message whose remainder was
# truncated, altered or extended, and flush would submit that while reporting
# the readback verified.
: > "$RUN_ROOT/flush-arm"
if printf 'MARK_MULTI head\nMARK_MULTI_TAIL' |
  "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "a multiline body the harness parks is a failed delivery" "send reported success"
else
  pass "a multiline body the harness parks is a failed delivery"
fi
contains "and every line of it is recorded, not just the first" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)" "MARK_MULTI_TAIL"
: > "$RUN_ROOT/flush-drain"
flush_settle

: > "$RUN_ROOT/flush-arm"
if printf 'MARK_PARKED' | "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "the flush world starts from a message the harness parked" "send reported success"
else
  pass "the flush world starts from a message the harness parked"
fi
parked_record="$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
contains "gang records exactly which body the harness parked" \
  "$parked_record" "MARK_PARKED"

# Nothing to read a composer back against is a refusal, not a blind keypress:
# without the record, the recall key would load and submit whatever happened to
# be there.
tmux set-option -uw -t "$parked_id" @gl_parked
if unrecorded_out="$("$GANG" flush parked 2>&1)"; then
  unrecorded_rc=0
else
  unrecorded_rc=$?
fi
equal "an unrecorded parked message refuses the flush" "3" "$unrecorded_rc"
contains "naming the record it will not proceed without" \
  "$unrecorded_out" "no record of a message parked"
tmux set-option -w -t "$parked_id" @gl_parked "$parked_record"

# A PARK UNDER A RUNNING TURN IS THE HARNESS WORKING, and every other flush
# precondition describes the input box, which a busy target satisfies. Recalling
# here re-parks on the Enter, and the re-park is what the terminal verdict —
# drop the agent, hitch --resume — reads as a queue nothing can drain. The
# bracket is set here rather than waited for, so this carries no sleep.
tmux set-option -w -t "$parked_id" @gl_turn "open $(date +%s)"
if busy_flush_out="$("$GANG" flush parked 2>&1)"; then
  busy_flush_rc=0
else
  busy_flush_rc=$?
fi
equal "flush against a running turn refuses rather than diagnosing" "3" \
  "$busy_flush_rc"
contains "naming the turn, not the queue" \
  "$busy_flush_out" "turn of 'parked' is still running"
excludes "and never tells the operator to drop a working agent" \
  "$busy_flush_out" "gang drop"
equal "and the record it would have recalled is untouched" "$parked_record" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)"

# Keyed on the bracket being OPEN, not on it being there at all: the case below
# is the whole recovery, and it runs with a closed bracket recorded.
tmux set-option -w -t "$parked_id" @gl_turn "closed $(date +%s)"
: > "$RUN_ROOT/flush-drain"
if flush_out="$("$GANG" flush parked 2>&1)"; then
  pass "flush recovers the parked message as a verified operation"
else
  fail "flush recovers the parked message as a verified operation" "$flush_out"
fi
contains "and reports what it verified" "$flush_out" "read back against gang's record"
equal "a verified flush retires the parked record" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
equal "and the staged record with it" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)"
tmux set-option -uw -t "$parked_id" @gl_turn

# The composer must still read as parked. With the queue drained, there is
# nothing to recover, and a recorded body is not evidence that outlives it.
tmux set-option -w -t "$parked_id" @gl_parked "$parked_record"
if drained_out="$("$GANG" flush parked 2>&1)"; then
  drained_rc=0
else
  drained_rc=$?
fi
equal "a composer showing no queue evidence refuses the flush" "3" "$drained_rc"
contains "rather than pressing the recall key blindly" \
  "$drained_out" "none of the parked-queue evidence"

# A DRAINED QUEUE AND A QUEUE THAT NEVER EXISTED ARE DIFFERENT ANSWERS. Both
# reach flush as an empty record, so an operator reaching for it a moment after
# the harness drained itself would otherwise be told the same sentence as one
# whose message never arrived. The record is retired the way a drain retires
# it — gang reading the composer back empty — rather than by unsetting it. The
# staged note is the obstruction record a real park leaves beside @gl_parked;
# both are present here, and the empty composer is what refutes them.
tmux set-option -w -t "$parked_id" @gl_staged \
  "'MARK_PARKED' was pasted and Enter sent, but the harness parked it in its own input queue"
flush_settle
"$GANG" status parked >/dev/null 2>&1 || :
equal "an empty composer retires the parked record" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
if drained_record_out="$("$GANG" flush parked 2>&1)"; then
  drained_record_rc=0
else
  drained_record_rc=$?
fi
equal "flushing a drained queue refuses" "3" "$drained_record_rc"
contains "and says the park is gone rather than never recorded" \
  "$drained_record_out" "NOT parked any more"
excludes "so it cannot be read as a message that never arrived" \
  "$drained_record_out" "no record of a message parked"
# Hand the world back the way the next case expects to find it: this one
# retired the record on purpose, and the readback cases below need it.
tmux set-option -w -t "$parked_id" @gl_parked "$parked_record"

# THE READBACK IS LOAD-BEARING, AND IT IS THE WHOLE BODY. Here the recall key
# loads a message that begins with exactly the recorded one and carries extra
# text after it — the case a containment check waves through. The Enter must not
# be pressed, because pressing it would submit words nobody sent.
parked_raw="${parked_record#"${parked_record%%[![:space:]]*}"}"
parked_raw="${parked_raw%"${parked_raw##*[![:space:]]}"}"
: > "$RUN_ROOT/flush-arm"
tmux send-keys -l -t "$parked_id" "$parked_raw EXTRA_WORDS_NOBODY_SENT"
tmux send-keys -t "$parked_id" Enter
flush_settle
mismatch_prompt_count="$(flush_prompt_count)"
if mismatch_out="$("$GANG" flush parked 2>&1)"; then
  fail "a readback that does not match the record is not flushed" \
    "flush reported success"
else
  pass "a readback that does not match the record is not flushed"
fi
# NOT performed, not merely NOT verified: the refusal has to be the readback's
# own, because the re-queue verdict fails this send too and would let a missing
# readback pass as a working one.
contains "refused by the readback rather than by anything downstream of it" \
  "$mismatch_out" "flush NOT performed"
contains "and says the Enter was not pressed" "$mismatch_out" "Enter was NOT pressed"
contains "the recalled body is recorded against the window" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)" "read back as something other than"

# COMPACTION IS A BRACKET AND CLOSING IT DELIVERS. The harness will not accept
# input while it compacts, and @gl_turn is CLOSED throughout, so nothing below
# the witness would have said so. What is asserted here is the thing we need
# rather than the thing anyone happened to see: a gang delivery landing
# immediately after PostCompact, not a parked message healing itself.
cat > "$RUN_ROOT/collars/bracketable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
SH
"$HITCH" bracket -c bracketable -d /tmp >/dev/null
bracket_id="$(window_id bracket)"
bracket_pane="$(tmux list-panes -t "$bracket_id" -F '#{pane_id}')"
bracket_hook() { # $1 = event name, $2... = extra JSON body
  printf '{"hook_event_name":"%s"%s}\n' "$1" "${2:-}" \
    | TMUX_PANE="$bracket_pane" "$GANG" hook >/dev/null 2>&1
}
bracket_hook Stop                       # a closed turn: the state gang used to trust
bracket_hook PreCompact ',"trigger":"manual"'
contains "PreCompact opens the compaction bracket" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction)" "open "
equal "and records the cause the harness named" "manual" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction_trigger)"
contains "a compacting agent reads busy though its turn bracket is closed" \
  "$("$GANG" status bracket 2>&1)" "-busy-"
bracket_out="$(printf 'MARK_MIDCOMPACT' \
  | "$GANG" send --to bracket --from tester --stdin 2>&1)" || :
contains "so a delivery into it is queued rather than typed" \
  "$bracket_out" "queued for bracket"
contains "and the message is waiting, not lost" "$("$GANG" roster)" "spooled=1"
excludes "and it has not entered the session before PostCompact" \
  "$(pane bracket)" "MARK_MIDCOMPACT"
# The drain is dispatched, not performed inline, and it signals when it is
# done. Waiting on that barrier is what makes the next two assertions read the
# finished state rather than a race.
tmux wait-for "gang-spool-drain-$bracket_id" &
bracket_waiter=$!
bracket_hook PostCompact
wait "$bracket_waiter"
equal "PostCompact closes the bracket" "closed" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction | cut -d' ' -f1)"
excludes "and closing it drains the spool the compaction filled" \
  "$("$GANG" roster)" "spooled="
# Read from the PANE, not the composer: a delivery that landed has left the
# box empty, because entering the session is what delivery means.
contains "so the message enters the session once the compaction ends" \
  "$(pane bracket)" "MARK_MIDCOMPACT"

# A REFUSED COMPACTION NEVER SENDS THE CLOSING EVENT. claude-code declines a short
# session after PreCompact has already fired, so the opening event cannot be
# trusted to be paired and an unpaired one holds the agent busy until it ages out.
# A turn event settles it: parked input raises nothing until it drains, so a turn
# witnesses a harness that is not compacting.
bracket_hook PreCompact ',"trigger":"manual"'
contains "an unpaired PreCompact leaves the bracket open" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction)" "open "
contains "and the refused compaction still reads busy" \
  "$("$GANG" status bracket 2>&1)" "-busy-"
bracket_hook UserPromptSubmit
equal "a turn opening settles the bracket a refusal left open" "closed" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction | cut -d' ' -f1)"
bracket_hook PreCompact ',"trigger":"manual"'
bracket_hook Stop
equal "and so does a turn ending" "closed" \
  "$(tmux show-options -wqv -t "$bracket_id" @gl_compaction | cut -d' ' -f1)"
# Settling is not fabrication: a window that never carried a bracket must not
# acquire one from an ordinary turn.
"$HITCH" unbracketed -c bracketable -d /tmp >/dev/null
unbracketed_pane="$(tmux list-panes -t "$(window_id unbracketed)" -F '#{pane_id}')"
printf '{"hook_event_name":"Stop"}\n' \
  | TMUX_PANE="$unbracketed_pane" "$GANG" hook >/dev/null 2>&1
equal "a turn on a window with no bracket writes none" "" \
  "$(tmux show-options -wqv -t "$(window_id unbracketed)" @gl_compaction)"
"$GANG" drop unbracketed >/dev/null 2>&1 || :
"$GANG" drop bracket >/dev/null 2>&1 || :
excludes "and the message gang recorded was never submitted twice" \
  "$mismatch_out" "flushed the parked message"
# THE TARGET SHELL, not gang's account of itself. Its prompt count changes only
# after it executes another command, and the ordered probe is independent of
# the collar reader whose mismatch caused the refusal.
equal "the refused recall submits no command to the target shell" \
  "$mismatch_prompt_count" "$(flush_prompt_probe)"
tmux send-keys -t "$parked_id" C-u

# AND IT IS BYTE-EQUAL, WITH NOTHING NORMALIZED AWAY. Trimming trailing blank
# space is line-oriented in every tool that offers it, so it erases the
# difference between a line that ends in two spaces and one that does not —
# a hard line break in Markdown, and a different message. A pane capture pads
# every line to the pane width, so trailing whitespace cannot survive one at
# all; this fixture's composer carries it through a token instead, which is
# what lets the world state the difference the comparison must not discard.
: > "$RUN_ROOT/flush-drain"
flush_settle
: > "$RUN_ROOT/flush-arm"
if printf 'MARK_TS head[TS]' |
  "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "a body whose line ends in blank space parks like any other" \
    "send reported success"
else
  pass "a body whose line ends in blank space parks like any other"
fi
ts_record="$(tmux show-options -wqv -t "$parked_id" @gl_parked)"
case "$ts_record" in
  *"] ") pass "and the blank space is part of what gang recorded" ;;
  *) fail "and the blank space is part of what gang recorded" "got [$ts_record]" ;;
esac
# The same body with that line ending stripped: byte-different, and identical
# under any per-line trailing-space normalization.
ts_tampered="${ts_record# }"
ts_tampered="${ts_tampered% }"
tmux send-keys -l -t "$parked_id" "$ts_tampered"
tmux send-keys -t "$parked_id" Enter
flush_settle
ts_prompt_count="$(flush_prompt_count)"
if ts_out="$("$GANG" flush parked 2>&1)"; then
  fail "a recalled body differing only in a line's trailing space is not flushed" \
    "flush reported success"
else
  pass "a recalled body differing only in a line's trailing space is not flushed"
fi
contains "refused by the readback, not by anything downstream of it" \
  "$ts_out" "flush NOT performed"
contains "and the body is recorded as sitting unsent, never as submitted" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)" "read back as something other than"
equal "with the target shell agreeing that no command was submitted" \
  "$ts_prompt_count" "$(flush_prompt_probe)"
tmux send-keys -t "$parked_id" C-u

# THE DIAGNOSIS THE OPEN-TURN REFUSAL WAS PROTECTING. Refusing a flush against a
# running turn is only right because a re-park under a turn means the harness is
# working; the verdict it withholds — this queue drains for nothing gang can
# send, so drop the agent and hitch --resume — has to stay reachable for the
# target the verdict is actually about, one at rest. Here nothing is holding a
# turn open and the strand stays up across the Enter, so the recalled body is
# parked a second time and that is what the operator is told.
: > "$RUN_ROOT/flush-drain"
flush_settle
: > "$RUN_ROOT/flush-arm"
if printf 'MARK_REPARK' |
  "$GANG" send --to parked --from tester --stdin >/dev/null 2>&1; then
  fail "the re-park world starts from a message the harness parked" \
    "send reported success"
else
  pass "the re-park world starts from a message the harness parked"
fi
equal "and no turn is open, so nothing refuses ahead of the recall" "" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_turn)"
if repark_out="$("$GANG" flush parked 2>&1)"; then
  fail "an at-rest target that parks the recalled body again is not reported flushed" \
    "flush reported success"
else
  pass "an at-rest target that parks the recalled body again is not reported flushed"
fi
contains "and the verdict is the terminal one, reached rather than described" \
  "$repark_out" "flush NOT verified"
contains "naming the recovery that outlives a queue nothing drains" \
  "$repark_out" "hitch --resume"
contains "with the second park recorded against the window" \
  "$(tmux show-options -wqv -t "$parked_id" @gl_staged)" "parked it again"
: > "$RUN_ROOT/flush-drain"
flush_settle
"$GANG" drop parked >/dev/null

# ATTRIBUTION LANDS BEFORE MID-TURN STEERING. A steering-capable collar may
# accept a claimed spool through a free composer while its turn stays open. A
# draft, tmux mode, or collar without that declaration still parks without a
# keystroke. PostToolUse is the later native opportunity: it must drain without
# waiting for a Stop to repair the continuously refreshed open-turn record.
cat > "$RUN_ROOT/steer-rc" <<'RC'
HISTCONTROL=ignorespace
PS1='❯ '
RC
cat > "$RUN_ROOT/collars/steering.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'ENV=$RUN_ROOT/steer-rc exec bash --posix' fixture"
GANG_MIDTURN_INPUT=steer
GANG_STOP_HOOK=1
GANG_QUEUED_REGEX='^[[:space:]]*Press up to edit queued messages[[:space:]]*$'
GANG_QUEUE_RECALL_KEY='Up'
_gl_steer_real="\$(declare -f collar_input)"
eval "steer_real_input \${_gl_steer_real#collar_input}"
collar_input() { # record the ownership facts at the first claimed box read
  local lock dir holder="" live=no claimed=0 f box state rc=0
  if [ -f "$RUN_ROOT/steer-claim-watch" ]; then
    lock="$GANG_LOCK_DIR/\$(printf '%s' "\$1" | tr -c 'A-Za-z0-9' '_').lock"
    if [ -L "\$lock" ]; then
      dir="$GANG_LOCK_DIR/spool/\$(tmux show-options -wqv -t "\$1" @gl_spool)"
      for f in "\$dir"/sending-*; do [ -f "\$f" ] && claimed=\$((claimed + 1)); done
      if [ "\$claimed" -gt 0 ]; then
        rm -f "$RUN_ROOT/steer-claim-watch"
        holder="\$(readlink "\$lock" 2>/dev/null)" || holder=""
        [ -n "\$holder" ] && kill -0 "\$holder" 2>/dev/null && live=yes
        printf 'holder-alive=%s claimed=%s\n' "\$live" "\$claimed" \
          > "$RUN_ROOT/steer-claim-observed"
      fi
    fi
  fi
  box="\$(steer_real_input "\$1")" || rc=\$?
  if [ -f "$RUN_ROOT/steer-queue-arm" ]; then
    state="\$(cat "$RUN_ROOT/steer-queue-arm")"
    if [ "\$state" = await ] && printf '%s' "\$box" | grep -q MARK_STEER_PARK; then
      printf queued > "$RUN_ROOT/steer-queue-arm"
    elif [ "\$state" = queued ] && ! grep -q '[^[:space:]]' <<<"\$box"; then
      printf 'Press up to edit queued messages'
      return 0
    fi
  fi
  [ "\$rc" -eq 0 ] || return "\$rc"
  printf '%s' "\$box"
}
SH
"$HITCH" steer -c steering -d /tmp >/dev/null
steer_id="$(window_id steer)"
steer_pane_id="$(tmux list-panes -t "$steer_id" -F '#{pane_id}')"
tmux set-option -w -t "$steer_id" @gl_turn "open $(date +%s)"
: > "$RUN_ROOT/steer-claim-watch"
if steer_out="$(printf 'MARK_STEER' | "$GANG" send --to steer --from tester --stdin 2>&1)"; then
  steer_rc=0
else
  steer_rc=$?
fi
equal "a free steering composer accepts a busy send" "0" "$steer_rc"
contains "the foreground send reports the verified mid-turn handoff" \
  "$steer_out" "collar-declared mid-turn input"
# source-guard: producer@0a69b90f8810: this send is the sole MARK_STEER producer, and the adjacent claim log independently proves the pane handoff came from its attributed spool
contains "the claimed mid-turn handoff reaches the target" "$(pane steer)" "MARK_STEER"
excludes "the verified handoff retires its spool entry" \
  "$("$GANG" status steer)" "spooled:"
# source-guard: producer@b7771528892a: MARK_STEER has one producer above, and the claim log plus retired spool bind every target-pane occurrence to that one handoff
equal "the steering handoff submits exactly once" "1" \
  "$(pane steer | grep -o MARK_STEER | wc -l | tr -d ' ')"
contains "the pane lock is live when the steering entry is read" \
  "$(<"$RUN_ROOT/steer-claim-observed")" "holder-alive=yes"
contains "and attribution is claimed before that composer read" \
  "$(<"$RUN_ROOT/steer-claim-observed")" "claimed=1"

# THE NATIVE QUEUE REMAINS ATTRIBUTED. Claude may accept the Enter by parking
# the steering body until its next tool batch. That is a successful handoff,
# but the exact composer read-back must remain available to status and flush
# after the spool claim retires. The fixture makes the queue hint follow only
# the marked paste, so park_record is load-bearing rather than inert.
printf await > "$RUN_ROOT/steer-queue-arm"
tmux set-option -w -t "$steer_id" @gl_turn "open $(date +%s)"
if printf 'MARK_STEER_PARK' |
  "$GANG" send --to steer --from tester --stdin >/dev/null 2>&1; then
  steer_park_rc=0
else
  steer_park_rc=$?
fi
equal "a native queued steering handoff remains successful" "0" "$steer_park_rc"
equal "the fixture observed the marked paste before showing queue evidence" \
  "queued" "$(<"$RUN_ROOT/steer-queue-arm")"
contains "the queued steering handoff keeps the body recovery record" \
  "$(tmux show-options -wqv -t "$steer_id" @gl_parked)" "MARK_STEER_PARK"
equal "a fresh queued steering record is not marked already drained" "" \
  "$(tmux show-options -wqv -t "$steer_id" @gl_parked_drained)"
contains "status exposes the collar-owned parked landing" \
  "$("$GANG" status steer)" "parked"
excludes "the accepted native queue landing retires the attributed spool" \
  "$("$GANG" status steer)" "spooled:"
rm -f "$RUN_ROOT/steer-queue-arm"
"$GANG" status steer >/dev/null

# COMPACTION IS NOT A PEER-STEERING SURFACE. Its native queue belongs to the
# attributed continuation that caused it; ordinary peer mail stays spooled
# until PostCompact closes the bracket, even if the drawn composer is empty.
tmux set-option -w -t "$steer_id" @gl_turn "closed $(date +%s)"
printf '%s' '{"hook_event_name":"PreCompact","trigger":"manual"}' |
  TMUX_PANE="$steer_pane_id" "$GANG" hook >/dev/null
compact_steer_out="$(printf 'MARK_COMPACT_STEER' |
  "$GANG" send --to steer --from tester --stdin 2>&1)"
contains "a peer send during compaction stays attributed" \
  "$compact_steer_out" "queued for steer"
contains "the compaction refusal names the PostCompact boundary" \
  "$compact_steer_out" "peer input waits for PostCompact"
excludes "peer steering types nothing into a live compaction" \
  "$(pane steer)" "MARK_COMPACT_STEER"
contains "the compaction-held peer body remains spooled" \
  "$("$GANG" status steer)" "spooled: 1"
tmux wait-for "gang-spool-drain-$steer_id" &
compact_steer_waiter=$!
printf '%s' '{"hook_event_name":"PostCompact"}' |
  TMUX_PANE="$steer_pane_id" "$GANG" hook >/dev/null
wait "$compact_steer_waiter"
# source-guard: producer@96108c54245a: MARK_COMPACT_STEER has one spooled producer, and the completed PostCompact barrier is its only permitted drain
contains "PostCompact delivers the deferred peer steering once" \
  "$(pane steer)" "MARK_COMPACT_STEER"
excludes "PostCompact retires the deferred peer spool" \
  "$("$GANG" status steer)" "spooled:"

# --live-only has no attributed landing and therefore never steers.
tmux set-option -w -t "$steer_id" @gl_turn "open $(date +%s)"
if steer_live="$(printf 'MARK_LIVE' | "$GANG" send --to steer --from tester --live-only --stdin 2>&1)"; then
  steer_live_rc=0
else
  steer_live_rc=$?
fi
equal "--live-only refuses rather than parking" "3" "$steer_live_rc"
contains "the live-only refusal names the no-keystroke landing" \
  "$steer_live" "cannot use the attributed spool"
excludes "live-only leaves no queued copy" "$("$GANG" status steer)" "spooled:"

# COPY-MODE OWNS THE PANE. The default send may park, but neither that live
# attempt nor a PostToolUse drain may claim or type until the operator leaves
# the mode. The mode itself must survive both attempts unchanged.
tmux copy-mode -t "$steer_id"
equal "the copy-mode world is genuinely in tmux mode" "1" \
  "$(tmux display-message -p -t "$steer_id" '#{pane_in_mode}')"
copy_out="$(printf 'MARK_COPY_MODE' |
  "$GANG" send --to steer --from tester --stdin 2>&1)"
contains "a live send in copy-mode parks without typing" "$copy_out" "queued for steer"
contains "the copy-mode refusal names tmux ownership" "$copy_out" "tmux mode owns"
equal "the live attempt preserves copy-mode" "1" \
  "$(tmux display-message -p -t "$steer_id" '#{pane_in_mode}')"
excludes "copy-mode receives no body or Enter" "$(pane steer)" "MARK_COPY_MODE"
contains "the refused body remains live in the spool" \
  "$("$GANG" status steer)" "spooled: 1"
tmux wait-for "gang-spool-drain-$steer_id" &
copy_drain_waiter=$!
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$steer_pane_id" "$GANG" hook >/dev/null
wait "$copy_drain_waiter"
equal "a native drain preserves copy-mode too" "1" \
  "$(tmux display-message -p -t "$steer_id" '#{pane_in_mode}')"
contains "the copy-mode drain leaves the entry unclaimed and live" \
  "$("$GANG" status steer)" "spooled: 1"
excludes "the refused native drain types nothing" "$(pane steer)" "MARK_COPY_MODE"
tmux send-keys -X -t "$steer_id" cancel
tmux wait-for "gang-spool-drain-$steer_id" &
copy_release_waiter=$!
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$steer_pane_id" "$GANG" hook >/dev/null
wait "$copy_release_waiter"
# source-guard: producer@9d9ca4a93dff: MARK_COPY_MODE has one spooled producer, and the completed PostToolUse barrier after mode cancellation is its only successful drain
contains "the next safe native opportunity delivers after copy-mode" \
  "$(pane steer)" "MARK_COPY_MODE"
# source-guard: producer@11cf7ab17966: the single MARK_COPY_MODE entry was live before the completed release barrier and retired after it, binding every occurrence to one handoff
equal "that deferred body submits exactly once" "1" \
  "$(pane steer | grep -o MARK_COPY_MODE | wc -l | tr -d ' ')"
excludes "the deferred copy-mode entry is retired" \
  "$("$GANG" status steer)" "spooled:"

# A HUMAN DRAFT is another occupied composer. Clearing it exposes a free box;
# PostToolUse drains while @gl_turn stays open, closing the issue-#133 shape.
tmux send-keys -l -t "$steer_id" HUMAN_DRAFT_BLOCK
draft_steer="$(printf 'MARK_POST_TOOL' |
  "$GANG" send --to steer --from tester --stdin 2>&1)"
contains "a drafted steering composer parks" "$draft_steer" "queued for steer"
excludes "the draft path types no message bytes" "$(pane steer)" "MARK_POST_TOOL"
tmux send-keys -t "$steer_id" C-u
tmux wait-for "gang-spool-drain-$steer_id" &
post_tool_waiter=$!
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$steer_pane_id" "$GANG" hook >/dev/null
wait "$post_tool_waiter"
equal "PostToolUse leaves the turn record continuously open" "open" \
  "$(tmux show-options -wqv -t "$steer_id" @gl_turn | cut -d' ' -f1)"
# source-guard: producer@6628b93910fb: MARK_POST_TOOL has one spooled producer and the completed PostToolUse barrier is its only drain
contains "the open-turn PostToolUse drains the free composer" \
  "$(pane steer)" "MARK_POST_TOOL"
# source-guard: producer@7af7dd8d2694: the one MARK_POST_TOOL entry is retired beside an open turn only after the completed native barrier, binding every occurrence to that handoff
equal "the stale-turn-shape handoff submits exactly once" "1" \
  "$(pane steer | grep -o MARK_POST_TOOL | wc -l | tr -d ' ')"
excludes "and retires its attributed entry" "$("$GANG" status steer)" "spooled:"
"$GANG" drop steer >/dev/null

# PARK REMAINS THE DECLARATION FOR A COLLAR THAT CANNOT TAKE MID-TURN INPUT.
cat > "$RUN_ROOT/collars/park-only.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_MIDTURN_INPUT=park
GANG_STOP_HOOK=1
_gl_park_real="\$(declare -f collar_input)"
eval "park_real_input \${_gl_park_real#collar_input}"
collar_input() {
  if [ -f "$RUN_ROOT/park-only-watch" ]; then
    case "\$(tmux show-options -wqv -t "\$1" @gl_turn 2>/dev/null)" in
      open*) : > "$RUN_ROOT/park-only-midturn-read" ;;
    esac
  fi
  park_real_input "\$1"
}
SH
"$HITCH" park-only -c park-only -d /tmp >/dev/null
park_only_id="$(window_id park-only)"
park_only_pane="$(tmux list-panes -t "$park_only_id" -F '#{pane_id}')"
tmux set-option -w -t "$park_only_id" @gl_turn "open $(date +%s)"
park_only_out="$(printf 'MARK_PARK_ONLY' |
  "$GANG" send --to park-only --from tester --stdin 2>&1)"
contains "a park collar keeps its busy send attributed" "$park_only_out" "queued for park-only"
excludes "park authorizes no mid-turn composer key" \
  "$(pane park-only)" "MARK_PARK_ONLY"
: > "$RUN_ROOT/park-only-watch"
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$park_only_pane" "$GANG" hook >/dev/null
tmux wait-for "gang-spool-drain-$park_only_id" &
park_only_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$park_only_pane" "$GANG" hook >/dev/null
wait "$park_only_waiter"
equal "PostToolUse never reads a park-only composer while its turn is open" "" \
  "$(if [ -e "$RUN_ROOT/park-only-midturn-read" ]; then printf read; fi)"
rm -f "$RUN_ROOT/park-only-watch"
# source-guard: producer@a550dad49d99: MARK_PARK_ONLY has one spooled producer and the completed Stop barrier is its only permitted drain
contains "the park collar drains at the idle boundary" \
  "$(pane park-only)" "MARK_PARK_ONLY"
# source-guard: producer@f753d5255c86: the one MARK_PARK_ONLY entry cannot drain at PostToolUse and retires only after the completed Stop barrier
equal "the park-only handoff submits exactly once" "1" \
  "$(pane park-only | grep -o MARK_PARK_ONLY | wc -l | tr -d ' ')"
"$GANG" drop park-only >/dev/null

# A keystroke gang cannot send by name is a broken declaration, refused before
# any window opens: tmux would deliver the letters into the composer instead.
cat > "$RUN_ROOT/collars/badkey.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_QUEUE_RECALL_KEY='M-x then y'
SH
if badkey_out="$("$GANG" hitch badkey -c badkey -d /tmp 2>&1)"; then
  fail "a recall key that is not a tmux key name is refused" "hitch accepted it"
else
  pass "a recall key that is not a tmux key name is refused"
fi
contains "naming the declaration rather than the operator" \
  "$badkey_out" "GANG_QUEUE_RECALL_KEY"
equal "and the refused declaration leaves no window behind" "" "$(window_id badkey)"

# INTERRUPTING IS A COLLAR'S KEYSTROKE AND A FACT GANG OWNS. The keystroke
# ends a turn the harness will never close for itself, so the bracket it opened
# has to be closed here or the target reads busy until that bound expires.
# Whether the harness actually stopped remains its own verdict; the fact does
# not claim otherwise.
if nokey_out="$("$GANG" interrupt alpha 2>&1)"; then
  fail "a collar with no declared interrupt key refuses to interrupt" \
    "interrupt reported success"
else
  pass "a collar with no declared interrupt key refuses to interrupt"
fi
contains "naming the declaration it would need" "$nokey_out" "GANG_INTERRUPT_KEY"

cat > "$RUN_ROOT/collars/badstop.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_INTERRUPT_KEY='ctrl then c'
SH
if badstop_out="$("$GANG" hitch badstop -c badstop -d /tmp 2>&1)"; then
  fail "an interrupt key that is not a tmux key name is refused" "hitch accepted it"
else
  pass "an interrupt key that is not a tmux key name is refused"
fi
contains "naming that declaration too" "$badstop_out" "GANG_INTERRUPT_KEY"
equal "and it leaves no window behind either" "" "$(window_id badstop)"

# A TARGET THAT WITNESSES THE KEYSTROKE. Every other assertion about interrupt
# reads something gang wrote — its own report, its own tmux option — so all of
# them hold just as well when no key leaves at all. This fixture binds the
# declared key to a mark of its own, which is the only artifact in the world
# that cannot exist unless the key arrived at the pane. It is also what pins the
# key to the COLLAR's declaration rather than to anything hard-coded in core:
# the bound key is C-g, so an interrupt that sent Escape would leave no mark.
cat > "$RUN_ROOT/interrupt-rc" <<'RC'
PS1='❯ '
bind -x '"\C-g": printf "INTERRUPT_KEY_RECEIVED\n"'
RC
cat > "$RUN_ROOT/collars/interruptible.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'ENV=$RUN_ROOT/interrupt-rc exec bash --posix' fixture"
GANG_INTERRUPT_KEY='C-g'
GANG_BUSY_REGEX='STILL_WORKING'
SH
"$HITCH" stoppable -c interruptible -d /tmp >/dev/null
stop_id="$(window_id stoppable)"
tmux set-option -w -t "$stop_id" @gl_turn "open $(date +%s)"
contains "an open turn bracket answers busy" \
  "$("$GANG" status stoppable)" "-busy-"
contains "interrupt reports the key it sent" \
  "$("$GANG" interrupt stoppable)" "C-g"
# Ordering barrier, not a wait on the thing under test: the pane consumes its
# input in order, so a command that runs at all runs after the keystroke ahead
# of it was processed. A missing or wrong keystroke therefore fails the mark
# below rather than hanging here — which it did until these two Enters existed.
# A line editor reads Escape as the start of a sequence and swallows whatever
# arrives next, so an interrupt key that leaves the pane mid-prefix would eat
# the first byte of this barrier's own command and the suite would wait forever
# on a signal that could never come.
tmux send-keys -t "$stop_id" Enter Enter
stop_settled="test-interrupt-settled-$$"
tmux wait-for "$stop_settled" &
stop_waiter=$!
tmux send-keys -l -t "$stop_id" "tmux wait-for -S $stop_settled"
tmux send-keys -t "$stop_id" Enter
wait "$stop_waiter"
contains "and the declared key actually reached the pane" \
  "$(pane stoppable)" "INTERRUPT_KEY_RECEIVED"
equal "an interrupt drops the bracket nothing else will close" "" \
  "$(tmux show-options -wqv -t "$stop_id" @gl_turn)"
contains "so the keystroke cannot strand a false busy" \
  "$("$GANG" status stoppable)" "~idle~"
"$GANG" drop stoppable >/dev/null

# AND IT MUST NOT MANUFACTURE THE OPPOSITE LIE. Gang saw a keystroke leave; it
# did not see a turn end. A harness that ignored the key is still painting its
# busy marker, and that evidence has to survive the interrupt — writing a fresh
# closed bracket would answer idle before anything looked at the pane, and the
# next send would enter mid-turn on gang's own say-so.
"$HITCH" stubborn -c interruptible -d /tmp >/dev/null
stubborn_id="$(window_id stubborn)"
tmux send-keys -l -t "$stubborn_id" 'printf STILL_WORKING\\n'
tmux send-keys -t "$stubborn_id" Enter
tmux set-option -w -t "$stubborn_id" @gl_turn "open $(date +%s)"
"$GANG" interrupt stubborn >/dev/null
equal "the bracket is dropped there too" "" \
  "$(tmux show-options -wqv -t "$stubborn_id" @gl_turn)"
stubborn_state="$("$GANG" status stubborn)"
contains "a target still painting work stays busy after the interrupt" \
  "$stubborn_state" "-busy-"
excludes "the interrupt never invents an idle the pane contradicts" \
  "$stubborn_state" "~idle~"
if midturn_out="$(printf 'MARK_MIDTURN' |
  "$GANG" send --to stubborn --from tester --stdin 2>&1)"; then
  fail "and it stays unreachable while that work is painted" "send entered mid-turn"
else
  pass "and it stays unreachable while that work is painted"
fi
# Refused FOR THAT REASON. A bare non-zero exit is satisfied by any refusal this
# pane could raise — a draft, a lock, an occupied composer — so on its own it
# stops pinning the interrupt's consequence the moment anything else refuses.
contains "refused because the turn is still running, not for some other reason" \
  "$midturn_out" "not safely reachable mid-turn"
excludes "and nothing was typed into it" "$(pane stubborn)" "MARK_MIDTURN"
"$GANG" drop stubborn >/dev/null

# The shipped harnesses that stop on Escape say so themselves; the ones whose
# interrupt gang has not observed declare nothing and refuse the command.
for stopping_collar in claude-code codex; do
  stopping_file="$ROOT/collars/$stopping_collar.sh"
  stopping_key="$(GANG_TEST_COLLARS='' ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
    '. "$1"; printf "%s" "${GANG_INTERRUPT_KEY:-}"' fixture "$stopping_file")"
  equal "the $stopping_collar collar declares the key that stops its turn" \
    "Escape" "$stopping_key"
done
for unstopping_collar in opencode pi; do
  unstopping_file="$ROOT/collars/$unstopping_collar.sh"
  unstopping_key="$(GANG_TEST_COLLARS='' ROOT="$ROOT" bash -c \
    '. "$1"; printf "%s" "${GANG_INTERRUPT_KEY:-}"' fixture "$unstopping_file")"
  equal "the $unstopping_collar collar declares no interrupt key until one is verified" \
    "" "$unstopping_key"
done
