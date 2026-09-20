#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# The fail-closed contract for test PATH shims. The self-target cases enter a
# transient service whose task ceiling exists before the first shim process.
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GUARD="${GANG_TEST_PATH_SHIM_GUARD:-$ROOT/test/path-shim-guard.sh}"
SYSTEMD_RUN=/usr/bin/systemd-run
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-path-shim.XXXXXX")"
trap 'rm -rf -- "$TEST_ROOT"' EXIT HUP INT TERM

failures=0
pass() { printf 'ok   %s\n' "$1"; }
fail() {
  printf 'FAIL %s\n       %s\n' "$1" "$2" >&2
  failures=$((failures + 1))
}
contains() {
  case "$2" in *"$3"*) pass "$1" ;; *) fail "$1" "missing [$3] in [$2]" ;; esac
}
equal() { [ "$2" = "$3" ] && pass "$1" || fail "$1" "expected [$2], got [$3]"; }

[ -r "$GUARD" ] || {
  fail "the shared PATH-shim guard exists" "$GUARD is absent or unreadable"
  exit 1
}
[ -x "$SYSTEMD_RUN" ] || {
  fail "the task-ceiling launcher exists" "$SYSTEMD_RUN is absent or not executable"
  exit 1
}

mkdir -p "$TEST_ROOT/bin"
cat > "$TEST_ROOT/bin/git" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
. "$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$GANG_TEST_PATH_SHIM_TARGET" "$0" git || exit $?
"$GANG_TEST_PATH_SHIM_TARGET" "$@"
SH
chmod +x "$TEST_ROOT/bin/git"

# A normal delegation proves the guard passes an absolute executable through
# and preserves its argument vector.
normal_out="$(GANG_TEST_PATH_SHIM_GUARD="$GUARD" \
  GANG_TEST_PATH_SHIM_TARGET=/bin/printf \
  "$TEST_ROOT/bin/git" '%s' delegated)"
equal "an absolute non-self target is delegated" delegated "$normal_out"

bare_rc=0
bare_out="$(GANG_TEST_PATH_SHIM_GUARD="$GUARD" \
  GANG_TEST_PATH_SHIM_TARGET=printf \
  "$TEST_ROOT/bin/git" '%s' delegated 2>&1)" || bare_rc=$?
equal "a PATH-resolved target is refused" 97 "$bare_rc"
contains "the refusal names the absolute-target invariant" \
  "$bare_out" "target is not absolute"

depth_rc=0
depth_out="$(GANG_TEST_PATH_SHIM_DEPTH=8 \
  GANG_TEST_PATH_SHIM_GUARD="$GUARD" \
  GANG_TEST_PATH_SHIM_TARGET=/bin/true \
  "$TEST_ROOT/bin/git" 2>&1)" || depth_rc=$?
equal "the ninth inherited delegation is refused" 97 "$depth_rc"
contains "the refusal names the depth ceiling" "$depth_out" "depth 8 reached the ceiling"

# This is the historical failure shape: a PATH-local git starts another copy
# of itself as a child and waits. The helper must refuse before that call. A
# regression in both local guards can therefore consume the service's process
# table, but it cannot spend more than the ceiling installed by systemd before
# the launcher runs.
write_launcher() {
  launcher="$1"
  target="$2"
  cgroup_root="${3:-/sys/fs/cgroup}"
  cat > "$launcher" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu
cgroup="\$(sed -n 's/^0:://p' /proc/self/cgroup)"
ceiling="\$(cat "$cgroup_root\$cgroup/pids.max")" || {
  printf 'path-shim-test: task ceiling is unreadable; refusing recursion fixture\n' >&2
  exit 90
}
printf '%s\n' "\$ceiling" > "$TEST_ROOT/pids.max"
[ "\$ceiling" = 12 ] || {
  printf 'path-shim-test: task ceiling is %s, not 12; refusing recursion fixture\n' \
    "\$ceiling" >&2
  exit 90
}
exec /usr/bin/env \
  PATH="$TEST_ROOT/bin:/usr/bin:/bin" \
  GANG_TEST_PATH_SHIM_GUARD="$GUARD" \
  GANG_TEST_PATH_SHIM_TARGET="$target" \
  git
SH
  chmod +x "$launcher"
}

# A missing task ceiling must stop before PATH dispatch. This safe control uses
# a marker target rather than either recursive target, so a broken precondition
# cannot escape into an unbounded recursion.
cat > "$TEST_ROOT/target-marker" <<SH
#!/bin/sh
: > "$TEST_ROOT/unbounded-exec"
SH
chmod +x "$TEST_ROOT/target-marker"
write_launcher "$TEST_ROOT/launch-unbounded" \
  "$TEST_ROOT/target-marker" "$TEST_ROOT/no-cgroup"
unbounded_rc=0
"$TEST_ROOT/launch-unbounded" > "$TEST_ROOT/unbounded.out" 2>&1 || unbounded_rc=$?
equal "a missing task ceiling has the precondition-refusal status" 90 "$unbounded_rc"
contains "a missing task ceiling names the precondition refusal" \
  "$(<"$TEST_ROOT/unbounded.out")" "task ceiling is unreadable"
[ ! -e "$TEST_ROOT/unbounded-exec" ] \
  && pass "a missing task ceiling never invokes even a harmless target" \
  || fail "a missing task ceiling never invokes even a harmless target" \
    "the marker target ran"

control_cgroup="$(sed -n 's/^0:://p' /proc/self/cgroup)"
mkdir -p "$TEST_ROOT/max-cgroup$control_cgroup"
printf 'max\n' > "$TEST_ROOT/max-cgroup$control_cgroup/pids.max"
write_launcher "$TEST_ROOT/launch-unbounded-max" \
  "$TEST_ROOT/target-marker" "$TEST_ROOT/max-cgroup"
unbounded_max_rc=0
"$TEST_ROOT/launch-unbounded-max" \
  > "$TEST_ROOT/unbounded-max.out" 2>&1 || unbounded_max_rc=$?
equal "a readable unbounded ceiling has the precondition-refusal status" \
  90 "$unbounded_max_rc"
contains "a readable unbounded ceiling names its observed value" \
  "$(<"$TEST_ROOT/unbounded-max.out")" "task ceiling is max, not 12"
[ ! -e "$TEST_ROOT/unbounded-exec" ] \
  && pass "a readable unbounded ceiling never invokes even a harmless target" \
  || fail "a readable unbounded ceiling never invokes even a harmless target" \
    "the marker target ran"

write_launcher "$TEST_ROOT/launch-self" "$TEST_ROOT/bin/git"

self_rc=0
"$SYSTEMD_RUN" --user --quiet --wait --pipe --collect --service-type=exec \
  --unit="gangline-path-shim-self-$$" --property=TasksMax=12 \
  --property=RuntimeMaxSec=30 \
  "$TEST_ROOT/launch-self" > "$TEST_ROOT/self.out" 2>&1 || self_rc=$?
self_out="$(<"$TEST_ROOT/self.out")"
equal "the historical recursion fixture enters a pre-bounded service" \
  12 "$(<"$TEST_ROOT/pids.max")"
equal "an absolute self-target is refused" 97 "$self_rc"
contains "the refusal names the self-file invariant" \
  "$self_out" "target resolves to the shim itself"

# String inequality is not file inequality. An absolute symlink to the shim is
# the same recursive target and must receive the same verdict.
ln -s "$TEST_ROOT/bin/git" "$TEST_ROOT/bin/git-alias"
write_launcher "$TEST_ROOT/launch-alias" "$TEST_ROOT/bin/git-alias"
alias_rc=0
"$SYSTEMD_RUN" --user --quiet --wait --pipe --collect --service-type=exec \
  --unit="gangline-path-shim-alias-$$" --property=TasksMax=12 \
  --property=RuntimeMaxSec=30 \
  "$TEST_ROOT/launch-alias" > "$TEST_ROOT/alias.out" 2>&1 || alias_rc=$?
equal "an absolute alias of the shim is refused" 97 "$alias_rc"
contains "the alias refusal uses file identity" "$(<"$TEST_ROOT/alias.out")" \
  "target resolves to the shim itself"

[ "$failures" -eq 0 ] || exit 1
printf 'path-shim-guard: all checks passed\n'
