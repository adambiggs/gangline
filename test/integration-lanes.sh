# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# The lanes around the suite: the release lane, the local gate, and the shared
# summary and teardown every suite ends through.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.

# An adoption runs from an ordinary client, where tmux 3.2a can take down the
# server if display-message targets the adopted pane.  Pane facts use the
# list/format reader; keep this structural guard so the dangerous form cannot
# return with a future refactor.
if ! command -v rg >/dev/null; then
  fail "adoption guard has its ripgrep dependency" \
    "rg is required to inspect the dangerous display-message form"
elif rg -Fq "display-message -p -t \"\$id\" '#{pane_current_path}'" "$ROOT/bin/gang"; then
  fail "adoption never reads pane path through display-message" \
    "bin/gang targets pane_current_path through display-message"
else
  pass "adoption never reads pane path through display-message"
fi

# THE PRE-RELEASE LANE CARRIES THE ASSERTIONS THE FIVE-MINUTE GATE CANNOT. Its
# fixture proves that it invokes all three checks in order and that a failed integration remains the lane's result instead of becoming a
# green contribution verdict.
release_dependencies="$(awk '
  $0 == "  release:" { in_release = 1; next }
  in_release && /^  [[:alnum:]_-]+:/ { exit }
  in_release && /^      - name: Dependencies$/ { in_dependencies = 1; next }
  in_dependencies && /^      - name:/ { exit }
  in_dependencies { print }
' "$ROOT/.github/workflows/shell.yml")"
contains "the release job installs ripgrep for the integration guard" \
  "$release_dependencies" "ripgrep"
contains "the release job installs util-linux for the release lock" \
  "$release_dependencies" "util-linux"
release_run="$RUN_ROOT/release-lane"
release_lock="$RUN_ROOT/release-lane.lock"
release_order="$RUN_ROOT/release-lane-order"
release_scope="$RUN_ROOT/release-lane-scope"
release_flock_bin="$RUN_ROOT/release-lane-flock-bin"
release_flock_args="$RUN_ROOT/release-lane-flock-args"
release_real_flock="$(command -v flock)"
mkdir -p "$release_run/test"
cp "$ROOT/test/release.sh" "$release_run/test/"
for release_step in lint smoke integration; do
  cat > "$release_run/test/$release_step.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf '$release_step\\n' >> "$release_order"
exit "\${RELEASE_FAIL_${release_step^^}:-0}"
SH
  chmod +x "$release_run/test/$release_step.sh"
done
cat > "$release_run/test/integration.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf 'integration\n' >> "$release_order"
printf 'parts=<%s> require=<%s>\n' "\${GANG_INTEGRATION_PARTS-}" \
  "\${GANG_INTEGRATION_REQUIRE_ALL-}" > "$release_scope"
exit "\${RELEASE_FAIL_INTEGRATION:-0}"
SH
chmod +x "$release_run/test/integration.sh"
release_run="$(cd -P "$release_run" && pwd)"
mkdir -p "$release_flock_bin"
export RELEASE_FLOCK_ARGS="$release_flock_args" RELEASE_REAL_FLOCK="$release_real_flock"
cat > "$release_flock_bin/flock" <<'SH'
#!/bin/sh
for argument in "$@"; do
  printf '<%s>\n' "$argument" >> "$RELEASE_FLOCK_ARGS"
done
exec "$RELEASE_REAL_FLOCK" "$@"
SH
chmod +x "$release_flock_bin/flock"
: > "$release_order"
# The hosted release lane runs this fixture inside a real test/release.sh, which
# has already exported GANG_RELEASE_LOCKED. Clearing it makes the nested lane
# take its own flock, so the capture below has something to read.
release_out="$(env -u GANG_RELEASE_LOCKED \
  PATH="$release_flock_bin:$PATH" GANG_RELEASE_LOCK="$release_lock" GANG_INTEGRATION_PARTS=cli \
  GANG_INTEGRATION_REQUIRE_ALL=0 \
  "$release_run/test/release.sh")"
equal "the release lane runs lint, smoke, and integration in order" \
  "$(printf 'lint\nsmoke\nintegration')" "$(<"$release_order")"
contains "the release lane reports its complete green proof" \
  "$release_out" "passed lint, smoke, and full integration"
equal "the release lane clears a focused selector and requires every part" \
  "parts=<> require=<1>" "$(<"$release_scope")"
if [ -s "$release_flock_args" ]; then
  contains "the release lane records its flock acquisition" \
    "$(<"$release_flock_args")" "<-o>"
  excludes "the release lane lock acquisition has no wait deadline" \
    "$(<"$release_flock_args")" "<-w>"
else
  fail "the release lane records its flock acquisition" \
    "the release fixture wrote no readable flock capture at $release_flock_args"
fi
: > "$release_order"
release_failed_rc=0
env -u GANG_RELEASE_LOCKED \
  PATH="$release_flock_bin:$PATH" GANG_RELEASE_LOCK="$release_lock" RELEASE_FAIL_INTEGRATION=7 \
  "$release_run/test/release.sh" >/dev/null 2>&1 || release_failed_rc=$?
equal "a failed release integration keeps its status" 7 "$release_failed_rc"
equal "a failed release integration still follows lint and smoke" \
  "$(printf 'lint\nsmoke\nintegration')" "$(<"$release_order")"


# THE GATE'S LAST LINE CARRIES ITS VERDICT, because `test/gate.sh 2>&1 | tail`
# keeps the line and loses the status. The marker skips the host lock and the
# run's timeout; stand-in steps record that both ran.
gate_run="$RUN_ROOT/gate"
gate_order="$RUN_ROOT/gate-order"
mkdir -p "$gate_run/test"
cp "$ROOT/test/gate.sh" "$gate_run/test/gate.sh"
for gate_step in lint smoke; do
  cat > "$gate_run/test/$gate_step.sh" <<SH
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
printf '$gate_step\\n' >> "$gate_order"
exit "\${GATE_FAIL_${gate_step^^}:-0}"
SH
  chmod +x "$gate_run/test/$gate_step.sh"
done
: > "$gate_order"
gate_out="$(_GANGLINE_GATE_LOCKED=1 "$gate_run/test/gate.sh" 2>&1)"
equal "a green gate ends on its verdict and says integration runs in CI" \
  "gate: VERDICT PASS (status 0); this gate ran lint and smoke only, and integration runs in CI." \
  "$(printf '%s\n' "$gate_out" | tail -n 1)"
equal "the gate runs lint and then smoke" "$(printf 'lint\nsmoke')" "$(<"$gate_order")"
: > "$gate_order"
gate_rc=0
gate_out="$(GATE_FAIL_LINT=3 _GANGLINE_GATE_LOCKED=1 "$gate_run/test/gate.sh" 2>&1)" \
  || gate_rc=$?
equal "a failed lint is the gate's status" 3 "$gate_rc"
equal "and smoke still runs after it" "$(printf 'lint\nsmoke')" "$(<"$gate_order")"
equal "and the last line refuses" "gate: VERDICT REFUSED (status 3)" \
  "$(printf '%s\n' "$gate_out" | tail -n 1)"

# THE LINE EVERYONE ACTUALLY READS, IN THE FORM NO PASSING RUN PRINTS. A suite
# cannot fail on purpose to demonstrate its own failing summary, so the branch
# that has to be right is the one every green run skips — and it went in
# untested for exactly that reason. Both suites now end on one shared function
# so the branch is reachable from here, driven from the same file they source
# with the counters a failed run would hand it.
tail_fix="$RUN_ROOT/suite-tail"
mkdir -p "$tail_fix"
cp "$ROOT/test/suite-tail.sh" "$tail_fix/suite-tail.sh"
cat > "$tail_fix/run.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
. "$(dirname "$0")/suite-tail.sh"
checks=0 fails=0 unknowns=0
pass() { checks=$((checks + 1)); }
fail() { checks=$((checks + 1)); fails=$((fails + 1)); }
unknown() { unknowns=$((unknowns + 1)); suite_unknown "$1" "$2"; }
pass; pass; pass
case "${1:-}" in
  red) fail; fail ;;
  unknown) unknown 'a claim this host cannot settle' 'the substrate answers neither way' ;;
esac
clause="${2:-}"
[ -n "$clause" ] || clause="$(suite_unknown_clause "$unknowns")"
suite_tail "$checks" "$fails" 7 "$clause"
SH
chmod +x "$tail_fix/run.sh"
equal "a suite whose checks all passed ends on a bare count" \
  "3 checks in 7s" "$("$tail_fix/run.sh")"
equal "and a suite that accumulated failures names them in that same line" \
  "5 checks, 2 FAIL in 7s" "$("$tail_fix/run.sh" red)"
equal "with the e2e lane's trailing clause after the verdict, not instead of it" \
  "5 checks, 2 FAIL in 7s against harness-x" \
  "$("$tail_fix/run.sh" red "against harness-x")"

# AND THE THIRD COLUMN, WHICH IS NOT A COLUMN. A claim the host could not settle
# is neither proved nor refuted, so it must not move the check count in either
# direction — a run that quietly counted it as a pass would report coverage it
# never had. On this host the suite's own unknown branch is unreachable (the
# tmux here returns raw user-option bytes, so the claim IS settled), which is
# exactly why the branch is driven from a fixture rather than left to a run that
# happens to be standing somewhere else.
equal "a claim the run could not settle is named without being counted either way" \
  "?    a claim this host cannot settle
       the substrate answers neither way
3 checks in 7s (1 unknown — see the ? lines above)" \
  "$("$tail_fix/run.sh" unknown)"

# AND THE BRANCH A WEDGED RUN TAKES, which is the one no run of any colour
# reaches while every barrier it waits on is answered. A barrier that expires is
# neither a failed assertion nor a slow box: it is a channel this suite waited
# on that nothing ever signalled, and before the wait was bounded it printed
# nothing at all, because the run never got as far as its own summary. The shim
# writes the channel down precisely so a fixture in a dying pane still leaves a
# record, so the record is what is driven here.
tail_ledger="$tail_fix/wedged"
: > "$tail_ledger"
equal "an empty barrier ledger says nothing at all" "" \
  "$(. "$ROOT/test/suite-tail.sh"; suite_wedged_barriers "$tail_ledger")"
equal "and a ledger that was never written says nothing either" "" \
  "$(. "$ROOT/test/suite-tail.sh"; suite_wedged_barriers "$tail_fix/absent")"
printf 'wait-for test-channel-nobody-signalled\t120\n' > "$tail_ledger"
tail_wedged_out="$(. "$ROOT/test/suite-tail.sh"; suite_wedged_barriers "$tail_ledger")"
contains "a recorded expiry is named rather than left to a process list" \
  "$tail_wedged_out" "BARRIER(S) NEVER SIGNALLED"
contains "and the channel it waited on is in that report" \
  "$tail_wedged_out" "test-channel-nobody-signalled"
contains "and how long it waited, so a ceiling is not mistaken for a slow box" \
  "$tail_wedged_out" "waited 120s"

# AND BOTH SUITES REACH IT THROUGH THAT FILE. A private copy of the summary in
# either one is a copy the three checks above do not cover, which is how this
# branch went unexercised in the first place.
for tail_suite in integration.sh e2e.sh; do
  if grep -q '^\. "\$ROOT/test/suite-tail\.sh"$' "$ROOT/test/$tail_suite" \
    && grep -q 'suite_tail "\$checks" "\$fails" "\$SECONDS"' "$ROOT/test/$tail_suite"; then
    pass "test/$tail_suite ends through the shared summary rather than a copy"
  else
    fail "test/$tail_suite ends through the shared summary rather than a copy" \
      "it does not both source test/suite-tail.sh and end on suite_tail"
  fi
done

# THE SERVERS A RUN STARTS DO NOT OUTLIVE IT. A tmux server daemonises, so the
# only thing that ends one is something that goes looking for it. A teardown
# written as an EXIT trap over a list of sockets does neither when the run is
# killed outright, and covers only the sockets on that list when it does run.
# Both gaps leave a server holding a pty for as long as the host is up, and
# neither is visible from a green run: the run passes, and the survivors are
# found days later in a process list.
#
# The stand-in below is a run of that shape — a private root, a server on the
# root's own socket, a second under a nested root the named teardown never
# mentions, and a third on a `-L` label outside the root entirely. It is ended
# three ways: an early exit, a signal its trap can answer, and a SIGKILL no
# trap can. Each ending is asked the same question.
#
# WHAT COUNTS AS GONE IS READ FROM THE KERNEL, not from the socket file. A run
# that removed its root while a server was still bound inside it leaves a
# server whose socket path no longer exists, and that answers every `tmux -S`
# probe exactly as a server that is not running does.
reaper_live_servers() { # $1 run root, $2 socket outside it -> one server pid per line
  local root="$1" outside="${2:-}" proc comm link inode path
  local -A bound=()
  # The bound path is the last field and may hold a space, so what is read here
  # is the remainder of the line rather than one word of it. Reading it either
  # way would make this instrument agree with a reaper that has the same fault.
  while read -r inode path; do bound["$inode"]="$path"; done \
    < <(awk 'FNR > 1 && NF >= 8 {
          inode = $7
          for (i = 1; i <= 7; i++) { sub(/^[^ \t]+[ \t]+/, "") }
          print inode, $0
        }' /proc/net/unix)
  for proc in /proc/[0-9]*; do
    { read -r comm < "$proc/comm"; } 2>/dev/null || continue
    [ "$comm" = "tmux: server" ] || continue
    while IFS= read -r link; do
      inode="${link#socket:[}"
      path="${bound[${inode%]}]:-}"
      [ -n "$path" ] || continue
      if [ "$path" != "${path#"$root"/}" ] \
        || { [ -n "$outside" ] && [ "$path" = "$outside" ]; }; then
        printf '%s\n' "${proc#/proc/}"
        break
      fi
    done < <(find "$proc/fd" -maxdepth 1 -type l -printf '%l\n' 2>/dev/null \
      | grep '^socket:\[')
  done | sort -u
}

reaper_wait_host_identity() { # $1 host pid, $2 start time -> return when it is gone
  "$(suite_reaper_python)" - "$1" "$2" <<'PY'
import errno
import os
import select
import sys

pid = int(sys.argv[1])
expected = sys.argv[2]
try:
    handle = os.pidfd_open(pid)
except OSError as error:
    if error.errno == errno.ESRCH:
        sys.exit(0)
    raise
try:
    with open("/proc/%d/stat" % pid, encoding="utf-8", errors="replace") as stream:
        stat = stream.read()
    if stat[stat.rindex(")") + 1:].split()[19] != expected:
        sys.exit(0)
    select.select([handle], [], [], None)
finally:
    os.close(handle)
PY
}

reaper_fix="$RUN_ROOT/suite-reaper"
mkdir -p "$reaper_fix"
reaper_label="gangline-reaper-outside-$$"
reaper_label_socket="/tmp/tmux-$(id -u)/$reaper_label"
# That label resolves under the host's own tmux directory rather than any run
# root, so this run adopts it as well: if the stand-in's reaper never fires,
# this run's does, and the label server does not become the leak under test.
suite_reaper_track "$reaper_label_socket"
# THIS FRAGMENT RUNS BESIDE THE SUITE, NOT AFTER IT, so the run's own tmux
# server may not exist yet when the barriers below are first used: the part that
# creates it is running in another process at the same time. A barrier on a
# server this fragment starts itself is up before it is waited on. It goes under
# the run root like everything else here, so the run's teardown reaps it.
reaper_barrier_root="$reaper_fix/barrier"
mkdir -p "$reaper_barrier_root"
reaper_barrier_socket="$reaper_barrier_root/tmux-$(id -u)/default"
TMUX_TMPDIR="$reaper_barrier_root" tmux new-session -d -s reaper-barrier -n idle \
  "PS1='> ' bash --norc"
equal "the reaper fixtures have a barrier server of their own" \
  "reaper-barrier" \
  "$(tmux -S "$reaper_barrier_socket" list-sessions -F '#{session_name}')"
cat > "$reaper_fix/run.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
unset TMUX TMUX_PANE
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
reaper_root="\$1"
reaper_mode="\$2"
mkdir -p "\$reaper_root/nested"
suite_reaper_start "\$reaper_root"
suite_reaper_track "$reaper_label_socket"
# The teardown a run of this shape writes by hand: the one socket it knows the
# name of, and the sweep that covers the rest of what it started.
cleanup() {
  tmux -S "\$reaper_root/tmux-\$(id -u)/default" kill-server 2>/dev/null || true
  suite_reaper_sweep "\$reaper_root"
}
on_signal() { trap - HUP INT TERM; exit "\$((128 + \$1))"; }
trap cleanup EXIT
trap 'on_signal 1' HUP
trap 'on_signal 2' INT
trap 'on_signal 15' TERM
TMUX_TMPDIR="\$reaper_root" tmux new-session -d -s reaper-primary -n seed \\
  "PS1='> ' bash --norc"
TMUX_TMPDIR="\$reaper_root/nested" tmux new-session -d -s reaper-nested -n seed \\
  "PS1='> ' bash --norc"
env -u TMUX_TMPDIR tmux -L "$reaper_label" new-session -d -s reaper-outside \\
  -n seed "PS1='> ' bash --norc"
mkfifo "\$reaper_root/hold"
tmux -S "$reaper_barrier_socket" wait-for -S "\$reaper_mode-reaper-up"
# The fifo is how this run is held still while its servers are counted; the
# caller either releases it or kills this process while it waits.
read -r _ < "\$reaper_root/hold" || true
# An early exit is a run that stops in the middle of its own work, which is
# what a failed check under set -e does.
[ "\$reaper_mode" = early ] && false
exit 0
SH
chmod +x "$reaper_fix/run.sh"

for reaper_case in early term kill; do
  reaper_root="$reaper_fix/$reaper_case"
  SUITE_REAPER_DONE_SOCKET="$reaper_barrier_socket" \
    SUITE_REAPER_DONE_CHANNEL="$reaper_case-reaper-done" \
    SUITE_REAPER_TMUX="$REAL_TMUX" \
    "$reaper_fix/run.sh" "$reaper_root" "$reaper_case" &
  reaper_pid=$!
  tmux -S "$reaper_barrier_socket" wait-for "$reaper_case-reaper-up"
  reaper_before="$(reaper_live_servers "$reaper_root" "$reaper_label_socket" | wc -l | tr -d ' ')"
  equal "the stand-in run starts three servers on three sockets to lose" \
    "3" "$reaper_before"
  case "$reaper_case" in
    early) : > "$reaper_root/hold" ;;
    term) kill -TERM "$reaper_pid" 2>/dev/null || true ;;
    kill) kill -KILL "$reaper_pid" 2>/dev/null || true ;;
  esac
  wait "$reaper_pid" 2>/dev/null || true
  tmux -S "$reaper_barrier_socket" wait-for "$reaper_case-reaper-done"
  reaper_after="$(reaper_live_servers "$reaper_root" "$reaper_label_socket")"
  case "$reaper_case" in
    early) reaper_ending="a run that stops in the middle of its own work" ;;
    term) reaper_ending="a run stopped by a signal its trap can answer" ;;
    kill) reaper_ending="a run killed outright, where no trap runs at all" ;;
  esac
  equal "$reaper_ending leaves no tmux server behind" "" "$reaper_after"
  if [ -e "$reaper_root" ]; then
    fail "$reaper_ending leaves no fixture root behind" "$reaper_root is still there"
  else
    pass "$reaper_ending leaves no fixture root behind"
  fi
done

# A NEW SESSION IS STILL IN THE CGroup THAT STARTED IT. The gate gives each
# mandatory step a killable execution boundary, and the harness that invoked
# the gate may give the whole run one too. A watcher left in that boundary is
# killed beside an abruptly ended parent, before it can sweep a server started
# outside the boundary. This fixture gives the stand-in run a transient unit,
# leaves its adopted server beside that unit, and waits for the exact watcher
# process to be gone before reading the server. That makes the result an
# ordering fact, not a race with the sweep.
reaper_unit_nonce="${SUITE_REAPER_TOKEN:0:16}"
reaper_cgroup_probe="gangline-reaper-probe-$reaper_unit_nonce"
reaper_cgroup_available=0
systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
  --unit="$reaper_cgroup_probe" /bin/true >/dev/null 2>&1 \
  && reaper_cgroup_available=1
if [ "$reaper_cgroup_available" -eq 1 ]; then
  reaper_cgroup_root="$reaper_fix/cgroup-parent"
  reaper_cgroup_label="gangline-reaper-cgroup-$reaper_unit_nonce"
  reaper_cgroup_socket="/tmp/tmux-$(id -u)/$reaper_cgroup_label"
  reaper_cgroup_unit="gangline-reaper-parent-$reaper_unit_nonce"
  suite_reaper_track "$reaper_cgroup_socket"
  env -u TMUX_TMPDIR tmux -L "$reaper_cgroup_label" new-session -d \
    -s reaper-cgroup -n seed "PS1='> ' bash --norc"
  equal "the cgroup fixture starts its adopted server outside the parent boundary" \
    "1" "$(reaper_live_servers "$reaper_cgroup_root" "$reaper_cgroup_socket" | wc -l | tr -d ' ')"
  cat > "$reaper_fix/cgroup-parent.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
suite_reaper_start "$reaper_cgroup_root"
suite_reaper_track "$reaper_cgroup_socket"
"$(suite_reaper_python)" - "$reaper_cgroup_root" \
  > "$reaper_fix/cgroup-watcher" <<'PY'
import os
import sys

root = os.fsencode(sys.argv[1])
matches = []
for entry in os.listdir("/proc"):
    if not entry.isdigit():
        continue
    try:
        with open("/proc/%s/cmdline" % entry, "rb") as stream:
            argv = stream.read().split(b"\0")
        if b"--watch" not in argv or root not in argv:
            continue
        with open("/proc/%s/stat" % entry, encoding="utf-8", errors="replace") as stream:
            stat = stream.read()
        started = stat[stat.rindex(")") + 1:].split()[19]
        matches.append((int(entry), started))
    except (OSError, ValueError, IndexError):
        continue
if len(matches) != 1:
    sys.stderr.write("expected one watcher for %r, found %r\n" % (sys.argv[1], matches))
    sys.exit(1)
sys.stdout.write("%s %s\n" % matches[0])
PY
tmux -S "$reaper_barrier_socket" wait-for -S reaper-cgroup-up
tmux -S "$reaper_barrier_socket" wait-for reaper-cgroup-hold
kill -KILL "\$\$"
SH
  chmod +x "$reaper_fix/cgroup-parent.sh"
  systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
    --unit="$reaper_cgroup_unit" "$reaper_fix/cgroup-parent.sh" &
  reaper_cgroup_client=$!
  tmux -S "$reaper_barrier_socket" wait-for reaper-cgroup-up
  read -r reaper_cgroup_watcher reaper_cgroup_started \
    < "$reaper_fix/cgroup-watcher"
  tmux -S "$reaper_barrier_socket" wait-for -S reaper-cgroup-hold
  reaper_cgroup_rc=0
  wait "$reaper_cgroup_client" || reaper_cgroup_rc=$?
  equal "the cgroup fixture's parent really was killed outright" \
    "255" "$reaper_cgroup_rc"
  reaper_cgroup_wait_rc=0
  reaper_wait_host_identity "$reaper_cgroup_watcher" "$reaper_cgroup_started" \
    || reaper_cgroup_wait_rc=$?
  equal "the cgroup fixture observes the exact watcher process exit" \
    "0" "$reaper_cgroup_wait_rc"
  equal "a parent execution boundary killed outright leaves no adopted tmux server behind" \
    "" "$(reaper_live_servers "$reaper_cgroup_root" "$reaper_cgroup_socket")"
  if [ -e "$reaper_cgroup_root" ]; then
    fail "and its watcher removes the fixture root" "$reaper_cgroup_root is still there"
  else
    pass "and its watcher removes the fixture root"
  fi
else
  unknown "a watcher survives the cgroup that ends its parent" \
    "this host has no reachable systemd user manager for an isolated fixture"
fi

# A CHILD PID NAMESPACE DIES WITH ITS INIT. Starting the watcher inside that
# namespace therefore gives it the same fate as the parent even after setsid.
# The adopted server below starts outside the child namespace, as a server a
# host-side fixture or an already-running tmux server does. The readiness log
# records both namespace and cgroup identities before the parent is released;
# the done barrier then proves that this exact watcher survived PID 1's death
# long enough to sweep.
reaper_namespace_probe="gangline-reaper-namespace-probe-$reaper_unit_nonce"
reaper_namespace_available=0
systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
  --unit="$reaper_namespace_probe" /usr/bin/unshare -Urpf --mount-proc \
  /bin/true >/dev/null 2>&1 && reaper_namespace_available=1
if [ "$reaper_namespace_available" -eq 1 ]; then
  reaper_namespace_root="$reaper_fix/namespaced-parent"
  reaper_namespace_label="gangline-reaper-namespace-$reaper_unit_nonce"
  reaper_namespace_socket="/tmp/tmux-$(id -u)/$reaper_namespace_label"
  reaper_namespace_unit="gangline-reaper-namespace-parent-$reaper_unit_nonce"
  reaper_namespace_log="$reaper_fix/namespace-watch.log"
  suite_reaper_track "$reaper_namespace_socket"
  env -u TMUX_TMPDIR tmux -L "$reaper_namespace_label" new-session -d \
    -s reaper-namespace -n seed "PS1='> ' bash --norc"
  cat > "$reaper_fix/namespace-parent.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
export SUITE_REAPER_DONE_SOCKET="$reaper_barrier_socket"
export SUITE_REAPER_DONE_CHANNEL=reaper-namespace-done
export SUITE_REAPER_TMUX="$REAL_TMUX"
export SUITE_REAPER_LOG="$reaper_namespace_log"
suite_reaper_start "$reaper_namespace_root"
suite_reaper_track "$reaper_namespace_socket"
tmux -S "$reaper_barrier_socket" wait-for -S reaper-namespace-up
tmux -S "$reaper_barrier_socket" wait-for reaper-namespace-hold
kill -KILL "\$\$"
SH
  chmod +x "$reaper_fix/namespace-parent.sh"
  systemd-run --user --quiet --wait --pipe --collect --service-type=exec \
    --unit="$reaper_namespace_unit" /usr/bin/unshare -Urpf --mount-proc \
    "$reaper_fix/namespace-parent.sh" &
  reaper_namespace_client=$!
  tmux -S "$reaper_barrier_socket" wait-for reaper-namespace-up
  reaper_namespace_armed="$(awk -F '\t' '$2 == "armed" { print; exit }' \
    "$reaper_namespace_log")"
  reaper_parent_cgroup="$(sed -n 's/.*\tparent-cgroup=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  reaper_watcher_cgroup="$(sed -n 's/.*\twatcher-cgroup=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  reaper_parent_pidns="$(sed -n 's/.*\tparent-pidns=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  reaper_watcher_pidns="$(sed -n 's/.*\twatcher-pidns=\([^\t]*\).*/\1/p' \
    <<<"$reaper_namespace_armed")"
  equal "the namespaced parent's watcher is armed from another cgroup" \
    "different" \
    "$([ -n "$reaper_parent_cgroup" ] && [ "$reaper_parent_cgroup" != "$reaper_watcher_cgroup" ] && printf different || printf same)"
  equal "and from outside the child PID namespace" "different" \
    "$([ -n "$reaper_parent_pidns" ] && [ "$reaper_parent_pidns" != "$reaper_watcher_pidns" ] && printf different || printf same)"
  tmux -S "$reaper_barrier_socket" wait-for -S reaper-namespace-hold
  reaper_namespace_rc=0
  wait "$reaper_namespace_client" || reaper_namespace_rc=$?
  case "$reaper_namespace_rc" in
    0 | 255)
      pass "the namespace fixture reports its forcibly ended init"
      ;;
    *)
      fail "the namespace fixture reports its forcibly ended init" \
        "systemd-run exited $reaper_namespace_rc, expected 0 or 255"
      ;;
  esac
  tmux -S "$reaper_barrier_socket" wait-for reaper-namespace-done
  equal "a child PID namespace ending leaves no adopted tmux server behind" \
    "" "$(reaper_live_servers "$reaper_namespace_root" "$reaper_namespace_socket")"
  if [ -e "$reaper_namespace_root" ]; then
    fail "and its outside watcher removes the fixture root" \
      "$reaper_namespace_root is still there"
  else
    pass "and its outside watcher removes the fixture root"
  fi
else
  unknown "a watcher survives the PID namespace that ends its parent" \
    "this host cannot create the isolated user and PID namespace fixture"
fi

# WHAT A RUN OWNS IS WHAT IT WROTE DOWN. Selection is a prefix of the socket's
# bound path, so a directory argument alone would let one unset variable reach
# the team's own socket. A directory is swept only while it carries the marker a
# run wrote into it.
reaper_unclaimed_root="$reaper_fix/unclaimed"
mkdir -p "$reaper_unclaimed_root"
TMUX_TMPDIR="$reaper_unclaimed_root" tmux new-session -d -s reaper-unclaimed \
  -n seed "PS1='> ' bash --norc"
equal "a server stands in a directory no run has claimed" \
  "1" "$(reaper_live_servers "$reaper_unclaimed_root" "" | wc -l | tr -d ' ')"
suite_reaper_sweep "$reaper_unclaimed_root"
equal "sweeping an unclaimed directory leaves its server standing" \
  "1" "$(reaper_live_servers "$reaper_unclaimed_root" "" | wc -l | tr -d ' ')"
if [ -e "$reaper_unclaimed_root/.suite-reaper" ]; then
  fail "and writes nothing into it" "a marker appeared"
else
  pass "and writes nothing into it"
fi
tmux -S "$reaper_unclaimed_root/tmux-$(id -u)/default" kill-server 2>/dev/null || true

reaper_claimed_root="$reaper_fix/claimed"
mkdir -p "$reaper_claimed_root"
suite_reaper_claim "$reaper_claimed_root" > /dev/null
TMUX_TMPDIR="$reaper_claimed_root" tmux new-session -d -s reaper-claimed \
  -n seed "PS1='> ' bash --norc"
equal "the same sweep ends a server under a directory the run did claim" \
  "" "$(suite_reaper_sweep "$reaper_claimed_root"
        reaper_live_servers "$reaper_claimed_root" "")"

# A CLAIM IS A PROMISE THAT THE DIRECTORY HOLDS NOTHING BUT THIS RUN'S WORK,
# because a sweep ends what is bound inside it and then removes it. A directory
# that already holds someone else's tmux server says otherwise, and so does one
# that contains the directory tmux puts this user's sockets in by default —
# which is where the team's own server lives, and which an unset TMPDIR would
# hand straight to a run root. The protected directory is a fixture here, so
# this drives the refusal without writing into the live one; the live one is
# only read.
if [ -e "/tmp/tmux-$(id -u)/.suite-reaper" ]; then
  fail "the host's own tmux directory carries no run's marker" \
    "/tmp/tmux-$(id -u)/.suite-reaper exists"
else
  pass "the host's own tmux directory carries no run's marker"
fi
reaper_stand_in_host="$reaper_fix/stand-in-host/tmux-$(id -u)"
mkdir -p "$reaper_stand_in_host"
ln -s "$reaper_stand_in_host" "$reaper_fix/stand-in-link"
for reaper_shared in \
  "protects|$reaper_stand_in_host" \
  "protects|${reaper_stand_in_host%/*}" \
  "spelling|$reaper_stand_in_host/." \
  "spelling|$reaper_stand_in_host//" \
  "spelling|${reaper_stand_in_host%/*}/./tmux-$(id -u)" \
  "spelling|$reaper_fix/stand-in-link"
do
  reaper_shared_why="${reaper_shared%%|*}"
  reaper_shared_path="${reaper_shared#*|}"
  reaper_shared_rc=0
  reaper_shared_said="$(
    export SUITE_REAPER_HOST_TMUX="$reaper_stand_in_host"
    suite_reaper_claim "$reaper_shared_path" 2>&1)" || reaper_shared_rc=$?
  equal "claiming [$reaper_shared_path] is refused" "2" "$reaper_shared_rc"
  case "$reaper_shared_why" in
    protects) reaper_shared_needle="holds this host's own tmux sockets" ;;
    *) reaper_shared_needle="does not name a run root" ;;
  esac
  contains "and [$reaper_shared_path] is refused by name" \
    "$reaper_shared_said" "$reaper_shared_needle"
  if [ -e "$reaper_stand_in_host/.suite-reaper" ]; then
    fail "and [$reaper_shared_path] writes no marker into the protected directory" \
      "a marker appeared"
    rm -f "$reaper_stand_in_host/.suite-reaper"
  else
    pass "and [$reaper_shared_path] writes no marker into the protected directory"
  fi
done
reaper_occupied_root="$reaper_fix/occupied"
mkdir -p "$reaper_occupied_root"
TMUX_TMPDIR="$reaper_occupied_root" tmux new-session -d -s reaper-occupied \
  -n seed "PS1='> ' bash --norc"
reaper_occupied_rc=0
reaper_occupied_said="$(suite_reaper_claim "$reaper_occupied_root" 2>&1)" \
  || reaper_occupied_rc=$?
equal "claiming a directory that already holds a server is refused" \
  "2" "$reaper_occupied_rc"
contains "and the refusal says a server is bound under it" \
  "$reaper_occupied_said" "already bound under it"
equal "so that server is still standing" \
  "1" "$(reaper_live_servers "$reaper_occupied_root" "" | wc -l | tr -d ' ')"
tmux -S "$reaper_occupied_root/tmux-$(id -u)/default" kill-server 2>/dev/null || true

# A PATH IS NOT AN IDENTITY: a directory can be removed and remade at the same
# path by a later run. A watcher carries the token it wrote, so it declines the
# successor rather than reaping a run it was never armed for.
reaper_gen_root="$reaper_fix/generation"
mkdir -p "$reaper_gen_root"
reaper_gen_first="$(suite_reaper_claim "$reaper_gen_root")"
reaper_gen_second="$(suite_reaper_claim "$reaper_gen_root")"
if [ -n "$reaper_gen_first" ] && [ "$reaper_gen_first" != "$reaper_gen_second" ]; then
  pass "each claim of the same directory writes a different token"
else
  fail "each claim of the same directory writes a different token" \
    "got [$reaper_gen_first] then [$reaper_gen_second]"
fi
TMUX_TMPDIR="$reaper_gen_root" tmux new-session -d -s reaper-generation \
  -n seed "PS1='> ' bash --norc"
"$(suite_reaper_python)" "$(suite_reaper_program)" \
  --sweep "$reaper_gen_root" "$reaper_gen_first"
equal "a sweep holding the earlier run's token leaves the later run standing" \
  "1" "$(reaper_live_servers "$reaper_gen_root" "" | wc -l | tr -d ' ')"
"$(suite_reaper_python)" "$(suite_reaper_program)" \
  --sweep "$reaper_gen_root" "$reaper_gen_second"
equal "the token the directory actually carries ends it" \
  "" "$(reaper_live_servers "$reaper_gen_root" "")"

# TMPDIR IS THE CALLER'S TO CHOOSE, so a run root can hold a space. The kernel
# writes the bound path as the last field of its socket table and a plain split
# would cut it at that space, losing the server rather than ending it.
reaper_space_root="$reaper_fix/holds a space"
mkdir -p "$reaper_space_root"
suite_reaper_claim "$reaper_space_root" > /dev/null
TMUX_TMPDIR="$reaper_space_root" tmux new-session -d -s reaper-space \
  -n seed "PS1='> ' bash --norc"
equal "a server under a root holding a space is there to be found" \
  "1" "$(reaper_live_servers "$reaper_space_root" "" | wc -l | tr -d ' ')"
suite_reaper_sweep "$reaper_space_root"
equal "and the teardown reaches it" \
  "" "$(reaper_live_servers "$reaper_space_root" "")"

# A NEIGHBOUR'S NAME IS NOT THIS RUN'S TO ENCODE. Finding the servers a root
# holds means reading a name for every process on the host, and those names
# belong to whoever started them. This suite builds fixtures out of raw bytes
# and several runs share a box, so a process whose name is not valid UTF-8 is
# an ordinary neighbour, not a fault. It must not decide whether this run may
# start: claim reads the same names, and the suites exit on a claim that fails.
#
# THE NEIGHBOUR CANNOT OUTLIVE THIS RUN. A process holding a name like that
# refuses every other run on the host until it goes, so it is arranged to die
# of this shell's death rather than of this shell's teardown. The write end is
# opened read-write before the copy starts, which never blocks and means the
# copy's own open cannot block either: from then on the only thing keeping it
# alive is a descriptor that closes when this process does, however it goes.
# Every process started while it is open is started with it closed instead.
# Inherited, it is a writer on the copy's own stdin, so the close below would
# leave the copy reading a pipe nothing can ever end; a tmux server inherits
# it the same way and outlives the run holding it.
reaper_neighbour="$reaper_fix/$(printf 'nm\377x')"
reaper_neighbour_in="$reaper_fix/neighbour-in"
reaper_neighbour_out="$reaper_fix/neighbour-out"
cp "$(command -v cat)" "$reaper_neighbour"
mkfifo "$reaper_neighbour_in" "$reaper_neighbour_out"
exec 7<> "$reaper_neighbour_in"
"$reaper_neighbour" < "$reaper_neighbour_in" > "$reaper_neighbour_out" 7>&- &
reaper_neighbour_pid=$!
printf 'up\n' >&7
# The copy cannot answer before it has been exec'd, and its name is set by the
# exec, so this line returning is the name being there. Nothing is waited on.
IFS= read -r reaper_neighbour_ack < "$reaper_neighbour_out"
equal "the neighbour answers from under a name of its own" \
  "up" "$reaper_neighbour_ack"
reaper_neighbour_named=no
LC_ALL=C grep -q "$(printf '\377')" "/proc/$reaper_neighbour_pid/comm" \
  && reaper_neighbour_named=yes
equal "and that name really is not UTF-8" "yes" "$reaper_neighbour_named"
reaper_neighbour_root="$reaper_fix/beside-a-neighbour"
mkdir -p "$reaper_neighbour_root"
reaper_neighbour_rc=0
reaper_neighbour_token="$(suite_reaper_claim "$reaper_neighbour_root" 2>&1)" \
  || reaper_neighbour_rc=$?
equal "a claim beside a name that is not UTF-8 is not refused" \
  "0" "$reaper_neighbour_rc"
case "$reaper_neighbour_token" in
  '' | *[!0-9a-f]*)
    fail "and what it returns is a token rather than a traceback" \
      "got [$reaper_neighbour_token]" ;;
  *) pass "and what it returns is a token rather than a traceback" ;;
esac
TMUX_TMPDIR="$reaper_neighbour_root" tmux new-session -d -s reaper-neighbour \
  -n seed "PS1='> ' bash --norc" 7>&-
equal "a server under that root is still there to be found" \
  "1" "$(reaper_live_servers "$reaper_neighbour_root" "" | wc -l | tr -d ' ')"
suite_reaper_sweep "$reaper_neighbour_root"
equal "and the sweep beside that neighbour still ends it" \
  "" "$(reaper_live_servers "$reaper_neighbour_root" "")"
exec 7>&-
wait "$reaper_neighbour_pid" 2>/dev/null || true
if kill -0 "$reaper_neighbour_pid" 2>/dev/null; then
  kill -KILL "$reaper_neighbour_pid" 2>/dev/null || true
  fail "the neighbour goes when this run stops holding it open" \
    "pid $reaper_neighbour_pid outlived the descriptor"
else
  pass "the neighbour goes when this run stops holding it open"
fi

# A NEWLINE HAS NO REPRESENTATION in the marker, the adopted-socket list, or the
# kernel's own table, so a run root or socket holding one is refused where it
# enters rather than going quietly missing at teardown.
reaper_newline_root="$(printf '%s/holds a\nnewline' "$reaper_fix")"
reaper_newline_rc=0
reaper_newline_said="$(suite_reaper_start "$reaper_newline_root" 2>&1)" \
  || reaper_newline_rc=$?
equal "a run root holding a newline is refused" "1" "$reaper_newline_rc"
contains "and the refusal names it as unreadable" \
  "$reaper_newline_said" "cannot be a run root"
reaper_newline_rc=0
reaper_newline_said="$(suite_reaper_track "$reaper_newline_root/sock" 2>&1)" \
  || reaper_newline_rc=$?
equal "an adopted socket holding a newline is refused too" "1" "$reaper_newline_rc"

# A SOCKET PATH MAY LEGALLY END IN A SPACE. Recording one and trimming it on the
# way back out records one path and looks for another, so the adopted list keeps
# every byte but the separator.
reaper_extra_root="$reaper_fix/adopted"
mkdir -p "$reaper_extra_root"
suite_reaper_claim "$reaper_extra_root" > /dev/null
reaper_extra_socket="$reaper_fix/outside sock "
( export SUITE_REAPER_ROOT="$reaper_extra_root"
  suite_reaper_track "$reaper_extra_socket" )
equal "an adopted socket path keeps its trailing space" \
  "$reaper_extra_socket" \
  "$(sed -n '1s/$//p' "$reaper_extra_root/.suite-reaper-extra")"

# THE WATCH IS EITHER PROVIDED OR REFUSED. It rests on Linux pidfds, a readable
# socket table, and a systemd user manager; on a host without one a watcher would report
# a protection it cannot give, and the run it was meant to cover would leak in
# exactly the way this file exists to stop.
equal "this host can be watched" "0" \
  "$("$(suite_reaper_python)" "$(suite_reaper_program)" --watchable >/dev/null 2>&1; echo $?)"
reaper_nosockets_rc=0
reaper_nosockets_said="$(
  export SUITE_REAPER_UNIX_SOCKETS="$reaper_fix/absent-table"
  suite_reaper_start "$reaper_fix/unwatchable" 2>&1)" || reaper_nosockets_rc=$?
equal "a host whose socket table cannot be read refuses to start a watch" \
  "1" "$reaper_nosockets_rc"
contains "and says which piece is missing" \
  "$reaper_nosockets_said" "cannot be read"
if [ -e "$reaper_fix/unwatchable/.suite-reaper" ]; then
  fail "a refused start claims nothing" "it wrote a marker anyway"
else
  pass "a refused start claims nothing"
fi
reaper_nosystemd_rc=0
reaper_nosystemd_said="$(
  # Emptying the search path is how a host without systemd-run is presented here;
  # it is confined to this subshell, which does nothing else.
  # shellcheck disable=SC2123
  PATH="$reaper_fix/empty-path"
  suite_reaper_start "$reaper_fix/ungrouped" 2>&1)" || reaper_nosystemd_rc=$?
equal "a host without systemd-run refuses too, rather than sharing the run's boundary" \
  "1" "$reaper_nosystemd_rc"
contains "and says so" "$reaper_nosystemd_said" "no systemd-run"

# A ROOT THAT WOULD NOT GO IS NAMED. Removal is best-effort — a teardown carries
# on either way — but a directory that survives one is a fixture the next run
# inherits, and a removal that failed silently reads exactly like one that
# worked. Declining a directory this caller does not own is not that, and says
# nothing.
reaper_watch_stuck_root="$reaper_fix/watcher-will-not-go"
cat > "$reaper_fix/stuck-parent.sh" <<SH
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
. "$ROOT/test/suite-python.sh"
. "$ROOT/test/suite-reaper.sh"
export SUITE_REAPER_DONE_SOCKET="$reaper_barrier_socket"
export SUITE_REAPER_DONE_CHANNEL=reaper-stuck-done
export SUITE_REAPER_TMUX="$REAL_TMUX"
suite_reaper_start "$reaper_watch_stuck_root"
mkdir -p "$reaper_watch_stuck_root/held"
: > "$reaper_watch_stuck_root/held/file"
chmod 500 "$reaper_watch_stuck_root/held"
tmux -S "$reaper_barrier_socket" wait-for -S reaper-stuck-up
tmux -S "$reaper_barrier_socket" wait-for reaper-stuck-hold
SH
chmod +x "$reaper_fix/stuck-parent.sh"
"$reaper_fix/stuck-parent.sh" &
reaper_stuck_parent=$!
tmux -S "$reaper_barrier_socket" wait-for reaper-stuck-up
tmux -S "$reaper_barrier_socket" wait-for -S reaper-stuck-hold
wait "$reaper_stuck_parent"
tmux -S "$reaper_barrier_socket" wait-for reaper-stuck-done
if [ -d "$reaper_watch_stuck_root" ]; then
  pass "a root the detached watcher could not remove survives for inspection"
else
  fail "a root the detached watcher could not remove survives for inspection" \
    "$reaper_watch_stuck_root is gone"
fi
reaper_watch_stuck_said=""
if reaper_watch_stuck_line="$(sed -n '1p' \
    "$reaper_watch_stuck_root/.suite-reaper-watch-error" 2>&1)"; then
  reaper_watch_stuck_said="$reaper_watch_stuck_line"
fi
contains "the detached watcher leaves its removal diagnostic in that root" \
  "$reaper_watch_stuck_said" \
  "$reaper_watch_stuck_root could not be removed and is still there to read"
chmod 700 "$reaper_watch_stuck_root/held"
suite_reaper_claim "$reaper_watch_stuck_root" > /dev/null
equal "the watcher-stuck fixture is removable after its permission is restored" \
  "" "$(suite_reaper_sweep "$reaper_watch_stuck_root" 2>&1)"

reaper_stuck_root="$reaper_fix/will-not-go"
mkdir -p "$reaper_stuck_root/held"
: > "$reaper_stuck_root/held/file"
suite_reaper_claim "$reaper_stuck_root" > /dev/null
chmod 500 "$reaper_stuck_root/held"
reaper_stuck_said="$(suite_reaper_sweep "$reaper_stuck_root" 2>&1)"
contains "a run root that could not be removed is named" \
  "$reaper_stuck_said" "$reaper_stuck_root could not be removed"
contains "and the teardown says the sweep did not come back clean" \
  "$reaper_stuck_said" "exited 4"
if [ -d "$reaper_stuck_root" ]; then
  pass "and it really is still there to read"
else
  fail "and it really is still there to read" "$reaper_stuck_root is gone"
fi
chmod 700 "$reaper_stuck_root/held"
suite_reaper_claim "$reaper_stuck_root" > /dev/null
equal "the same teardown says nothing once the root can go" \
  "" "$(suite_reaper_sweep "$reaper_stuck_root" 2>&1)"
if [ -e "$reaper_stuck_root" ]; then
  fail "and the root is gone" "$reaper_stuck_root is still there"
else
  pass "and the root is gone"
fi
reaper_unowned_root="$reaper_fix/not-ours"
mkdir -p "$reaper_unowned_root"
equal "declining a directory this caller does not own says nothing" \
  "" "$(suite_reaper_sweep "$reaper_unowned_root" 2>&1)"
if [ -d "$reaper_unowned_root" ]; then
  pass "and leaves it exactly as it was"
else
  fail "and leaves it exactly as it was" "$reaper_unowned_root was removed"
fi

# A TEARDOWN TRAPPED DIRECTLY ON A SIGNAL RUNS AND THEN RETURNS to the flow it
# interrupted, so the run carries on with its fixtures deleted and its claim on
# its own directory gone — and every server it starts after that point belongs
# to no run and is swept by nobody. Both suites answer a signal with a handler
# that exits, whatever order the dispositions are written in.
for reaper_trapped in test/integration.sh test/role-briefs.sh; do
  equal "$reaper_trapped binds its teardown to no signal" "" \
    "$(grep -E '^trap .*cleanup.*(HUP|INT|TERM|QUIT)' "$ROOT/$reaper_trapped")"
  for reaper_signal in HUP INT TERM; do
    contains "$reaper_trapped answers $reaper_signal with a handler that exits" \
      "$(grep -E "^trap .*$reaper_signal\$" "$ROOT/$reaper_trapped")" "on_signal"
  done
  contains "$reaper_trapped's handler exits on the signal it was given" \
    "$(sed -n '/^on_signal()/,/^}/p' "$ROOT/$reaper_trapped")" 'exit "$((128 + $1))"'
done
