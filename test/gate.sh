#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# The local gate.
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX

# Keep an agent pane's return route and team selection out of checks.
unset TMUX TMUX_PANE GANG_TMUX GANG_TMUX_SOCKET GANG_CONFIG_DIR GANG_SESSION GANG_COLLARS
unset GANGLINE_ACCEPTANCE_COLLARS

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"

# THE LAST LINE CARRIES THE VERDICT, because `test/gate.sh 2>&1 | tail` loses
# the exit status. A run that ends before the checks report is UNKNOWN, not
# REFUSED: it produced no verdict on the tree.
decided=0
started=$SECONDS
verdict() {
  local rc=$?
  printf 'gate: total wall time %ss\n' "$((SECONDS - started))"
  if [ "$decided" -ne 1 ]; then
    printf 'gate: VERDICT UNKNOWN (status %s)\n' "$rc"
  elif [ "$rc" -eq 0 ]; then
    printf 'gate: VERDICT PASS (status 0); deterministic checks completed.\n'
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
