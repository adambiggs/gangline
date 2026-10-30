#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX
unset TMUX TMUX_PANE GANG_TMUX GANG_TMUX_SOCKET GANG_CONFIG_DIR GANG_SESSION GANG_COLLARS
unset GANGLINE_ACCEPTANCE_COLLARS

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if [ "$#" -ne 0 ]; then
  echo 'usage: test/acceptance.sh' >&2
  exit 2
fi

# Live providers share their native profiles. Keep concurrent lanes apart;
# -o prevents fixture children from inheriting the host lock.
if [ -z "${_GANGLINE_ACCEPTANCE_LOCKED:-}" ]; then
  export _GANGLINE_ACCEPTANCE_LOCKED=1
  rc=0
  flock -E 75 -o -n "${TMPDIR:-/tmp}/gangline-heavy.lock" "$0" || rc=$?
  if [ "$rc" -eq 75 ]; then
    printf 'acceptance: VERDICT UNKNOWN (status 75); another acceptance lane owns the host lock.\n'
  fi
  exit "$rc"
fi

started=$SECONDS
part_start=$SECONDS
decided=0
verdict() {
  local rc=$?
  printf 'acceptance: total wall time %ss\n' "$((SECONDS - started))"
  if [ "$decided" -ne 1 ]; then
    printf 'acceptance: VERDICT UNKNOWN (status %s)\n' "$rc"
  elif [ "$rc" -eq 0 ]; then
    printf 'acceptance: VERDICT PASS (status 0); available checks completed; provider skips are reported above.\n'
  else
    printf 'acceptance: VERDICT REFUSED (status %s)\n' "$rc"
  fi
}
trap verdict EXIT
trap 'decided=0; exit 129' HUP
trap 'decided=0; exit 130' INT
trap 'decided=0; exit 143' TERM
passed() {
  printf 'acceptance: %s (%ss)\n' "$1" "$((SECONDS - part_start))"
  part_start=$SECONDS
}

# Set the verdict before each check so its failure refuses the lane.
decided=1
test/distribution.sh
passed 'distribution passed'
skip='^TestLive(ClaudeCode|Codex)$'
if [ "$(uname -s)" = Linux ]; then
  if bus_output="$(systemctl --user show-environment 2>&1)"; then
    :
  else
    printf 'acceptance: SKIP bus-dependent lifecycle: %s\n' "$bus_output"
    skip='^Test(CommandLifecycleOnPrivateTmux|LiveClaudeCode|LiveCodex)$'
  fi
fi
go test -tags=acceptance -count=1 -timeout=0 -skip "$skip" ./...
passed 'service and persistence tests passed'
GANGLINE_LIVE_PROVIDERS=1 go test -tags=acceptance -v -count=1 -timeout=0 -run '^TestLive(ClaudeCode|Codex)$' ./acceptance
passed 'live provider checks completed'
