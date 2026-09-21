#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# The local gate: lint, smoke, and Go checks against this working tree.
# Integration, the full shell lint set and checker self-tests run in CI.
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX

# The gate prepares and runs disposable fixture lanes. Keep an agent pane's
# return route and team selection from reaching them.
unset TMUX TMUX_PANE GANG_TMUX_SOCKET GANG_TMUX_GUARD_AGENT \
  GANG_CONFIG_DIR GANG_SESSION GANG_COLLARS \
  GANG_LOCK_DIR GANG_ARCHIVE_DIR GANG_SCOPE

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"

# One gate at a time per host. `flock -o` keeps the lock in flock's own
# process, so a tmux server a fixture starts cannot inherit the descriptor and
# hold the lock after the gate ends. The timeout bounds the run, not the wait.
if [ -z "${_GANGLINE_GATE_LOCKED:-}" ]; then
  export _GANGLINE_GATE_LOCKED=1
  exec flock -o /tmp/gangline-heavy.lock timeout 900 "$0" "$@"
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
    printf 'gate: VERDICT PASS (status 0); this gate ran lint, smoke, and Go checks; shell integration runs in CI.\n'
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
test/lint.sh --fast || rc=$?
smoke_rc=0
test/smoke.sh || smoke_rc=$?
[ "$rc" -ne 0 ] || rc=$smoke_rc
go_rc=0
test/go.sh || go_rc=$?
[ "$rc" -ne 0 ] || rc=$go_rc
decided=1
exit "$rc"
