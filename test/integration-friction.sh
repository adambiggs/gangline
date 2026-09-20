# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Native recovery seams: automatic recap and scope-confirmed copy-mode. This
# part owns one separately named disposable team; every wait below is signalled
# by a fixture event, never a clock.

friction_original_session="$GANG_SESSION"
friction_original_collars="${GANG_COLLARS:-}"
export GANG_SESSION="gangfriction-$$"
friction_collars="$RUN_ROOT/friction-collars"
friction_recap_arm="$RUN_ROOT/friction-recap-arm"
friction_recap_channel="gang-friction-recap-$$"
mkdir -p "$friction_collars"
cat > "$RUN_ROOT/friction-bashrc" <<SH
PS1='❯ '
command_not_found_handle() {
  [ -e '$friction_recap_arm' ] || return 127
  tmux wait-for -S '$friction_recap_channel'
  return 127
}
SH
cat > "$friction_collars/friction-recap.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="ENV='$RUN_ROOT/friction-bashrc' bash --posix"
GANG_STOP_HOOK=1
# The core transaction only needs the collar's positive boundary verdict. The
# shipped Codex reader is exercised below against the exact painted frame; this
# minimal collar keeps that parser assertion independent of core delivery.
collar_recap_boundary() {
  return 0
}
SH
export GANG_COLLARS="$friction_collars"
tmux new-session -d -s "$GANG_SESSION" -n caller "PS1='❯ ' exec bash --norc"
friction_socket="$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')"
"$GANG" adopt caller -c bash >/dev/null
friction_caller_id="$(window_id caller)"
friction_caller_pane="$(tmux list-panes -t "$friction_caller_id" -F '#{pane_id}')"
"$HITCH" recap -c friction-recap -d "$RUN_ROOT" >/dev/null
friction_recap_id="$(window_id recap)"
friction_recap_pane="$(tmux list-panes -t "$friction_recap_id" -F '#{pane_id}')"
# The collar's recap surface arrives after a native turn boundary. Close the
# launch turn first; otherwise the core correctly ranks the open turn over the
# recap frame and reports busy rather than inventing an idle boundary.
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_recap_pane" "$GANG" hook >/dev/null
: > "$friction_recap_arm"
# The first roster observation discovers the current recap frame and arms the
# ordinary delivery worker. Its injection barrier proves that discovery
# happened. A second observation is the roster promise while that durable
# continuation remains pending; it cannot be masked by the now-free composer.
friction_recap_trigger="$("$GANG" roster)"
contains "the initial recap roster observes its still-visible native frame" \
  "$friction_recap_trigger" "recap"
tmux wait-for "$friction_recap_channel"
contains "the recap boundary submits one owned continuation" \
  "$(pane_all recap)" "Your context was just compacted."
friction_recap_pending="$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_pending)"
equal "the recap continuation keeps its exact durable marker before delivery completes" 16 \
  "${#friction_recap_pending}"
friction_recap_roster="$("$GANG" roster)"
contains "an empty native recap is visible as work pending continuation" \
  "$friction_recap_roster" "~wait~ (post-compaction continuation pending)"
equal "the recap continuation is recorded once while its old screen remains" 1 \
  "$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_handled)"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_recap_pane" "$GANG" hook >/dev/null
equal "the submitted continuation clears its parked-work marker" "" \
  "$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_pending)"
# A native build may send only PostCompact. That closing edge must make a
# future automatic recap observable even though the old recap paint's marker
# deliberately survives the continuation submission above.
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_recap_pane" "$GANG" hook >/dev/null
equal "an unowned PostCompact opens the next recap episode" "" \
  "$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_handled)"

# Two independent state readers can arrive on the same painted native recap.
# The collar holds reader one after the claim; reader two either loses that
# claim and reports unknown, or (in the mutation) reaches its own barrier.
# The event race lets the test deterministically release both old unclaimed
# readers while the fixed code releases just its one claim holder.
friction_race_first="gang-friction-race-first-$$"
friction_race_entered="gang-friction-race-entered-$$"
friction_race_release_one="gang-friction-race-release-one-$$"
friction_race_release_two="gang-friction-race-release-two-$$"
cat > "$friction_collars/friction-race.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_recap_boundary() {
  case "\${GANG_TEST_RECAP_OBSERVER:-}" in
    one)
      tmux -S '$friction_socket' wait-for -S '$friction_race_first'
      tmux -S '$friction_socket' wait-for '$friction_race_release_one' ;;
    two)
      tmux -S '$friction_socket' wait-for -S '$friction_race_entered'
      tmux -S '$friction_socket' wait-for '$friction_race_release_two' ;;
  esac
  return 0
}
SH
"$HITCH" recap-race -c friction-race -d "$RUN_ROOT" >/dev/null
friction_race_id="$(window_id recap-race)"
friction_race_pane="$(tmux list-panes -t "$friction_race_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_race_pane" "$GANG" hook >/dev/null
tmux copy-mode -t "$friction_race_id"
friction_race_events="$RUN_ROOT/friction-race-events"
friction_race_one="$RUN_ROOT/friction-race-one"
friction_race_two="$RUN_ROOT/friction-race-two"
mkfifo "$friction_race_events"
exec 9<>"$friction_race_events"
GANG_TEST_RECAP_OBSERVER=one "$GANG" roster > "$friction_race_one" 2>&1 &
friction_race_one_pid=$!
tmux wait-for "$friction_race_first"
(
  race_two_rc=0
  GANG_TEST_RECAP_OBSERVER=two "$GANG" roster > "$friction_race_two" 2>&1 || race_two_rc=$?
  printf 'returned:%s\n' "$race_two_rc" > "$friction_race_events"
) &
friction_race_two_pid=$!
( tmux wait-for "$friction_race_entered"; printf 'entered\n' > "$friction_race_events" ) &
friction_race_entered_pid=$!
IFS= read -r friction_race_event <&9
case "$friction_race_event" in
  entered)
    tmux wait-for -S "$friction_race_release_one"
    tmux wait-for -S "$friction_race_release_two" ;;
  returned:*)
    tmux wait-for -S "$friction_race_release_one"
    # The fixed observer loses the recap claim before it enters its collar.
    # Release the fixture's separate waiter rather than killing its shell: a
    # killed parent can leave the tmux wait-for child as an unaccounted barrier.
    tmux wait-for -S "$friction_race_entered"
    wait "$friction_race_entered_pid" ;;
  *) fail "the concurrent recap observer reports a determinate race event" \
       "event [$friction_race_event]" ;;
esac
wait "$friction_race_one_pid"
wait "$friction_race_two_pid"
equal "concurrent recap observers leave one durable continuation" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "recap-race" { print $4 }')"
friction_race_pending="$(tmux show-options -wqv -t "$friction_race_id" @gl_recap_pending)"
equal "concurrent recap observers leave one marker nonce" 16 \
  "${#friction_race_pending}"
tmux send-keys -t "$friction_race_id" -X cancel
"$GANG" drop recap-race >/dev/null
exec 9>&-
rm -f -- "$friction_race_events"

# A compact request records ownership before inject so its later native bracket
# does not arm a duplicate continuation. That marker belongs only to a command
# that could have reached Enter: peer copy-mode and a deferred self request
# forced into a draft both refuse before typing, so each must retire it.
cat > "$friction_collars/friction-rollback.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD='printf ignored'
collar_recap_boundary() { return 0; }
SH
"$HITCH" recap-rollback -c friction-rollback -d "$RUN_ROOT" >/dev/null
friction_rollback_id="$(window_id recap-rollback)"
friction_rollback_pane="$(tmux list-panes -t "$friction_rollback_id" -F '#{pane_id}')"
friction_rollback_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$friction_rollback_id" @gl_spool)"
friction_rollback_delivered="gang-friction-rollback-delivered-$$"
friction_rollback_real_rm="$(command -v rm)"
tmux copy-mode -t "$friction_rollback_id"
friction_rollback_rc=0
if TMUX_PANE="$friction_caller_pane" "$GANG" compact recap-rollback; then
  friction_rollback_rc=0
else
  friction_rollback_rc=$?
fi
equal "a peer copy-mode refusal is proved pre-Enter" 3 "$friction_rollback_rc"
equal "a refused peer compaction retires its ownership marker" "" \
  "$(tmux show-options -wqv -t "$friction_rollback_id" @gl_compaction_issued)"
# The copy-mode refusal proved the command did not reach Enter. Leave that
# operator-owned mode before presenting the separate native recap episode, so
# its continuation can be observed and delivered rather than rightly blocked.
tmux send-keys -t "$friction_rollback_id" -X cancel
printf '%s' '{"hook_event_name":"PreCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_rollback_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_rollback_pane" "$GANG" hook >/dev/null
# The continuation delivery is asynchronous. Its durable acceptance below is
# not its retirement. Signal the
# exact successful retirement so teardown begins only after the work this test
# does not exercise is finished; no clock or repeated filesystem read stands
# in for that event.
cat > "$RUN_ROOT/bin/rm" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$friction_rollback_real_rm" "\$0" rm || exit \$?
for argument do
  case "\$argument" in
    "\${GANG_TEST_FRICTION_ROLLBACK_SPOOL:-}"/sending-*)
      "$friction_rollback_real_rm" "\$@" || exit \$?
      tmux -S '$friction_socket' wait-for -S "\$GANG_TEST_FRICTION_ROLLBACK_DELIVERED" || exit \$?
      exit 0 ;;
  esac
done
exec "$friction_rollback_real_rm" "\$@"
SH
chmod +x "$RUN_ROOT/bin/rm"
# PostCompact schedules its ordinary drain; the following state read owns the
# fresh automatic recap observation and arms the one durable continuation.
friction_rollback_recap="$(GANG_TEST_FRICTION_ROLLBACK_SPOOL="$friction_rollback_spool" \
  GANG_TEST_FRICTION_ROLLBACK_DELIVERED="$friction_rollback_delivered" \
  "$GANG" roster)"
contains "an unowned native recap after a refused peer request is observed" \
  "$friction_rollback_recap" "~wait~ (post-compaction continuation pending)"
friction_rollback_pending="$(tmux show-options -wqv -t "$friction_rollback_id" @gl_recap_pending)"
equal "an unowned native recap after a refused peer request gets one continuation" 16 \
  "${#friction_rollback_pending}"
# `@gl_recap_handled` is written only after the one continuation is accepted
# into durable delivery. The worker can claim that entry before a second roster
# sees it, so its transient spool count is not evidence of how many episodes
# the observer armed.
equal "the unowned native recap keeps one continuation after the refused pane mode ends" 1 \
  "$(tmux show-options -wqv -t "$friction_rollback_id" @gl_recap_handled)"
tmux wait-for "$friction_rollback_delivered"
friction_rollback_claim=""
for friction_rollback_entry in "$friction_rollback_spool"/sending-*; do
  [ -f "$friction_rollback_entry" ] || continue
  friction_rollback_claim="$friction_rollback_entry"
done
equal "the accepted rollback continuation retires before fixture teardown" "" \
  "$friction_rollback_claim"
rm -f -- "$RUN_ROOT/bin/rm"
"$GANG" drop recap-rollback >/dev/null

friction_self_ready="gang-friction-self-ready-$$"
friction_self_release="gang-friction-self-release-$$"
cat > "$friction_collars/friction-self-rollback.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_SELF_COMPACT=deferred
GANG_SELF_COMPACT_WITNESS=native-idle
GANG_COMPACT_CMD='printf ignored'
collar_native_idle() { return 0; }
_friction_base_input="\$(declare -f collar_input)"
eval "\${_friction_base_input/collar_input/friction_base_input}"
collar_input() {
  if [ "\${GANG_TEST_SELF_ROLLBACK_SYNC:-}" = 1 ] \
     && [ -z "\$(tmux show-options -wqv -t \"\$1\" @gl_compaction_issued 2>/dev/null)" ]; then
    tmux -S '$friction_socket' wait-for -S '$friction_self_ready'
    tmux -S '$friction_socket' wait-for '$friction_self_release'
  fi
  friction_base_input "\$1"
}
SH
"$HITCH" self-rollback -c friction-self-rollback -d "$RUN_ROOT" >/dev/null
friction_self_id="$(window_id self-rollback)"
friction_self_pane="$(tmux list-panes -t "$friction_self_id" -F '#{pane_id}')"
TMUX_PANE="$friction_self_pane" "$GANG" compact >/dev/null
friction_self_request="$(tmux show-options -wqv -t "$friction_self_id" @gl_self_compact_requested)"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual GANG_TEST_SELF_ROLLBACK_SYNC=1 \
    TMUX_PANE="$friction_self_pane" "$GANG" hook >/dev/null
tmux wait-for "$friction_self_ready"
tmux copy-mode -t "$friction_self_id"
tmux wait-for -S "$friction_self_release"
tmux wait-for "gang-self-compact-$friction_self_request"
equal "a deferred pre-Enter refusal retires its ownership marker" "" \
  "$(tmux show-options -wqv -t "$friction_self_id" @gl_compaction_issued)"
equal "the deferred pre-Enter refusal preserves its retry request" "$friction_self_request" \
  "$(tmux show-options -wqv -t "$friction_self_id" @gl_self_compact_requested)"
tmux send-keys -t "$friction_self_id" -X cancel
"$GANG" drop self-rollback >/dev/null

# The shipped parser must recognise the native current-screen shape itself,
# not merely the test collar used to exercise the dispatcher transaction. The
# fixture signals only after the full screen is painted; no timed look stands
# in for that native fact.
friction_reader_channel="gang-friction-reader-$$"
friction_reader_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n recap-reader -c "$RUN_ROOT" \
  "printf '%s\\n' '─ Conversation recap ─' 'unfinished lane is preserved' '› '
tmux wait-for -S '$friction_reader_channel'
exec cat")"
friction_reader_pane="$(tmux list-panes -t "$friction_reader_id" -F '#{pane_id}')"
tmux wait-for "$friction_reader_channel"
friction_reader_rc=0
env -u GANG_TMUX_GUARD_AGENT TMUX="$friction_socket,0,0" \
  bash -c '. "$1"; collar_recap_boundary "$2"' fixture \
  "$ROOT/collars/codex.sh" "$friction_reader_pane" || friction_reader_rc=$?
equal "the shipped recap reader recognises an empty native recap" 0 \
  "$friction_reader_rc"
friction_draft_channel="gang-friction-draft-$$"
friction_draft_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n recap-draft -c "$RUN_ROOT" \
  "printf '%s\\n' '─ Conversation recap ─' 'unfinished lane is preserved' '› draft'
tmux wait-for -S '$friction_draft_channel'
exec cat")"
friction_draft_pane="$(tmux list-panes -t "$friction_draft_id" -F '#{pane_id}')"
tmux wait-for "$friction_draft_channel"
friction_draft_rc=0
env -u GANG_TMUX_GUARD_AGENT TMUX="$friction_socket,0,0" \
  bash -c '. "$1"; collar_recap_boundary "$2"' fixture \
  "$ROOT/collars/codex.sh" "$friction_draft_pane" >/dev/null || friction_draft_rc=$?
equal "the shipped recap reader rejects a nonempty post-recap draft" 1 \
  "$friction_draft_rc"

"$GANG" down "$GANG_SESSION" >/dev/null
export GANG_SESSION="$friction_original_session"
if [ -n "$friction_original_collars" ]; then
  export GANG_COLLARS="$friction_original_collars"
else
  unset GANG_COLLARS
fi
