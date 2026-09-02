# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Lead-facing unusable-state notes: qualified native wakes, retained delivery,
# tick reconciliation, and a positive-only Codex root-process witness.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# before the tick fixture replaces the disposable team.

notify_original_collars="${GANG_COLLARS:-}"
mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/state-notify.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=1
collar_bricked() {
  case "\$(cat "$RUN_ROOT/state-notify-bricked" 2>/dev/null)" in
    yes) printf 'fixture fatal turn'; return 0 ;;
    *) return 1 ;;
  esac
}
collar_blocked() {
  case "\$(cat "$RUN_ROOT/state-notify-blocked" 2>/dev/null)" in
    yes) printf 'fixture turn ended without producing work'; return 0 ;;
    *) return 1 ;;
  esac
}
SH
cat > "$RUN_ROOT/collars/state-codex-adopt.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/codex.sh"
# The stand-in process below needs Codex's identity reader, but it was not
# launched with Codex's native Stop hook and cannot truthfully claim one.
GANG_STOP_HOOK=
SH

"$HITCH" state-raise -c state-notify -d /tmp >/dev/null
"$HITCH" state-lead -c bash -d /tmp >/dev/null
state_raise_id="$(window_id state-raise)"
state_raise_pane="$(tmux list-panes -t "$state_raise_id" -F '#{pane_id}')"
state_lead_id="$(window_id state-lead)"
state_lead_pane="$(tmux list-panes -t "$state_lead_id" -F '#{pane_id}')"
"$GANG" notify state-lead >/dev/null

# idle_prompt is deliberately only a wake. A collar capable of reading a
# terminal state does not need to declare it as a generic stall: quietness must
# never acquire a state-notification transition on its own.
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
# source-guard: whole-surface@cf1c726b28e0: the state-alert wording is generated only by the new transition delivery and is absent from the fixture's static shell prompt
excludes "ordinary idle_prompt does not claim the agent is blocked" \
  "$(pane_all state-lead)" "state-raise is blocked"

printf '%s' yes > "$RUN_ROOT/state-notify-blocked"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
# source-guard: whole-surface@cf1c726b28e0: the blocked transition body is unique to the state-notification producer and the target pane can only receive it through attributed delivery
contains "a qualified idle wake forwards a blocked transition to the lead" \
  "$(pane_all state-lead)" "state-raise is blocked"
# source-guard: whole-surface@fc823991b6c2: Gangline's envelope attribution is rendered only after the state-transition delivery reaches this target pane
contains "the automatic blocked transition is attributed to Gangline" \
  "$(pane_all state-lead)" "[gang:gangline#"
excludes "an automatic transition does not impersonate its raising peer" \
  "$(pane_all state-lead)" "[gang:state-raise#"
contains "an accepted blocked transition records its exact notified state" \
  "$(tmux show-options -wqv -t "$state_raise_id" @gl_state_note_noted)" "blocked"
state_blocked_count="$(pane_all state-lead | grep -oF 'state-raise is blocked' | wc -l | tr -d ' ' || true)"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
if [ "$state_blocked_count" -eq 0 ]; then
  fail "a blocked transition exists before duplicate suppression is measured" \
    "no first transition body was delivered"
else
  # source-guard: whole-surface@7a7fc386b318: the count is taken from the one lead pane whose only producer of this phrase is the source's attributed blocked transition
  equal "one native wake transition does not duplicate its blocked note" \
    "$state_blocked_count" \
    "$(pane_all state-lead | grep -oF 'state-raise is blocked' | wc -l | tr -d ' ')"
fi

# A native prompt begins another harness turn, so a later, distinct failed turn
# is another transition rather than a duplicate of the old one.
rm -f -- "$RUN_ROOT/state-notify-blocked"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
printf '%s' yes > "$RUN_ROOT/state-notify-blocked"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
# source-guard: whole-surface@161b8e483516: the repeated occurrence count is valid only because the source fixture alone can deliver this agent-specific blocked phrase to the lead pane
equal "a recovered then newly blocked agent raises a new transition" \
  "$(( state_blocked_count + 1 ))" \
  "$(pane_all state-lead | grep -oF 'state-raise is blocked' | wc -l | tr -d ' ')"

printf '%s' yes > "$RUN_ROOT/state-notify-bricked"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
# source-guard: whole-surface@45e4631ce0f5: the fatal state phrase is emitted only by the qualified state-notification path for this source fixture
contains "the same qualified wake forwards a bricked transition" \
  "$(pane_all state-lead)" "state-raise is bricked"
rm -f -- "$RUN_ROOT/state-notify-bricked" "$RUN_ROOT/state-notify-blocked"

# A busy lead is not a dropped lead. The ordinary verified/parked sender must
# commit the alert to its spool, then a later native boundary drains it.
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
tmux send-keys -l -t "$state_lead_pane" HUMAN_LEAD_DRAFT
printf '%s' yes > "$RUN_ROOT/state-notify-blocked"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_raise_pane" "$GANG" hook >/dev/null
state_lead_busy_status="$("$GANG" status state-lead)"
contains "a busy lead receives the state note through its ordinary spool" \
  "$state_lead_busy_status" "spooled:"
# The mandatory gate must also turn red on the unfixed tree rather than wait
# forever for a drain that no state note ever created. The positive spool
# witness licenses the event barrier below; otherwise the failed assertion
# above is the whole outcome and the fixture proceeds to its later checks.
if [[ "$state_lead_busy_status" == *"spooled:"* ]]; then
  tmux send-keys -t "$state_lead_pane" C-u
  tmux wait-for "gang-spool-drain-$state_lead_id" &
  state_lead_drain_waiter=$!
  printf '%s' '{"hook_event_name":"Stop"}' \
    | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_lead_pane" "$GANG" hook >/dev/null
  wait "$state_lead_drain_waiter"
  # source-guard: whole-surface@07b669188430: the nonce-free transition body still originates solely in the source fixture and appears here only after the asserted spool drain
  contains "the parked blocked transition drains at the lead's verified boundary" \
    "$(pane_all state-lead)" "state-raise is blocked"
else
  tmux send-keys -t "$state_lead_pane" C-u
fi
rm -f -- "$RUN_ROOT/state-notify-blocked"

# A notify declaration whose target has no window is not an accepted delivery.
# The source preserves the transition, and a cooperative tick retries the exact
# pending note once that target returns without waiting for a second idle_prompt.
"$HITCH" state-return -c state-notify -d /tmp >/dev/null
state_return_id="$(window_id state-return)"
state_return_pane="$(tmux list-panes -t "$state_return_id" -F '#{pane_id}')"
"$GANG" notify state-returned >/dev/null
printf '%s' yes > "$RUN_ROOT/state-notify-blocked"
printf '%s' '{"hook_event_name":"Notification","notification_type":"idle_prompt"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$state_return_pane" "$GANG" hook >/dev/null
equal "a missing lead retains the exact state transition for reconciliation" \
  "blocked idle_prompt" \
  "$(tmux show-options -wqv -t "$state_return_id" @gl_state_note_pending)"
contains "the source makes a missing lead visible instead of accepting the note" \
  "$("$GANG" status state-return)" "state note NOT accepted"
"$HITCH" state-returned -c bash -d /tmp >/dev/null
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
# source-guard: whole-surface@b0439db65780: this target did not exist at the native wake, so its only route to the historical alert is the tick's retained pending transition
contains "a tick delivers the original transition after the lead returns" \
  "$(pane_all state-returned)" "state-return is blocked"
equal "an accepted reconciliation retires the pending transition" "" \
  "$(tmux show-options -wqv -t "$state_return_id" @gl_state_note_pending)"
state_return_count="$(pane_all state-returned | grep -oF 'state-return is blocked' | wc -l | tr -d ' ' || true)"
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
if [ "$state_return_count" -eq 0 ]; then
  fail "a reconciled transition exists before repeated ticks are measured" \
    "no reconciled transition body was delivered"
else
  # source-guard: whole-surface@e2da1b0fcafe: only the retained state-note transition can add this exact phrase to the returned target's pane across repeated ticks
  equal "repeated ticks do not duplicate a returned-lead transition" \
    "$state_return_count" \
    "$(pane_all state-returned | grep -oF 'state-return is blocked' | wc -l | tr -d ' ')"
fi
rm -f -- "$RUN_ROOT/state-notify-blocked"

# pane-died is intentionally not a barrier: tmux can lose it. The pane's FIFO
# gives the test a settled EOF, then the existing tick supplies the guaranteed
# reconciliation path. A fast hook may win first, but neither path may send two
# notes.
"$HITCH" state-dead -c bash -d /tmp >/dev/null
state_dead_id="$(window_id state-dead)"
state_dead_pane="$(tmux list-panes -t "$state_dead_id" -F '#{pane_id}')"
state_dead_fifo="$RUN_ROOT/state-dead.fifo"
mkfifo "$state_dead_fifo"
tmux set-option -w -t "$state_dead_id" remain-on-exit on
tmux send-keys -l -t "$state_dead_pane" "exec 9<>$state_dead_fifo"
tmux send-keys -t "$state_dead_pane" Enter
exec 3<"$state_dead_fifo"
tmux send-keys -l -t "$state_dead_pane" 'exit 23'
tmux send-keys -t "$state_dead_pane" Enter
cat <&3 >/dev/null
exec 3<&-
tmux run-shell true >/dev/null
equal "the retained death is settled before reconciliation reads it" 1 \
  "$(tmux display-message -p -t "$state_dead_pane" '#{pane_dead}')"
"$GANG" notify state-lead >/dev/null
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
# source-guard: whole-surface@0eb1c792c6ce: only the dead-state transition producer names this unique agent as dead, whether the fast hook or the reconciler won
contains "pane death reaches the lead no later than the next tick" \
  "$(pane_all state-lead)" "state-dead is dead"
state_dead_count="$(pane_all state-lead | grep -oF 'state-dead is dead' | wc -l | tr -d ' ' || true)"
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
if [ "$state_dead_count" -eq 0 ]; then
  fail "a pane-death transition exists before hook and tick are compared" \
    "no pane-death transition body was delivered"
else
  # source-guard: whole-surface@1bf0a334df38: the count covers a phrase emitted solely by the one dead transition, so a second path can only be detected as a duplicate occurrence
  equal "the pane-died hook and tick share one dead transition" \
    "$state_dead_count" \
    "$(pane_all state-lead | grep -oF 'state-dead is dead' | wc -l | tr -d ' ')"
fi

# Codex's rollout cannot distinguish a healthy in-flight task_started from a
# process that vanished. This fixture writes both observed harmless shapes, but
# proves liveness only from a positively identified pane root. Linux is the
# only shipped environment with the needed kernel start stamp; elsewhere the
# absence must be explicit rather than synthesized from ps output.
ln -s /bin/sleep "$RUN_ROOT/codex"
state_codex_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n state-codex "exec '$RUN_ROOT/codex' 600")"
"$GANG" adopt state-codex -c state-codex-adopt >/dev/null
state_codex_rollout="$RUN_ROOT/state-codex-rollout.jsonl"
printf '%s\n%s\n' \
  '{"type":"event_msg","payload":{"type":"task_started","turn_id":"fixture-open"}}' \
  '{"type":"event_msg","payload":{"type":"turn_aborted","reason":"interrupted"}}' \
  > "$state_codex_rollout"
tmux set-option -w -t "$state_codex_id" @gl_session "$state_codex_rollout"
"$GANG" notify state-lead >/dev/null
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
if [ "$(uname -s)" = Linux ]; then
  if [[ "$(tmux show-options -wqv -t "$state_codex_id" @gl_harness_identity)" =~ ^[0-9]+[[:space:]][0-9]+$ ]]; then
    pass "a demonstrable Codex pane root records pid plus kernel start stamp"
  else
    fail "a demonstrable Codex pane root records pid plus kernel start stamp" \
      "got [$(tmux show-options -wqv -t "$state_codex_id" @gl_harness_identity)]"
  fi
  state_codex_identity="$(tmux show-options -wqv -t "$state_codex_id" @gl_harness_identity)"
  IFS=$'\t' read -r state_codex_pid state_codex_token <<<"$state_codex_identity"
  state_codex_kernel_token="$(python3 - "$state_codex_pid" <<'PY'
import sys

with open(f"/proc/{sys.argv[1]}/stat", encoding="utf-8") as stream:
    print(stream.read().strip().rpartition(")")[2].split()[19])
PY
)"
  equal "the shared Codex identity reader records kernel start field 22" \
    "$state_codex_kernel_token" "$state_codex_token"
  contains "explain makes the recorded liveness coverage visible" \
    "$("$GANG" explain state-codex)" "harness identity: recorded"
  excludes "an unclosed or interrupted healthy Codex turn is not harness-lost" \
    "$("$GANG" status state-codex)" "!harness-lost!"
  # A new live root in the same registered window is not the recorded Codex
  # process. Its PID or command identity changed, while the pane remains able
  # to draw and receive the eventual alert.
  tmux respawn-pane -k -t "$state_codex_id" "PS1='❯ ' exec bash --norc"
  GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
  contains "a replaced recorded harness is named harness-lost, not dead" \
    "$("$GANG" status state-codex)" "!harness-lost!"
  contains "explain distinguishes a lost identity from unavailable coverage" \
    "$("$GANG" explain state-codex)" "harness identity: lost"
  # source-guard: whole-surface@bfeb5778c372: only the liveness reconciliation can send this agent-specific harness-lost body into the declared lead pane
  contains "a lost root process notifies the lead through reconciliation" \
    "$(pane_all state-lead)" "state-codex is harness-lost"
  state_lost_count="$(pane_all state-lead | grep -oF 'state-codex is harness-lost' | wc -l | tr -d ' ' || true)"
  GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
  if [ "$state_lost_count" -eq 0 ]; then
    fail "a lost-root transition exists before repeated ticks are measured" \
      "no harness-lost transition body was delivered"
  else
    # source-guard: whole-surface@aa79ce5c02fe: only the persisted lost transition can add this phrase to the lead pane during a repeated reconciliation pass
    equal "a persistent lost root does not notify once per tick" "$state_lost_count" \
      "$(pane_all state-lead | grep -oF 'state-codex is harness-lost' | wc -l | tr -d ' ')"
  fi
else
  equal "a platform without the kernel stamp leaves Codex coverage unrecorded" "" \
    "$(tmux show-options -wqv -t "$state_codex_id" @gl_harness_identity)"
  contains "explain makes unavailable liveness coverage visible" \
    "$("$GANG" explain state-codex)" "harness identity: not recorded"
fi

"$GANG" notify clear >/dev/null
"$GANG" drop state-codex >/dev/null
"$GANG" drop state-dead >/dev/null
"$GANG" drop state-returned >/dev/null
"$GANG" drop state-return >/dev/null
"$GANG" drop state-raise >/dev/null
"$GANG" drop state-lead >/dev/null

if [ -n "$notify_original_collars" ]; then
  export GANG_COLLARS="$notify_original_collars"
else
  unset GANG_COLLARS
fi
