# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Hooks and lights: the universal hook endpoint, stall and context lights, stop, the debounce critical section, hooksPath, the pre-push gate, and gated teardown.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# The universal hook endpoint records immediately against the firing pane.
alpha_id="$(window_id alpha)"
alpha_tmux_pane="$(tmux list-panes -t "$alpha_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' |
  TMUX_PANE="$alpha_tmux_pane" "$GANG" hook >/dev/null
turn_open="$(tmux show-options -wqv -t "$alpha_id" @gl_turn)"
contains "a native prompt hook opens the turn record" "$turn_open" "open"

printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$alpha_tmux_pane" "$GANG" hook >/dev/null
turn_closed="$(tmux show-options -wqv -t "$alpha_id" @gl_turn)"
contains "a native stop hook closes the turn record" "$turn_closed" "closed"

# WAIT IS AN OPT-IN EVENT BARRIER. The test observes successful hook arming
# through another latched tmux channel, so no sleep, polling, or timeout stands
# between the background caller and the native Stop it means to consume.
cat > "$RUN_ROOT/collars/waitable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
GANG_BUSY_REGEX='EXPLAIN_BUSY_[0-9]+'
GANG_OCCUPIED_REGEX='EXPLAIN_OCCUPIED_[0-9]+'
SH
cat > "$RUN_ROOT/wait-arm-env" <<'SH'
tmux() {
  local rc=0
  command tmux "$@" || rc=$?
  if [ "$rc" -eq 0 ] && [ "${1:-}" = set-hook ] \
     && [ "${2:-}" = -ag ] && [ "${3:-}" = pane-exited ]; then
    command tmux wait-for -S "$GANG_TEST_WAIT_ARM"
  fi
  return "$rc"
}
SH
"$HITCH" waitable -c waitable -d /tmp >/dev/null
waitable_id="$(window_id waitable)"
waitable_pane="$(tmux list-panes -t "$waitable_id" -F '#{pane_id}')"
tmux set-option -w -t "$waitable_id" @gl_turn "open $(date +%s)"
wait_arm="gang-test-wait-arm-$$-done"
BASH_ENV="$RUN_ROOT/wait-arm-env" GANG_TEST_WAIT_ARM="$wait_arm" \
  "$GANG" wait waitable --until "done" \
  >"$RUN_ROOT/wait-done.out" 2>"$RUN_ROOT/wait-done.err" &
wait_done_pid=$!
tmux wait-for "$wait_arm"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$waitable_pane" "$GANG" hook >/dev/null
if wait "$wait_done_pid"; then
  pass "a caller-chosen done barrier is released by the target's Stop"
else
  fail "a caller-chosen done barrier is released by the target's Stop" \
    "$(<"$RUN_ROOT/wait-done.err")"
fi
if "$GANG" wait waitable --until idle >/dev/null; then
  pass "an already-idle target satisfies an idle barrier immediately"
else
  fail "an already-idle target satisfies an idle barrier immediately" \
    "gang wait returned nonzero"
fi

# EXPLAIN INSTRUMENTS THIS STATE READ, rather than taking a diagnostic capture
# after the verdict. The fixture's own command signals only after producing the
# one line its busy rule can match, making that pane evidence immediately ready.
tmux set-option -uw -t "$waitable_id" @gl_turn
explain_painted="gang-test-explain-painted-$$"
printf -v explain_command \
  "printf 'EXPLAIN_BUSY_%%s\\\\n' \"\$((6*7))\"; tmux wait-for -S %q" \
  "$explain_painted"
tmux send-keys -l -t "$waitable_pane" "$explain_command"
tmux send-keys -t "$waitable_pane" Enter
tmux wait-for "$explain_painted"
explain_out="$("$GANG" explain waitable)"
contains "explain reports the live state its rules produced" \
  "$explain_out" "state: -busy-"
contains "explain names the collar busy rule that matched" \
  "$explain_out" "GANG_BUSY_REGEX: matched"
contains "explain prints the exact busy regex" \
  "$explain_out" "rule: EXPLAIN_BUSY_[0-9]+"
contains "explain prints the pane fragment that matched" \
  "$explain_out" "fragment: EXPLAIN_BUSY_42"
contains "explain distinguishes a tested occupancy miss" \
  "$explain_out" "GANG_OCCUPIED_REGEX: did not match"

refuses "a target without a Stop source cannot arm a hanging barrier" \
  "declares no GANG_STOP_HOOK" "$GANG" wait alpha --until "done"

tmux set-option -w -t "$waitable_id" @gl_turn malformed
refuses "unknown state is refused before a barrier can hang" \
  "refusing to hang" "$GANG" wait waitable --until idle
tmux set-option -w -t "$waitable_id" @gl_turn "open $(date +%s)"
wait_arm="gang-test-wait-arm-$$-vanish"
BASH_ENV="$RUN_ROOT/wait-arm-env" GANG_TEST_WAIT_ARM="$wait_arm" \
  "$GANG" wait waitable --until "done" \
  >"$RUN_ROOT/wait-vanish.out" 2>"$RUN_ROOT/wait-vanish.err" &
wait_vanish_pid=$!
tmux wait-for "$wait_arm"
"$GANG" drop waitable >/dev/null
if wait "$wait_vanish_pid"; then
  fail "a vanished target releases the barrier loudly" \
    "gang wait unexpectedly returned success"
else
  contains "a vanished target releases the barrier loudly" \
    "$(<"$RUN_ROOT/wait-vanish.err")" "vanished"
fi

# Stall lights forward only native awaiting-input witnesses to one optional
# declared target. Every outcome is synchronous: the hook returns after the
# note is accepted live, parked, or recorded as failed.
cat > "$RUN_ROOT/collars/stallable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
GANG_STALL_TYPES="idle_prompt agent_needs_input"
SH
equal "a team starts without an inferred notify target" \
  "no notify target declared" "$("$GANG" notify)"
"$HITCH" stall-raise -c stallable -d /tmp >/dev/null
"$HITCH" stall-target -c stallable -d /tmp >/dev/null
stall_raise_id="$(window_id stall-raise)"
stall_raise_pane="$(tmux list-panes -t "$stall_raise_id" -F '#{pane_id}')"
stall_target_id="$(window_id stall-target)"
stall_target_pane="$(tmux list-panes -t "$stall_target_id" -F '#{pane_id}')"
equal "a notify target may be declared without inference" \
  "notify target: stall-target" "$("$GANG" notify stall-target)"
equal "the notify declaration is readable" \
  "notify target: stall-target" "$("$GANG" notify)"

# A collar with no declared stall kinds has no Notification witness. In
# particular, an absent notification_type must not match an empty declaration.
cat > "$RUN_ROOT/collars/no-stalls.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
SH
"$HITCH" stall-silent -c no-stalls -d /tmp >/dev/null
stall_silent_id="$(window_id stall-silent)"
stall_silent_pane="$(tmux list-panes -t "$stall_silent_id" -F '#{pane_id}')"
stall_silent_before="$(pane_all stall-target)"
printf '%s' '{"hook_event_name":"Notification"}' |
  TMUX_PANE="$stall_silent_pane" "$GANG" hook >/dev/null
equal "a collar without a declared stall source raises no empty-kind note" \
  "$stall_silent_before" "$(pane_all stall-target)"
"$GANG" drop stall-silent >/dev/null

# Stop owns the universal turn boundary even when the window's old collar no
# longer resolves. Collar loading belongs only to collar-dependent work.
"$HITCH" stall-vanished -c stallable -d /tmp >/dev/null
stall_vanished_id="$(window_id stall-vanished)"
stall_vanished_pane="$(tmux list-panes -t "$stall_vanished_id" -F '#{pane_id}')"
tmux set-option -w -t "$stall_vanished_id" @gl_turn open
tmux set-option -w -t "$stall_vanished_id" @gl_collar vanished
if vanished_stop="$(printf '%s' '{"hook_event_name":"Stop"}' |
    TMUX_PANE="$stall_vanished_pane" "$GANG" hook 2>&1)"; then
  pass "a native Stop closes the turn after its collar vanishes"
else
  fail "a native Stop closes the turn after its collar vanishes" \
    "hook failed before the universal boundary: [$vanished_stop]"
fi
contains "the collar-independent Stop boundary is recorded" \
  "$(tmux show-options -wqv -t "$stall_vanished_id" @gl_turn)" "closed"
"$GANG" drop stall-vanished >/dev/null

printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_first="$(pane_all stall-target)"
contains "a native awaiting-input witness reaches the declared target" \
  "$stall_first" "stall: stall-raise is awaiting input (idle_prompt)"
contains "the stall note keeps the raising window's attribution" \
  "$stall_first" "[gang:stall-raise#"
stall_first_count="$(printf '%s\n' "$stall_first" | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_repeat_count="$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
equal "the same native stall inside the debounce is one note" \
  "$stall_first_count" "$stall_repeat_count"

printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_after_move_count="$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
equal "a native movement event opens a new stall epoch" \
  "$(( stall_first_count + 1 ))" "$stall_after_move_count"

stall_old_now="$(date +%s)"
tmux set-option -w -t "$stall_raise_id" @gl_stall \
  "idle_prompt $(( stall_old_now - 601 ))"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_old_count="$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"
equal "an old debounce stamp permits another native note" \
  "$(( stall_after_move_count + 1 ))" "$stall_old_count"

printf '%s' '{"hook_event_name":"Notification","notification_type":"auth_success"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "a notification kind outside the collar declaration raises nothing" \
  "$stall_old_count" "$(pane_all stall-target | grep -oF 'awaiting input (idle_prompt)' | wc -l | tr -d ' ')"

printf '%s' '{"hook_event_name":"PermissionRequest"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a native permission request is an awaiting-input witness" \
  "$(pane_all stall-target)" "awaiting input (permission_prompt)"

"$GANG" notify clear >/dev/null
stall_cleared_before="$(pane_all stall-target)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "no declaration is the silent off state" \
  "$stall_cleared_before" "$(pane_all stall-target)"
equal "clear removes the session declaration" \
  "no notify target declared" "$("$GANG" notify)"

"$GANG" notify stall-raise >/dev/null
stall_self_before="$(pane_all stall-raise)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "a stall note is never sent into the raising pane" \
  "$stall_self_before" "$(pane_all stall-raise)"
equal "self-target suppression records no delivery failure" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"

# An existing tmux window that has not been adopted reaches the attempted
# delivery region and fails there. That failure must not stamp the debounce:
# adopting the same window makes an immediate retry observable.
tmux new-window -d -t "=$GANG_SESSION" -n stall-unadopted -c /tmp \
  "PS1='❯ ' bash --norc"
"$GANG" notify stall-unadopted >/dev/null
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_unaccepted_before="$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "an unadopted notify target fails inside the delivery attempt" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)" \
  "exists but is not a gang agent"
equal "an unaccepted note leaves the debounce stamp unchanged" \
  "$stall_unaccepted_before" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)"
"$GANG" adopt stall-unadopted -c stallable >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "adoption lets the unaccepted note retry without advancing time" \
  "$(pane_all stall-unadopted)" "awaiting input (agent_needs_input)"
equal "the accepted retry retires the attempted-delivery failure" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"
"$GANG" drop stall-unadopted >/dev/null

"$GANG" notify ghost >/dev/null
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a missing notify target is visible in status" \
  "$("$GANG" status stall-raise)" "stall note NOT accepted"
contains "roster carries a failed stall light" \
  "$("$GANG" roster)" "stall-note-failed"
equal "a missing target writes no debounce stamp" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)"

"$HITCH" ghost -c stallable -d /tmp >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a failed note retries once its target exists" \
  "$(pane_all ghost)" "awaiting input (agent_needs_input)"
equal "an accepted repair retires the delivery failure" "" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"
excludes "status no longer reports the repaired stall light" \
  "$("$GANG" status stall-raise)" "stall note NOT accepted"

"$GANG" drop ghost >/dev/null
"$GANG" notify ghost >/dev/null
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
stall_failure_before="$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
equal "movement does not retire a still-broken stall light" \
  "$stall_failure_before" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall_failed)"

"$GANG" notify stall-target >/dev/null
tmux send-keys -l -t "$stall_target_id" 'HUMAN_DRAFT'
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a refused stall note is accepted into a drainable target's spool" \
  "$("$GANG" status stall-target)" "spooled: 1"
contains "parking a stall note records the debounce" \
  "$(tmux show-options -wqv -t "$stall_raise_id" @gl_stall)" "agent_needs_input"
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_needs_input"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null
contains "a parked stall note is debounced without a duplicate" \
  "$("$GANG" status stall-target)" "spooled: 1"
tmux send-keys -t "$stall_target_id" C-u
tmux wait-for "gang-spool-drain-$stall_target_id" &
stall_target_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$stall_target_pane" "$GANG" hook >/dev/null
wait "$stall_target_waiter"
contains "the parked stall note drains through the ordinary delivery path" \
  "$(pane_all stall-target)" "awaiting input (agent_needs_input)"

# THE DEBOUNCE IS A CRITICAL SECTION, NOT A PAIR OF READS. Two native witnesses
# of one kind for one window can both read the stale stamp and both deliver, so
# the declared target gets the same envelope twice inside the interval the
# debounce exists to enforce. Every assertion above it is sequential and cannot
# see that. This one holds the first delivery open INSIDE the target's own
# collar_input on a tmux barrier — an event, not a clock — and fires the second
# while the first is provably still in flight.
stall_gate_inside="test-stall-gate-inside-$$"
stall_gate_release="test-stall-gate-release-$$"
cat > "$RUN_ROOT/collars/stall-gated.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
GANG_STALL_TYPES="idle_prompt agent_needs_input"
_gl_gate_real="\$(declare -f collar_input)"
eval "stall_gate_real_input \${_gl_gate_real#collar_input}"
collar_input() { # hold exactly one delivery open, once, on an armed barrier
  if [ -f "$RUN_ROOT/stall-gate-arm" ]; then
    rm -f "$RUN_ROOT/stall-gate-arm"
    tmux wait-for -S "$stall_gate_inside"
    tmux wait-for "$stall_gate_release"
  fi
  stall_gate_real_input "\$1"
}
SH
"$HITCH" stall-gated -c stall-gated -d /tmp >/dev/null
stall_gated_id="$(window_id stall-gated)"
"$GANG" notify stall-gated >/dev/null
tmux set-option -uw -t "$stall_raise_id" @gl_stall
tmux set-option -uw -t "$stall_raise_id" @gl_stall_failed
: > "$RUN_ROOT/stall-gate-arm"
( printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null 2>&1 ) &
stall_race_pid=$!
tmux wait-for "$stall_gate_inside"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' |
  TMUX_PANE="$stall_raise_pane" "$GANG" hook >/dev/null 2>&1 || true
tmux wait-for -S "$stall_gate_release"
wait "$stall_race_pid" || true
stall_gated_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$stall_gated_id" @gl_spool)"
stall_race_spooled=0
for stall_race_entry in "$stall_gated_spool"/[0-9]* \
    "$stall_gated_spool"/sending-* "$stall_gated_spool"/failed-*; do
  [ -f "$stall_race_entry" ] || continue
  stall_race_spooled=$((stall_race_spooled + 1))
done
stall_race_live="$(pane_all stall-gated |
  grep -cF 'awaiting input (idle_prompt)' || true)"
equal "concurrent native witnesses of one kind deliver exactly one note" \
  "1 0" "$stall_race_live $stall_race_spooled"
"$GANG" drop stall-gated >/dev/null
"$GANG" notify stall-target >/dev/null

if "$GANG" notify 'bad name' >/dev/null 2>&1; then
  fail "notify rejects a name outside the agent-name contract" \
    "invalid name was accepted"
else
  pass "notify rejects a name outside the agent-name contract"
fi
"$GANG" notify clear >/dev/null
"$GANG" drop stall-raise >/dev/null
"$GANG" drop stall-target >/dev/null

# Optional context guidance has two edge-triggered states and no clock path.
cat > "$RUN_ROOT/collars/lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_context() {
  tmux show-options -wqv -t "\$1" @test_context
}
SH
GANG_CONTEXT_LIGHTS=100000,200000 "$HITCH" lit -c lights -d /tmp >/dev/null
lit_id="$(window_id lit)"
lit_tmux_pane="$(tmux list-panes -t "$lit_id" -F '#{pane_id}')"
warming="$(printf '%s' '{"hook_event_name":"UserPromptSubmit"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
equal "context warm-up is silent before the first completed turn" "" "$warming"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook >/dev/null
tmux set-option -w -t "$lit_id" @test_context '150k/300k (50%)'
yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "enabled lights expose yellow" "$yellow" "Yellow context light"
repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
equal "a context light is emitted once per context epoch" "" "$repeat"
tmux set-option -w -t "$lit_id" @test_context '250k/300k (83%)'
red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "enabled lights expose red" "$red" "Red context light"
tmux set-option -w -t "$lit_id" @test_context '50k/300k (17%)'
printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook >/dev/null
tmux set-option -w -t "$lit_id" @test_context '150k/300k (50%)'
yellow_again="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "dropping below yellow starts a new context epoch" \
  "$yellow_again" "Yellow context light"
tmux set-option -w -t "$lit_id" @test_context 'unreadable'
unavailable="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
contains "an enabled light source fails visibly to its own agent" \
  "$unavailable" "Context lights unavailable"
unavailable_repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_tmux_pane" "$GANG" hook)"
equal "an unavailable context source reports once per failure epoch" \
  "" "$unavailable_repeat"
excludes "status does not inspect another agent's context" \
  "$("$GANG" status lit)" "context"

# ONE RELATIVE SPEC SERVES DIFFERENT NATIVE WINDOWS. The same percentages are
# copied at hitch; each hook resolves them against the window in its own native
# reading rather than spending one team's absolute thresholds on both harnesses.
for relative_case in small large zero; do
  GANG_CONTEXT_LIGHTS=50%,80% "$HITCH" "lit-$relative_case" \
    -c lights -d /tmp >/dev/null
done
lit_small_id="$(window_id lit-small)"
lit_small_pane="$(tmux list-panes -t "$lit_small_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_small_id" @test_context '130k/258k (50%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_small_pane" "$GANG" hook >/dev/null
small_yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_small_pane" "$GANG" hook)"
contains "relative yellow resolves inside the smaller native window" \
  "$small_yellow" "Yellow context light"
tmux set-option -w -t "$lit_small_id" @test_context '210k/258k (81%)'
small_red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_small_pane" "$GANG" hook)"
contains "relative red resolves inside the smaller native window" \
  "$small_red" "Red context light"

lit_large_id="$(window_id lit-large)"
lit_large_pane="$(tmux list-panes -t "$lit_large_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_large_id" @test_context '600k/1000k (60%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_large_pane" "$GANG" hook >/dev/null
large_yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_large_pane" "$GANG" hook)"
contains "the same relative yellow resolves inside the larger native window" \
  "$large_yellow" "Yellow context light"
tmux set-option -w -t "$lit_large_id" @test_context '850k/1000k (85%)'
large_red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_large_pane" "$GANG" hook)"
contains "the same relative red resolves inside the larger native window" \
  "$large_red" "Red context light"

lit_zero_id="$(window_id lit-zero)"
lit_zero_pane="$(tmux list-panes -t "$lit_zero_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_zero_id" @test_context '0k/0k (0%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_zero_pane" "$GANG" hook >/dev/null
zero_window="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_zero_pane" "$GANG" hook)"
contains "a zero native window is invalid rather than an immediate red light" \
  "$zero_window" "native context source reported a zero-token window"

GANG_CONTEXT_LIGHTS=350000,500000 "$HITCH" lit-impossible \
  -c lights -d /tmp >/dev/null
lit_impossible_id="$(window_id lit-impossible)"
lit_impossible_pane="$(tmux list-panes -t "$lit_impossible_id" -F '#{pane_id}')"
tmux set-option -w -t "$lit_impossible_id" @test_context '100k/258k (39%)'
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$lit_impossible_pane" "$GANG" hook >/dev/null
tmux set-option -w -t "$lit_impossible_id" @gl_context_light red
impossible="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_impossible_pane" "$GANG" hook)"
contains "an impossible absolute spec supersedes a stale red latch" \
  "$impossible" "red threshold 500000 cannot fire in this harness's 258000-token window"
impossible_repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$lit_impossible_pane" "$GANG" hook)"
equal "an impossible absolute spec reports once per invalid epoch" \
  "" "$impossible_repeat"

if mixed_light_out="$(GANG_CONTEXT_LIGHTS=50%,200000 "$GANG" hitch \
    lit-mixed-units -c lights -d /tmp 2>&1)"; then
  fail "context-light thresholds in mixed units are refused" \
    "hitch unexpectedly succeeded"
else
  contains "the mixed-unit refusal names the one-unit rule" \
    "$mixed_light_out" "must use the same unit for both thresholds"
fi
equal "the mixed-unit refusal opens no window" "" \
  "$(window_id lit-mixed-units)"

# Provider-usage lights use a collar's non-interactive correctness source. The
# native rows carry their own observation and reset clocks; status and roster
# only report the last sampled fact and never drive a pane while observing.
cat > "$RUN_ROOT/collars/usage-lights.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
GANG_USAGE_LIGHT_INTERVAL=60
GANG_USAGE_LIMIT_MAX_AGE=300
collar_usage_limits() {
  calls="\$(tmux show-options -wqv -t "\$1" @test_usage_calls 2>/dev/null)"
  case "\$calls" in ''|*[!0-9]*) calls=0 ;; esac
  tmux set-option -w -t "\$1" @test_usage_calls "\$(( calls + 1 ))"
  tmux show-options -wqv -t "\$1" @test_usage
}
SH
usage_now="$(date +%s)"
GANG_USAGE_LIGHTS=90%,95% "$HITCH" usage-lit -c usage-lights -d /tmp >/dev/null
usage_lit_id="$(window_id usage-lit)"
usage_lit_pane="$(tmux list-panes -t "$usage_lit_id" -F '#{pane_id}')"
tmux set-option -w -t "$usage_lit_id" @test_usage \
  "Current session"$'\t'"91"$'\t'"$(( usage_now + 600 ))"$'\t'"$usage_now"$'\n'\
"Current week"$'\t'"80"$'\t'"$(( usage_now + 86400 ))"$'\t'"$usage_now"
usage_yellow="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$usage_lit_pane" "$GANG" hook)"
contains "a native provider reading crosses the configured yellow band" \
  "$usage_yellow" "Yellow usage light"
usage_repeat="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$usage_lit_pane" "$GANG" hook)"
equal "a provider-usage edge is emitted once per usage epoch" "" "$usage_repeat"
equal "a heavyweight native reader is throttled between nearby hooks" "1" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @test_usage_calls)"
tmux set-option -w -t "$usage_lit_id" @test_usage \
  "Current session"$'\t'"95"$'\t'"$(( usage_now + 600 ))"$'\t'"$usage_now"$'\n'\
"Current week"$'\t'"85"$'\t'"$(( usage_now + 86400 ))"$'\t'"$usage_now"
tmux set-option -w -t "$usage_lit_id" @gl_usage_checked "$(( usage_now - 60 ))"
usage_red="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$usage_lit_pane" "$GANG" hook)"
contains "a native provider reading crosses the configured red band" \
  "$usage_red" "Red usage light"
contains "status carries the last provider-usage warning without sampling" \
  "$("$GANG" status usage-lit)" "provider usage: Current session is 95% used"
contains "roster carries the provider-usage warning at a glance" \
  "$("$GANG" roster | grep '^usage-lit ')" "usage=red"

usage_limits_out="$("$GANG" limits usage-lit)"
contains "limits prints the collar's native reset and sample age" \
  "$usage_limits_out" "Current session: 95% used"
tmux set-option -w -t "$usage_lit_id" @test_usage \
  $'provider\033[31mred\033[0m\t95\t'"$(( usage_now + 600 ))"$'\t'"$usage_now"
sanitized_usage_limits="$("$GANG" limits usage-lit)"
case "$sanitized_usage_limits" in
  *$'\033'*) fail "provider labels cannot write terminal control bytes" \
    "raw escape survived" ;;
  *) pass "provider labels cannot write terminal control bytes" ;;
esac
contains "provider label controls become visible placeholders" \
  "$sanitized_usage_limits" "provider?[31mred?[0m"

tmux set-option -w -t "$usage_lit_id" @test_usage \
  "Current session"$'\t'"95"$'\t'"$(( usage_now + 600 ))"$'\t'"$(( usage_now - 301 ))"
tmux set-option -w -t "$usage_lit_id" @gl_usage_checked "$(( usage_now - 60 ))"
usage_stale="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$usage_lit_pane" "$GANG" hook)"
contains "a stale native event cannot drive a usage light" \
  "$usage_stale" "too old to act on"
tmux set-option -w -t "$usage_lit_id" @test_usage \
  "Current session"$'\t'"95"$'\t'"$(( usage_now + 600 ))"$'\t'"$usage_now"$'\n'\
"Current week"$'\t'"85"$'\t'"$(( usage_now + 86400 ))"$'\t'"$usage_now"

GANG_USAGE_LIGHTS=90%,95% "$HITCH" usage-absent -c stallable -d /tmp >/dev/null
usage_absent_id="$(window_id usage-absent)"
usage_absent_pane="$(tmux list-panes -t "$usage_absent_id" -F '#{pane_id}')"
usage_absent_note="$(printf '%s' '{"hook_event_name":"PostToolUse"}' |
  TMUX_PANE="$usage_absent_pane" "$GANG" hook)"
contains "a collar with no correctness source degrades loudly" \
  "$usage_absent_note" "declares no non-interactive provider-limit source"
contains "status retains an unavailable provider-usage capability" \
  "$("$GANG" status usage-absent)" "provider usage unavailable"
refuses "limits fabricates nothing for a collar with no source" \
  "declares no non-interactive provider-limit source" "$GANG" limits usage-absent

if invalid_usage_lights="$(GANG_USAGE_LIGHTS=95%,90% "$GANG" hitch \
    usage-invalid -c usage-lights -d /tmp 2>&1)"; then
  fail "decreasing provider-usage thresholds are refused" "hitch succeeded"
else
  contains "the provider-usage threshold refusal names the ordering" \
    "$invalid_usage_lights" "must increase from yellow to red"
fi
equal "an invalid provider-usage spec opens no window" "" \
  "$(window_id usage-invalid)"

# A reset wake is a transient systemd user timer, not a watcher. The fake
# accepts the unit synchronously and records its exact one-shot invocation;
# firing is then driven directly against the already-established reset state.
usage_timer_bin="$RUN_ROOT/usage-timer-bin"
usage_timer_args="$RUN_ROOT/usage-timer.args"
usage_timer_stops="$RUN_ROOT/usage-timer.stops"
usage_real_date="$(command -v date)"
mkdir -p "$usage_timer_bin"
cat > "$usage_timer_bin/systemd-run" <<SH
#!/bin/sh
printf '%s\n' "\$@" > "$usage_timer_args"
SH
# The fake systemctl is a state machine, not a yes-man: a committed fixture
# where every stop succeeds cannot tell a unit that is gone from one that is
# still running, and those are exactly the two cases --clear has to separate.
cat > "$usage_timer_bin/systemctl" <<SH
#!/bin/sh
printf '%s\n' "\$@" >> "$usage_timer_stops"
case "\${GANG_TEST_SYSTEMCTL:-ok}" in
  gone)  # the unit already fired and was collected: stop fails, nothing is active
    case "\$*" in *is-active*) exit 3 ;; *) exit 5 ;; esac ;;
  stuck) # the stop failed and the unit is still running
    case "\$*" in *is-active*) exit 0 ;; *) exit 5 ;; esac ;;
  *) exit 0 ;;
esac
SH
cat > "$usage_timer_bin/date" <<SH
#!/bin/sh
if [ "\${1:-}" = +%s ] && [ -n "\${GANG_TEST_NOW:-}" ]; then
  printf '%s\n' "\$GANG_TEST_NOW"
else
  exec "$usage_real_date" "\$@"
fi
SH
chmod +x "$usage_timer_bin/systemd-run" "$usage_timer_bin/systemctl" \
  "$usage_timer_bin/date"
usage_wait_out="$(PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit \
  --resume 'Resume the assigned usage-light fixture.')"
contains "wait-limit reports the native window it scheduled" \
  "$usage_wait_out" "Current session, 95% used"
contains "wait-limit creates a transient collected user timer" \
  "$(<"$usage_timer_args")" "--collect"
contains "the transient timer fires the exact Gangline reset command" \
  "$(<"$usage_timer_args")" "--fire"
contains "the timer inherits a private tmux socket directory" \
  "$(<"$usage_timer_args")" "TMUX_TMPDIR=$TMUX_TMPDIR"
contains "arming a reset wake preserves the red roster light" \
  "$("$GANG" roster | grep '^usage-lit ')" "usage=red"
usage_wake_record="$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"
IFS=$'\t' read -r usage_wake_reset usage_wake_unit _ <<<"$usage_wake_record"
contains "the timer callback carries the unit it was armed as" \
  "$(<"$usage_timer_args")" "$usage_wake_unit"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$usage_lit_pane" "$GANG" hook >/dev/null
PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit \
  --fire "$usage_wake_reset" --unit "$usage_wake_unit" >/dev/null
equal "an early internal callback leaves the reset wake armed" \
  "$usage_wake_record" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"

# A TIMER THAT OUTLIVED ITS TEAM must not consume a declaration it did not arm.
# The reset and the agent name are the same; only the unit differs, which is
# what the recreated-team case looks like from the callback's side.
refuses "an internal callback with no unit is refused" \
  "requires the --unit that armed it" \
  env PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit \
    --fire "$usage_wake_reset"
GANG_TEST_NOW="$usage_wake_reset" PATH="$usage_timer_bin:$PATH" \
  "$GANG" wait-limit usage-lit --fire "$usage_wake_reset" \
  --unit "${usage_wake_unit}-stale" >/dev/null
equal "a callback naming another unit consumes nothing" \
  "$usage_wake_record" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"
excludes "and delivers nothing into the window it was aimed at" \
  "$(pane_all usage-lit)" "Resume the assigned usage-light fixture."

GANG_TEST_NOW="$usage_wake_reset" PATH="$usage_timer_bin:$PATH" \
  "$GANG" wait-limit usage-lit --fire "$usage_wake_reset" \
  --unit "$usage_wake_unit" >/dev/null
# source-guard: producer@db5579b969d6: the exact body is unique to the pending wake record, and the adjacent empty declaration proves this fire consumed that record through its success path rather than merely finding unrelated transcript text
contains "the reset wake resumes through attributed verified delivery" \
  "$(pane_all usage-lit)" "Resume the assigned usage-light fixture."
equal "a fired reset wake retires its tmux declaration" "" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"

: > "$usage_timer_stops"
PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit >/dev/null
usage_clear_unit="$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake |
  cut -f2)"
PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit --clear >/dev/null
contains "wait-limit clear stops the exact transient timer" \
  "$(<"$usage_timer_stops")" "$usage_clear_unit.timer"
contains "and the service that timer would already have started" \
  "$(<"$usage_timer_stops")" "$usage_clear_unit.service"
equal "clearing a reset wake retires its tmux declaration" "" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"

# A COLLECTED UNIT IS THE STATE --clear ASKED FOR. `systemctl stop` reports a
# unit that is not loaded as an error, and treating that as a failure left the
# declaration behind for status to keep promising.
PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit >/dev/null
usage_gone_rc=0
GANG_TEST_SYSTEMCTL=gone PATH="$usage_timer_bin:$PATH" \
  "$GANG" wait-limit usage-lit --clear >/dev/null 2>&1 || usage_gone_rc=$?
equal "clearing a collected timer succeeds" "0" "$usage_gone_rc"
equal "and the declaration is gone with it" "" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"

# A UNIT STILL RUNNING AFTER THE STOP is the operator's problem to hear about,
# and the declaration is still cleared: a wake nothing will deliver must not be
# left where status reads it as scheduled.
PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit >/dev/null
usage_stuck_rc=0
usage_stuck_err="$(GANG_TEST_SYSTEMCTL=stuck PATH="$usage_timer_bin:$PATH" \
  "$GANG" wait-limit usage-lit --clear 2>&1 >/dev/null)" || usage_stuck_rc=$?
if [ "$usage_stuck_rc" -eq 0 ]; then
  fail "an unstoppable timer refuses loudly" "clear reported success"
else
  contains "an unstoppable timer refuses loudly" "$usage_stuck_err" \
    "is still running"
fi
contains "the refusal hands over the exact recovery command" \
  "$usage_stuck_err" "systemctl --user stop"
equal "and the declaration is cleared anyway" "" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"

cat > "$usage_timer_bin/systemd-run" <<'SH'
#!/bin/sh
exit 17
SH
if PATH="$usage_timer_bin:$PATH" "$GANG" wait-limit usage-lit >/dev/null 2>&1; then
  fail "a refused transient timer is never reported as scheduled" "wait-limit succeeded"
else
  pass "a refused transient timer is never reported as scheduled"
fi
equal "a refused transient timer leaves no wake declaration" "" \
  "$(tmux show-options -wqv -t "$usage_lit_id" @gl_usage_wake)"

"$GANG" drop usage-absent >/dev/null
"$GANG" drop usage-lit >/dev/null

# A repository-local hooksPath shadows the operator's global path, so the
# tracked pre-push hook must delegate outward before it runs Gangline's gates.
# An ambient legacy depth marker must not suppress either delegation or local
# checks. Recursive chaining is an identity decision in Snubline's dispatcher.
delegation_root="$RUN_ROOT/pre-push-delegation"
delegation_hooks="$delegation_root/hooks"
delegation_config="$delegation_root/gitconfig"
delegation_record="$delegation_root/record"
delegation_input="$delegation_root/input"
mkdir -p "$delegation_hooks"
cat > "$delegation_hooks/pre-push" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cat > "$GANGLINE_OUTER_INPUT"
{
  printf 'argv:%s|%s\n' "$1" "$2"
  cat "$GANGLINE_OUTER_INPUT"
} > "$GANGLINE_OUTER_RECORD"
printf '%s\n' OUTER_STDOUT_MARKER
printf '%s\n' OUTER_STDERR_MARKER >&2
exit "${GANGLINE_OUTER_RC:-0}"
SH
chmod +x "$delegation_hooks/pre-push"
git config --file "$delegation_config" core.hooksPath "$delegation_hooks"
export GANGLINE_OUTER_INPUT="$delegation_input"
export GANGLINE_OUTER_RECORD="$delegation_record"
zero_oid="$(printf '%040d' 0)"
remote_oid="$(printf '%040d' 1)"
deletion_record="refs/heads/topic $zero_oid refs/heads/topic $remote_oid"
outer_success_output="$(printf '%s\n' "$deletion_record" |
  HOOK_DELEGATION_DEPTH=1 GIT_CONFIG_GLOBAL="$delegation_config" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote 2>&1)"
equal "an ambient delegation marker cannot suppress the outer gate" \
  "argv:origin|/tmp/remote
$deletion_record" "$(cat "$delegation_record")"
# The outer gate's own two lines are the whole of this reading: no banner, no
# duplicate, no wrapping, nothing else. Exactly one dimension is normalised away
# and it is the only one that is not determined — with the output inherited
# rather than captured, the outer hook's stdout is block-buffered into this
# capture pipe while its stderr is not, so which marker lands first is a
# property of stdio, not of Gangline. Sorting settles that and nothing else.
equal "the outer gate's two lines are the whole of the output, once each" \
  "OUTER_STDERR_MARKER
OUTER_STDOUT_MARKER" \
  "$(printf '%s\n' "$outer_success_output" | LC_ALL=C sort)"

# Exercise the ordering property with local output after the outer gate. An
# empty-tree commit has no suite, so Gangline's own missing-gate refusal is the
# following line without paying for a nested integration run.
order_hooks="$RUN_ROOT/pre-push-ordering-hooks"
order_config="$RUN_ROOT/pre-push-ordering-gitconfig"
order_remote="$RUN_ROOT/pre-push-ordering-remote.git"
mkdir -p "$order_hooks"
GIT_CONFIG_GLOBAL="$order_config" git init -q --bare "$order_remote"
cat > "$order_hooks/pre-push" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cat > /dev/null
printf 'OUTER-VERDICT-MARKER\n' >&2
exit 0
SH
chmod +x "$order_hooks/pre-push"
git config --file "$order_config" core.hooksPath "$order_hooks"
order_oid="$(GIT_AUTHOR_NAME='Gangline test' \
  GIT_AUTHOR_EMAIL='test@gangline.invalid' \
  GIT_COMMITTER_NAME='Gangline test' \
  GIT_COMMITTER_EMAIL='test@gangline.invalid' \
  git -C "$ROOT" commit-tree \
  "$(git -C "$ROOT" mktree </dev/null)" -m 'feat: lintless ordering unit')"
order_output="$(
  cd "$ROOT"
  printf 'refs/heads/main %s refs/heads/main %s\n' \
    "$order_oid" "$(printf '%040d' 0)" |
  GIT_CONFIG_GLOBAL="$order_config" \
    "$ROOT/.githooks/pre-push" origin "$order_remote" 2>&1
)" || true
order_gate_line="$(printf '%s\n' "$order_output" |
  awk '/carries no test\/lint.sh/ { print NR; exit }')"
order_marker_line="$(printf '%s\n' "$order_output" |
  awk '/OUTER-VERDICT-MARKER/ { print NR; exit }')"
contains "the ordering fixture reaches Gangline's own gate" \
  "$order_output" "carries no test/lint.sh"
contains "the ordering fixture reaches the outer gate" \
  "$order_output" "OUTER-VERDICT-MARKER"
equal "the outer verdict is printed before Gangline's own output" \
  "before" \
  "$(if [ -n "$order_gate_line" ] && [ -n "$order_marker_line" ] \
        && [ "$order_marker_line" -lt "$order_gate_line" ]; then
       printf before
     else
       printf after
     fi)"

# Ordering after the fact is not the property. A capture that replayed the
# outer gate's output immediately before Gangline's own gates would satisfy
# every assertion above and still withhold each progress line until the outer
# process exited, which is the whole defect. So the outer fixture proves the
# live property from inside its own run, and needs no barrier to hang on: it
# writes, and then — still running — reads the file the local hook's own output
# is going to. Inherited, its lines are already there. Captured, they are in a
# temporary file this fixture cannot name.
#
# Both streams, because the contract is both and a capture of either one alone
# passes every other assertion here. stderr is unbuffered and needs nothing.
# stdout is written by awk rather than by this shell: a child's exit flushes it
# into the inherited descriptor, where this shell's own printf would still be
# sitting in a stdio buffer with nothing to distinguish it from a capture. The
# record names the streams that arrived rather than answering yes or no, so a
# failure says which one was withheld.
live_hooks="$RUN_ROOT/pre-push-live-hooks"
live_config="$RUN_ROOT/pre-push-live-gitconfig"
live_out="$RUN_ROOT/pre-push-live.out"
live_record="$RUN_ROOT/pre-push-live.record"
mkdir -p "$live_hooks"
cat > "$live_hooks/pre-push" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cat > /dev/null
awk 'BEGIN { print "OUTER-PROGRESS-STDOUT" }'
printf 'OUTER-PROGRESS-STDERR\n' >&2
record=""
if grep -Fxq 'OUTER-PROGRESS-STDOUT' "$GANGLINE_LIVE_OUT"; then record="stdout"; fi
if grep -Fxq 'OUTER-PROGRESS-STDERR' "$GANGLINE_LIVE_OUT"; then record="$record stderr"; fi
printf '%s\n' "${record# }" > "$GANGLINE_LIVE_RECORD"
exit 0
SH
chmod +x "$live_hooks/pre-push"
git config --file "$live_config" core.hooksPath "$live_hooks"
export GANGLINE_LIVE_OUT="$live_out"
export GANGLINE_LIVE_RECORD="$live_record"
printf '%s\n' "$deletion_record" |
  GIT_CONFIG_GLOBAL="$live_config" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote > "$live_out" 2>&1
equal "both of the outer gate's streams are on screen before that gate returns" \
  "stdout stderr" "$(cat "$live_record")"

outer_refusal_rc=0
outer_refusal_output="$(printf '%s\n' "$deletion_record" |
  GANGLINE_OUTER_RC=23 GIT_CONFIG_GLOBAL="$delegation_config" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote 2>&1)" || outer_refusal_rc=$?
equal "an outer refusal is the local hook's exact status" 23 "$outer_refusal_rc"
contains "an outer refusal is named before Gangline's gates run" \
  "$outer_refusal_output" "outer gate refused with status 23"

empty_global="$delegation_root/empty-global"
: > "$empty_global"
no_outer_rc=0
no_outer_output="$(printf '%s\n' "$deletion_record" |
  GIT_CONFIG_GLOBAL="$empty_global" \
  "$ROOT/.githooks/pre-push" origin /tmp/remote 2>&1)" || no_outer_rc=$?
equal "no global hook is a silent no-op before Gangline's gates" \
  "0|" "$no_outer_rc|$no_outer_output"

# `ln -sf` DEREFERENCES a symlink-to-directory: with the destination already a
# link to a directory it writes <that directory>/gang and leaves the link
# standing, so the installer mutates a directory nobody named and only fails
# afterwards, when it tries to execute the still-directory destination.
installer_root="$RUN_ROOT/installer"
installer_src="$installer_root/src"
mkdir -p "$installer_src/bin"
cat > "$installer_src/bin/gang" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
echo bash
SH
chmod +x "$installer_src/bin/gang"
git init -q "$installer_src"
git -C "$installer_src" config user.name 'Gangline installer test'
git -C "$installer_src" config user.email 'installer@fixture.invalid'
git -C "$installer_src" add .
git -C "$installer_src" commit -qm 'test: installer source fixture'

installer_bin="$installer_root/bin"
installer_decoy="$installer_root/decoy"
mkdir -p "$installer_bin" "$installer_decoy"
ln -s "$installer_decoy" "$installer_bin/gang"
installer_rc=0
GANGLINE_REPO="$installer_src" \
  GANGLINE_HOME="$installer_root/home" GANGLINE_BIN="$installer_bin" \
  sh "$ROOT/install.sh" >/dev/null 2>&1 || installer_rc=$?
equal "the installer replaces a symlinked destination instead of writing through it" \
  "0 absent $installer_root/ho""me/bin/gang" \
  "$installer_rc $([ -e "$installer_decoy/gang" ] && printf written || printf absent) $(readlink "$installer_bin/gang" || true)"

installer_dir_bin="$installer_root/bin-dir"
mkdir -p "$installer_dir_bin/gang"
installer_dir_rc=0
installer_dir_out="$(GANGLINE_REPO="$installer_src" \
  GANGLINE_HOME="$installer_root/home-dir" GANGLINE_BIN="$installer_dir_bin" \
  sh "$ROOT/install.sh" 2>&1)" || installer_dir_rc=$?
equal "the installer refuses a destination it cannot replace rather than filling it" \
  "refused empty named" \
  "$([ "$installer_dir_rc" -ne 0 ] && printf refused || printf installed) $([ -e "$installer_dir_bin/gang/gang" ] && printf filled || printf empty) $([[ "$installer_dir_out" = *'is not a file or a symlink'* ]] && printf named || printf unnamed)"

# The local sibling gate uses destination identity rather than remote-tracking
# names, and refuses non-commit refs instead of treating an empty traversal as
# a clean result. Its fixtures isolate the local hook from the host-global gate.
gate_root="$RUN_ROOT/local-pre-push"
gate_global="$gate_root/empty-global"
mkdir -p "$gate_root"
: > "$gate_global"

gate_bare() { # $1 name
  local repo="$gate_root/$1.git"
  rm -rf "$repo"
  GIT_CONFIG_GLOBAL="$gate_global" git init -q --bare "$repo"
  printf '%s\n' "$repo"
}

gate_repo() { # $1 name
  local repo="$gate_root/$1"
  rm -rf "$repo"
  GIT_CONFIG_GLOBAL="$gate_global" git init -q "$repo"
  git -C "$repo" config user.name 'Gangline gate test'
  git -C "$repo" config user.email 'gangline@fixture.invalid'
  mkdir -p "$repo/test" "$repo/.githooks"
  cp "$ROOT/.githooks/commit-msg" "$repo/.githooks/commit-msg"
  cat > "$repo/test/lint.sh" <<'SH'
#!/bin/sh
exit 0
SH
  cp "$repo/test/lint.sh" "$repo/test/integration.sh"
  chmod +x "$repo/test/"*.sh "$repo/.githooks/commit-msg"
  printf '%s\n' clean > "$repo/content"
  git -C "$repo" add .
  git -C "$repo" commit -qm 'test: local gate base'
  git -C "$repo" config core.hooksPath "$ROOT/.githooks"
  printf '%s\n' "$repo"
}

gate_bad_commit() { # $1 repo, $2 content
  printf '%s\n' "$2" > "$1/content"
  git -C "$1" add content
  git -C "$1" commit --no-verify -qm 'not a conventional commit'
}

gate_push() { # $1 repo, remaining git-push args
  local repo="$1"
  shift
  GATE_PUSH_RC=0
  GATE_PUSH_OUTPUT="$(GIT_CONFIG_GLOBAL="$gate_global" \
    git -C "$repo" push "$@" 2>&1)" || GATE_PUSH_RC=$?
}

gate_ref() { git --git-dir="$1" rev-parse --verify "$2" 2>/dev/null || true; }

pushurl_repo="$(gate_repo local-gate-pushurl)"
pushurl_private="$(gate_bare local-gate-pushurl-private)"
pushurl_public="$(gate_bare local-gate-pushurl-public)"
git -C "$pushurl_repo" remote add dest "$pushurl_private"
gate_bad_commit "$pushurl_repo" clean-pushurl
GIT_CONFIG_GLOBAL="$gate_global" git -C "$pushurl_repo" push \
  --no-verify -qu dest HEAD:refs/heads/topic
git -C "$pushurl_repo" remote set-url --add --push dest "$pushurl_public"
gate_push "$pushurl_repo" dest HEAD:refs/heads/public-topic
equal "the repository gate refuses a same-name pushurl destination escape" \
  "blocked absent named" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([ -z "$(gate_ref "$pushurl_public" refs/heads/public-topic)" ] && printf absent || printf received) $([[ "$GATE_PUSH_OUTPUT" = *'do not conform'* ]] && printf named || printf unnamed)"

retarget_repo="$(gate_repo local-gate-retarget)"
retarget_old="$(gate_bare local-gate-retarget-old)"
retarget_new="$(gate_bare local-gate-retarget-new)"
git -C "$retarget_repo" remote add dest "$retarget_old"
gate_bad_commit "$retarget_repo" clean-retarget
GIT_CONFIG_GLOBAL="$gate_global" git -C "$retarget_repo" push \
  --no-verify -qu dest HEAD:refs/heads/topic
git -C "$retarget_repo" remote set-url dest "$retarget_new"
gate_push "$retarget_repo" dest HEAD:refs/heads/topic
equal "the repository gate refuses a same-name retargeted destination escape" \
  "blocked absent named" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([ -z "$(gate_ref "$retarget_new" refs/heads/topic)" ] && printf absent || printf received) $([[ "$GATE_PUSH_OUTPUT" = *'do not conform'* ]] && printf named || printf unnamed)"

blob_repo="$(gate_repo local-gate-blob)"
blob_remote="$(gate_bare local-gate-blob)"
blob_oid="$(printf '%s\n' clean-blob | git -C "$blob_repo" hash-object -w --stdin)"
git -C "$blob_repo" update-ref refs/blobs/direct "$blob_oid"
gate_push "$blob_repo" "$blob_remote" refs/blobs/direct:refs/blobs/direct
blob_present=0
git --git-dir="$blob_remote" cat-file -e "$blob_oid" 2>/dev/null && blob_present=1
equal "the repository gate refuses a direct blob ref without destination receipt" \
  "blocked blob absent" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([[ "$GATE_PUSH_OUTPUT" = *'blob, not a commit'* ]] && printf blob || printf unnamed) $([ "$blob_present" -eq 0 ] && printf absent || printf received)"

blob_mirror="$(gate_bare local-gate-blob-mirror)"
gate_push "$blob_repo" --mirror "$blob_mirror"
mirror_present=0
git --git-dir="$blob_mirror" cat-file -e "$blob_oid" 2>/dev/null && mirror_present=1
equal "the repository gate refuses a mirror carrying a blob ref" \
  "blocked absent" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([ "$mirror_present" -eq 0 ] && printf absent || printf received)"

# The pre-push gate must run git-aware lint from the pushed tree even when Git
# gives the hook a GIT_DIR pointing at the main repository. A staged-only file
# makes a leaked main index distinguishable from the detached worktree's index.
hook_repo="$RUN_ROOT/pre-push-repo"
hook_probe="$RUN_ROOT/pre-push-probe"
mkdir -p "$hook_repo/.githooks" "$hook_repo/test" "$hook_probe"
cp "$ROOT/.githooks/pre-push" "$ROOT/.githooks/commit-msg" \
  "$hook_repo/.githooks/"
chmod +x "$hook_repo/.githooks/pre-push" "$hook_repo/.githooks/commit-msg"
cat > "$hook_repo/test/lint.sh" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu
: "${PROBE_DIR:?}"
git ls-files >/dev/null
git rev-parse --absolute-git-dir > "$PROBE_DIR/gitdir"
git ls-files main-index-only > "$PROBE_DIR/index"
SH
cat > "$hook_repo/test/integration.sh" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
exit 0
SH
chmod +x "$hook_repo/test/lint.sh" "$hook_repo/test/integration.sh"
git -C "$hook_repo" init -q
git -C "$hook_repo" config user.name 'Gangline Test'
git -C "$hook_repo" config user.email 'gangline-test@example.invalid'
git -C "$hook_repo" add .
git -C "$hook_repo" commit -q -m 'test: seed hook worktree'
hook_sha="$(git -C "$hook_repo" rev-parse HEAD)"
touch "$hook_repo/main-index-only"
git -C "$hook_repo" add main-index-only
hook_zero=0000000000000000000000000000000000000000
hook_remote="$RUN_ROOT/pre-push-hook-remote.git"
GIT_CONFIG_GLOBAL="$gate_global" git init -q --bare "$hook_remote"
hook_missing=1111111111111111111111111111111111111111
hook_base_rc=0
hook_base_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_missing" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      ./.githooks/pre-push origin "$hook_remote"
} 2>&1)" || hook_base_rc=$?
equal "pre-push refuses an unavailable base of the pushed ref" \
  "refused named" \
  "$([ "$hook_base_rc" -ne 0 ] && printf refused || printf passed) $([[ "$hook_base_out" = *"remote base $hook_missing"* ]] && printf named || printf unnamed)"
hook_range_rc=0
hook_range_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      ./.githooks/pre-push origin "$RUN_ROOT/missing-pre-push-remote"
} 2>&1)" || hook_range_rc=$?
equal "pre-push refuses an indeterminate new-ref range" \
  "refused named" \
  "$([ "$hook_range_rc" -ne 0 ] && printf refused || printf passed) $([[ "$hook_range_out" = *'pushed commit range is indeterminate'* ]] && printf named || printf unnamed)"
hook_unbounded_remote="$RUN_ROOT/pre-push-unbounded-remote.git"
GIT_CONFIG_GLOBAL="$gate_global" git init -q --bare "$hook_unbounded_remote"
mkdir -p "$hook_unbounded_remote/refs/pull/100"
printf '%s\n' "$hook_missing" > "$hook_unbounded_remote/refs/pull/100/head"
hook_unbounded_rc=0
hook_unbounded_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      ./.githooks/pre-push origin "$hook_unbounded_remote"
} 2>&1)" || hook_unbounded_rc=$?
equal "pre-push refuses a nonempty advertisement with no usable boundary" \
  "refused named" \
  "$([ "$hook_unbounded_rc" -ne 0 ] && printf refused || printf passed) $([[ "$hook_unbounded_out" = *"none of the destination's advertised commits is available locally"* ]] && printf named || printf unnamed)"
mkdir -p "$hook_remote/refs/heads" "$hook_remote/refs/pull/100"
printf '%s\n' "$hook_sha" > "$hook_remote/refs/heads/main"
printf '%s\n' "$hook_missing" > "$hook_remote/refs/pull/100/head"
if hook_out="$({
  cd "$hook_repo"
  printf 'refs/heads/main %s refs/heads/main %s\n' "$hook_sha" "$hook_zero" |
    env GIT_CONFIG_GLOBAL="$gate_global" GIT_DIR="$hook_repo/.git" \
      HOOK_DELEGATION_DEPTH=1 \
      PROBE_DIR="$hook_probe" \
      ./.githooks/pre-push origin "$hook_remote"
} 2>&1)"; then
  pass "pre-push ignores an unrelated advertised commit absent locally"
else
  fail "pre-push ignores an unrelated advertised commit absent locally" "$hook_out"
fi
hook_gitdir="$(<"$hook_probe/gitdir")"
case "$hook_gitdir" in
  "$hook_repo/.git/worktrees/"*)
    pass "pre-push lint reads a detached-worktree git directory" ;;
  *) fail "pre-push lint reads a detached-worktree git directory" "$hook_gitdir" ;;
esac
if [ "$hook_gitdir" != "$hook_repo/.git" ]; then
  pass "pre-push lint does not read the main repository git directory"
else
  fail "pre-push lint does not read the main repository git directory" "$hook_gitdir"
fi
equal "pre-push lint does not read the main repository index" \
  "" "$(<"$hook_probe/index")"

# The message gate is the PUSHED one, for the same reason lint is: a working
# tree carries edits nobody is sending. The two copies are made to disagree in
# both directions, so a hook reading the wrong tree cannot pass either half.
msg_repo="$gate_root/pushed-message-gate"
msg_remote="$(gate_bare pushed-message-gate)"
rm -rf "$msg_repo"
GIT_CONFIG_GLOBAL="$gate_global" git init -q "$msg_repo"
git -C "$msg_repo" config user.name 'Gangline gate test'
git -C "$msg_repo" config user.email 'gangline@fixture.invalid'
mkdir -p "$msg_repo/test" "$msg_repo/.githooks"
cat > "$msg_repo/test/lint.sh" <<'SH'
#!/bin/sh
exit 0
SH
cp "$msg_repo/test/lint.sh" "$msg_repo/test/integration.sh"
msg_gate() { # $1 destination path, $2 verdict, $3 marker
  cat > "$1" <<SH
#!/bin/sh
echo '$3' >&2
exit $2
SH
  chmod +x "$1"
}
msg_gate "$msg_repo/.githooks/commit-msg" 1 'committed-gate: refusing'
chmod +x "$msg_repo/test/lint.sh" "$msg_repo/test/integration.sh"
printf '%s\n' base > "$msg_repo/content"
git -C "$msg_repo" add .
git -C "$msg_repo" commit -qm 'test: pushed message gate base'
git -C "$msg_repo" remote add dest "$msg_remote"
GIT_CONFIG_GLOBAL="$gate_global" git -C "$msg_repo" push \
  --no-verify -qu dest HEAD:refs/heads/main
git -C "$msg_repo" config core.hooksPath "$ROOT/.githooks"
printf '%s\n' refusing > "$msg_repo/content"
git -C "$msg_repo" add content
git -C "$msg_repo" commit -qm 'test: judged by the committed gate'
# Uncommitted, and permissive: nothing here may reach the verdict.
msg_gate "$msg_repo/.githooks/commit-msg" 0 'worktree-gate: accepting'
gate_push "$msg_repo" dest HEAD:refs/heads/main
equal "the pre-push message gate obeys the pushed hook, not the working tree" \
  "blocked committed absent" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([[ "$GATE_PUSH_OUTPUT" = *'committed-gate: refusing'* ]] && printf committed || printf unnamed) $([[ "$GATE_PUSH_OUTPUT" != *'worktree-gate: accepting'* ]] && printf absent || printf consulted)"

msg_gate "$msg_repo/.githooks/commit-msg" 0 'committed-gate: accepting'
git -C "$msg_repo" add .githooks/commit-msg
git -C "$msg_repo" commit -qm 'test: accept from the committed gate'
msg_gate "$msg_repo/.githooks/commit-msg" 1 'worktree-gate: refusing'
gate_push "$msg_repo" dest HEAD:refs/heads/main
equal "and a refusing working-tree copy cannot block a conforming push" \
  "pushed absent" \
  "$([ "$GATE_PUSH_RC" -eq 0 ] && printf pushed || printf blocked) $([[ "$GATE_PUSH_OUTPUT" != *'worktree-gate: refusing'* ]] && printf absent || printf consulted)"

git -C "$msg_repo" rm -q --cached .githooks/commit-msg
git -C "$msg_repo" commit -qm 'test: push a tip carrying no message gate'
gate_push "$msg_repo" dest HEAD:refs/heads/main
equal "pre-push refuses a pushed tip that carries no message gate" \
  "blocked named" \
  "$([ "$GATE_PUSH_RC" -ne 0 ] && printf blocked || printf leaked) $([[ "$GATE_PUSH_OUTPUT" = *'carries no executable'* ]] && printf named || printf unnamed)"

# A traversal that FAILS is not an empty range. The fixture removes one
# reachable commit object, so rev-list must abort where the tip, its tree, and
# the excluded base all still resolve — and the control run proves the same
# invocation passes while that object is present.
walk_repo="$gate_root/rev-list-failure"
rm -rf "$walk_repo"
GIT_CONFIG_GLOBAL="$gate_global" git init -q "$walk_repo"
git -C "$walk_repo" config user.name 'Gangline gate test'
git -C "$walk_repo" config user.email 'gangline@fixture.invalid'
mkdir -p "$walk_repo/test" "$walk_repo/.githooks"
cp "$ROOT/.githooks/commit-msg" "$walk_repo/.githooks/commit-msg"
cat > "$walk_repo/test/lint.sh" <<'SH'
#!/bin/sh
exit 0
SH
cp "$walk_repo/test/lint.sh" "$walk_repo/test/integration.sh"
chmod +x "$walk_repo/test/"*.sh "$walk_repo/.githooks/commit-msg"
printf '%s\n' a > "$walk_repo/content"
git -C "$walk_repo" add .
git -C "$walk_repo" commit -qm 'test: traversal base'
walk_base="$(git -C "$walk_repo" rev-parse HEAD)"
printf '%s\n' b > "$walk_repo/content"
git -C "$walk_repo" commit -qam 'test: traversal middle'
walk_middle="$(git -C "$walk_repo" rev-parse HEAD)"
printf '%s\n' c > "$walk_repo/content"
git -C "$walk_repo" commit -qam 'test: traversal tip'
walk_tip="$(git -C "$walk_repo" rev-parse HEAD)"
walk_drive() { # runs the shipped hook over walk_base..walk_tip
  WALK_RC=0
  WALK_OUT="$({
    cd "$walk_repo"
    printf 'refs/heads/main %s refs/heads/main %s\n' "$walk_tip" "$walk_base" |
      env GIT_CONFIG_GLOBAL="$gate_global" "$ROOT/.githooks/pre-push" \
        dest "$msg_remote"
  } 2>&1)" || WALK_RC=$?
}
walk_drive
equal "pre-push passes a traversable range" "passed" \
  "$([ "$WALK_RC" -eq 0 ] && printf passed || printf '%s' "refused: $WALK_OUT")"
rm -f "$walk_repo/.git/objects/${walk_middle:0:2}/${walk_middle:2}"
walk_drive
equal "pre-push refuses a failed traversal instead of reading it as empty" \
  "refused named" \
  "$([ "$WALK_RC" -ne 0 ] && printf refused || printf passed) $([[ "$WALK_OUT" = *'git rev-list exited'* ]] && printf named || printf unnamed)"

# `!` promises callers a break; the footer is what they can act on. The gate
# enforces the pairing its diagnostic and CONTRIBUTING.md advertise.
msg_file="$RUN_ROOT/commit-msg-subject"
commit_msg_verdict() { # $1 = whole message
  local rc=0 out
  printf '%s' "$1" > "$msg_file"
  out="$("$ROOT/.githooks/commit-msg" "$msg_file" 2>&1)" || rc=$?
  [ "$rc" -eq 0 ] && { printf 'accepted'; return 0; }
  case "$out" in
    *'not a footer'*) printf 'refused-placement' ;;
    *'no BREAKING CHANGE: footer'*) printf 'refused-footer' ;;
    *) printf 'refused-other' ;;
  esac
}
equal "the commit gate refuses a breaking subject with no footer" \
  "refused-footer" \
  "$(commit_msg_verdict 'feat!: breaks callers
')"
equal "and refuses one whose footer is only a git comment" \
  "refused-footer" \
  "$(commit_msg_verdict 'feat(send)!: breaks callers

# BREAKING CHANGE: callers must pass --to
')"
equal "and accepts a breaking subject that carries the footer" \
  "accepted" \
  "$(commit_msg_verdict 'feat(send)!: breaks callers

BREAKING CHANGE: callers must pass --to.
')"
equal "and accepts the hyphenated footer spelling" \
  "accepted" \
  "$(commit_msg_verdict 'refactor!: breaks callers

BREAKING-CHANGE: callers must pass --to.
')"
equal "and a nonbreaking subject still needs no footer" \
  "accepted" "$(commit_msg_verdict 'fix(spool): ordinary change
')"
# A FOOTER IS A PLACE. Glued to the end of a paragraph it is a sentence that
# begins with those words: `git interpret-trailers` does not see it, and the
# gate that accepted it called it a footer in its own diagnostic.
equal "and refuses a breaking line glued to the body" \
  "refused-placement" \
  "$(commit_msg_verdict 'feat(send)!: breaks callers

The old path is gone.
BREAKING CHANGE: callers must pass --to.
')"
equal "a comment between body and footer does not unmake the footer" \
  "accepted" \
  "$(commit_msg_verdict 'feat(send)!: breaks callers

The old path is gone.

# Please enter the commit message for your changes.
BREAKING CHANGE: callers must pass --to.
')"

# GATED TEARDOWN. `down` is the one irreversible verb, so it is exercised
# against a session of its own: a guard that is missing must not end this run.
teardown_session="teardown-probe-$$"
tmux new-session -d -s "$teardown_session" -n bystander \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
refuses "bare gang down names the session it would end" \
  "gang down" env GANG_SESSION="$teardown_session" "$GANG" down
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the session it refused to end is still running"
else
  fail "and the session it refused to end is still running" "session is gone"
fi
refuses "gang down refuses a session name that is not this team" \
  "refusing to end a session you did not name" \
  env GANG_SESSION="$teardown_session" "$GANG" down some-other-name
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the mismatched session is still running"
else
  fail "and the mismatched session is still running" "session is gone"
fi
teardown_pane="$(tmux list-panes -t "=$teardown_session" -F '#{pane_id}' | head -1)"
refuses "an agent cannot end the session it is running in" \
  "cannot end the session it is running in" \
  env GANG_SESSION="$teardown_session" TMUX_PANE="$teardown_pane" \
  "$GANG" down "$teardown_session"
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the session its agent could not end is still running"
else
  fail "and the session its agent could not end is still running" "session is gone"
fi
teardown_socket="$(tmux display-message -p '#{socket_path}')"
refuses "a scrubbed pane cannot end a session on its own server" \
  "cannot tell whether it is running inside the team" \
  env -u TMUX_PANE GANG_SESSION="$teardown_session" \
  TMUX="$teardown_socket,1,0" "$GANG" down "$teardown_session"
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  pass "and the scrubbed pane left the session running"
else
  fail "and the scrubbed pane left the session running" "session is gone"
fi
refuses "gang down refuses a second argument" \
  "down: unexpected argument 'extra'" \
  env GANG_SESSION="$teardown_session" "$GANG" down "$teardown_session" extra
env GANG_SESSION="$teardown_session" "$GANG" hitch spoolable \
  -c spoolable -d /tmp >/dev/null
teardown_spoolable_id="$(window_id_in "$teardown_session" spoolable)"
tmux send-keys -l -t "$teardown_spoolable_id" 'HUMAN_DRAFT'
printf 'MARK_TEARDOWN_ARCHIVE' |
  env GANG_SESSION="$teardown_session" "$GANG" send \
    --to spoolable --from tester --stdin >/dev/null
teardown_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$teardown_spoolable_id" @gl_spool)"
teardown_entry=""
for spool_entry in "$teardown_spool"/[0-9]*; do
  [ -f "$spool_entry" ] || continue
  teardown_entry="$spool_entry"
  break
done
[ -n "$teardown_entry" ] && cp "$teardown_entry" "$RUN_ROOT/pre-down-body"
teardown_down_out="$(env -u TMUX -u TMUX_PANE \
  GANG_SESSION="$teardown_session" "$GANG" down "$teardown_session")"
contains "the named teardown reports where it archived pending mail" \
  "$teardown_down_out" "$GANG_ARCHIVE_DIR"
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  fail "a named teardown from outside the session still ends it" \
    "session still exists"
else
  pass "a named teardown from outside the session still ends it"
fi
teardown_archived_entry=""
for archived_entry in "$GANG_ARCHIVE_DIR"/*/spoolable/${teardown_entry##*/}; do
  [ -f "$archived_entry" ] || continue
  teardown_archived_entry="$archived_entry"
  break
done
if [ -n "$teardown_archived_entry" ] \
   && cmp "$RUN_ROOT/pre-down-body" "$teardown_archived_entry"; then
  pass "a whole-team teardown preserves its pending message in the archive"
else
  fail "a whole-team teardown preserves its pending message in the archive" \
    "${teardown_archived_entry:-archived entry is absent}"
fi
tmux new-session -d -s "$teardown_session" -n bystander \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
observer_session="${teardown_session}-obs"
tmux new-session -d -s "$observer_session" -n observer \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
observer_pane="$(tmux list-panes -t "=$observer_session" -F '#{pane_id}' | head -1)"
env GANG_SESSION="$teardown_session" TMUX="$teardown_socket,1,0" \
  TMUX_PANE="$observer_pane" "$GANG" down "$teardown_session" >/dev/null
if tmux has-session -t "=$teardown_session" 2>/dev/null; then
  fail "a pane in another session can end the named team" "session still exists"
else
  pass "a pane in another session can end the named team"
fi
tmux kill-session -t "=$observer_session"

"$GANG" down "$GANG_SESSION" >/dev/null
if tmux has-session -t "=$GANG_SESSION" 2>/dev/null; then
  fail "down removes the exact test session" "session still exists"
else
  pass "down removes the exact test session"
fi
# shellcheck disable=SC2154  # set in test/integration-spool.sh
[ ! -d "$lingering_spool" ] \
  && pass "and takes the spool of every window in it" \
  || fail "and takes the spool of every window in it" "$lingering_spool survived"
