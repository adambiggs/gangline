#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# The local gate for the Go implementation.
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX

# The gate prepares and runs disposable fixture lanes. Keep an agent pane's
# return route and team selection from reaching them.
unset TMUX TMUX_PANE GANG_TMUX_SOCKET GANG_CONFIG_DIR GANG_SESSION GANG_COLLARS

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"

# One gate at a time per host. `flock -o` keeps the lock in flock's own
# process, so a tmux server a fixture starts cannot inherit the descriptor and
# hold the lock after the gate ends. The timeout bounds the run, not the wait.
if [ -z "${_GANGLINE_GATE_LOCKED:-}" ]; then
  export _GANGLINE_GATE_LOCKED=1
  outer_rc=0
  flock -E 75 -o -w 3 /tmp/gangline-heavy.lock timeout 110 "$0" "$@" || outer_rc=$?
  if [ "$outer_rc" -eq 75 ]; then
    printf 'gate: VERDICT UNKNOWN (status 75); another gate owns the host lock.\n'
  fi
  exit "$outer_rc"
fi

# THE LAST LINE CARRIES THE VERDICT, because `test/gate.sh 2>&1 | tail` loses
# the exit status. A run that ends before both steps report is UNKNOWN, not
# REFUSED: it produced no verdict on the tree.
decided=0
verdict() {
  local rc=$?
  if [ "$decided" -ne 1 ]; then
    printf 'gate: VERDICT UNKNOWN (status %s)\n' "$rc"
  elif [ "$rc" -eq 0 ]; then
    printf 'gate: VERDICT PASS (status 0); this gate ran the Go checks and private-tmux acceptance scenarios.\n'
  else
    printf 'gate: VERDICT REFUSED (status %s)\n' "$rc"
  fi
}
trap verdict EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

cd "$ROOT"
rc=0
test/go.sh || rc=$?
decided=1
exit "$rc"
