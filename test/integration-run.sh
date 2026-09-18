#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
# Host-service run requests: durable result, bounded delivery, cancellation, and a gone requester.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file in
# order and it reads that shell's fixtures, helpers and counters.
run_bin="$RUN_ROOT/run-bin"
run_args="$RUN_ROOT/run-systemd-args"
run_stops="$RUN_ROOT/run-systemd-stops"
run_state="$RUN_ROOT/run-state"
mkdir -p "$run_bin" "$run_state"
: > "$run_args"
: > "$run_stops"
printf '%s\n' \
  '#!/bin/sh' \
  'printf '\''%s\n'\'' "$*" > "$GANG_TEST_RUN_ARGS"' \
  'if [ -n "${GANG_TEST_RUN_HOLD:-}" ]; then' \
  '  printf '\''%s\n'\'' "$$" > "$GANG_TEST_RUN_MANAGER_PID"' \
  '  printf x > "$GANG_TEST_RUN_READY"' \
  '  IFS= read -r _ < "$GANG_TEST_RUN_HOLD"' \
  'fi' \
  'test "${GANG_TEST_RUN_FAIL:-0}" != 1 || exit 1' \
  'exit 0' > "$run_bin/systemd-run"
printf '%s\n' \
  '#!/bin/sh' \
  'printf '\''%s\n'\'' "$*" >> "$GANG_TEST_RUN_STOPS"' \
  'case "$*" in' \
  '  *'\''show --property=Version --value'\''*) printf '\''test-manager\n'\''; exit 0 ;;' \
  '  *'\''show --property=LoadState --value'\''*)' \
  '    case "${GANG_TEST_RUN_UNIT_STATE:-live}" in' \
  '      live) printf '\''loaded\n'\''; exit 0 ;;' \
  '      absent) printf '\''not-found\n'\''; exit 0 ;;' \
  '      unreadable) printf '\''fixture manager unreadable\n'\'' >&2; exit 1 ;;' \
  '    esac ;;' \
  '  *'\''stop --'\''*) exit 0 ;;' \
  '  *) printf '\''inactive\n'\''; exit 3 ;;' \
  'esac' > "$run_bin/systemctl"
chmod +x "$run_bin/systemd-run" "$run_bin/systemctl"

"$HITCH" run-requester -c bash -d "$run_state"
run_requester_id="$(window_id run-requester)"
run_requester_pane="$(tmux list-panes -t "$run_requester_id" -F '#{pane_id}')"

run_start() { # command words -> start a run as the dedicated fixture requester
  TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
    TMPDIR="$run_state/requester-tmp" \
    GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
    PATH="$run_bin:$PATH" "$GANG" run -- "$@"
}

run_recover() { # requester successor reads a prior launch declaration
  TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
    GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
    PATH="$run_bin:$PATH" "$GANG" run --active
}

run_record_for() { # first argument is a unique command marker
  local marker="$1" candidate
  for candidate in "$run_state"/gangline/runs/*/*; do
    [ -f "$candidate/argv" ] || continue
    if tr '\000' '\n' < "$candidate/argv" | grep -F "$marker" >/dev/null; then
      printf '%s' "$candidate"
      return 0
    fi
  done
  return 1
}

run_runner_direct() { # gang-runner invocation in the same environment as the service fixture
  if [ "${RUN_RUNNER_EXEC:-0}" = 1 ]; then
    exec env -u TMUX -u TMUX_PANE \
      XDG_STATE_HOME="$run_state" GANG_SESSION="$GANG_SESSION" \
      GANG_CONFIG_DIR="$GANG_CONFIG_DIR" GANG_LOCK_DIR="$GANG_LOCK_DIR" \
      GANG_ARCHIVE_DIR="${GANG_ARCHIVE_DIR:-$RUN_ROOT/run-archive}" GANG_RUN_TEAM_ROOT="${1%/*}" \
      GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
      PATH="$run_bin:$PATH" "$ROOT/libexec/gang-runner" "${@:2}"
  fi
  env -u TMUX -u TMUX_PANE \
    XDG_STATE_HOME="$run_state" GANG_SESSION="$GANG_SESSION" \
    GANG_CONFIG_DIR="$GANG_CONFIG_DIR" GANG_LOCK_DIR="$GANG_LOCK_DIR" \
    GANG_ARCHIVE_DIR="${GANG_ARCHIVE_DIR:-$RUN_ROOT/run-archive}" GANG_RUN_TEAM_ROOT="${1%/*}" \
    GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
    PATH="$run_bin:$PATH" "$ROOT/libexec/gang-runner" "${@:2}"
}

run_finalize_direct() { # $1 record, $2 systemd result kind, $3 status
  local record="$1" kind="$2" status="$3"
  env -u TMUX -u TMUX_PANE \
    EXIT_CODE="$kind" EXIT_STATUS="$status" \
    XDG_STATE_HOME="$run_state" GANG_SESSION="$GANG_SESSION" \
    GANG_CONFIG_DIR="$GANG_CONFIG_DIR" GANG_LOCK_DIR="$GANG_LOCK_DIR" \
    GANG_ARCHIVE_DIR="${GANG_ARCHIVE_DIR:-$RUN_ROOT/run-archive}" GANG_RUN_TEAM_ROOT="${record%/*}" \
    GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
    PATH="$run_bin:$PATH" "$ROOT/libexec/gang-runner" --finalize "$record"
}

run_finish_direct() { # $1 result record: runner plus its deterministic ExecStopPost
  local record="$1" rc=0
  run_runner_direct "$record" "$record" || rc=$?
  run_finalize_direct "$record" exited "$rc"
}

run_small_out="$(run_start sh -c 'printf MARK_RUN_SMALL')"
contains "run accepts a host command from its requesting agent" "$run_small_out" "started run"
run_small="$(run_record_for MARK_RUN_SMALL)" || run_small=""
if [ -n "$run_small" ]; then
  pass "run writes a durable result declaration before its service starts"
else
  fail "run writes a durable result declaration before its service starts" \
    "no record under $run_state"
fi
contains "run arms a host-machine transient service" "$(<"$run_args")" "--machine=$(id -un)@.host"
contains "the service starts Gangline's result runner" "$(<"$run_args")" "$ROOT/libexec/gang-runner"
contains "the service receives its immutable result directory" "$(<"$run_args")" "$run_small"
contains "the service preserves the requesting PATH" "$(<"$run_args")" "--setenv=PATH=$run_bin:"
contains "the service preserves the requesting durable TMPDIR" "$(<"$run_args")" \
  "--setenv=TMPDIR=$run_state/requester-tmp"
contains "the service arms a post-stop finalizer" "$(<"$run_args")" \
  "--property=ExecStopPost="

# A human draft forces completion through the ordinary durable spool, making
# the completion envelope itself the evidence rather than a later shell error
# caused by Bash treating that envelope as a command.
tmux send-keys -l -t "$run_requester_id" 'HUMAN_DRAFT'
run_finish_direct "$run_small"
contains "the runner records the completed exit code" "$(<"$run_small/result")" $'0\t'
equal "the runner preserves complete combined output" "MARK_RUN_SMALL" "$(<"$run_small/output")"
run_small_mail="$(XDG_STATE_HOME="$run_state" "$GANG" mail run-requester)"
contains "completion arrives through an enveloped Gangline delivery" "$run_small_mail" "[gang:self-declared:gang-run"
contains "the completion carries the durable output path" "$run_small_mail" "$run_small/output"
contains "the completion carries the command output tail" "$run_small_mail" "MARK_RUN_SMALL"
run_audit="${run_small%/*}/audit.tsv"
contains "completion appends the requester to the durable run audit" "$(<"$run_audit")" $'\trun-requester\t'
contains "completion appends its output path to the durable run audit" "$(<"$run_audit")" \
  "$run_small/output"

# A COMPLETED RUN MEETS A LIVE DELIVERY LOCK after its result and audit are
# durable. The lock is owned by this fixture process, so the finalizer gets an
# immediate, deterministic contention answer without waiting on a clock. Its
# completion still has to enter the requester's ordinary spool exactly once;
# releasing the lock then gives the next cooperative tick a normal delivery
# opportunity.
tmux send-keys -t "$run_requester_id" C-u
run_locked_out="$(run_start sh -c 'printf MARK_RUN_DELIVERY_LOCK')"
contains "a delivery-lock run is accepted" "$run_locked_out" "started run"
run_locked="$(run_record_for MARK_RUN_DELIVERY_LOCK)" || run_locked=""
run_requester_lock="$GANG_LOCK_DIR/$(printf '%s' "$run_requester_id" | tr -c 'A-Za-z0-9' '_').lock"
run_locked_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$run_requester_id" @gl_spool)"
run_locked_spool_before=0
for run_locked_candidate in "$run_locked_spool"/[0-9]*; do
  [ -f "$run_locked_candidate" ] || continue
  run_locked_spool_before=$((run_locked_spool_before + 1))
done
run_locked_spool_expected=$((run_locked_spool_before + 1))
ln -s "$$" "$run_requester_lock"
run_locked_finish_rc=0
run_finish_direct "$run_locked" \
  >"$RUN_ROOT/run-locked-finish.out" 2>"$RUN_ROOT/run-locked-finish.err" \
  || run_locked_finish_rc=$?
equal "a competing delivery lock does not reject a durable completion" \
  0 "$run_locked_finish_rc"
contains "the contended completion enters the requester delivery path" \
  "$(<"$run_locked/delivery")" "accepted into requester delivery path"
run_finalize_direct "$run_locked" exited 0
run_locked_envelopes="$(grep -rl "run ${run_locked##*/} completed" "$run_locked_spool" 2>/dev/null || :)"
run_locked_envelope=""
run_locked_envelope_count=0
while IFS= read -r run_locked_candidate; do
  [ -n "$run_locked_candidate" ] || continue
  run_locked_envelope_count=$((run_locked_envelope_count + 1))
  [ -n "$run_locked_envelope" ] || run_locked_envelope="$run_locked_candidate"
done <<< "$run_locked_envelopes"
equal "a repeated finalizer leaves exactly one eligible completion" \
  1 "$run_locked_envelope_count"
run_locked_spool_after=0
for run_locked_candidate in "$run_locked_spool"/[0-9]*; do
  [ -f "$run_locked_candidate" ] || continue
  run_locked_spool_after=$((run_locked_spool_after + 1))
done
equal "the contended completion adds exactly one deliverable spool entry" \
  "$run_locked_spool_expected" "$run_locked_spool_after"
run_locked_message_id="${run_locked_envelope##*/}"
run_locked_queued="$(XDG_STATE_HOME="$run_state" "$GANG" log run-requester \
  --kind delivery.queued 2>&1)"
run_locked_queued_count="$(printf '%s\n' "$run_locked_queued" |
  grep -Fc "\"message_id\": \"$run_locked_message_id\"" || :)"
run_locked_verified_before="$(XDG_STATE_HOME="$run_state" "$GANG" log run-requester \
  --kind delivery.verified 2>&1)"
run_locked_verified_before_count="$(printf '%s\n' "$run_locked_verified_before" |
  grep -Fc "\"message_id\": \"$run_locked_message_id\"" || :)"
equal "the contended completion has one queued event" \
  1 "$run_locked_queued_count"
equal "the contended completion does not fabricate verification" \
  0 "$run_locked_verified_before_count"
run_locked_status="$(XDG_STATE_HOME="$run_state" "$GANG" status run-requester)"
run_locked_status_count="$(printf '%s\n' "$run_locked_status" |
  sed -n 's/.*spooled: \([0-9][0-9]*\).*/\1/p')"
equal "status exposes the exact unsettled completion count" \
  "$run_locked_spool_expected" "$run_locked_status_count"
contains "status names the unsettled completion recovery action" \
  "$run_locked_status" "retry now with gang tick"
run_locked_roster="$(XDG_STATE_HOME="$run_state" "$GANG" roster |
  grep '^run-requester ' || :)"
run_locked_roster_count="$(printf '%s\n' "$run_locked_roster" |
  sed -n 's/.* spooled=\([0-9][0-9]*\).*/\1/p')"
equal "roster exposes the same unsettled completion count" \
  "$run_locked_spool_expected" "$run_locked_roster_count"
rm -f -- "$run_requester_lock"
run_locked_tick_rc=0
XDG_STATE_HOME="$run_state" "$GANG" tick \
  >"$RUN_ROOT/run-locked-tick.out" 2>"$RUN_ROOT/run-locked-tick.err" \
  || run_locked_tick_rc=$?
equal "the released completion delivery settles at the next opportunity" \
  0 "$run_locked_tick_rc"
run_locked_verified="$(XDG_STATE_HOME="$run_state" "$GANG" log run-requester \
  --kind delivery.verified 2>&1)"
run_locked_verified_count="$(printf '%s\n' "$run_locked_verified" |
  grep -Fc "\"message_id\": \"$run_locked_message_id\"" || :)"
equal "the released completion is verified exactly once" \
  1 "$run_locked_verified_count"

# THE STABLE REQUESTER MAY RENAME BETWEEN ITS TOKEN SNAPSHOT AND BY-NAME
# RESOLUTION. A one-shot tmux shim returns the real pre-rename stable-agent
# rows, then changes both the registered identity and window title before the
# following resolve. The finalizer must re-read the stable token and accept one
# completion for that same window; an uncaught resolve exit would instead leave
# the host result with only the runner's generic transport-failure record.
run_identity_out="$(run_start sh -c 'printf MARK_RUN_IDENTITY_RACE')"
contains "an identity-race run is accepted" "$run_identity_out" "started run"
run_identity="$(run_record_for MARK_RUN_IDENTITY_RACE)" || run_identity=""
run_identity_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$run_requester_id" @gl_spool)"
run_identity_bin="$RUN_ROOT/run-identity-bin"
run_identity_once="$RUN_ROOT/run-identity-once"
run_identity_real_tmux="$(command -v tmux)"
mkdir -p "$run_identity_bin"
cat > "$run_identity_bin/tmux" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$run_identity_real_tmux" "\$0" tmux || exit \$?
stable_rows=0
for argument do
  [ "\$argument" != '#{window_id} #{@gl_spool} #{@gl_agent}' ] || stable_rows=1
done
if [ "\$stable_rows" -eq 1 ] && [ ! -e "$run_identity_once" ]; then
  rows="\$("$run_identity_real_tmux" "\$@")" || exit \$?
  : > "$run_identity_once" || exit \$?
  "$run_identity_real_tmux" set-option -w -t "$run_requester_id" \
    @gl_agent run-requester-raced || exit \$?
  "$run_identity_real_tmux" rename-window -t "$run_requester_id" \
    run-requester-raced || exit \$?
  printf '%s\n' "\$rows"
  exit 0
fi
exec "$run_identity_real_tmux" "\$@"
SH
chmod +x "$run_identity_bin/tmux"
tmux send-keys -l -t "$run_requester_id" 'HUMAN_DRAFT'
run_identity_finish_rc=0
PATH="$run_identity_bin:$PATH" run_finish_direct "$run_identity" \
  >"$RUN_ROOT/run-identity-finish.out" 2>"$RUN_ROOT/run-identity-finish.err" \
  || run_identity_finish_rc=$?
equal "a mid-resolution requester rename does not reject its completion" \
  0 "$run_identity_finish_rc"
contains "the renamed requester still accepts the completion" \
  "$(<"$run_identity/delivery")" "accepted into requester delivery path"
run_identity_envelopes="$(grep -rl "run ${run_identity##*/} completed" \
  "$run_identity_spool" 2>/dev/null || :)"
run_identity_envelope=""
run_identity_envelope_count=0
while IFS= read -r run_identity_candidate; do
  [ -n "$run_identity_candidate" ] || continue
  run_identity_envelope_count=$((run_identity_envelope_count + 1))
  [ -n "$run_identity_envelope" ] || run_identity_envelope="$run_identity_candidate"
done <<< "$run_identity_envelopes"
equal "the identity race leaves exactly one eligible completion" \
  1 "$run_identity_envelope_count"
tmux set-option -w -t "$run_requester_id" @gl_agent run-requester
tmux rename-window -t "$run_requester_id" run-requester
tmux send-keys -t "$run_requester_id" C-u
run_identity_tick_rc=0
XDG_STATE_HOME="$run_state" "$GANG" tick \
  >"$RUN_ROOT/run-identity-tick.out" 2>"$RUN_ROOT/run-identity-tick.err" \
  || run_identity_tick_rc=$?
equal "the renamed requester completion settles after identity restoration" \
  0 "$run_identity_tick_rc"
run_identity_message_id="${run_identity_envelope##*/}"
run_identity_verified="$(XDG_STATE_HOME="$run_state" "$GANG" log run-requester \
  --kind delivery.verified 2>&1)"
run_identity_verified_count="$(printf '%s\n' "$run_identity_verified" |
  grep -Fc "\"message_id\": \"$run_identity_message_id\"" || :)"
equal "the identity-race completion is verified exactly once" \
  1 "$run_identity_verified_count"
rm -f -- "$run_identity_bin/tmux"
tmux send-keys -l -t "$run_requester_id" 'HUMAN_DRAFT'

run_existing_tmpdir="$run_state/requester-existing-tmp"
mkdir -p "$run_existing_tmpdir"
chmod 755 "$run_existing_tmpdir"
run_existing_tmpdir_out="$(TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
  TMPDIR="$run_existing_tmpdir" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run -- sh -c 'printf MARK_RUN_EXISTING_TMPDIR')"
contains "run accepts an existing requester TMPDIR" "$run_existing_tmpdir_out" "started run"
run_existing_tmpdir_record="$(run_record_for MARK_RUN_EXISTING_TMPDIR)" || run_existing_tmpdir_record=""
equal "run preserves an existing requester TMPDIR mode" "755" \
  "$(stat -c %a "$run_existing_tmpdir")"
run_finish_direct "$run_existing_tmpdir_record"

run_large_out="$(run_start sh -c 'head -c 1049600 /dev/zero')"
contains "a second run is accepted while the team has capacity" "$run_large_out" "started run"
run_large="$(run_record_for 'head -c 1049600 /dev/zero')" || run_large=""
if [ -n "$run_large" ]; then
  pass "the large-output run has its own durable record"
else
  fail "the large-output run has its own durable record" \
    "no record under $run_state"
fi
run_finish_direct "$run_large"
run_large_bytes="$(wc -c < "$run_large/output" | tr -d ' ')"
if [ "$run_large_bytes" -gt 1048576 ]; then
  pass "full output above one megabyte remains in the result file"
else
  fail "full output above one megabyte remains in the result file" \
    "recorded $run_large_bytes bytes"
fi
run_requester_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$run_requester_id" @gl_spool)"
run_large_envelope=""
if run_large_envelopes="$(grep -rl "run ${run_large##*/} completed" "$run_requester_spool")"; then
  run_large_envelope="${run_large_envelopes%%$'\n'*}"
fi
if [ -n "$run_large_envelope" ]; then
  pass "the large-output completion is retained in the ordinary spool"
else
  fail "the large-output completion is retained in the ordinary spool" \
    "no matching completion under $run_requester_spool"
fi
if [ -n "$run_large_envelope" ]; then
  run_large_envelope_bytes="$(wc -c < "$run_large_envelope" | tr -d ' ')"
  if [ "$run_large_envelope_bytes" -lt 4096 ]; then
    pass "the delivered completion tail stays bounded"
  else
    fail "the delivered completion tail stays bounded" \
      "the envelope was $run_large_envelope_bytes bytes"
  fi
else
  fail "the delivered completion tail stays bounded" \
    "no completion envelope was available to measure"
fi

run_cancel_out="$(run_start sh -c 'printf MARK_RUN_CANCEL')"
contains "a cancellable run is accepted" "$run_cancel_out" "started run"
run_cancel_record="$(run_record_for MARK_RUN_CANCEL)" || run_cancel_record=""
run_cancel_id="${run_cancel_record##*/}"
run_cancel_reply="$(TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run --cancel "$run_cancel_id")"
contains "the requesting stable identity may cancel its own run" "$run_cancel_reply" "cancellation requested"
run_cancel_unit="$(<"$run_cancel_record/unit")"
contains "cancellation addresses the service exact to that run" "$(<"$run_stops")" \
  "stop -- $run_cancel_unit"

# A host command can outlive the sandbox that asked for it. The next shell in
# this pane has only the stable spool identity, not the predecessor's run-id
# output, so it must recover both the declaration and a usable cancellation.
run_successor_out="$(run_start sh -c 'printf MARK_RUN_SUCCESSOR')"
contains "an interrupt-shaped host run is accepted" "$run_successor_out" "started run"
run_successor_record="$(run_record_for MARK_RUN_SUCCESSOR)" || run_successor_record=""
run_successor_id="${run_successor_record##*/}"
run_successor_active="$(TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run --active)"
contains "a successor sandbox discovers its predecessor's host run" \
  "$run_successor_active" "active host run $run_successor_id"
contains "the successor receives an exact cancellation command" \
  "$run_successor_active" "gang run --cancel $run_successor_id"
run_successor_roster="$(XDG_STATE_HOME="$run_state" "$GANG" roster)"
contains "roster makes an interrupted host run visible" \
  "$run_successor_roster" "host-run="
contains "roster names the interrupted host run" \
  "$run_successor_roster" "$run_successor_id"
run_successor_status="$(XDG_STATE_HOME="$run_state" "$GANG" status run-requester)"
contains "status makes an interrupted host run visible" \
  "$run_successor_status" "active host run(s):"
contains "status names the interrupted host run" \
  "$run_successor_status" "$run_successor_id"

# There is a separate lifecycle edge before systemd-run accepts a service. Hold
# the fixture manager at that point, terminate the requester, then let a fresh
# shell prove that the durable declaration is visibly *launching*, not falsely
# active. An unreadable manager must preserve it; a positive not-found answer
# may settle it without attempting a cancellation against a unit that never
# existed.
run_launch_hold="$RUN_ROOT/run-launch-hold"
run_launch_ready="$RUN_ROOT/run-launch-ready"
run_launch_manager_pid="$RUN_ROOT/run-launch-manager-pid"
run_launch_output="$RUN_ROOT/run-launch-output"
mkfifo "$run_launch_hold" "$run_launch_ready"
TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
  TMPDIR="$run_state/requester-tmp" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  GANG_TEST_RUN_HOLD="$run_launch_hold" GANG_TEST_RUN_READY="$run_launch_ready" \
  GANG_TEST_RUN_MANAGER_PID="$run_launch_manager_pid" \
  PATH="$run_bin:$PATH" "$GANG" run -- sh -c 'printf MARK_RUN_UNACCEPTED' \
  > "$run_launch_output" 2>&1 &
run_launch_pid=$!
IFS= read -r -N 1 _ < "$run_launch_ready"
run_launch_manager="$(<"$run_launch_manager_pid")"
kill -TERM "$run_launch_pid"
kill -TERM "$run_launch_manager"
run_launch_rc=0
wait "$run_launch_pid" || run_launch_rc=$?
if [ "$run_launch_rc" -ne 0 ]; then
  pass "an interrupted requester leaves its pre-acceptance declaration behind"
else
  fail "an interrupted requester leaves its pre-acceptance declaration behind" \
    "the held requester unexpectedly returned success"
fi
run_unaccepted_record="$(run_record_for MARK_RUN_UNACCEPTED)" || run_unaccepted_record=""
equal "the interrupted declaration records launching rather than active" launching \
  "$(<"$run_unaccepted_record/active")"
run_unaccepted_roster="$(XDG_STATE_HOME="$run_state" "$GANG" roster)"
contains "roster exposes an interrupted pre-acceptance host launch" \
  "$run_unaccepted_roster" "host-run-launching=${run_unaccepted_record##*/}"
run_unaccepted_unknown="$(GANG_TEST_RUN_UNIT_STATE=unreadable run_recover)"
contains "an unreadable manager preserves a launching declaration as unknown" \
  "$run_unaccepted_unknown" "is unconfirmed"
equal "an unreadable manager does not erase the launching declaration" launching \
  "$(<"$run_unaccepted_record/active")"
run_unaccepted_settled="$(GANG_TEST_RUN_UNIT_STATE=absent run_recover)"
contains "an absent manager settles the unaccepted request without cancellation" \
  "$run_unaccepted_settled" "settled unaccepted run ${run_unaccepted_record##*/}"
if [ -e "$run_unaccepted_record/active" ]; then
  fail "settling an unaccepted request releases its active slot" "active remained at $run_unaccepted_record"
else
  pass "settling an unaccepted request releases its active slot"
fi
rm -f -- "$run_launch_hold" "$run_launch_ready"

run_kill_out="$(run_start sh -c 'printf MARK_RUN_KILL')"
contains "a SIGKILL-shaped run is accepted" "$run_kill_out" "started run"
run_kill_record="$(run_record_for MARK_RUN_KILL)" || run_kill_record=""
run_finalize_direct "$run_kill_record" killed KILL
contains "a SIGKILL result uses systemd's signal-name status" "$(<"$run_kill_record/result")" $'137\t'

run_term_ready="$RUN_ROOT/run-term-ready"
mkfifo "$run_term_ready"
run_term_out="$(run_start sh -c 'trap "printf MARK_RUN_TERM_CHILD; exit 0" TERM; printf MARK_RUN_TERM_READY; printf x > "$1"; while :; do :; done' sh "$run_term_ready")"
contains "a TERM-forwarding run is accepted" "$run_term_out" "started run"
run_term_record="$(run_record_for MARK_RUN_TERM_READY)" || run_term_record=""
( RUN_RUNNER_EXEC=1 run_runner_direct "$run_term_record" "$run_term_record" ) &
run_term_runner=$!
IFS= read -r -N 1 _ < "$run_term_ready"
kill -TERM "$run_term_runner"
run_term_runner_rc=0
wait "$run_term_runner" || run_term_runner_rc=$?
run_finalize_direct "$run_term_record" exited "$run_term_runner_rc"
contains "a cancellation forwards TERM to the command" "$(<"$run_term_record/output")" \
  "MARK_RUN_TERM_CHILD"

"$HITCH" run-other -c bash -d "$run_state"
run_other_id="$(window_id run-other)"
run_other_pane="$(tmux list-panes -t "$run_other_id" -F '#{pane_id}')"
run_other_cancel_rc=0
run_other_cancel="$(TMUX_PANE="$run_other_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run --cancel "$run_cancel_id" 2>&1)" || run_other_cancel_rc=$?
if [ "$run_other_cancel_rc" -ne 0 ] && [[ "$run_other_cancel" == *"only the stable identity"* ]]; then
  pass "a different live agent cannot cancel another request"
else
  fail "a different live agent cannot cancel another request" "reply [$run_other_cancel]"
fi
run_other_active="$(TMUX_PANE="$run_other_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run --active)"
excludes "a different agent cannot list another owner's host run" \
  "$run_other_active" "$run_successor_id"
"$GANG" drop run-other

run_successor_cancel="$(TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run --cancel "$run_successor_id")"
contains "the successor may cancel its predecessor's host run" \
  "$run_successor_cancel" "cancellation requested"
run_successor_unit="$(<"$run_successor_record/unit")"
contains "successor cancellation reaches the recovered exact service" \
  "$(<"$run_stops")" "stop -- $run_successor_unit"
run_finalize_direct "$run_successor_record" killed TERM

run_finalize_direct "$run_cancel_record" killed TERM
contains "a killed runner receives a fallback completion result" "$(<"$run_cancel_record/result")" $'143\t'
if [ -f "$run_cancel_record/active" ]; then
  fail "a killed runner releases its team concurrency slot" "active remained at $run_cancel_record"
else
  pass "a killed runner releases its team concurrency slot"
fi

run_max_a="$(run_start sh -c 'printf MARK_RUN_MAX_A')"
run_max_b="$(run_start sh -c 'printf MARK_RUN_MAX_B')"
run_max_c="$(run_start sh -c 'printf MARK_RUN_MAX_C')"
run_max_d="$(run_start sh -c 'printf MARK_RUN_MAX_D')"
contains "the first capacity-filling run is accepted" "$run_max_a" "started run"
contains "the second capacity-filling run is accepted" "$run_max_b" "started run"
contains "the third capacity-filling run is accepted" "$run_max_c" "started run"
contains "the fourth capacity-filling run is accepted" "$run_max_d" "started run"
run_max_refusal_rc=0
run_max_refusal="$(run_start sh -c 'printf MARK_RUN_MAX_REFUSAL' 2>&1)" || run_max_refusal_rc=$?
if [ "$run_max_refusal_rc" -ne 0 ] && [[ "$run_max_refusal" == *"already has 4 active commands"* ]]; then
  pass "the per-team active-run bound refuses a fifth command"
else
  fail "the per-team active-run bound refuses a fifth command" "reply [$run_max_refusal]"
fi
for run_max_marker in MARK_RUN_MAX_A MARK_RUN_MAX_B MARK_RUN_MAX_C MARK_RUN_MAX_D; do
  run_max_record="$(run_record_for "$run_max_marker")" || run_max_record=""
  [ -z "$run_max_record" ] || run_finish_direct "$run_max_record"
done

run_launch_fail_rc=0
run_launch_fail="$(TMUX_PANE="$run_requester_pane" XDG_STATE_HOME="$run_state" \
  TMPDIR="$run_state/requester-tmp" GANG_TEST_RUN_FAIL=1 \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run -- sh -c 'printf MARK_RUN_LAUNCH_FAIL' 2>&1)" || run_launch_fail_rc=$?
if [ "$run_launch_fail_rc" -ne 0 ] && ! run_record_for MARK_RUN_LAUNCH_FAIL >/dev/null; then
  pass "a refused service launch removes its unarmed result record"
else
  fail "a refused service launch removes its unarmed result record" "reply [$run_launch_fail]"
fi

run_rename_out="$(run_start sh -c 'printf MARK_RUN_RENAME')"
contains "a rename-safe run is accepted" "$run_rename_out" "started run"
run_rename_record="$(run_record_for MARK_RUN_RENAME)" || run_rename_record=""
"$GANG" rename run-requester run-requester-renamed
run_finish_direct "$run_rename_record"
run_rename_mail="$(XDG_STATE_HOME="$run_state" "$GANG" mail run-requester-renamed)"
contains "a renamed requester receives its completion by stable token" "$run_rename_mail" \
  "run ${run_rename_record##*/} completed"

"$HITCH" run-dropped -c bash -d "$run_state"
run_dropped_id="$(window_id run-dropped)"
run_dropped_pane="$(tmux list-panes -t "$run_dropped_id" -F '#{pane_id}')"
run_dropped_out="$(TMUX_PANE="$run_dropped_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run -- sh -c 'printf MARK_RUN_DROPPED')"
contains "a second agent can request its own run" "$run_dropped_out" "started run"
run_dropped="$(run_record_for MARK_RUN_DROPPED)" || run_dropped=""
"$GANG" drop run-dropped
run_finish_direct "$run_dropped"
contains "a dropped requester leaves a named retained result" "$(<"$run_dropped/delivery")" \
  "discarded: requester is gone; output retained"
equal "a dropped requester does not delete its command output" "MARK_RUN_DROPPED" \
  "$(<"$run_dropped/output")"

"$GANG" drop run-requester-renamed
