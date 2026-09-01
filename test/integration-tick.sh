# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Cooperative tick: global retries, copy-mode recovery, native identity, and rails.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# after the ordinary substrate checks and supplies its fixtures and assertions.

tick_original_session="$GANG_SESSION"
tick_original_collars="${GANG_COLLARS:-}"
export GANG_SESSION="gangtick-test-$$"
export GANG_TICK_DEADLINE_SECONDS=60

tick_monotonic_ns() {
  python3 -c 'import time; print(time.monotonic_ns())'
}

tmux new-session -d -s "$GANG_SESSION" -n caller "PS1='❯ ' bash --norc"
"$GANG" adopt caller -c bash >/dev/null
tick_caller_id="$(window_id caller)"
tick_caller_pane="$(tmux list-panes -t "$tick_caller_id" -F '#{pane_id}')"

# The recipient fixture closes the hook-owned turn fact before each prompt
# becomes observable. Actual Stop events establish that fact at the two
# delivery decisions below; keeping the prompt callback immediate lets the
# compressed-clock suite verify Enter without turning native-hook runtime into
# evidence. The FIFO arm is optional and one-shot: it orders the stale occupied
# paint behind the closed fact without a clock or poll.
tick_prompt_arm="$RUN_ROOT/tick-prompt-arm"
tick_prompt_fifo="$RUN_ROOT/tick-prompt-fifo"
tick_prompt_enable="$RUN_ROOT/tick-prompt-enable"
mkfifo "$tick_prompt_fifo"
cat > "$RUN_ROOT/tick-bashrc" <<SH
PS1='❯ '
tick_prompt() {
  [ -e "$tick_prompt_enable" ] || return 0
  tmux set-option -w -t "\$TMUX_PANE" @gl_turn "closed \$(date +%s)"
  tmux wait-for -S "gang-tick-prompt-\${TMUX_PANE#%}"
  if [ -e "$tick_prompt_arm" ]; then
    rm -f -- "$tick_prompt_arm"
    printf x > "$tick_prompt_fifo"
  fi
}
PROMPT_COMMAND=tick_prompt
SH

tick_compacted="$RUN_ROOT/tick-compacted"
mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/tick-native.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="ENV='$RUN_ROOT/tick-bashrc' bash --posix"
GANG_STOP_HOOK=1
GANG_SELF_COMPACT=deferred
GANG_COMPACT_CMD="printf TICK_COMPACT; : > '$tick_compacted'"
SH

tick_false_probe="$RUN_ROOT/tick-false-occupied"
cat > "$RUN_ROOT/collars/tick-false-occupied.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/tick-native.sh"
GANG_OCCUPIED_REGEX='^› [0-9]+\. '
_gl_tick_real_input="\$(declare -f collar_input)"
eval "tick_real_input \${_gl_tick_real_input#collar_input}"
collar_input() {
  if [ -e "$tick_false_probe" ]; then
    case " \${FUNCNAME[*]} " in *' occupied '*) return 1 ;; esac
  fi
  tick_real_input "\$1"
}
SH

"$HITCH" tick-copy -c tick-native -d /tmp >/dev/null
"$HITCH" tick-false -c tick-false-occupied -d /tmp >/dev/null
"$HITCH" tick-mode -c tick-native -d /tmp >/dev/null
tick_copy_id="$(window_id tick-copy)"
tick_copy_pane="$(tmux list-panes -t "$tick_copy_id" -F '#{pane_id}')"
tick_false_id="$(window_id tick-false)"
tick_false_pane="$(tmux list-panes -t "$tick_false_id" -F '#{pane_id}')"
tick_mode_id="$(window_id tick-mode)"
tick_mode_pane="$(tmux list-panes -t "$tick_mode_id" -F '#{pane_id}')"

# Keep the startup prompt free of hook work so hitch can verify its contract,
# then establish one explicit native Stop in each fixture with event barriers.
# Later prompt callbacks close the same hook-owned fact immediately; the false
# paint decision below is stamped again through the real hook endpoint.
: > "$tick_prompt_enable"
tmux wait-for "gang-tick-prompt-${tick_copy_pane#%}" &
tick_copy_prompt_waiter=$!
tmux wait-for "gang-tick-prompt-${tick_false_pane#%}" &
tick_false_prompt_waiter=$!
tmux wait-for "gang-tick-prompt-${tick_mode_pane#%}" &
tick_mode_prompt_waiter=$!
tmux send-keys -t "$tick_copy_id" Enter
tmux send-keys -t "$tick_false_id" Enter
tmux send-keys -t "$tick_mode_id" Enter
wait "$tick_copy_prompt_waiter" "$tick_false_prompt_waiter" "$tick_mode_prompt_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_copy_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_false_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_mode_pane" "$GANG" hook >/dev/null

# A WINDOW GLYPH IS NOT TMUX MODE STATE. The issue arrived with ?name? on the
# window while tmux itself reported pane_in_mode=0. Reproduce the consequential
# race deterministically: a PATH-local tmux returns one stale 1 for the first
# authoritative mode read, then the real zero. Delivery must re-read before it
# refuses, rather than park an idle recipient until some unrelated boundary.
tmux rename-window -t "$tick_mode_id" '?tick-mode?'
equal "the issue-shaped window carries unknown decoration" '?tick-mode?' \
  "$(tmux display-message -p -t "$tick_mode_id" '#{window_name}')"
equal "and tmux says its pane is not in a mode" 0 \
  "$(tmux display-message -p -t "$tick_mode_id" '#{pane_in_mode}')"
tick_mode_bin="$RUN_ROOT/tick-mode-bin"
tick_mode_once="$RUN_ROOT/tick-mode-once"
tick_mode_ledger="$RUN_ROOT/tick-mode-ledger"
tick_mode_delivered="$RUN_ROOT/tick-mode-delivered"
tick_real_tmux="$(command -v tmux)"
mkdir -p "$tick_mode_bin"
cat > "$tick_mode_bin/tmux" <<SH
#!/usr/bin/env bash
if [ "\${1:-}" = display-message ] && [ "\${*: -1}" = '#{pane_in_mode}' ]; then
  if [ ! -e '$tick_mode_once' ]; then
    : > '$tick_mode_once'
    printf '1\n' >> '$tick_mode_ledger'
    printf '1\n'
    exit 0
  fi
  mode="\$('$tick_real_tmux' "\$@")" || exit \$?
  printf '%s\n' "\$mode" >> '$tick_mode_ledger'
  printf '%s\n' "\$mode"
  exit 0
fi
exec '$tick_real_tmux' "\$@"
SH
chmod +x "$tick_mode_bin/tmux"
tick_mode_out="$(printf ": > '%s'" "$tick_mode_delivered" |
  PATH="$tick_mode_bin:$PATH" GANG_TEST_TICK_MODE=sync \
  "$GANG" send --to tick-mode --from tester --stdin 2>&1)"
excludes "a stale mode sample is re-read before refusing the decorated window" \
  "$tick_mode_out" "tmux mode owns"
equal "the decision consumed the stale one and the current zero" '1 0 ' \
  "$(awk 'NR <= 2 { printf "%s ", $0 }' "$tick_mode_ledger")"
equal "the same invocation reaches the idle recipient" present \
  "$([ -e "$tick_mode_delivered" ] && printf present || printf absent)"
equal "and leaves no delivery waiting for an idle turn" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-mode" { print $4 }')"

# Copy-mode owns tmux's key table, so both actions remain live state and type
# nothing. No later recipient boundary is raised between this refusal and the
# cross-window command that supplies the cooperative tick.
tmux copy-mode -t "$tick_copy_id"
printf 'TICK_COPY_MESSAGE' \
  | "$GANG" send --to tick-copy --from tester --stdin >/dev/null
TMUX_PANE="$tick_copy_pane" "$GANG" compact >/dev/null
equal "copy-mode leaves the peer message parked" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-copy" { print $4 }')"
contains "copy-mode leaves the self-compaction request standing" \
  "$("$GANG" status tick-copy)" "self-compaction requested"
equal "neither copy-mode action typed before a later invocation" absent \
  "$([ ! -e "$tick_compacted" ] && printf absent || printf present)"

# A readable native Stop is stronger than a stale numbered menu line during a
# tick. The one-shot collar probe makes the ordinary send take the old painted
# occupied answer, then remains armed so removing the hook preference turns the
# tick red by taking that same false answer again.
: > "$tick_prompt_arm"
tmux send-keys -l -t "$tick_false_id" "printf '› 1. stale occupied transcript line\\n'"
tmux send-keys -t "$tick_false_id" Enter
IFS= read -r -N 1 _ < "$tick_prompt_fifo"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_false_pane" "$GANG" hook >/dev/null
: > "$tick_false_probe"
printf 'TICK_FALSE_OCCUPIED_MESSAGE' \
  | "$GANG" send --to tick-false --from tester --stdin >/dev/null
equal "the painted false occupied reading parks before the cooperative pass" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-false" { print $4 }')"

tmux send-keys -t "$tick_copy_id" -X cancel
tick_cross_rc=0
TMUX_PANE="$tick_caller_pane" GANG_TEST_TICK_MODE=sync \
  "$GANG" whoami >/dev/null || tick_cross_rc=$?
equal "a command from another window keeps its own successful result" 0 "$tick_cross_rc"
equal "that command's tick drains the copy-mode message" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-copy" { print $4 }')"
# source-guard: whole-surface@2b9b8301b2d3: the nonce-marked peer body is unique to this test and verified delivery may render it anywhere in the recipient transcript
contains "the copy-mode message reached the recipient without its own new boundary" \
  "$(pane_all tick-copy)" "TICK_COPY_MESSAGE"
equal "the same tick submits the self-compaction after the delivered turn closes" present \
  "$([ -e "$tick_compacted" ] && printf present || printf absent)"
excludes "the completed self-compaction no longer reads as pending" \
  "$("$GANG" status tick-copy)" "self-compaction requested"
equal "one global pass also drains the other hitched window" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-false" { print $4 }')"
# source-guard: whole-surface@87a1cbbad112: the nonce-marked peer body is unique to this test and any transcript rendering proves the target consumed it
contains "the closed native turn beats false occupied paint for delivery" \
  "$(pane_all tick-false)" "TICK_FALSE_OCCUPIED_MESSAGE"
equal "the completed pass retires every tick delivery owner marker" absent \
  "$(if [ -n "$(tmux show-options -wqv -t "$tick_copy_id" @gl_tick_delivery)$(tmux show-options -wqv -t "$tick_false_id" @gl_tick_delivery)" ]; then printf present; else printf absent; fi)"

# A live holder is dirtied, not joined or piled up. FIFO edges make the exact
# crossing deterministic: the contender runs only after the holder owns its
# symlink and the holder cannot finish its first pass until released.
tick_ready_fifo="$RUN_ROOT/tick-ready"
tick_release_fifo="$RUN_ROOT/tick-release"
tick_ledger="$RUN_ROOT/tick-ledger"
mkfifo "$tick_ready_fifo" "$tick_release_fifo"
GANG_TEST_TICK_READY_FIFO="$tick_ready_fifo" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_release_fifo" \
GANG_TEST_TICK_LEDGER="$tick_ledger" \
  "$GANG" tick > "$RUN_ROOT/tick-owner.out" 2> "$RUN_ROOT/tick-owner.err" &
tick_owner_pid=$!
IFS= read -r -N 1 _ < "$tick_ready_fifo"
tick_lock_path="$(find "$GANG_LOCK_DIR/tick" -maxdepth 1 -type l -name '*.lock' -print)"
"$GANG" tick >/dev/null
equal "a concurrent candidate exits after touching one dirty marker" 1 \
  "$(find "$GANG_LOCK_DIR/tick" -maxdepth 1 -type f -name '*.dirty' | wc -l | tr -d ' ')"
printf '\n' > "$tick_release_fifo"
wait "$tick_owner_pid"
equal "the singleton consumes the dirty edge with exactly one rerun" "1 2 " \
  "$(tr '\n' ' ' < "$tick_ledger")"
equal "the completed singleton leaves no lock or dirty residue" 0 \
  "$(find "$GANG_LOCK_DIR/tick" -maxdepth 1 \( -type l -name '*.lock' -o -type f -name '*.dirty' \) | wc -l | tr -d ' ')"

# A LIVE TICK OWNER MAY RELEASE AFTER -L BUT BEFORE READLINK. The shim is the
# exact seam: tick_lock_acquire made its own failed ln and successful -L
# observation before invoking this external readlink. The internal worker
# keeps an expected lock fault off the health and alerts surfaces under test.
tick_lock_race_bin="$RUN_ROOT/tick-lock-race-bin"
tick_lock_race_seen="$RUN_ROOT/tick-lock-race-seen"
mkdir -p "$tick_lock_race_bin"
{
  printf '#!/bin/sh\n'
  printf 'REAL=%q\n' "$(command -v readlink)"
  printf 'LOCK=%q\n' "$tick_lock_path"
  printf 'SEEN=%q\n' "$tick_lock_race_seen"
  cat <<'SH'
if [ "${1:-}" = "$LOCK" ] && [ ! -e "$SEEN" ]; then
  : > "$SEEN"
  rm -f -- "$LOCK"
fi
exec "$REAL" "$@"
SH
} > "$tick_lock_race_bin/readlink"
chmod +x "$tick_lock_race_bin/readlink"
ln -s "$$" "$tick_lock_path"
tick_lock_race_rc=0
GANG_TICK_INTERNAL=1 PATH="$tick_lock_race_bin:$PATH" \
  "$GANG" __tick-worker > "$RUN_ROOT/tick-lock-race.out" 2>&1 \
  || tick_lock_race_rc=$?
equal "a released live tick lock is retried atomically" 0 "$tick_lock_race_rc"
equal "the tick owner released after the symlink observation" present \
  "$([ -e "$tick_lock_race_seen" ] && printf present || printf absent)"
equal "the retried tick worker releases its lock" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"

# A PERSISTENT UNREADABLE OWNER REMAINS A FAULT. The vanished-owner retry must
# not turn an actually malformed symlink into contention that resolved.
ln -s not-a-pid "$tick_lock_path"
tick_bad_lock_rc=0
GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
  > "$RUN_ROOT/tick-bad-lock.out" 2>&1 || tick_bad_lock_rc=$?
equal "a present nonnumeric tick lock stays fail-closed" 1 "$tick_bad_lock_rc"
contains "a present nonnumeric tick lock names its unreadable owner" \
  "$(<"$RUN_ROOT/tick-bad-lock.out")" "unreadable pid 'not-a-pid'"
equal "the malformed tick lock is not deleted" not-a-pid \
  "$(readlink "$tick_lock_path")"
rm -f -- "$tick_lock_path"

# TWO RELEASES CANNOT TURN THE RETRY INTO A LOOP. The first readlink removes
# the observed owner. The ln wrapper installs a replacement before the one
# permitted retry, and the next readlink removes that owner too.
tick_lock_bound_bin="$RUN_ROOT/tick-lock-bound-bin"
tick_lock_bound_reads="$RUN_ROOT/tick-lock-bound-reads"
tick_lock_bound_lns="$RUN_ROOT/tick-lock-bound-lns"
mkdir -p "$tick_lock_bound_bin"
{
  printf '#!/bin/sh\n'
  printf 'REAL=%q\n' "$(command -v readlink)"
  printf 'LOCK=%q\n' "$tick_lock_path"
  printf 'READS=%q\n' "$tick_lock_bound_reads"
  cat <<'SH'
if [ "${1:-}" = "$LOCK" ] && [ -L "$LOCK" ]; then
  printf x >> "$READS"
  rm -f -- "$LOCK"
fi
exec "$REAL" "$@"
SH
} > "$tick_lock_bound_bin/readlink"
{
  printf '#!/bin/sh\n'
  printf 'REAL=%q\n' "$(command -v ln)"
  printf 'LOCK=%q\n' "$tick_lock_path"
  printf 'HOLDER=%q\n' "$$"
  printf 'LNS=%q\n' "$tick_lock_bound_lns"
  cat <<'SH'
last=''
for arg in "$@"; do last=$arg; done
if [ "$last" = "$LOCK" ]; then
  lns=0
  [ ! -f "$LNS" ] || IFS= read -r lns < "$LNS"
  lns=$((lns + 1))
  printf '%s\n' "$lns" > "$LNS"
  [ "$lns" -ne 2 ] || "$REAL" -s "$HOLDER" "$LOCK"
fi
exec "$REAL" "$@"
SH
} > "$tick_lock_bound_bin/ln"
chmod +x "$tick_lock_bound_bin/readlink" "$tick_lock_bound_bin/ln"
ln -s "$$" "$tick_lock_path"
tick_lock_bound_rc=0
GANG_TICK_INTERNAL=1 PATH="$tick_lock_bound_bin:$PATH" \
  "$GANG" __tick-worker > "$RUN_ROOT/tick-lock-bound.out" 2>&1 \
  || tick_lock_bound_rc=$?
equal "a tick lock that disappears twice fails at its retry bound" \
  1 "$tick_lock_bound_rc"
contains "the twice-vanished tick lock explains its bounded failure" \
  "$(<"$RUN_ROOT/tick-lock-bound.out")" "disappeared twice"
equal "the tick retry bound observes exactly two released owners" xx \
  "$(<"$tick_lock_bound_reads")"

# The same pid-bearing symlink is reclaimable after a killed owner. Reuse the
# exact team lock name observed above; a made-up parallel path would not prove
# the production stale-lock branch.
ln -s 99999999 "$tick_lock_path"
tmux set-option -t "=$GANG_SESSION:" status-right \
  "operator-left #('/stale/snapshot/gang-tick-health.sh' '/stale/health') operator-right"
tmux set-option -u -t "=$GANG_SESSION:" @gl_tick_health_segment
"$GANG" tick >/dev/null
equal "a dead pid in the team tick lock is reclaimed" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"

# A ZOMBIE HAS EXITED EVEN WHILE ITS PID REMAINS ALLOCATED. waitid observes
# the exit without reaping it, and only then publishes the PID to the test.
tick_zombie_ready="$RUN_ROOT/tick-zombie-ready"
mkfifo "$tick_zombie_ready"
cat > "$RUN_ROOT/tick-zombie.py" <<'PY'
import os
import signal
import sys

child = os.fork()
if child == 0:
    os._exit(0)
os.waitid(os.P_PID, child, os.WEXITED | os.WNOWAIT)
with open(sys.argv[1], "w", encoding="utf-8") as ready:
    ready.write(str(child) + "\n")
signal.pause()
PY
python3 "$RUN_ROOT/tick-zombie.py" "$tick_zombie_ready" &
tick_zombie_parent=$!
IFS= read -r tick_zombie_pid < "$tick_zombie_ready"
ln -s "$tick_zombie_pid" "$tick_lock_path"
"$GANG" tick >/dev/null
equal "a zombie owner is dead and its tick lock is reclaimed" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"

# A NEW RECORD DISTINGUISHES PID ALLOCATION FROM THE RECORDED GENERATION.
# The live shell is deliberately paired with an impossible start token; it
# must not be dirtied or signalled as though it were the vanished owner.
tick_shell_identity="$("$ROOT/libexec/gang-process-identity" --tick "$$" "$GANG_SESSION")"
IFS=$'\t' read -r _ tick_shell_token tick_shell_pgrp _ _ _ _ \
  <<<"$tick_shell_identity"
ln -s "v2:$$:$tick_shell_token:$tick_shell_pgrp:later" "$tick_lock_path"
tick_bad_v2_rc=0
GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
  > "$RUN_ROOT/tick-bad-v2.out" 2>&1 || tick_bad_v2_rc=$?
equal "an alphabetic v2 acquisition time is unreadable" 1 "$tick_bad_v2_rc"
contains "the malformed v2 record is named before arithmetic" \
  "$(<"$RUN_ROOT/tick-bad-v2.out")" "has an unreadable owner"
equal "the malformed v2 record remains fail-closed" \
  "v2:$$:$tick_shell_token:$tick_shell_pgrp:later" \
  "$(readlink "$tick_lock_path")"
rm -f -- "$tick_lock_path"

ln -s "v2:$$:$tick_shell_token:$tick_shell_pgrp:$(tick_monotonic_ns):" \
  "$tick_lock_path"
tick_trailing_v2_rc=0
GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
  > "$RUN_ROOT/tick-trailing-v2.out" 2>&1 || tick_trailing_v2_rc=$?
equal "a trailing empty v2 field is rejected by the record validator" \
  1 "$tick_trailing_v2_rc"
equal "the trailing-field record remains fail-closed" present \
  "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
rm -f -- "$tick_lock_path"

ln -s "v2:$$:0:$tick_shell_pgrp:$(date +%s)" "$tick_lock_path"
"$GANG" tick >/dev/null
equal "a generation mismatch is reclaimed without signalling the reused pid" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"

# AN OLD OBSERVATION CANNOT DELETE A REPLACEMENT OWNER. The interpreter shim
# blocks the identity helper after the contender has read the planted record.
# A real public worker then publishes its own production record and holds its
# first pass. Releasing the old observation must make both dead-owner and
# replaced-generation retirement re-read the symlink before unlinking it.
tick_reclaim_replacement_race() { # $1 suffix, $2 planted record, $3 observed pid
  local suffix="$1" planted="$2" observed_pid="$3"
  local race_bin="$RUN_ROOT/tick-retire-$suffix-bin"
  local identity_ready="$RUN_ROOT/tick-retire-$suffix-identity-ready"
  local identity_release="$RUN_ROOT/tick-retire-$suffix-identity-release"
  local owner_ready="$RUN_ROOT/tick-retire-$suffix-owner-ready"
  local owner_release="$RUN_ROOT/tick-retire-$suffix-owner-release"
  local wrapper_once="$RUN_ROOT/tick-retire-$suffix-once"
  local contender_pid owner_pid contender_rc=0 replacement_record=""
  mkdir -p "$race_bin"
  mkfifo "$identity_ready" "$identity_release" "$owner_ready" "$owner_release"
  {
    printf '#!/bin/sh\n'
    printf 'REAL=%q\n' "$(command -v python3)"
    printf 'HELPER=%q\n' "$ROOT/libexec/gang-process-identity"
    printf 'TARGET=%q\n' "$observed_pid"
    printf 'READY=%q\n' "$identity_ready"
    printf 'RELEASE=%q\n' "$identity_release"
    printf 'ONCE=%q\n' "$wrapper_once"
    cat <<'SH'
if [ "${1:-}" = "$HELPER" ] && [ "${2:-}" = --tick ] \
   && [ "${3:-}" = "$TARGET" ] && [ ! -e "$ONCE" ]; then
  : > "$ONCE"
  printf x > "$READY"
  IFS= read -r _ < "$RELEASE"
fi
exec "$REAL" "$@"
SH
  } > "$race_bin/python3"
  chmod +x "$race_bin/python3"

  ln -s "$planted" "$tick_lock_path"
  GANG_TICK_INTERNAL=1 PATH="$race_bin:$PATH" \
    "$GANG" __tick-worker > "$RUN_ROOT/tick-retire-$suffix-contender.out" 2>&1 &
  contender_pid=$!
  IFS= read -r -N 1 _ < "$identity_ready"
  rm -f -- "$tick_lock_path"
  GANG_TEST_TICK_READY_FIFO="$owner_ready" \
  GANG_TEST_TICK_RELEASE_FIFO="$owner_release" \
    "$GANG" tick > "$RUN_ROOT/tick-retire-$suffix-owner.out" 2>&1 &
  owner_pid=$!
  IFS= read -r -N 1 _ < "$owner_ready"
  replacement_record="$(readlink "$tick_lock_path")"
  printf '\n' > "$identity_release"
  wait "$contender_pid" || contender_rc=$?
  equal "$suffix retirement re-reads before deleting a replacement lock" \
    "$replacement_record" "$(readlink "$tick_lock_path" 2>/dev/null || true)"
  equal "$suffix retirement leaves the replacement owner alive" live \
    "$(if kill -0 "$owner_pid" 2>/dev/null; then printf live; else printf dead; fi)"
  equal "$suffix replacement contention reports the live replacement" 75 "$contender_rc"
  printf '\n' > "$owner_release"
  wait "$owner_pid"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
}

tick_reclaim_replacement_race dead-owner 99999999 99999999
tick_reclaim_replacement_race replaced-generation \
  "v2:$$:0:$tick_shell_pgrp:1" "$$"

# UNKNOWN PROCESS EVIDENCE IS NOT DEATH. One helper shim returns the unknown
# verdict; the other appends the malformed empty field owned by the helper-output
# validator. Both conditions must retain the observed lock.
tick_identity_shape_probe() { # $1 suffix, $2 wrapper body
  local suffix="$1" body="$2" probe_bin=""
  local probe_rc=0
  probe_bin="$RUN_ROOT/tick-$suffix-bin"
  mkdir -p "$probe_bin"
  {
    printf '#!/bin/sh\n'
    printf 'REAL=%q\n' "$(command -v python3)"
    printf 'HELPER=%q\n' "$ROOT/libexec/gang-process-identity"
    printf 'TARGET=%q\n' "$$"
    printf '%s\n' "$body"
  } > "$probe_bin/python3"
  chmod +x "$probe_bin/python3"
  ln -s "v2:$$:$tick_shell_token:$tick_shell_pgrp:1" "$tick_lock_path"
  GANG_TICK_INTERNAL=1 PATH="$probe_bin:$PATH" \
    "$GANG" __tick-worker > "$RUN_ROOT/tick-$suffix.out" 2>&1 || probe_rc=$?
  equal "$suffix process evidence remains a loud lock fault" 1 "$probe_rc"
  equal "$suffix process evidence retains the observed lock" present \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
}

# shellcheck disable=SC2016
tick_identity_shape_probe unknown-identity '
if [ "${1:-}" = "$HELPER" ] && [ "${2:-}" = --tick ] && [ "${3:-}" = "$TARGET" ]; then
  exit 2
fi
exec "$REAL" "$@"'
# shellcheck disable=SC2016
tick_identity_shape_probe trailing-helper-field '
if [ "${1:-}" = "$HELPER" ] && [ "${2:-}" = --tick ] && [ "${3:-}" = "$TARGET" ]; then
  out="$("$REAL" "$@")" || exit $?
  printf "%s\\t\\n" "$out"
  exit 0
fi
exec "$REAL" "$@"'

# ROLLING UPGRADE SAFETY RETAINS A RECENT LEGACY OWNER. This live shell is not
# a worker, but before the published budget expires the old record lacks the
# evidence needed to call it stale.
ln -s "$$" "$tick_lock_path"
"$GANG" tick >/dev/null
equal "a recent live legacy lock remains owned" "$$" \
  "$(readlink "$tick_lock_path")"
equal "a recent live legacy owner receives the cooperative dirty edge" present \
  "$([ -e "${tick_lock_path%.lock}.dirty" ] && printf present || printf absent)"
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"

# THE FIRST DERIVED EDGE IS HEALTH, NOT A KILL. A matching generation at 61s
# is surfaced through the public seam while the 120s reclaim edge stays shut.
tick_over_budget=$(( $(tick_monotonic_ns) - 61000000000 ))
ln -s "v2:$$:$tick_shell_token:$tick_shell_pgrp:$tick_over_budget" \
  "$tick_lock_path"
tick_over_budget_rc=0
"$GANG" tick > "$RUN_ROOT/tick-over-budget.out" 2>&1 \
  || tick_over_budget_rc=$?
equal "an over-budget live owner fails instead of returning silent contention" \
  1 "$tick_over_budget_rc"
contains "the over-budget failure names the worker and reclaim budgets" \
  "$(<"$RUN_ROOT/tick-over-budget.out")" "generation-verified reclaim starts at 120s"
equal "the first expiry edge retains the exact live generation" present \
  "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
equal "an over-budget live owner still receives the cooperative dirty edge" present \
  "$([ -e "${tick_lock_path%.lock}.dirty" ] && printf present || printf absent)"
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
"$GANG" tick >/dev/null

# A LEGACY PID MAY NOW NAME A DIFFERENT LIVE PROCESS. The readiness-proven
# Python parent is positively not a tick worker for this team, so it cannot be
# the process that acquired a production legacy lock. The public seam must
# retire that legacy incident and surface that no cooperative pass ran without
# signalling the unrelated live generation.
ln -s "$tick_zombie_parent" "$tick_lock_path"
python3 - "$tick_lock_path" <<'PY'
import os
import sys
import time

then = time.time() - 120
os.utime(sys.argv[1], (then, then), follow_symlinks=False)
PY
tick_legacy_reuse_rc=0
"$GANG" tick > "$RUN_ROOT/tick-legacy-reuse.out" 2>&1 \
  || tick_legacy_reuse_rc=$?
equal "a reused live pid cannot retain an expired legacy tick lock" \
  1 "$tick_legacy_reuse_rc"
equal "the expired legacy lock is retired before recovery is reported" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
if kill -0 "$tick_zombie_parent" 2>/dev/null; then
  pass "legacy PID-reuse migration does not signal the unrelated generation"
else
  fail "legacy PID-reuse migration does not signal the unrelated generation" \
    "the readiness-proven process $tick_zombie_parent died"
fi
kill "$tick_zombie_parent"
wait "$tick_zombie_parent" 2>/dev/null || true
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
"$GANG" tick >/dev/null

# SIGKILL AUTHORIZATION IS HELD AT BOTH LAYERS. A readiness-proven unrelated
# session leader has a valid generation record but no tick-worker role. The
# shell contender and the pidfd helper must each refuse it independently.
tick_bystander_ready="$RUN_ROOT/tick-bystander-ready"
mkfifo "$tick_bystander_ready"
setsid python3 - "$tick_bystander_ready" <<'PY' &
import signal
import sys

with open(sys.argv[1], "w", encoding="utf-8") as ready:
    ready.write("x")
signal.pause()
PY
tick_bystander=$!
IFS= read -r -N 1 _ < "$tick_bystander_ready"
tick_bystander_identity="$("$ROOT/libexec/gang-process-identity" \
  --tick "$tick_bystander" "$GANG_SESSION")"
IFS=$'\t' read -r _ tick_bystander_token tick_bystander_pgrp \
  tick_bystander_session _ tick_bystander_role tick_bystander_safe \
  <<<"$tick_bystander_identity"
equal "the kill-authorization bystander is its own process-group leader" \
  "$tick_bystander" "$tick_bystander_pgrp"
equal "the kill-authorization bystander is its own session leader" \
  "$tick_bystander" "$tick_bystander_session"
equal "the kill-authorization bystander lacks the tick-worker role" 0 \
  "$tick_bystander_role"
equal "the kill-authorization bystander is signal-capable on this host" 1 \
  "$tick_bystander_safe"
tick_bystander_old=$(( $(tick_monotonic_ns) - 120000000000 ))
ln -s "v2:$tick_bystander:$tick_bystander_token:$tick_bystander_pgrp:$tick_bystander_old" \
  "$tick_lock_path"
tick_bystander_contender_rc=0
GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
  > "$RUN_ROOT/tick-bystander-contender.out" 2>&1 \
  || tick_bystander_contender_rc=$?
equal "the shell authorization gate refuses an unrelated expired leader" \
  1 "$tick_bystander_contender_rc"
equal "the shell authorization gate leaves the unrelated leader alive" live \
  "$(if kill -0 "$tick_bystander" 2>/dev/null; then printf live; else printf dead; fi)"
tick_bystander_helper_rc=0
"$ROOT/libexec/gang-process-identity" --kill "$tick_bystander" \
  "$tick_bystander_token" "$GANG_SESSION" >/dev/null 2>&1 \
  || tick_bystander_helper_rc=$?
equal "the pidfd helper independently refuses a non-worker leader" \
  2 "$tick_bystander_helper_rc"
equal "the helper role gate leaves the unrelated leader alive" live \
  "$(if kill -0 "$tick_bystander" 2>/dev/null; then printf live; else printf dead; fi)"
kill "$tick_bystander" 2>/dev/null || true
wait "$tick_bystander" 2>/dev/null || true
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"

# THE PIDFD HELPER ALSO BINDS THE RECORDED START TOKEN. A real production
# tick-worker satisfies every role, group, session, and platform gate, so only
# the deliberately wrong generation token can prevent this signal.
tick_token_ready="$RUN_ROOT/tick-token-ready"
tick_token_release="$RUN_ROOT/tick-token-release"
mkfifo "$tick_token_ready" "$tick_token_release"
GANG_TEST_TICK_READY_FIFO="$tick_token_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_token_release" \
  "$GANG" tick > "$RUN_ROOT/tick-token-owner.out" 2>&1 &
tick_token_owner=$!
IFS= read -r -N 1 _ < "$tick_token_ready"
tick_token_record="$(readlink "$tick_lock_path")"
IFS=: read -r _ tick_token_worker tick_token_value _ _ \
  <<<"$tick_token_record"
tick_wrong_token_rc=0
"$ROOT/libexec/gang-process-identity" --kill "$tick_token_worker" \
  "${tick_token_value}x" "$GANG_SESSION" >/dev/null 2>&1 \
  || tick_wrong_token_rc=$?
equal "the pidfd helper refuses a mismatched generation token" \
  1 "$tick_wrong_token_rc"
tick_token_worker_state=0
"$ROOT/libexec/gang-process-identity" --tick "$tick_token_worker" "$GANG_SESSION" \
  >/dev/null 2>&1 || tick_token_worker_state=$?
equal "the mismatched token leaves the exact worker generation alive" \
  0 "$tick_token_worker_state"
if [ "$tick_token_worker_state" -eq 0 ]; then
  printf '\n' > "$tick_token_release"
fi
wait "$tick_token_owner" 2>/dev/null || true
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
"$GANG" tick >/dev/null

# THE LOCK BUDGET USES THE CONTROLLER'S MONOTONIC CLOCK DOMAIN. A PATH-local
# wall clock jumps five minutes after a real worker publishes its production
# record. That input must neither age nor kill the still-in-budget generation.
# The same held worker then crosses only the 60s monotonic health edge: it must
# stay alive, retain its lock, and keep the dirty rerun request.
tick_clock_ready="$RUN_ROOT/tick-clock-ready"
tick_clock_release="$RUN_ROOT/tick-clock-release"
tick_clock_bin="$RUN_ROOT/tick-clock-bin"
mkdir -p "$tick_clock_bin"
mkfifo "$tick_clock_ready" "$tick_clock_release"
GANG_TEST_TICK_READY_FIFO="$tick_clock_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_clock_release" \
  "$GANG" tick > "$RUN_ROOT/tick-clock-owner.out" 2>&1 &
tick_clock_owner=$!
IFS= read -r -N 1 _ < "$tick_clock_ready"
tick_clock_record="$(readlink "$tick_lock_path")"
IFS=: read -r _ tick_clock_worker tick_clock_token tick_clock_pgrp _ \
  <<<"$tick_clock_record"
{
  printf '#!/bin/sh\n'
  printf 'REAL=%q\n' "$(command -v date)"
  cat <<'SH'
if [ "${1:-}" = +%s ]; then
  now="$("$REAL" +%s)" || exit $?
  printf '%s\n' "$((now + 300))"
  exit 0
fi
exec "$REAL" "$@"
SH
} > "$tick_clock_bin/date"
chmod +x "$tick_clock_bin/date"
tick_clock_jump_rc=0
GANG_TICK_INTERNAL=1 PATH="$tick_clock_bin:$PATH" \
  "$GANG" __tick-worker > "$RUN_ROOT/tick-clock-jump.out" 2>&1 \
  || tick_clock_jump_rc=$?
tick_clock_worker_state=0
"$ROOT/libexec/gang-process-identity" --tick "$tick_clock_worker" "$GANG_SESSION" \
  >/dev/null 2>&1 || tick_clock_worker_state=$?
equal "a five-minute wall-clock step does not expire a monotonic lock" \
  75 "$tick_clock_jump_rc"
equal "the in-budget worker survives the wall-clock step" 0 \
  "$tick_clock_worker_state"
equal "the wall-clock step leaves the production owner record unchanged" \
  "$tick_clock_record" "$(readlink "$tick_lock_path" 2>/dev/null || true)"

if [ "$tick_clock_worker_state" -eq 0 ]; then
  tick_clock_edge=$(( $(tick_monotonic_ns) - 61000000000 ))
  rm -f -- "$tick_lock_path"
  ln -s "v2:$tick_clock_worker:$tick_clock_token:$tick_clock_pgrp:$tick_clock_edge" \
    "$tick_lock_path"
  tick_clock_edge_rc=0
  GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
    > "$RUN_ROOT/tick-clock-edge.out" 2>&1 || tick_clock_edge_rc=$?
  equal "the 60s edge reports health without killing the live worker" \
    1 "$tick_clock_edge_rc"
  tick_clock_edge_worker_state=0
  "$ROOT/libexec/gang-process-identity" --tick "$tick_clock_worker" "$GANG_SESSION" \
    >/dev/null 2>&1 || tick_clock_edge_worker_state=$?
  equal "the 60s edge retains the exact worker generation" live \
    "$(if [ "$tick_clock_edge_worker_state" -eq 0 ]; then printf live; else printf dead; fi)"
  equal "the 60s edge retains the generation-bearing lock" present \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  equal "the 60s edge preserves the cooperative dirty request" present \
    "$([ -e "${tick_lock_path%.lock}.dirty" ] && printf present || printf absent)"
  if [ "$tick_clock_edge_worker_state" -eq 0 ]; then
    rm -f -- "$tick_lock_path"
    ln -s "$tick_clock_record" "$tick_lock_path"
    printf '\n' > "$tick_clock_release"
  fi
fi
wait "$tick_clock_owner" 2>/dev/null || true
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
"$GANG" tick >/dev/null

# AN EXPIRED GENERATION IS TERMINATED BY PIDFD, NEVER BY ITS BARE PROCESS
# GROUP. Hold a real public worker after acquisition, then move only the
# immutable acquisition stamp behind the derived reclaim edge. The contender
# must surface recovery without claiming that it ran a pass.
tick_expired_ready="$RUN_ROOT/tick-expired-ready"
tick_expired_release="$RUN_ROOT/tick-expired-release"
mkfifo "$tick_expired_ready" "$tick_expired_release"
GANG_TEST_TICK_READY_FIFO="$tick_expired_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_expired_release" \
  "$GANG" tick > "$RUN_ROOT/tick-expired-owner.out" 2>&1 &
tick_expired_owner=$!
IFS= read -r -N 1 _ < "$tick_expired_ready"
tick_expired_record="$(readlink "$tick_lock_path")"
IFS=: read -r tick_expired_version tick_expired_pid tick_expired_token \
  tick_expired_pgrp _ <<<"$tick_expired_record"
equal "the expiry fixture owns a generation-bearing production lock" v2 \
  "$tick_expired_version"
equal "the expiry fixture worker is its own process-group leader" \
  "$tick_expired_pid" "$tick_expired_pgrp"
tick_expired_old=$(( $(tick_monotonic_ns) - 120000000000 ))
rm -f -- "$tick_lock_path"
ln -s "v2:$tick_expired_pid:$tick_expired_token:$tick_expired_pgrp:$tick_expired_old" \
  "$tick_lock_path"
tick_expired_rc=0
"$GANG" tick > "$RUN_ROOT/tick-expired-reclaim.out" 2>&1 \
  || tick_expired_rc=$?
equal "an expired exact worker generation is reclaimed as a surfaced failure" \
  1 "$tick_expired_rc"
equal "pidfd-confirmed expiry retires the worker lock" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
tick_expired_owner_rc=0
wait "$tick_expired_owner" || tick_expired_owner_rc=$?
equal "the expired worker owner reports the surfaced recovery failure" \
  1 "$tick_expired_owner_rc"
"$GANG" tick >/dev/null

# A CATCHABLE CONTROLLER DEATH MUST NOT ORPHAN ITS NEW-SESSION WORKER. The
# worker publishes its production lock, then blocks reading a FIFO and writes
# nothing to the controller pipes, so EPIPE cannot end it before controller
# cleanup is observed. Resolve the whole relationship in completed reads before
# signalling this exact controller.
tick_controller_ready="$RUN_ROOT/tick-controller-ready"
tick_controller_release="$RUN_ROOT/tick-controller-release"
mkfifo "$tick_controller_ready" "$tick_controller_release"
GANG_TEST_TICK_READY_FIFO="$tick_controller_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_controller_release" \
  "$GANG" tick > "$RUN_ROOT/tick-controller-owner.out" 2>&1 &
tick_controller_owner=$!
IFS= read -r -N 1 _ < "$tick_controller_ready"
tick_controller_record="$(readlink "$tick_lock_path")"
IFS=: read -r tick_controller_version tick_controller_worker \
  _ tick_controller_pgrp _ <<<"$tick_controller_record"
tick_controller_identity="$("$ROOT/libexec/gang-process-identity" \
  --tick "$tick_controller_worker" "$GANG_SESSION")"
IFS=$'\t' read -r _ _ _ tick_controller_session _ tick_controller_role _ \
  <<<"$tick_controller_identity"
tick_controller_pid="$(ps -o ppid= -p "$tick_controller_worker" | tr -d ' ')"
tick_controller_command="$(ps -o command= -p "$tick_controller_pid")"
equal "the controller-death fixture owns a generation-bearing lock" v2 \
  "$tick_controller_version"
equal "the controller-death fixture resolved its worker session leader" \
  "$tick_controller_worker" "$tick_controller_pgrp"
equal "the controller-death fixture resolved its worker session" \
  "$tick_controller_worker" "$tick_controller_session"
equal "the controller-death fixture resolved the tick-worker role" 1 \
  "$tick_controller_role"
contains "the resolved parent is this tree's deadline controller" \
  "$tick_controller_command" "$ROOT/libexec/gang-tick-deadline"
kill -TERM "$tick_controller_pid"
tick_controller_owner_rc=0
wait "$tick_controller_owner" || tick_controller_owner_rc=$?
equal "controller TERM remains a surfaced tick failure" 1 \
  "$tick_controller_owner_rc"
tick_controller_worker_rc=0
"$ROOT/libexec/gang-process-identity" \
  --tick "$tick_controller_worker" "$GANG_SESSION" >/dev/null 2>&1 \
  || tick_controller_worker_rc=$?
equal "controller TERM leaves no live worker generation" 1 \
  "$tick_controller_worker_rc"
"$GANG" tick >/dev/null
equal "the next tick reclaims the controller's dead worker lock" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"

# This branch is unreachable once controller cleanup terminates the worker. If
# it remains alive, the exact generation and unchanged record observed above
# identify the process this fixture owns; terminate it, then retire its lock.
if [ "$tick_controller_worker_rc" -eq 0 ]; then
  equal "the EPIPE negative control retained the same worker lock" \
    "$tick_controller_record" "$(readlink "$tick_lock_path")"
  kill -KILL "$tick_controller_worker" 2>/dev/null || true
  "$GANG" tick >/dev/null
fi

tick_repaired_right="$(tmux show-options -qv -t "=$GANG_SESSION:" status-right)"
excludes "a tick replaces a health segment owned by an obsolete snapshot" \
  "$tick_repaired_right" "/stale/snapshot"
contains "status repair preserves the operator's unrelated left segment" \
  "$tick_repaired_right" "operator-left"
contains "status repair preserves the operator's unrelated right segment" \
  "$tick_repaired_right" "operator-right"
contains "status repair installs the running tree's health reader" \
  "$tick_repaired_right" "$ROOT/statusline/gang-tick-health.sh"
contains "the deadline controller fixes the production budget at sixty seconds" \
  "$(<"$ROOT/libexec/gang-tick-deadline")" "DEADLINE_SECONDS = 60"
contains "deadline expiry kills the worker's whole process group" \
  "$(<"$ROOT/libexec/gang-tick-deadline")" "os.killpg(child.pid, signal.SIGKILL)"
contains "deadline expiry reaches the shared pre-reap group cleanup" \
  "$(<"$ROOT/libexec/gang-tick-deadline")" \
  $'except subprocess.TimeoutExpired:\n        kill_worker_group()'

# The synchronous test mode takes the same post-command epilogue without a
# detached race. Its ledger is the evidence that an unrelated invocation, not
# an explicit tick, initiated one full pass.
tick_auto_ledger="$RUN_ROOT/tick-auto-ledger"
GANG_TEST_TICK_MODE=sync GANG_TEST_TICK_LEDGER="$tick_auto_ledger" \
  "$GANG" teams >/dev/null
equal "every ordinary invocation initiates a cooperative pass after its work" 1 \
  "$(wc -l < "$tick_auto_ledger" | tr -d ' ')"

# A restarted Codex-shaped process keeps both exact native file descriptors
# open. The pane id is not enough: the new live id contradicts the registered
# one, becomes session-lost, and blocks its already-parked delivery.
tick_codex_root="$RUN_ROOT/tick-codex"
tick_codex_ready="$RUN_ROOT/tick-codex-ready"
mkdir -p "$tick_codex_root/thread-writer-locks" "$tick_codex_root/sessions/2026/08/27"
tick_codex_lock="$tick_codex_root/thread-writer-locks/live-session-222.lock"
tick_codex_rollout="$tick_codex_root/sessions/2026/08/27/rollout-fixture-live-session-222.jsonl"
: > "$tick_codex_lock"
: > "$tick_codex_rollout"
mkfifo "$tick_codex_ready"
cat > "$RUN_ROOT/tick-codex-process.py" <<'PY'
import signal
import sys

lock = open(sys.argv[1])
rollout = open(sys.argv[2])
with open(sys.argv[3], "w") as ready:
    ready.write("x")
signal.pause()
PY
tick_restart_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-restart \
  "exec python3 '$RUN_ROOT/tick-codex-process.py' '$tick_codex_lock' '$tick_codex_rollout' '$tick_codex_ready'")"
IFS= read -r -N 1 _ < "$tick_codex_ready"
"$GANG" adopt tick-restart -c codex >/dev/null
tmux set-option -w -t "$tick_restart_id" @gl_session_id registered-session-111
printf 'TICK_MUST_NOT_REACH_RESTART' \
  | "$GANG" send --to tick-restart --from tester --stdin >/dev/null
tick_identity_rc=0
"$GANG" tick > "$RUN_ROOT/tick-identity.out" 2> "$RUN_ROOT/tick-identity.err" \
  || tick_identity_rc=$?
equal "a live native id mismatch fails the explicit health pass loudly" 1 "$tick_identity_rc"
equal "the Codex host-process fd witness records the new exact live id" \
  live-session-222 \
  "$(tmux show-options -wqv -t "$tick_restart_id" @gl_session_live_id)"
contains "the restarted harness is a session-lost state, not an idle agent" \
  "$("$GANG" status tick-restart 2>/dev/null)" "!session-lost!"
contains "roster carries the same loud session-lost verdict" \
  "$("$GANG" roster 2>/dev/null)" "session-lost"
tick_restart_capture="$(tmux capture-pane -pJ -S - -t "$tick_restart_id")"
# source-guard: whole-surface@16f80b8dd733: this process never reads stdin, so the unique body can appear in its pane only if Gangline typed it
equal "the contradicted pane receives none of the parked delivery" absent \
  "$(case "$tick_restart_capture" in *TICK_MUST_NOT_REACH_RESTART*) printf present ;; *) printf absent ;; esac)"
equal "and its delivery remains parked for an intended replacement" 1 \
  "$("$GANG" roster --porcelain 2>/dev/null | awk -F '\t' '$1 == "tick-restart" { print $4 }')"

# Health is per team (socket plus session), not a singleton below XDG state.
# Another private team can legitimately have ticked earlier in this integration
# run, so selecting the first file would make this fixture read its health and
# then call the current team's teardown incomplete.
tick_health_socket="$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')"
tick_health_digest="$(python3 -c 'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$tick_health_socket" "$GANG_SESSION")"
tick_health_file="$XDG_STATE_HOME/gangline/tick/$tick_health_digest/health"
tick_log_file="${tick_health_file%/*}/tick.log"
contains "the failed tick writes its per-team health state" \
  "$(<"$tick_health_file")" $'failed\t'
contains "status surfaces the last tick failure" \
  "$("$GANG" status tick-restart 2>/dev/null)" "last tick failed:"
contains "the attached-human status-right contains the health reader" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" status-right)" "gang-tick-health.sh"
tick_alert_id="$(tmux list-windows -t "=$GANG_SESSION" -F '#{window_id} #{@gl_tick_alerts}' \
  | awk '$2 == 1 { print $1 }')"
equal "failure creates one dedicated alerts window" 1 \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#{@gl_tick_alerts}' | grep -c '^1$')"
equal "the alerts window raises both activity and bell monitors" "1 1" \
  "$(tmux display-message -p -t "$tick_alert_id" '#{monitor-activity} #{monitor-bell}')"
excludes "roster does not mistake the dedicated alerts window for an agent" \
  "$("$GANG" roster 2>/dev/null)" "gangline-alerts"

tick_next_err="$RUN_ROOT/tick-next.err"
"$GANG" teams >/dev/null 2> "$tick_next_err"
contains "the next Gangline invocation repeats the last tick failure" \
  "$(<"$tick_next_err")" "last tick failed:"
tick_isolation_rc=0
GANG_TEST_TICK_MODE=sync "$GANG" teams >/dev/null 2>&1 || tick_isolation_rc=$?
equal "a detached tick failure never changes its spawning command result" 0 "$tick_isolation_rc"

"$GANG" drop tick-restart >/dev/null 2>&1
"$GANG" tick >/dev/null
excludes "a later successful pass clears the health failure" \
  "$(<"$tick_health_file")" $'failed\t'
# source-guard: producer@5a0aff3445c2: the synchronous tick immediately above is the only writer in this fixture and an ok-prefixed record is its successful result
equal "the clean pass records an ok log fixture" ok \
  "$(case "$(<"$tick_log_file")" in $'ok\t'*) printf ok ;; *) printf other ;; esac)"

# THE ALERT BODY MUST REJECT A SUCCESS RECORD AT THE LAST POSSIBLE READ. A
# failed controller can reach the alert launch after a newer clean controller
# replaces the shared log. The event barriers run the real one-shot body in a
# disposable window on this suite's private server and hold the pane after the
# body returns, so both its status and complete output are immediate evidence.
tick_ok_alert_start="gang-tick-ok-alert-start-$$"
tick_ok_alert_done="gang-tick-ok-alert-done-$$"
tick_ok_alert_release="gang-tick-ok-alert-release-$$"
printf -v tick_ok_alert_command \
  '%q wait-for %q; %q %q; alert_rc=$?; %q set-option -w -t "$TMUX_PANE" @test_tick_alert_rc "$alert_rc"; %q wait-for -S %q; %q wait-for %q; exit "$alert_rc"' \
  "$REAL_TMUX" "$tick_ok_alert_start" \
  "$ROOT/statusline/gang-tick-alert.sh" "$tick_log_file" \
  "$REAL_TMUX" "$REAL_TMUX" "$tick_ok_alert_done" \
  "$REAL_TMUX" "$tick_ok_alert_release"
tick_ok_alert_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-ok-alert-probe "$tick_ok_alert_command")"
tmux wait-for -S "$tick_ok_alert_start"
tmux wait-for "$tick_ok_alert_done"
# source-guard: producer@a4c3578f10d4: the prior assertion independently verifies the ok record and the done barrier proves this option came from the real alert body's completed status
equal "an ok tick log makes the alert body decline cleanly" 0 \
  "$(tmux show-options -wqv -t "$tick_ok_alert_id" @test_tick_alert_rc)"
tick_ok_alert_capture="$(tmux capture-pane -pJ -S - -t "$tick_ok_alert_id")"
# source-guard: producer@d1c27ea27709: the verified ok fixture and done barrier prove the real alert body completed while the held pane preserves its entire output
equal "an ok tick log emits no alert header, body, or bell" "" \
  "$tick_ok_alert_capture"
tmux wait-for -S "$tick_ok_alert_release"

# A CODEX SESSION BEFORE ITS FIRST TURN HOLDS ITS LOCK AND NO ROLLOUT. Codex
# opens the thread-writer lock as the session opens but creates the rollout
# lazily, on the first turn, so a freshly hitched agent sitting at a composer
# nobody has prompted holds exactly one witness. Verified against codex-cli
# 0.149.1: before the first turn the rollout is absent as an fd and absent from
# the sessions tree; after it, both descriptors are open. Requiring both made
# the identity probe unsatisfiable for the whole of the state hitch leaves an
# agent in.
tick_fresh_root="$RUN_ROOT/tick-codex-fresh"
tick_fresh_ready="$RUN_ROOT/tick-codex-fresh-ready"
mkdir -p "$tick_fresh_root/thread-writer-locks"
tick_fresh_lock="$tick_fresh_root/thread-writer-locks/fresh-session-333.lock"
: > "$tick_fresh_lock"
mkfifo "$tick_fresh_ready"
cat > "$RUN_ROOT/tick-codex-holder.py" <<'PY'
import signal
import sys

held = [open(path) for path in sys.argv[2:]]
with open(sys.argv[1], "w") as ready:
    ready.write("x")
signal.pause()
PY
tick_fresh_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-fresh \
  "exec python3 '$RUN_ROOT/tick-codex-holder.py' '$tick_fresh_ready' '$tick_fresh_lock'")"
IFS= read -r -N 1 _ < "$tick_fresh_ready"
"$GANG" adopt tick-fresh -c codex >/dev/null
tmux set-option -w -t "$tick_fresh_id" @gl_session_id fresh-session-333
"$GANG" tick >/dev/null
equal "a Codex session that has taken no turn yet is still identified by its lock" \
  fresh-session-333 \
  "$(tmux show-options -wqv -t "$tick_fresh_id" @gl_session_live_id)"
equal "and its identity is verified rather than left unread" "" \
  "$(tmux show-options -wqv -t "$tick_fresh_id" @gl_session_probe_failed)"
excludes "so a pre-first-turn agent is not reported session-lost" \
  "$("$GANG" status tick-fresh 2>/dev/null)" "!session-lost!"

# CORROBORATION IS STILL REQUIRED WHEREVER IT EXISTS. The lock is the authority
# only when nothing contradicts it: a rollout naming another thread is evidence
# against the lock rather than evidence missing, and it keeps refusing.
tick_wrong_root="$RUN_ROOT/tick-codex-wrong"
tick_wrong_ready="$RUN_ROOT/tick-codex-wrong-ready"
mkdir -p "$tick_wrong_root/thread-writer-locks" "$tick_wrong_root/sessions"
tick_wrong_lock="$tick_wrong_root/thread-writer-locks/wrong-session-444.lock"
tick_wrong_rollout="$tick_wrong_root/sessions/rollout-fixture-other-session-555.jsonl"
: > "$tick_wrong_lock"
: > "$tick_wrong_rollout"
mkfifo "$tick_wrong_ready"
tick_wrong_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-wrong \
  "exec python3 '$RUN_ROOT/tick-codex-holder.py' '$tick_wrong_ready' '$tick_wrong_lock' '$tick_wrong_rollout'")"
IFS= read -r -N 1 _ < "$tick_wrong_ready"
"$GANG" adopt tick-wrong -c codex >/dev/null
"$GANG" tick >/dev/null
equal "a rollout naming another thread refuses rather than trusting the lock" "" \
  "$(tmux show-options -wqv -t "$tick_wrong_id" @gl_session_live_id)"
contains "and that refusal is recorded where status and roster read it" \
  "$(tmux show-options -wqv -t "$tick_wrong_id" @gl_session_probe_failed)" \
  "live harness session id could not be read"

# AN EXPECTED MISS MUST NOT LAND ON THE AGENT'S SCREEN. tmux renders a
# run-shell that exits nonzero into the target pane: it drops that pane into
# view-mode over the harness TUI and prints "'<command>' returned 1" there,
# taking the window's name to [tmux] as well wherever one is not pinned. The
# mode is the witness asserted below, because it is the part no naming choice
# can mask. The identity probe misses for ordinary reasons -- a harness still
# starting, one that has not opened its lock -- and every ordinary miss used to
# cover the agent's screen and divert its keystrokes into a copy-mode overlay,
# once per tick, which is once per Gangline invocation.
tick_bare_ready="$RUN_ROOT/tick-codex-bare-ready"
mkfifo "$tick_bare_ready"
tick_bare_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-bare \
  "exec python3 '$RUN_ROOT/tick-codex-holder.py' '$tick_bare_ready'")"
IFS= read -r -N 1 _ < "$tick_bare_ready"
"$GANG" adopt tick-bare -c codex >/dev/null

# CALIBRATE THE INSTRUMENT ON THE FAULT IT MUST CATCH. A pane that is never
# hijacked and a reader that cannot see a hijack look identical from the
# assertion below, so the unguarded command shape is driven once against a
# throwaway window first and required to produce exactly what the fix removes.
tick_calib_ready="$RUN_ROOT/tick-codex-calib-ready"
mkfifo "$tick_calib_ready"
tick_calib_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-calib \
  "exec python3 '$RUN_ROOT/tick-codex-holder.py' '$tick_calib_ready'")"
IFS= read -r -N 1 _ < "$tick_calib_ready"
tmux run-shell -t "$tick_calib_id" "exit 1" >/dev/null 2>&1 || :
equal "an unguarded run-shell failure does put its target pane in view-mode" \
  "view-mode" \
  "$(tmux display-message -p -t "$tick_calib_id" '#{?pane_in_mode,#{pane_mode},none}')"
tmux kill-window -t "$tick_calib_id" 2>/dev/null || :

"$GANG" tick >/dev/null
equal "a probe that finds no id leaves the agent's pane out of any mode" \
  "none" \
  "$(tmux display-message -p -t "$tick_bare_id" '#{?pane_in_mode,#{pane_mode},none}')"
contains "while the miss itself is still recorded on the window" \
  "$(tmux show-options -wqv -t "$tick_bare_id" @gl_session_probe_failed)" \
  "live harness session id could not be read"
equal "and a silent probe miss does not fail the tick" 0 \
  "$( "$GANG" tick >/dev/null 2>&1; printf '%s' $?)"

"$GANG" drop tick-fresh >/dev/null 2>&1
"$GANG" drop tick-wrong >/dev/null 2>&1
"$GANG" drop tick-bare >/dev/null 2>&1

# Team teardown ignores the special alert pane as an agent and retires the
# ephemeral health files with the session that gave them meaning.
"$GANG" down "$GANG_SESSION" >/dev/null
equal "tick test teardown ends only its exact disposable session" absent \
  "$(if tmux has-session -t "=$GANG_SESSION" 2>/dev/null; then printf present; else printf absent; fi)"
equal "team teardown removes its ephemeral tick health file" absent \
  "$([ ! -e "$tick_health_file" ] && printf absent || printf present)"

export GANG_SESSION="$tick_original_session"
if [ -n "$tick_original_collars" ]; then
  export GANG_COLLARS="$tick_original_collars"
else
  unset GANG_COLLARS
fi
