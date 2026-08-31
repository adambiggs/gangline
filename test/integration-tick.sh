# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Cooperative tick: global retries, copy-mode recovery, native identity, and rails.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# after the ordinary substrate checks and supplies its fixtures and assertions.

tick_original_session="$GANG_SESSION"
tick_original_collars="${GANG_COLLARS:-}"
export GANG_SESSION="gangtick-test-$$"

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
  "$(<"$ROOT/libexec/gang-tick-deadline")" "os.killpg(worker.pid, signal.SIGKILL)"

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
