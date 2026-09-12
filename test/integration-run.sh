#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
# Host-service run requests: durable result, bounded delivery, cancellation, and a gone requester.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file in
# order and it reads that shell's fixtures, helpers and counters.
: "${alpha_id:?test/integration-run.sh requires alpha_id from test/integration-substrate.sh}"
: "${alpha_tmux_pane:?test/integration-run.sh requires alpha_tmux_pane from test/integration-substrate.sh}"
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
  'test "${GANG_TEST_RUN_FAIL:-0}" != 1 || exit 1' \
  'exit 0' > "$run_bin/systemd-run"
printf '%s\n' \
  '#!/bin/sh' \
  'printf '\''%s\n'\'' "$*" >> "$GANG_TEST_RUN_STOPS"' \
  'case "$*" in' \
  '  *'\''show --property=Version --value'\''*) printf '\''test-manager\n'\''; exit 0 ;;' \
  '  *'\''stop --'\''*) exit 0 ;;' \
  '  *) printf '\''inactive\n'\''; exit 3 ;;' \
  'esac' > "$run_bin/systemctl"
chmod +x "$run_bin/systemd-run" "$run_bin/systemctl"

run_start() { # command words -> start a run as alpha, with an immediate fake service arm
  TMUX_PANE="$alpha_tmux_pane" XDG_STATE_HOME="$run_state" \
    TMPDIR="$run_state/requester-tmp" \
    GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
    PATH="$run_bin:$PATH" "$GANG" run -- "$@"
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
tmux send-keys -l -t "$alpha_id" 'HUMAN_DRAFT'
run_finish_direct "$run_small"
contains "the runner records the completed exit code" "$(<"$run_small/result")" $'0\t'
equal "the runner preserves complete combined output" "MARK_RUN_SMALL" "$(<"$run_small/output")"
run_small_mail="$(XDG_STATE_HOME="$run_state" "$GANG" mail alpha)"
contains "completion arrives through an enveloped Gangline delivery" "$run_small_mail" "[gang:self-declared:gang-run"
contains "the completion carries the durable output path" "$run_small_mail" "$run_small/output"
contains "the completion carries the command output tail" "$run_small_mail" "MARK_RUN_SMALL"
run_audit="${run_small%/*}/audit.tsv"
contains "completion appends the requester to the durable run audit" "$(<"$run_audit")" $'\talpha\t'
contains "completion appends its output path to the durable run audit" "$(<"$run_audit")" \
  "$run_small/output"

run_existing_tmpdir="$run_state/requester-existing-tmp"
mkdir -p "$run_existing_tmpdir"
chmod 755 "$run_existing_tmpdir"
run_existing_tmpdir_out="$(TMUX_PANE="$alpha_tmux_pane" XDG_STATE_HOME="$run_state" \
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
alpha_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$alpha_id" @gl_spool)"
run_large_envelope=""
if run_large_envelopes="$(grep -rl "run ${run_large##*/} completed" "$alpha_spool")"; then
  run_large_envelope="${run_large_envelopes%%$'\n'*}"
fi
if [ -n "$run_large_envelope" ]; then
  pass "the large-output completion is retained in the ordinary spool"
else
  fail "the large-output completion is retained in the ordinary spool" \
    "no matching completion under $alpha_spool"
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
run_cancel_reply="$(TMUX_PANE="$alpha_tmux_pane" XDG_STATE_HOME="$run_state" \
  GANG_TEST_RUN_ARGS="$run_args" GANG_TEST_RUN_STOPS="$run_stops" \
  PATH="$run_bin:$PATH" "$GANG" run --cancel "$run_cancel_id")"
contains "the requesting stable identity may cancel its own run" "$run_cancel_reply" "cancellation requested"
run_cancel_unit="$(<"$run_cancel_record/unit")"
contains "cancellation addresses the service exact to that run" "$(<"$run_stops")" \
  "stop -- $run_cancel_unit"
run_finalize_direct "$run_cancel_record" killed TERM
contains "a killed runner receives a fallback completion result" "$(<"$run_cancel_record/result")" $'143\t'
if [ -f "$run_cancel_record/active" ]; then
  fail "a killed runner releases its team concurrency slot" "active remained at $run_cancel_record"
else
  pass "a killed runner releases its team concurrency slot"
fi

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
run_runner_direct "$run_term_record" "$run_term_record" &
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
"$GANG" drop run-other

run_max_a="$(run_start sh -c 'printf MARK_RUN_MAX_A')"
run_max_b="$(run_start sh -c 'printf MARK_RUN_MAX_B')"
run_max_c="$(run_start sh -c 'printf MARK_RUN_MAX_C')"
contains "the first capacity-filling run is accepted" "$run_max_a" "started run"
contains "the second capacity-filling run is accepted" "$run_max_b" "started run"
contains "the third capacity-filling run is accepted" "$run_max_c" "started run"
run_max_refusal_rc=0
run_max_refusal="$(run_start sh -c 'printf MARK_RUN_MAX_REFUSAL' 2>&1)" || run_max_refusal_rc=$?
if [ "$run_max_refusal_rc" -ne 0 ] && [[ "$run_max_refusal" == *"already has 4 active commands"* ]]; then
  pass "the per-team active-run bound refuses a fifth command"
else
  fail "the per-team active-run bound refuses a fifth command" "reply [$run_max_refusal]"
fi
for run_max_marker in MARK_RUN_MAX_A MARK_RUN_MAX_B MARK_RUN_MAX_C; do
  run_max_record="$(run_record_for "$run_max_marker")" || run_max_record=""
  [ -z "$run_max_record" ] || run_finish_direct "$run_max_record"
done

run_launch_fail_rc=0
run_launch_fail="$(TMUX_PANE="$alpha_tmux_pane" XDG_STATE_HOME="$run_state" \
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
"$GANG" rename alpha alpha-renamed
run_finish_direct "$run_rename_record"
run_rename_mail="$(XDG_STATE_HOME="$run_state" "$GANG" mail alpha-renamed)"
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

tmux send-keys -t "$alpha_id" C-u
"$GANG" tick
