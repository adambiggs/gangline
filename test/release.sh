#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# THE PRE-RELEASE LANE. The mandatory gate protects every contribution in five
# minutes; this lane retains the complete integration proof before a
# release-please pull request merges.
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_PREFIX

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
HEAVY_LOCK="${GANG_RELEASE_LOCK:-/tmp/gangline-heavy.lock}"
HEAVY_OWNER_LOCK="${HEAVY_LOCK}.owner"

# The two flock parents own their descriptors rather than this shell. That
# keeps an integration-created tmux server from inheriting either descriptor
# and holding the shared lock after this lane exits.
if [ "${GANG_RELEASE_LOCKED:-0}" != 1 ]; then
  command -v flock >/dev/null 2>&1 \
    || { echo "release: flock is required to serialize the full integration lane" >&2; exit 1; }
  export GANG_RELEASE_LOCKED=1
  exec flock -o "$HEAVY_LOCK" "$0" "$@"
fi
if [ "${GANG_RELEASE_LOCK_OWNER:-0}" != 1 ]; then
  : > "$HEAVY_LOCK"
  export GANG_RELEASE_LOCK_OWNER=1
  exec flock -E 201 -n -o "$HEAVY_OWNER_LOCK" "$0" "$@"
fi

release_clear_lock_record() {
  : > "$HEAVY_LOCK"
}
trap release_clear_lock_record EXIT

lock_owner_pid=$$
if [ -r /proc/self/status ]; then
  lock_owner_pid="$(awk '/^NSpid:/ { print $2; exit }' /proc/self/status)"
  [ -n "$lock_owner_pid" ] || lock_owner_pid=$$
fi
lock_started="$(date +%s)"
lock_lease="${lock_owner_pid}-${lock_started}-${RANDOM}-${BASHPID}"
printf 'pid=%s\tstarted=%s\tcwd=%q\tlease=%s\n' \
  "$lock_owner_pid" "$lock_started" "$ROOT" "$lock_lease" > "$HEAVY_LOCK"

# Direct suites require a settled tree. Record that identity before the first
# assertion and re-read it after each stage, so a release verdict never joins
# evidence from bytes that changed while this lane ran.
release_identity="$("$ROOT/test/gate.sh" --assert-owned)"
release_assert_unmoved() {
  "$ROOT/test/gate.sh" --assert-unmoved "$release_identity"
}

# This lane is the full-suite proof. A focused selector is useful while
# developing one fragment but would make a release result attest to assertions
# it never ran, so the release entry point owns the complete scope explicitly.
unset GANG_INTEGRATION_PARTS GANG_INTEGRATION_REQUIRE_ALL_PROBE
export GANG_INTEGRATION_REQUIRE_ALL=1

"$ROOT/test/lint.sh"
release_assert_unmoved
"$ROOT/test/smoke.sh"
release_assert_unmoved
"$ROOT/test/integration.sh"
release_assert_unmoved

printf 'release: the settled tree passed lint, smoke, and full integration.\n'
