#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Exercise the production pane-lock functions under the invoking Bash. macOS
# keeps Bash 3.2, so syntax-only CI there cannot prove that stale delivery
# locks still recover.
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
GANG="${GANG_UNDER_TEST:-$ROOT/bin/gang}"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gangline-stale-pane-lock.XXXXXX")"
namespace_launcher=""
namespace_owner=""
cleanup() {
  if [ -n "$namespace_owner" ] && kill -0 "$namespace_owner" 2>/dev/null; then
    kill -KILL "$namespace_owner"
  fi
  if [ -n "$namespace_launcher" ] && kill -0 "$namespace_launcher" 2>/dev/null; then
    kill -KILL "$namespace_launcher"
  fi
  [ -z "$namespace_launcher" ] || wait "$namespace_launcher" 2>/dev/null || :
  rm -rf -- "$TEST_ROOT"
}
trap 'cleanup' EXIT HUP INT TERM

[ -r "$GANG" ] || {
  printf 'stale-pane-lock: cannot read production source %s\n' "$GANG" >&2
  exit 1
}
awk '/^PROCESS_ID_PID=""/{p=1} p{print} /^lock_pane\(\)/{inpane=1} inpane&&/^}/{exit}' \
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
ROOT="$4"
SESSION=stale-pane-lock-fixture
. "$3"
gangline_state_root_resolve "$GANGLINE_PROCESS_UID"
. "$1"

fixture_root="$2"
GANG_LOCK_DIR="$fixture_root/locks"
mkdir -p -m 700 "$GANG_LOCK_DIR"
path="$GANG_LOCK_DIR/pane.lock"
fixture_mode="${5:-standard}"
retry_ledger="${6:-}"
mkdir -p "$fixture_root/bin"
cat > "$fixture_root/bin/sleep" <<'CLOCK'
#!/bin/sh
[ -n "${GANG_TEST_PANE_LOCK_RETRY_LEDGER:-}" ] || {
  printf 'stale-pane-lock fixture reached an uninstrumented clock wait\n' >&2
  exit 98
}
printf '%s\n' "$1" >> "$GANG_TEST_PANE_LOCK_RETRY_LEDGER"
if [ "${GANG_TEST_PANE_LOCK_RETRY_MODE:-}" = retry-release ] \
   && [ ! -e "$GANG_TEST_PANE_LOCK_RETRY_ROOT/released" ]; then
  : > "$GANG_TEST_PANE_LOCK_RETRY_ROOT/released"
  rm -f -- "$GANG_TEST_PANE_LOCK_RETRY_PATH"
fi
CLOCK
chmod +x "$fixture_root/bin/sleep"
PATH="$fixture_root/bin:$PATH"
export PATH
GANG_TEST_PANE_LOCK_RETRY_LEDGER="$retry_ledger"
GANG_TEST_PANE_LOCK_RETRY_MODE="$fixture_mode"
GANG_TEST_PANE_LOCK_RETRY_ROOT="$fixture_root"
GANG_TEST_PANE_LOCK_RETRY_PATH="$path"
export GANG_TEST_PANE_LOCK_RETRY_LEDGER GANG_TEST_PANE_LOCK_RETRY_MODE \
  GANG_TEST_PANE_LOCK_RETRY_ROOT GANG_TEST_PANE_LOCK_RETRY_PATH
ln() {
  local argument="" last="" once="$fixture_root/vanished-once"
  for argument do last="$argument"; done
  if [ "$fixture_mode" = vanish ] && [ "$last" = "$path" ] && [ ! -e "$once" ]; then
    : > "$once"
    printf "ln: failed to create symbolic link '%s': File exists\n" "$path" >&2
    return 1
  fi
  command ln "$@"
}
case "$fixture_mode" in
  owner)
    lock_pane pane
    "$ROOT/libexec/gang-process-identity" --tick "$LOCK_SELF_PID" "$SESSION" > "$6"
    IFS= read -r _ < "$7"
    trap - EXIT
    exit 0
    ;;
  claim)
    lock_pane pane
    lock_release
    exit 0
    ;;
  vanish)
    lock_pane pane
    lock_release
    exit 0
    ;;
  reused)
    lock_self_pid
    reused_identity="$("$ROOT/libexec/gang-process-identity" \
      --tick "$LOCK_SELF_PID" "$SESSION")"
    IFS=$'\t' read -r _ _ _ _ _ _ _ reused_namespace <<<"$reused_identity"
    command ln -s "v1:$LOCK_SELF_PID:0:$reused_namespace" "$path"
    lock_pane pane
    lock_release
    exit 0
    ;;
  retry-held|retry-release|no-wait)
    lock_self_pid
    command ln -s "$LOCK_SELF_PID" "$path"
    if [ "$fixture_mode" = no-wait ]; then
      lock_pane pane
    else
      lock_pane pane send
    fi
    lock_release
    exit 0
    ;;
  malformed)
    command ln -s not-a-pid "$path"
    lock_pane pane send
    exit 0
    ;;
  standard) ;;
  *) printf 'unknown stale-pane-lock fixture mode: %s\n' "$5" >&2; exit 2 ;;
esac

ln -s 99999999 "$path"

lock_pane pane
[ "$(readlink "$path")" = "$LOCK_OWNER" ] || {
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

"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" "$TEST_ROOT" \
  "$(dirname "$GANG")/../libexec/gang-state-root" "$ROOT"
"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" "$TEST_ROOT/vanish" \
  "$(dirname "$GANG")/../libexec/gang-state-root" "$ROOT" vanish
"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" "$TEST_ROOT/reused" \
  "$(dirname "$GANG")/../libexec/gang-state-root" "$ROOT" reused

retry_held_ledger="$TEST_ROOT/retry-held.ledger"
retry_held_err="$TEST_ROOT/retry-held.err"
retry_held_rc=0
"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
  "$TEST_ROOT/retry-held" "$(dirname "$GANG")/../libexec/gang-state-root" \
  "$ROOT" retry-held "$retry_held_ledger" 2> "$retry_held_err" || retry_held_rc=$?
[ "$retry_held_rc" -eq 3 ] || {
  printf 'held send-mode pane lock exited %s instead of 3: %s\n' \
    "$retry_held_rc" "$(<"$retry_held_err")" >&2
  exit 1
}
[ "$(cat "$retry_held_ledger" 2>/dev/null)" = $'0.1\n0.2\n0.4\n0.8\n1.6' ] || {
  printf 'held send-mode pane lock did not spend its bounded backoff ledger: %s\n' \
    "$(cat "$retry_held_ledger" 2>/dev/null)" >&2
  exit 1
}
case "$(<"$retry_held_err")" in
  *'another Gangline process is delivering to pane — inspect it with gang capture pane before retrying'*) ;;
  *) printf 'exhausted send-mode retry changed the contention diagnostic: %s\n' \
       "$(<"$retry_held_err")" >&2; exit 1 ;;
esac

retry_release_ledger="$TEST_ROOT/retry-release.ledger"
"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
  "$TEST_ROOT/retry-release" "$(dirname "$GANG")/../libexec/gang-state-root" \
  "$ROOT" retry-release "$retry_release_ledger"
[ "$(<"$retry_release_ledger")" = 0.1 ] || {
  printf 'released send-mode pane lock did not succeed after one backoff: %s\n' \
    "$(<"$retry_release_ledger")" >&2
  exit 1
}

no_wait_ledger="$TEST_ROOT/no-wait.ledger"
no_wait_rc=0
"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
  "$TEST_ROOT/no-wait" "$(dirname "$GANG")/../libexec/gang-state-root" \
  "$ROOT" no-wait "$no_wait_ledger" 2> "$TEST_ROOT/no-wait.err" || no_wait_rc=$?
[ "$no_wait_rc" -eq 3 ] && [ ! -e "$no_wait_ledger" ] || {
  printf 'a non-send pane-lock caller retried contention (rc=%s ledger=%s)\n' \
    "$no_wait_rc" "$(cat "$no_wait_ledger" 2>/dev/null)" >&2
  exit 1
}

malformed_ledger="$TEST_ROOT/malformed.ledger"
malformed_rc=0
"$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
  "$TEST_ROOT/malformed" "$(dirname "$GANG")/../libexec/gang-state-root" \
  "$ROOT" malformed "$malformed_ledger" 2> "$TEST_ROOT/malformed.err" || malformed_rc=$?
[ "$malformed_rc" -eq 1 ] && [ ! -e "$malformed_ledger" ] || {
  printf 'a malformed pane-lock owner was retried (rc=%s ledger=%s)\n' \
    "$malformed_rc" "$(cat "$malformed_ledger" 2>/dev/null)" >&2
  exit 1
}

# A PID WRITTEN INSIDE A HARNESS SANDBOX NAMES ONLY THAT PID NAMESPACE. The
# sandbox's pid 1 is an unrelated live process on the host, so a bare-pid lock
# survives forever after the sandbox dies: kill -0 keeps finding the host's pid
# 1. The lock record must carry enough process identity for a host contender to
# distinguish the dead owner without stealing from it while it is live.
namespace_run() {
  unshare --kill-child=SIGKILL -U --map-user="$(id -u)" --map-group="$(id -g)" \
    -pf --mount-proc "$@"
}
namespace_host_pid() { # $1 namespace inode, $2 pid inside it
  local status="" inner="" candidate=""
  for status in /proc/[0-9]*/status; do
    inner="$(awk '/^NSpid:/ { if (NF > 2) print $NF }' "$status" 2>/dev/null)" \
      || continue
    [ "$inner" = "$2" ] || continue
    candidate="${status%/status}"
    [ "$(readlink "$candidate/ns/pid" 2>/dev/null)" = "pid:[$1]" ] || continue
    printf '%s' "${candidate#/proc/}"
    return 0
  done
  return 1
}
if [ "$(readlink /proc/self/ns/pid 2>/dev/null)" != 'pid:[4026531836]' ]; then
  printf 'stale-pane-lock: namespaced dead-owner case not exercised outside the initial pid namespace\n'
elif ! namespace_run true 2>/dev/null; then
  printf 'stale-pane-lock: namespaced dead-owner case not exercised because unprivileged pid namespaces are unavailable\n'
else
  namespace_root="$TEST_ROOT/namespaced"
  namespace_ready="$TEST_ROOT/namespace-ready"
  namespace_release="$TEST_ROOT/namespace-release"
  namespace_live_err="$TEST_ROOT/namespace-live.err"
  namespace_dead_err="$TEST_ROOT/namespace-dead.err"
  mkfifo "$namespace_ready" "$namespace_release"
  namespace_run "$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
    "$namespace_root" "$(dirname "$GANG")/../libexec/gang-state-root" "$ROOT" \
    owner "$namespace_ready" "$namespace_release" &
  namespace_launcher=$!
  IFS=$'\t' read -r namespace_pid namespace_token _ _ _ _ _ namespace_inode \
    < "$namespace_ready"
  [ "$namespace_pid" = 1 ] || {
    printf 'namespaced pane-lock owner did not see itself as pid 1: %s\n' \
      "$namespace_pid" >&2
    exit 1
  }
  namespace_owner="$(namespace_host_pid "$namespace_inode" "$namespace_pid")" || {
    printf 'the namespaced pane-lock owner is absent from the host process table\n' >&2
    exit 1
  }
  namespace_owner_command="$(tr '\0' ' ' < "/proc/$namespace_owner/cmdline")"
  case "$namespace_owner_command" in
    *"$TEST_ROOT/harness.sh"*) ;;
    *) printf 'the resolved namespaced owner is not this fixture: %s\n' \
         "$namespace_owner_command" >&2; exit 1 ;;
  esac
  namespace_live_rc=0
  "$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
    "$namespace_root" "$(dirname "$GANG")/../libexec/gang-state-root" "$ROOT" claim \
    2> "$namespace_live_err" || namespace_live_rc=$?
  [ "$namespace_live_rc" -eq 3 ] || {
    printf 'a live namespaced pane-lock owner was not retained (rc=%s): %s\n' \
      "$namespace_live_rc" "$(<"$namespace_live_err")" >&2
    exit 1
  }

  printf 'release\n' > "$namespace_release"
  wait "$namespace_launcher"
  namespace_owner=""
  namespace_launcher=""
  namespace_dead_state=0
  "$ROOT/libexec/gang-process-identity" --tick "$namespace_pid" \
    stale-pane-lock-fixture "$namespace_inode" "$namespace_token" \
    >/dev/null 2>&1 || namespace_dead_state=$?
  [ "$namespace_dead_state" -eq 1 ] || {
    printf 'the namespaced pane-lock owner was not proven dead (state=%s)\n' \
      "$namespace_dead_state" >&2
    exit 1
  }
  namespace_dead_rc=0
  "$BASH" "$TEST_ROOT/harness.sh" "$TEST_ROOT/lock-functions.sh" \
    "$namespace_root" "$(dirname "$GANG")/../libexec/gang-state-root" "$ROOT" claim \
    2> "$namespace_dead_err" || namespace_dead_rc=$?
  [ "$namespace_dead_rc" -eq 0 ] || {
    printf 'dead namespaced pane lock was not reclaimed (rc=%s): %s\n' \
      "$namespace_dead_rc" "$(<"$namespace_dead_err")" >&2
    exit 1
  }
  [ ! -L "$namespace_root/locks/pane.lock" ] || {
    printf 'reclaimed namespaced pane lock was not released\n' >&2
    exit 1
  }
  printf 'stale-pane-lock: reclaimed a dead owner from a child pid namespace\n'
fi
printf 'stale-pane-lock: recovered and released under Bash %s.%s\n' \
  "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}"
