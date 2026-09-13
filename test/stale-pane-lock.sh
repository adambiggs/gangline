#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Exercise the production pane-lock functions under the invoking Bash. macOS
# keeps Bash 3.2, so syntax-only CI there cannot prove that stale delivery
# locks still recover.
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="${GANG_UNDER_TEST:-$ROOT/bin/gang}"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-stale-pane-lock.XXXXXX")"
trap 'rm -rf -- "$TEST_ROOT"' EXIT HUP INT TERM

[ -r "$GANG" ] || {
  printf 'stale-pane-lock: cannot read production source %s\n' "$GANG" >&2
  exit 1
}
awk '/^LOCK_PATH="" LOCK_OWNER=""/{p=1} p{print} /^lock_pane\(\)/{inpane=1} inpane&&/^}/{exit}' \
  "$GANG" > "$TEST_ROOT/lock-functions.sh"

cat > "$TEST_ROOT/harness.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

fail() { local code="$1"; shift; printf 'gang: %s\n' "$*" >&2; exit "$code"; }
die() { fail 1 "$@"; }
refuse() { fail 3 "$@"; }
agent_name_of() { printf '%s' "$1"; }
on_exit() { local rc=$?; set +e; lock_release; return "$rc"; }
GANGLINE_PROCESS_UID="$(id -u)"
. "$1"

GANG_LOCK_DIR="$2/locks"
mkdir -m 700 "$GANG_LOCK_DIR"
path="$GANG_LOCK_DIR/pane.lock"
ln -s 99999999 "$path"

lock_pane pane
[ "$(readlink "$path")" = "$LOCK_SELF_PID" ] || {
  printf 'stale pane lock was not replaced by its live reclaimer\n' >&2
  exit 1
}
lock_release
[ ! -L "$path" ] || {
  printf 'reclaimed pane lock was not released\n' >&2
  exit 1
}

lock_pane pane
rm -f -- "$path"
ln -s 12345 "$path"
lock_release
[ "$(readlink "$path")" = 12345 ] || {
  printf 'a stale releaser removed a replacement pane lock\n' >&2
  exit 1
}
SH
chmod +x "$TEST_ROOT/harness.sh"

"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" "$TEST_ROOT"
printf 'stale-pane-lock: recovered and released under Bash %s.%s\n' \
  "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}"
