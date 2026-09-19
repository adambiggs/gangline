#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# THE PRE-RELEASE LANE. The local gate runs lint and smoke; this lane runs the
# complete integration proof before a release-please pull request merges.
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
HEAVY_LOCK="${GANG_RELEASE_LOCK:-/tmp/gangline-heavy.lock}"

# `flock -o` keeps the descriptor in flock's own process, so a tmux server the
# suite starts cannot inherit it and hold the lock after this lane exits.
if [ "${GANG_RELEASE_LOCKED:-0}" != 1 ]; then
  command -v flock >/dev/null 2>&1 \
    || { echo "release: flock is required to serialize the full integration lane" >&2; exit 1; }
  export GANG_RELEASE_LOCKED=1
  exec flock -o "$HEAVY_LOCK" "$0" "$@"
fi

# This lane is the full-suite proof. A focused selector is useful while
# developing one fragment but would make a release result attest to assertions
# it never ran, so the release entry point owns the complete scope explicitly.
unset GANG_INTEGRATION_PARTS GANG_INTEGRATION_REQUIRE_ALL_PROBE
export GANG_INTEGRATION_REQUIRE_ALL=1

"$ROOT/test/lint.sh"
"$ROOT/test/smoke.sh"
"$ROOT/test/integration.sh"

printf 'release: the tree passed lint, smoke, and full integration.\n'
