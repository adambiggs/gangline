# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Cooperative tick: global retries, copy-mode recovery, native identity, and rails.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# after the ordinary substrate checks and supplies its fixtures and assertions.

tick_original_session="$GANG_SESSION"
tick_original_collars="${GANG_COLLARS:-}"

# TICK RESULT ORDERING GETS ITS OWN TMUX SERVER. These fixtures hold a pass at
# its health commit and drive a second team pass across it, so they run on an
# exact private socket holding one adopted window, away from the substrate
# team the later fixtures share.
tick_order_root="$RUN_ROOT/tick-order-server"
tick_order_session="gang-tick-order-$$"
mkdir -p "$tick_order_root"
tick_order_tmux() { env -u TMUX TMUX_TMPDIR="$tick_order_root" tmux "$@"; }
tick_order_gang() {
  TMUX_TMPDIR="$tick_order_root" GANG_SESSION="$tick_order_session" \
    GANG_LOCK_DIR="$RUN_ROOT/tick-order-locks" \
    GANG_ARCHIVE_DIR="$RUN_ROOT/tick-order-archive" \
    XDG_STATE_HOME="$RUN_ROOT/tick-order-state" "$GANG" "$@"
}

tick_order_tmux new-session -d -s "$tick_order_session" -n caller \
  "PS1='❯ ' exec bash --norc"
tick_order_gang adopt caller -c bash >/dev/null
tick_order_caller_id="$(tick_order_tmux list-windows -t "=$tick_order_session" \
  -F '#{window_id} #{@gl_agent}' | awk '$2 == "caller" { print $1 }')"
equal "the tick-order fixture has one readiness-proven adopted window" \
  caller "$(tick_order_tmux show-options -wqv -t "$tick_order_caller_id" @gl_agent)"
tick_order_socket="$(tick_order_tmux display-message -p \
  -t "=$tick_order_session" '#{socket_path}')"
tick_order_digest="$(python3 -c \
  'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$tick_order_socket" "$tick_order_session")"
tick_order_health="$RUN_ROOT/tick-order-state/gangline/tick/$tick_order_digest/health"

# A PASS OWNS THE RUN LOCK THROUGH ITS HEALTH COMMIT, AND A LAUNCH DURING IT IS
# SERVED AFTERWARDS. Hold a failing pass at that exact seam, repair the
# condition, and launch: the launch queues behind the held pass instead of
# committing ahead of it, and its own pass then records the recovery. Taking
# the queue lock and then the run lock waits out the queued pass, which holds
# the first until it holds the second.
tick_order_queue_lock="$RUN_ROOT/tick-order-locks/tick/$tick_order_digest.queue"
tick_order_run_lock="$RUN_ROOT/tick-order-locks/tick/$tick_order_digest.run"
tick_order_queued_wait() {
  flock "$tick_order_queue_lock" true
  flock "$tick_order_run_lock" true
}
tick_order_commit_ready="$RUN_ROOT/tick-order-commit-ready"
tick_order_commit_release="$RUN_ROOT/tick-order-commit-release"
tick_order_commit_ledger="$RUN_ROOT/tick-order-commit-ledger"
mkfifo "$tick_order_commit_ready" "$tick_order_commit_release"
tick_order_tmux set-option -w -t "$tick_order_caller_id" @gl_collar missing-order-collar
GANG_TEST_TICK_COMMIT_READY_FIFO="$tick_order_commit_ready" \
GANG_TEST_TICK_COMMIT_RELEASE_FIFO="$tick_order_commit_release" \
GANG_TEST_TICK_LEDGER="$tick_order_commit_ledger" \
  tick_order_gang tick > "$RUN_ROOT/tick-order-commit-owner.out" 2>&1 &
tick_order_commit_owner=$!
IFS= read -r -N 1 _ < "$tick_order_commit_ready"
tick_order_tmux set-option -w -t "$tick_order_caller_id" @gl_collar bash
GANG_TEST_TICK_MODE=async GANG_TEST_TICK_LEDGER="$tick_order_commit_ledger" \
  tick_order_gang roster >/dev/null
tick_order_queued_rc=0
flock -n "$tick_order_queue_lock" true || tick_order_queued_rc=$?
equal "a launch while the older result is uncommitted queues a pass" \
  1 "$tick_order_queued_rc"
printf '\n' > "$tick_order_commit_release"
tick_order_commit_owner_rc=0
wait "$tick_order_commit_owner" || tick_order_commit_owner_rc=$?
equal "the held pass commits its own failure" 1 "$tick_order_commit_owner_rc"
tick_order_queued_wait
equal "the queued launch runs one pass after the held one" '1 1 ' \
  "$(tr '\n' ' ' < "$tick_order_commit_ledger")"
contains "the newer recovery is the final health record" \
  "$(<"$tick_order_health")" $'ok\t'
excludes "the newer recovery leaves roster no tick failure to report" \
  "$(tick_order_gang roster 2>&1)" "tick failed:"

# Deadline/controller failures return after their worker is gone. Hold the
# older parent at its failure commit and launch a later pass: it queues until
# the parent has committed, so the older failure cannot overwrite recovery.
tick_order_bad_clock="$RUN_ROOT/tick-order-bad-clock"
cat > "$tick_order_bad_clock" <<'SH'
#!/bin/sh
case "${1:-}" in
  now) printf '1\n'; exit 0 ;;
  elapsed) exit 2 ;;
  *) exit 2 ;;
esac
SH
chmod +x "$tick_order_bad_clock"
tick_order_parent_ready="$RUN_ROOT/tick-order-parent-ready"
tick_order_parent_release="$RUN_ROOT/tick-order-parent-release"
mkfifo "$tick_order_parent_ready" "$tick_order_parent_release"
GANG_TEST_CLOCK="$tick_order_bad_clock" \
GANG_TEST_TICK_PARENT_COMMIT_READY_FIFO="$tick_order_parent_ready" \
GANG_TEST_TICK_PARENT_COMMIT_RELEASE_FIFO="$tick_order_parent_release" \
  tick_order_gang tick > "$RUN_ROOT/tick-order-parent-failure.out" 2>&1 &
tick_order_parent_owner=$!
IFS= read -r -N 1 _ < "$tick_order_parent_ready"
GANG_TEST_TICK_MODE=async tick_order_gang roster >/dev/null
tick_order_parent_queued_rc=0
flock -n "$tick_order_queue_lock" true || tick_order_parent_queued_rc=$?
equal "a launch during an uncommitted controller failure queues a pass" \
  1 "$tick_order_parent_queued_rc"
printf '\n' > "$tick_order_parent_release"
tick_order_parent_rc=0
wait "$tick_order_parent_owner" || tick_order_parent_rc=$?
tick_order_queued_wait
equal "the older controller failure still returns its own failure" \
  1 "$tick_order_parent_rc"
contains "the older controller failure retains its diagnostic" \
  "$(<"$RUN_ROOT/tick-order-parent-failure.out")" \
  "cannot compare the shared monotonic deadline"
contains "an older controller failure cannot overwrite newer health" \
  "$(<"$tick_order_health")" $'ok\t'
tick_order_gang down "$tick_order_session" >/dev/null
unset -f tick_order_tmux tick_order_gang tick_order_queued_wait

export GANG_SESSION="gangtick-test-$$"
export GANG_TICK_DEADLINE_SECONDS=60

tick_monotonic_ns() {
  "$ROOT/libexec/gang-clock" now
}

tmux new-session -d -s "$GANG_SESSION" -n caller "PS1='❯ ' bash --norc"
"$GANG" adopt caller -c bash >/dev/null
tick_caller_id="$(window_id caller)"
tick_caller_pane="$(tmux list-panes -t "$tick_caller_id" -F '#{pane_id}')"

# A TICK'S TEAM KEY AND UID ARE INVOCATION-SCOPED INPUTS. The worker, its
# deadline child, and every guarded tmux client in the pass all address the
# same team as the parent. Pin their external resolutions here so adding a new
# path or probe cannot silently restore per-window fan-out.
tick_cost_bin="$RUN_ROOT/tick-cost-bin"
tick_cost_uid_calls="$RUN_ROOT/tick-cost-uid-calls"
tick_cost_hash_calls="$RUN_ROOT/tick-cost-hash-calls"
tick_cost_real_id="$(command -v id)"
tick_cost_real_python="$(python3 -c 'import sys; print(sys.executable)')"
mkdir -p "$tick_cost_bin"
cat > "$tick_cost_bin/id" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$tick_cost_real_id' "\$0" id || exit \$?
if [ "\$#" -eq 1 ] && [ "\$1" = -u ]; then
  printf 'uid\n' >> '$tick_cost_uid_calls'
fi
exec '$tick_cost_real_id' "\$@"
SH
cat > "$tick_cost_bin/python3" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$tick_cost_real_python' "\$0" python3 || exit \$?
case "\${1:-}:\${2:-}" in
  -c:*hashlib.sha256*) printf 'hash\n' >> '$tick_cost_hash_calls' ;;
esac
exec '$tick_cost_real_python' "\$@"
SH
chmod +x "$tick_cost_bin/id" "$tick_cost_bin/python3"
PATH="$tick_cost_bin:$PATH" id -u >/dev/null
equal "the uid execution counter sees its PATH-local instrument" 1 \
  "$(wc -l < "$tick_cost_uid_calls" | tr -d ' ')"
: > "$tick_cost_uid_calls"
PATH="$tick_cost_bin:$ROOT/libexec/gang-tmux-guard:$PATH" \
  "$GANG" tick >/dev/null
equal "one explicit tick launches no uid lookup across its whole worker tree" 0 \
  "$(wc -l < "$tick_cost_uid_calls" | tr -d ' ')"
equal "one explicit tick computes its team hash once across its whole worker tree" 1 \
  "$(wc -l < "$tick_cost_hash_calls" | tr -d ' ')"

# A PUBLIC COMMAND MAY CREATE THE TEAM BEFORE ITS EXIT-LAUNCHED TICK RUNS. An
# internal carrier inherited from the caller cannot name that new team: the
# public boundary discards it, and the synchronous test tick leaves exactly
# one observable state directory under the newly created session's own root.
tick_carrier_plant=000000000000000000000000
tick_carrier_session="gangtick-carrier-$$"
tick_carrier_state="$RUN_ROOT/tick-carrier-state"
GANGLINE_TICK_DIGEST="$tick_carrier_plant" \
  GANG_SESSION="$tick_carrier_session" \
  GANG_TEST_TICK_MODE=sync \
  XDG_STATE_HOME="$tick_carrier_state" \
  "$GANG" hitch carrier -c bash -d "$RUN_ROOT" >/dev/null
tick_carrier_dirs="$(find "$tick_carrier_state/gangline/tick" \
  -mindepth 1 -maxdepth 1 -type d -print 2>/dev/null || true)"
equal "a public hitch launches one tick against the team it created" 1 \
  "$(printf '%s\n' "$tick_carrier_dirs" | awk 'NF { count++ } END { print count + 0 }')"
excludes "a public hitch discards an inherited internal team key" \
  "$tick_carrier_dirs" "/$tick_carrier_plant"
tmux kill-session -t "=$tick_carrier_session"

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
  tmux -S "\$GANG_TMUX_SOCKET" set-option -w -t "\$TMUX_PANE" \
    @gl_turn "closed \$(date +%s)"
  tmux wait-for -S "gang-tick-prompt-\${TMUX_PANE#%}"
  if [ -e "$tick_prompt_arm" ]; then
    rm -f -- "$tick_prompt_arm"
    printf x > "$tick_prompt_fifo"
  fi
}
PROMPT_COMMAND=tick_prompt
SH

tick_compacted="$RUN_ROOT/tick-compacted"
tick_cache_ledger="$RUN_ROOT/tick-cache-compactions"
tick_cache_stamp="$RUN_ROOT/tick-cache-transcript"
mkdir -p "$RUN_ROOT/collars"
export GANG_COLLARS="$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/tick-native.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="ENV='$RUN_ROOT/tick-bashrc' bash --posix"
GANG_STOP_HOOK=1
GANG_SELF_COMPACT=deferred
GANG_COMPACT_CMD="printf 'TICK_COMPACT\\n'; : > '$tick_compacted'"
SH
cat > "$RUN_ROOT/collars/tick-cache.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="ENV='$RUN_ROOT/tick-bashrc' bash --posix"
GANG_STOP_HOOK=1
GANG_COMPACT_CMD="printf '%s\\n' 'TICK_CACHE_COMPACT {{instructions}}' >> '$tick_cache_ledger'"
collar_context() { printf '80k/100k\\n'; }
collar_cache_stamp() {
  local file
  file="\$(tmux show-options -wqv -t "\$1" @gl_session)" || return 1
  [ -f "\$file" ] || return 1
  stat -c %Y -- "\$file"
}
SH
cat > "$RUN_ROOT/collars/tick-occupied-gone.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/tick-cache.sh"
GANG_OCCUPIED_REGEX='TICK_OCCUPIED_GONE'
SH
cat > "$RUN_ROOT/collars/tick-occupied-late.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/tick-native.sh"
GANG_BUSY_REGEX='TICK_OCCUPIED_LATE'
SH
cat > "$RUN_ROOT/collars/tick-codex-adopt.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/codex.sh"
# These stand-in processes exercise Codex's native identity reader, but none
# was launched with Codex's Stop hook. Keep that unavailable capability honest.
GANG_STOP_HOOK=
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

# A cache-expiry compaction is a tick-only action. The fixture's transcript
# stamp is deliberately older than the margin but younger than the TTL, while
# its native context source is over the first configured light. Each negative
# case changes exactly one eligibility fact from that ready state.
: > "$tick_cache_stamp"
GANG_CACHE_COMPACTION='tick-cache=120:90' "$HITCH" tick-cache -c tick-cache \
  -l 50000,75000 -d /tmp >/dev/null
tick_cache_id="$(window_id tick-cache)"
tmux set-option -w -t "$tick_cache_id" @gl_session "$tick_cache_stamp"
tick_cache_ready() {
  tmux set-option -w -t "$tick_cache_id" @gl_context_lights 50000,75000
  tmux set-option -w -t "$tick_cache_id" @gl_turn "closed $(date +%s)"
  tmux set-option -uw -t "$tick_cache_id" @gl_cache_compact_gap
  tmux set-option -uw -t "$tick_cache_id" @gl_cache_compact_pending
  touch -d '45 seconds ago' "$tick_cache_stamp"
}
tick_cache_count() { wc -l < "$tick_cache_ledger" | tr -d ' '; }
tick_cache_ready
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "an idle context inside its cache-expiry margin is compacted" 1 \
  "$(tick_cache_count)"
contains "without cache bands automatic compaction keeps its built-in instruction" \
  "$(sed -n '1p' "$tick_cache_ledger")" \
  'TICK_CACHE_COMPACT Keep the brief you were given, the durable state you have already written down, and what is still outstanding in your lane, including anything you were asked to report.'
tick_cache_digest="$(python3 -c 'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')" "$GANG_SESSION")"
tick_cache_journal="$XDG_STATE_HOME/gangline/tick/$tick_cache_digest/cache-compactions"
equal "the automatic compaction writes its bounded journal record" \
  '1 120 90' \
  "$(awk -F '\t' '$2 == "tick-cache" { print ($3 ~ /^[0-9]+$/), $4, $5 }' "$tick_cache_journal")"
contains "explain keeps the automatic compaction in a durable journal" \
  "$("$GANG" explain tick-cache)" "cache compactions:"
contains "explain renders the automatic compaction's cache age inputs" \
  "$("$GANG" explain tick-cache)" 'cache stamp '

# The automatic command can update a transcript as it makes the short summary.
# That is still the same idle gap, so settle the agent idle and require the
# stored mark to prevent a second compaction of that summary.
tmux set-option -w -t "$tick_cache_id" @gl_turn "closed $(date +%s)"
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "one idle gap receives at most one automatic compaction" 1 \
  "$(tick_cache_count)"

# A later request creates another idle gap. Its fresh transcript mtime is the
# generic, harness-owned witness; the tick needs no pane scrape or guessed
# request timestamp to permit the next near-expiry compaction.
touch -d '45 seconds ago' "$tick_cache_stamp"
tmux set-option -w -t "$tick_cache_id" @gl_turn "closed $(date +%s)"
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "a later idle gap may compact after its own cache approaches expiry" 2 \
  "$(tick_cache_count)"

# Cache-compaction configuration is settled when an agent is hitched, as
# context and usage lights are. A malformed environment supplied only to a
# later tick must therefore neither kill the pass nor alter this agent's
# already-selected TTL and margin.
tick_cache_ready
tick_cache_map_rc=0
GANG_CACHE_COMPACTION='not-a-cache-compaction-map' \
  GANG_TEST_TICK_MODE=sync "$GANG" tick >/dev/null || tick_cache_map_rc=$?
equal "a malformed ambient cache map does not abort a tick" 0 "$tick_cache_map_rc"
equal "the hitch-resolved cache entry still compacts inside its margin" 3 \
  "$(tick_cache_count)"

# A malformed once-per-gap marker is not an ordinary ineligible state. The
# tick must retain both the nonzero health result and its diagnostic instead of
# swallowing the helper's stderr while returning green.
tick_cache_ready
tmux set-option -w -t "$tick_cache_id" @gl_cache_compact_gap malformed
tick_cache_marker_rc=0
tick_cache_marker_out="$(GANG_TEST_TICK_MODE=sync "$GANG" tick 2>&1)" \
  || tick_cache_marker_rc=$?
equal "a malformed cache-compaction marker fails the tick" 1 \
  "$tick_cache_marker_rc"
contains "the malformed marker is visible in tick output" \
  "$tick_cache_marker_out" "tick cache compaction marker for tick-cache is malformed"
equal "a malformed marker never submits another automatic compaction" 3 \
  "$(tick_cache_count)"
tmux set-option -uw -t "$tick_cache_id" @gl_cache_compact_gap

tick_cache_ready
tmux set-option -w -t "$tick_cache_id" @gl_context_lights 95000,99000
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "context below the first band is not compacted" 3 "$(tick_cache_count)"

tick_cache_ready
tmux set-option -w -t "$tick_cache_id" @gl_turn "open $(date +%s)"
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "a busy agent is not compacted" 3 "$(tick_cache_count)"

tick_cache_ready
tick_cache_spool="$(tmux show-options -wqv -t "$tick_cache_id" @gl_spool)"
: > "$GANG_LOCK_DIR/spool/$tick_cache_spool/sending-cache-test"
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "a spool held mid-delivery blocks automatic compaction" 3 \
  "$(tick_cache_count)"
rm -f -- "$GANG_LOCK_DIR/spool/$tick_cache_spool/sending-cache-test"

tick_cache_ready
touch -d '121 seconds ago' "$tick_cache_stamp"
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "an already-cold cache is not compacted" 3 "$(tick_cache_count)"

"$GANG" drop tick-cache >/dev/null

# A custom-band candidate reaches the same tick reader as the legacy one. Put
# it before another ready cache candidate: an abort while reading the custom
# registration would leave the later candidate's compaction unsubmitted.
tick_bands_stamp="$RUN_ROOT/tick-bands-transcript"
tick_bands_after_stamp="$RUN_ROOT/tick-bands-after-transcript"
: > "$tick_bands_stamp"
: > "$tick_bands_after_stamp"
GANG_CONTEXT_BANDS='*=checkpoint@50%:Checkpoint {band}' \
  GANG_CACHE_COMPACTION='tick-cache=120:90' "$HITCH" tick-bands -c tick-cache \
  -d /tmp >/dev/null
GANG_CONTEXT_BANDS='*=checkpoint@50%:Checkpoint {band}' \
  GANG_CACHE_COMPACTION='tick-cache=120:90' "$HITCH" tick-bands-after -c tick-cache \
  -d /tmp >/dev/null
tick_bands_id="$(window_id tick-bands)"
tick_bands_after_id="$(window_id tick-bands-after)"
tmux set-option -w -t "$tick_bands_id" @gl_session "$tick_bands_stamp"
tmux set-option -w -t "$tick_bands_after_id" @gl_session "$tick_bands_after_stamp"
tmux set-option -w -t "$tick_bands_id" @gl_turn "closed $(date +%s)"
tmux set-option -w -t "$tick_bands_after_id" @gl_turn "closed $(date +%s)"
touch -d '45 seconds ago' "$tick_bands_stamp" "$tick_bands_after_stamp"
GANG_CACHE_COMPACTION='tick-cache=120:90' GANG_TEST_TICK_MODE=sync \
  "$GANG" tick >/dev/null
equal "a custom-band candidate leaves the later tick candidate reachable" 5 \
  "$(tick_cache_count)"
"$GANG" drop tick-bands >/dev/null
"$GANG" drop tick-bands-after >/dev/null

# The global opt-out is likewise resolved on the hitch that would otherwise
# arm the backstop. A later tick has no raw map to reinterpret.
tick_cache_off_stamp="$RUN_ROOT/tick-cache-off-transcript"
: > "$tick_cache_off_stamp"
GANG_CACHE_COMPACTION=off "$HITCH" tick-cache-off -c tick-cache \
  -l 50000,75000 -d /tmp >/dev/null
tick_cache_off_id="$(window_id tick-cache-off)"
tmux set-option -w -t "$tick_cache_off_id" @gl_session "$tick_cache_off_stamp"
tmux set-option -w -t "$tick_cache_off_id" @gl_turn "closed $(date +%s)"
touch -d '45 seconds ago' "$tick_cache_off_stamp"
GANG_TEST_TICK_MODE=sync "$GANG" tick >/dev/null
equal "the operator opt-out disables automatic compaction" 5 \
  "$(tick_cache_count)"
"$GANG" drop tick-cache-off >/dev/null

# CACHE BANDS CHANGE ONLY THE PRE-CACHE-EXPIRY DECISION. The first fixture is
# over the legacy context threshold but below its selected cache band; the
# second is below legacy context warnings but crosses two cache bands, so it
# both flips eligibility and proves that the highest crossed template replaces
# the built-in compaction instruction.
tick_cache_bands_low_stamp="$RUN_ROOT/tick-cache-bands-low-transcript"
tick_cache_bands_high_stamp="$RUN_ROOT/tick-cache-bands-high-transcript"
: > "$tick_cache_bands_low_stamp"
: > "$tick_cache_bands_high_stamp"
GANG_CACHE_BANDS='*=preserve@90%:Keep only the durable state.' \
  GANG_CACHE_COMPACTION='tick-cache=120:90' "$HITCH" tick-cache-bands-low \
  -c tick-cache -l 50000,75000 -d /tmp >/dev/null
tick_cache_bands_low_id="$(window_id tick-cache-bands-low)"
tmux set-option -w -t "$tick_cache_bands_low_id" @gl_session "$tick_cache_bands_low_stamp"
tmux set-option -w -t "$tick_cache_bands_low_id" @gl_turn "closed $(date +%s)"
touch -d '45 seconds ago' "$tick_cache_bands_low_stamp"
GANG_TEST_TICK_MODE=sync "$GANG" tick >/dev/null
equal "an active cache-band map does not fall back to a crossed context warning" 5 \
  "$(tick_cache_count)"

GANG_CACHE_BANDS='*=checkpoint@50%:Keep checkpoint state.|urgent@75%:Keep urgent state.' \
  GANG_CACHE_COMPACTION='tick-cache=120:90' "$HITCH" tick-cache-bands-high \
  -c tick-cache -l 95000,99000 -d /tmp >/dev/null
tick_cache_bands_high_id="$(window_id tick-cache-bands-high)"
tmux set-option -w -t "$tick_cache_bands_high_id" @gl_session "$tick_cache_bands_high_stamp"
tmux set-option -w -t "$tick_cache_bands_high_id" @gl_turn "closed $(date +%s)"
touch -d '45 seconds ago' "$tick_cache_bands_high_stamp"
GANG_TEST_TICK_MODE=sync "$GANG" tick >/dev/null
equal "a crossed cache band can compact below the first context warning" 6 \
  "$(tick_cache_count)"
equal "the highest crossed cache band supplies the compaction instruction" \
  'TICK_CACHE_COMPACT Keep urgent state.' "$(sed -n '6p' "$tick_cache_ledger")"
"$GANG" drop tick-cache-bands-low >/dev/null
"$GANG" drop tick-cache-bands-high >/dev/null

# A CACHE POLICY THAT CANNOT BE READ IS NOT AN ORDINARY BELOW-THRESHOLD
# result. The operator explicitly selected it for a cache-expiry decision, so
# each broken native reading fails the cooperative pass loudly and leaves the
# reason in `gang explain`; none may silently disable preservation forever.
cat > "$RUN_ROOT/collars/tick-cache-diagnostic.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="ENV='$RUN_ROOT/tick-bashrc' bash --posix"
GANG_STOP_HOOK=1
GANG_COMPACT_CMD="printf '%s\\n' 'TICK_CACHE_DIAGNOSTIC {{instructions}}' >> '$tick_cache_ledger'"
collar_context() { tmux show-options -wqv -t "\$1" @gl_test_context; }
collar_cache_stamp() {
  local file
  file="\$(tmux show-options -wqv -t "\$1" @gl_session)" || return 1
  [ -f "\$file" ] || return 1
  stat -c %Y -- "\$file"
}
SH
tick_cache_zero_stamp="$RUN_ROOT/tick-cache-zero-transcript"
tick_cache_unreachable_stamp="$RUN_ROOT/tick-cache-unreachable-transcript"
tick_cache_unreadable_stamp="$RUN_ROOT/tick-cache-unreadable-transcript"
: > "$tick_cache_zero_stamp"
: > "$tick_cache_unreachable_stamp"
: > "$tick_cache_unreadable_stamp"
GANG_CACHE_BANDS='*=zero@50%:Keep zero-window state.' \
  GANG_CACHE_COMPACTION='tick-cache-diagnostic=120:90' "$HITCH" tick-cache-zero \
  -c tick-cache-diagnostic -l off -d /tmp >/dev/null
GANG_CACHE_BANDS='*=unreachable@120000:Keep unreachable state.' \
  GANG_CACHE_COMPACTION='tick-cache-diagnostic=120:90' "$HITCH" tick-cache-unreachable \
  -c tick-cache-diagnostic -l off -d /tmp >/dev/null
GANG_CACHE_BANDS='*=unreadable@50%:Keep unreadable state.' \
  GANG_CACHE_COMPACTION='tick-cache-diagnostic=120:90' "$HITCH" tick-cache-unreadable \
  -c tick-cache-diagnostic -l off -d /tmp >/dev/null
tick_cache_zero_id="$(window_id tick-cache-zero)"
tick_cache_unreachable_id="$(window_id tick-cache-unreachable)"
tick_cache_unreadable_id="$(window_id tick-cache-unreadable)"
tmux set-option -w -t "$tick_cache_zero_id" @gl_session "$tick_cache_zero_stamp"
tmux set-option -w -t "$tick_cache_unreachable_id" @gl_session "$tick_cache_unreachable_stamp"
tmux set-option -w -t "$tick_cache_unreadable_id" @gl_session "$tick_cache_unreadable_stamp"
tmux set-option -w -t "$tick_cache_zero_id" @gl_test_context '80k/0k'
tmux set-option -w -t "$tick_cache_unreachable_id" @gl_test_context '80k/100k'
tmux set-option -uw -t "$tick_cache_unreadable_id" @gl_test_context
tmux set-option -w -t "$tick_cache_zero_id" @gl_turn "closed $(date +%s)"
tmux set-option -w -t "$tick_cache_unreachable_id" @gl_turn "closed $(date +%s)"
tmux set-option -w -t "$tick_cache_unreadable_id" @gl_turn "closed $(date +%s)"
touch -d '45 seconds ago' "$tick_cache_zero_stamp" "$tick_cache_unreachable_stamp" \
  "$tick_cache_unreadable_stamp"
tick_cache_diagnostic_rc=0
tick_cache_diagnostic_out="$(GANG_TEST_TICK_MODE=sync "$GANG" tick 2>&1)" \
  || tick_cache_diagnostic_rc=$?
equal "an invalid selected cache band fails the cooperative tick" 1 \
  "$tick_cache_diagnostic_rc"
contains "a zero native context window names the cache-band failure" \
  "$tick_cache_diagnostic_out" \
  "tick cache bands for tick-cache-zero: Cache bands invalid: this harness's native context source reported a zero-token window.; refusing automatic compaction"
contains "an unreachable absolute cache band names its threshold" \
  "$tick_cache_diagnostic_out" \
  "tick cache bands for tick-cache-unreachable: Cache bands invalid: configured 'unreachable' threshold 120000 cannot fire in this harness's 100000-token window. Re-hitch with reachable token thresholds or percentages.; refusing automatic compaction"
contains "an unreadable native cache source fails loud" \
  "$tick_cache_diagnostic_out" \
  "tick cache bands for tick-cache-unreadable: Gangline cannot read or interpret this harness's native context source; refusing automatic compaction"
contains "explain preserves a cache-band decision failure" \
  "$("$GANG" explain tick-cache-unreadable)" \
  "tick action: automatic cache compaction refused: cache bands Gangline cannot read or interpret this harness's native context source"
equal "invalid cache-band policies never submit a compaction" 6 \
  "$(tick_cache_count)"
for tick_cache_diagnostic_agent in tick-cache-zero tick-cache-unreachable tick-cache-unreadable; do
  "$GANG" drop "$tick_cache_diagnostic_agent" >/dev/null
done

# Codex 0.151.0 draws the provider wait chooser over its composer while the
# turn itself remains live. This stand-in speaks the same terminal contract:
# bracketed paste, a Codex-shaped composer, the narrow hard-wrapped chooser,
# and the bare numeric shortcut its non-search selection view accepts. The
# pane-side key ledger distinguishes choosing option 2 from merely waiting for
# Codex to close the menu itself, while the delivery marker is written only
# after the fake TUI consumes the submitted envelope.
codex_queue_evidence_probe="$RUN_ROOT/codex-queue-evidence-probe.sh"
codex_queue_evidence_file="$RUN_ROOT/codex-queue-evidence"
codex_queue_evidence_prefix='[gang:self-declared:tester#0123456789abcdef'
codex_queue_evidence_body="$codex_queue_evidence_prefix reply-to=0000000000000000,1111111111111111,2222222222222222,3333333333333333,4444444444444444] BODY [/gang:self-declared:tester#0123456789abcdef]"
awk '
  /^queued_envelope_confirmed\(\)/ { keep=1 }
  keep { print }
  keep && /^}/ { exit }
' "$GANG" > "$codex_queue_evidence_probe"
cat >> "$codex_queue_evidence_probe" <<'SH'
collar_queued() { printf '%s' "$2" > "$CODEX_QUEUE_EVIDENCE_FILE"; }
queued_envelope_confirmed '%1' "$CODEX_QUEUE_EVIDENCE_BODY"
SH
CODEX_QUEUE_EVIDENCE_FILE="$codex_queue_evidence_file" \
  CODEX_QUEUE_EVIDENCE_BODY="$codex_queue_evidence_body" \
  bash "$codex_queue_evidence_probe"
equal "queue confirmation excludes a five-nonce reply-to suffix from its wrap-safe evidence" \
  "$codex_queue_evidence_prefix" "$(<"$codex_queue_evidence_file")"

codex_menu_channel="gang-codex-menu-${tick_caller_pane#%}"
codex_menu_dismissed="${codex_menu_channel}-dismissed"
codex_menu_keys="$RUN_ROOT/codex-menu.keys"
codex_menu_delivered="$RUN_ROOT/codex-menu.delivered"
cat > "$RUN_ROOT/codex-menu-fixture.py" <<'PY'
#!/usr/bin/env python3
import os
import subprocess
import sys
import termios
import textwrap
import tty

channel, dismissed_channel, key_log, delivered = sys.argv[1:]
fd = sys.stdin.fileno()
saved = termios.tcgetattr(fd)
tty.setraw(fd)
buffer = ""
transcript = []
menu = None
busy = False
queued_body = None
ordinary_next = False
replace_next_queue = False
paste = False
escape = b""


def render():
    out = ["\x1b[2J\x1b[H", "\x1b[?2004h"]
    out.extend(line + "\r\n" for line in transcript[-3:])
    if menu == "provider":
        out.extend(
            line + "\r\n"
            for line in (
                "esc to interrupt",
                "Our systems are thinking a bit more about",
                "this request before responding.",
                "Hang tight or retry with a faster model for a",
                "quicker response, though it may be less capable",
                "of handling complex requests.",
                "› 1. Retry with a faster model",
                "  2. Dismiss and keep",
                "     waiting",
                "  3. Learn more",
                "No action is required. Codex will keep waiting,",
                "and this menu will close when the response is",
                "ready.",
            )
        )
    elif menu == "other_history":
        out.extend(
            line + "\r\n"
            for line in (
                "Our systems are thinking a bit more about this request before responding.",
                "› 1. Retry with a faster model",
                "  2. Dismiss and keep waiting",
                "  3. Learn more",
                "No action is required. Codex will keep waiting, and this menu will close when the response is ready.",
                "Unrelated numbered chooser",
                "› 1. Keep the current setting",
                "  2. Change the current setting",
                "  3. Learn more",
            )
        )
    elif menu == "no_retry":
        out.extend(
            line + "\r\n"
            for line in (
                "esc to interrupt",
                "Our systems are thinking a bit more about",
                "this request before responding.",
                "› 1. Dismiss and keep waiting",
                "  2. Learn more",
                "No action is required. Codex will keep waiting,",
                "and this menu will close when the response is ready.",
            )
        )
    elif menu == "rate_limit":
        out.extend(
            line + "\r\n"
            for line in (
                "esc to interrupt",
                "Approaching rate limits",
                "Switch to gpt-5-codex-mini for lower credit usage?",
                "  1. Switch to gpt-5-codex-mini",
                "› 2. Keep current model",
                "Press enter to confirm or esc to go back",
            )
        )
    elif menu == "unmarked_history":
        out.extend(
            line + "\r\n"
            for line in (
                "Our systems are thinking a bit more about this request before responding.",
                "› 1. Retry with a faster model",
                "  2. Dismiss and keep waiting",
                "  3. Learn more",
                "No action is required. Codex will keep waiting, and this menu will close when the response is ready.",
                "Tell us more about what happened",
                "Press Enter to submit feedback or Esc to cancel",
            )
        )
    else:
        if queued_body is not None:
            queued_lines = []
            width = max(1, os.get_terminal_size(fd).columns - 4)
            for line in queued_body.splitlines() or [""]:
                queued_lines.extend(
                    textwrap.wrap(
                        line,
                        width=width,
                        break_long_words=True,
                        break_on_hyphens=True,
                        replace_whitespace=False,
                    )
                    or [""]
                )
            out.append("• Queued follow-up inputs\r\n")
            if queued_lines:
                out.append("  ↳ " + queued_lines[0] + "\r\n")
                out.extend("    " + line + "\r\n" for line in queued_lines[1:3])
                if len(queued_lines) > 3:
                    out.append("    …\r\n")
            out.append("shift + ← edit last queued message\r\n")
        if busy:
            out.append("esc to interrupt\r\n")
        lines = buffer.split("\n")
        out.append("›" + (" " + lines[0] if lines[0] else "") + "\r\n")
        out.extend("  " + line + "\r\n" for line in lines[1:])
    sys.stdout.write("".join(out))
    sys.stdout.flush()


def signal_ready():
    subprocess.run(["tmux", "wait-for", "-S", channel], check=True)


def signal_dismissed():
    subprocess.run(["tmux", "wait-for", "-S", dismissed_channel], check=True)


def submit():
    global buffer, menu, busy, queued_body, ordinary_next, replace_next_queue
    body = buffer
    buffer = ""
    if body == "SHOW_PROVIDER_MENU":
        menu = "provider"
        busy = True
        render()
        signal_ready()
        return
    if body == "SHOW_PROVIDER_END_MENU":
        menu = "provider"
        busy = True
        ordinary_next = True
        render()
        signal_ready()
        return
    if body == "SHOW_PROVIDER_RACE_MENU":
        menu = "provider"
        busy = True
        replace_next_queue = True
        render()
        signal_ready()
        return
    if body == "SHOW_OTHER_MENU":
        menu = "other_history"
        busy = False
        render()
        signal_ready()
        return
    if body == "SHOW_NO_RETRY_MENU":
        menu = "no_retry"
        busy = True
        render()
        signal_ready()
        return
    if body == "SHOW_RATE_LIMIT_MENU":
        menu = "rate_limit"
        busy = True
        render()
        signal_ready()
        return
    if body == "SHOW_UNMARKED_MENU":
        menu = "unmarked_history"
        busy = True
        render()
        signal_ready()
        return
    if body == "CLEAR_BUSY":
        busy = False
        queued_body = None
        render()
        signal_ready()
        return
    if body == "DRAIN_QUEUE":
        busy = False
        queued_body = None
        render()
        signal_ready()
        return
    transcript.append("accepted input")
    if "DELIVERY" in body:
        if replace_next_queue:
            replace_next_queue = False
            queued_body = "[gang:tester#1111111111111111] an earlier queued message [/gang:tester#1111111111111111]"
        else:
            with open(delivered, "w", encoding="utf-8") as stream:
                stream.write(body)
            if ordinary_next:
                ordinary_next = False
                busy = False
                queued_body = None
            elif busy:
                queued_body = body
            else:
                busy = False
    render()


try:
    render()
    while True:
        char = os.read(fd, 1)
        if not char:
            break
        if menu is not None:
            if char.isdigit():
                with open(key_log, "ab") as stream:
                    stream.write(char)
                if (menu == "no_retry" and char == b"1") or (
                    menu != "no_retry" and char == b"2"
                ):
                    menu = None
                    render()
                    signal_dismissed()
            continue
        if paste:
            escape = (escape + char)[-6:]
            if escape.endswith(b"\x1b[201~"):
                paste = False
                buffer = buffer[:-5]
                escape = b""
                render()
            else:
                buffer += char.decode("utf-8", "replace")
            continue
        escape = (escape + char)[-6:]
        if escape.endswith(b"\x1b[200~"):
            paste = True
            buffer = buffer[:-5]
            escape = b""
        elif char in (b"\r", b"\n"):
            escape = b""
            submit()
        elif char in (b"\x7f", b"\x08"):
            escape = b""
            buffer = buffer[:-1]
            render()
        elif char >= b" ":
            buffer += char.decode("utf-8", "replace")
            render()
finally:
    termios.tcsetattr(fd, termios.TCSADRAIN, saved)
PY
chmod +x "$RUN_ROOT/codex-menu-fixture.py"
: > "$codex_menu_keys"
cat > "$RUN_ROOT/collars/tick-codex-menu.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/codex.sh"
GANG_LAUNCH="'$RUN_ROOT/codex-menu-fixture.py' '$codex_menu_channel' '$codex_menu_dismissed' '$codex_menu_keys' '$codex_menu_delivered'"
GANG_RESUME_LAUNCH=
GANG_STOP_HOOK=1
GANG_SELF_COMPACT=
GANG_SELF_COMPACT_WITNESS=
SH
"$HITCH" codex-menu -c tick-codex-menu -d "$RUN_ROOT" >/dev/null
codex_menu_id="$(window_id codex-menu)"
codex_menu_pane="$(tmux list-panes -t "$codex_menu_id" -F '#{pane_id}')"
# This collar advertises the real Codex Stop hook, so its fake drives the same
# turn bracket around each simulated provider turn. Close the hitch contract's
# submitted turn first, then open the turn whose wait menu is under test.
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null

# The dismissal action belongs to the retry-capable advisory, not to a queued
# envelope. First hold that menu with an empty Gangline spool and ask the
# cooperative pass directly. Keep the fixture answer below only as fails-first
# cleanup: it cannot make either assertion green because the key ledger is read
# before the cleanup runs.
: > "$codex_menu_keys"
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_PROVIDER_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
equal "the retry-capable provider menu starts with no Gangline mail" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
GANG_TEST_TICK_MODE=sync "$GANG" roster >/dev/null
equal "a team command dismisses the provider menu without pending mail" 2 \
  "$(<"$codex_menu_keys")"
if [ ! -s "$codex_menu_keys" ]; then
  tmux wait-for "$codex_menu_dismissed" &
  codex_menu_dismissed_waiter=$!
  tmux send-keys -t "$codex_menu_id" 2
  wait "$codex_menu_dismissed_waiter"
fi

# A native boundary on another agent launches the same global cooperative
# pass. The actionable menu has no attributed mail, so its collar declaration
# alone must make the pass visit it.
: > "$codex_menu_keys"
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_PROVIDER_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
equal "the second retry-capable menu also has no Gangline mail" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=sync TMUX_PANE="$tick_copy_pane" "$GANG" hook >/dev/null
equal "another agent's native hook dismisses the provider menu without pending mail" 2 \
  "$(<"$codex_menu_keys")"
if [ ! -s "$codex_menu_keys" ]; then
  tmux wait-for "$codex_menu_dismissed" &
  codex_menu_dismissed_waiter=$!
  tmux send-keys -t "$codex_menu_id" 2
  wait "$codex_menu_dismissed_waiter"
fi

: > "$codex_menu_keys"
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_PROVIDER_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf '%s' 'CODEX_MENU_DELIVERY is an ordinary peer message long enough to cross Codex 0.151.0 queue preview limit after wrapping. Its trailing prose must be absent from the three retained rows while the unique Gangline attribution prefix at the head remains visible and proves which single pasted bundle entered the follow-up queue.' \
  | "$GANG" send --to codex-menu --from tester --stdin >/dev/null
equal "the provider wait menu holds delivery before the cooperative tick" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
equal "no chooser key is spent while the delivery is only parked" "" \
  "$(<"$codex_menu_keys")"
codex_menu_turn_before="$(tmux show-options -wqv -t "$codex_menu_id" @gl_turn)"
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
"$GANG" tick >/dev/null
wait "$codex_menu_dismissed_waiter"
equal "the cooperative tick chooses only the provider menu's keep-waiting option" 2 \
  "$(<"$codex_menu_keys")"
equal "the truncated provider-menu delivery reaches the fake Codex TUI after dismissal" present \
  "$([ -e "$codex_menu_delivered" ] && printf present || printf absent)"
equal "the delivered provider-menu message leaves no Gangline spool entry" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
codex_menu_queue_pane="$("$GANG" capture codex-menu)"
contains "the fake Codex queue applies its three-row overflow marker" \
  "$codex_menu_queue_pane" "…"
excludes "the truncated queue does not render the whole body used for confirmation" \
  "$codex_menu_queue_pane" "proves which single pasted bundle"
equal "the queued mid-turn delivery does not invent a new turn edge" \
  "$codex_menu_turn_before" \
  "$(tmux show-options -wqv -t "$codex_menu_id" @gl_turn)"
contains "explain records the cooperative tick's provider-menu action" \
  "$("$GANG" explain codex-menu)" \
  "tick action: dismissed Codex's provider wait menu with option 2"

# The real Codex mid-turn Enter lands in its native follow-up queue. Retire the
# fake queue after the assertions so later negative menus start from a clean
# composer; the successful delivery above has already proved the queue body.
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" DRAIN_QUEUE
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null

# Keep the remainder of the fails-first run observable even on the unfixed
# tree: after the assertions above have recorded the held delivery, answer the
# fixture by hand and let the next tick retire it.
if [ "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')" -ne 0 ]; then
  tmux wait-for "$codex_menu_dismissed" &
  codex_menu_dismissed_waiter=$!
  tmux send-keys -t "$codex_menu_id" 2
  wait "$codex_menu_dismissed_waiter"
  tmux wait-for "$codex_menu_channel" &
  codex_menu_waiter=$!
  tmux send-keys -l -t "$codex_menu_id" CLEAR_BUSY
  tmux send-keys -t "$codex_menu_id" Enter
  wait "$codex_menu_waiter"
  "$GANG" tick >/dev/null
fi

# The provider turn can end after the dismissal reading but before Gangline's
# Enter. That is an ordinary new submission, not a native-queue landing: its
# verified no-queue fall-through must acquire the turn edge that was deferred
# while the post-Enter surface was still unknown.
: > "$codex_menu_keys"
: > "$codex_menu_delivered"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_PROVIDER_END_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf '%s' 'TURN_EDGE_DELIVERY whose provider turn ends after dismissal and before Enter' \
  | "$GANG" send --to codex-menu --from tester --stdin >/dev/null
codex_menu_turn_before="$(tmux show-options -wqv -t "$codex_menu_id" @gl_turn)"
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
"$GANG" tick >/dev/null
wait "$codex_menu_dismissed_waiter"
equal "the ending provider turn still spends only option 2" 2 \
  "$(<"$codex_menu_keys")"
contains "the post-dismissal ordinary delivery reaches Codex" \
  "$(<"$codex_menu_delivered")" "TURN_EDGE_DELIVERY"
if [ "$codex_menu_turn_before" != "$(tmux show-options -wqv -t "$codex_menu_id" @gl_turn)" ]; then
  pass "the verified no-queue landing records its real turn edge"
else
  fail "the verified no-queue landing records its real turn edge" \
    "the turn record stayed [$codex_menu_turn_before]"
fi
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null

: > "$codex_menu_keys"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_OTHER_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf OTHER_NUMBERED_DELIVERY \
  | "$GANG" send --to codex-menu --from tester --stdin >/dev/null
"$GANG" tick >/dev/null
equal "provider text in scrollback cannot answer the unrelated live menu" "" \
  "$(<"$codex_menu_keys")"
equal "the unrelated live menu keeps its delivery parked" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
tmux send-keys -t "$codex_menu_id" 2
wait "$codex_menu_dismissed_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
"$GANG" tick >/dev/null
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null

: > "$codex_menu_keys"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_UNMARKED_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf UNMARKED_MENU_DELIVERY \
  | "$GANG" send --to codex-menu --from tester --stdin >/dev/null
"$GANG" tick >/dev/null
equal "provider text above an unmarked live view cannot spend a key" "" \
  "$(<"$codex_menu_keys")"
equal "the unmarked live view keeps its delivery parked" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
tmux send-keys -t "$codex_menu_id" 2
wait "$codex_menu_dismissed_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
"$GANG" tick >/dev/null
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null

: > "$codex_menu_keys"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_NO_RETRY_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf NO_RETRY_MENU_DELIVERY \
  | "$GANG" send --to codex-menu --from tester --stdin >/dev/null
"$GANG" tick >/dev/null
equal "the no-retry provider menu never spends its Learn-more option 2" "" \
  "$(<"$codex_menu_keys")"
equal "the no-retry provider menu keeps its delivery parked" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
tmux send-keys -t "$codex_menu_id" 1
wait "$codex_menu_dismissed_waiter"
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" CLEAR_BUSY
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
"$GANG" tick >/dev/null

# The approaching-rate-limit chooser changes the active model. Even though an
# actionable collar makes an empty-spool pass visit this pane, that different
# menu cannot authorize its currently selected option 2 or any other key.
: > "$codex_menu_keys"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_RATE_LIMIT_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
equal "the approaching-rate-limit menu starts with no Gangline mail" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "codex-menu" { print $4 }')"
excludes "the approaching-rate-limit chooser is not reported as an advisory" \
  "$("$GANG" status codex-menu)" "!occupied! (advisory:"
"$GANG" tick >/dev/null
equal "the approaching-rate-limit menu never spends its selected option 2" "" \
  "$(<"$codex_menu_keys")"
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
tmux send-keys -t "$codex_menu_id" 2
wait "$codex_menu_dismissed_waiter"
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" CLEAR_BUSY
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null

# A different queued envelope can win the narrow interval between inject's
# empty preflight and its post-Enter queue reading. Its unique attribution tag
# must not confirm this claim. Replacing tag confirmation with the old
# park_record shortcut makes this fixture falsely retire the spool.
: > "$codex_menu_keys"
: > "$codex_menu_delivered"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$codex_menu_pane" "$GANG" hook >/dev/null
tmux wait-for "$codex_menu_channel" &
codex_menu_waiter=$!
tmux send-keys -l -t "$codex_menu_id" SHOW_PROVIDER_RACE_MENU
tmux send-keys -t "$codex_menu_id" Enter
wait "$codex_menu_waiter"
printf '%s' 'QUEUE_RACE_DELIVERY must not be credited when an older envelope occupies the native queue instead' \
  | "$GANG" send --to codex-menu --from tester --stdin >/dev/null
tmux wait-for "$codex_menu_dismissed" &
codex_menu_dismissed_waiter=$!
codex_menu_race_rc=0
codex_menu_race_out="$("$GANG" tick 2>&1)" || codex_menu_race_rc=$?
wait "$codex_menu_dismissed_waiter"
equal "a mismatched queued attribution makes the cooperative tick fail closed" 1 \
  "$codex_menu_race_rc"
contains "the mismatched queue reports an unknown submission outcome" \
  "$codex_menu_race_out" "submission outcome unknown"
contains "status retains the queue-race failure for the operator" \
  "$("$GANG" status codex-menu)" "spool drain NOT verified"
equal "the raced delivery was not accepted by the fake Codex TUI" absent \
  "$([ ! -s "$codex_menu_delivered" ] && printf absent || printf present)"
"$GANG" drop codex-menu >/dev/null

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
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$tick_real_tmux' "\$0" tmux || exit \$?
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
tick_copy_roster="$("$GANG" roster)"
contains "copy-mode is surfaced as generic operator-owned pane state" \
  "$tick_copy_roster" "pane-mode=active (attach and leave it before continuing)"
excludes "copy-mode is not prescribed a mode-specific recovery key" \
  "$tick_copy_roster" "send-keys -X cancel"
equal "neither copy-mode action typed before a later invocation" absent \
  "$([ ! -e "$tick_compacted" ] && printf absent || printf present)"

# pane_in_mode is intentionally only a portable ownership bit. clock-mode sets
# the same bit as copy-mode but rejects copy-mode's cancel command, so roster
# must not invent either a mode name or a keystroke it cannot prove. The
# independent tick-mode window lets this proof end by removing its disposable
# window; the product deliberately gives no mode-specific exit command.
tmux clock-mode -t "$tick_mode_id"
equal "clock-mode also exposes tmux pane ownership" 1 \
  "$(tmux display-message -p -t "$tick_mode_id" '#{pane_in_mode}')"
tick_clock_roster="$("$GANG" roster)"
contains "clock-mode receives the same generic pane-mode report" \
  "$tick_clock_roster" "pane-mode=active (attach and leave it before continuing)"
excludes "clock-mode is never mislabelled as copy-mode" \
  "$tick_clock_roster" "pane-mode=copy-mode"
excludes "clock-mode is never given copy-mode's cancel key" \
  "$tick_clock_roster" "send-keys -X cancel"
"$GANG" drop tick-mode >/dev/null
tick_clock_removed="$(tmux list-windows -F '#{window_id}\t#{@gl_agent}' \
  | awk -F '\t' '$2 == "tick-mode" { found=1 } END { print found ? "present" : "absent" }')"
equal "the disposable clock-mode proof is removed before later work" absent \
  "$tick_clock_removed"

# PostCompact is the first boundary after a deferred request, but copy-mode is
# still a dialog owned by tmux. The request must survive that failed attempt,
# name the dialog on roster, and need no fresh Stop before the next boundary
# retries it.
tick_copy_request="$(tmux show-options -wqv -t "$tick_copy_id" @gl_self_compact_requested)"
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_copy_pane" "$GANG" hook >/dev/null
tmux wait-for "gang-self-compact-$tick_copy_request"
equal "a dialog-blocked PostCompact keeps the same self-compaction request" \
  "$tick_copy_request" \
  "$(tmux show-options -wqv -t "$tick_copy_id" @gl_self_compact_requested)"
contains "roster identifies the dialog blocking the deferred self-compaction" \
  "$("$GANG" roster)" "self-compact-blocked=dialog"

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
# A deferred compact that met copy-mode has no new Stop to rescue it. The
# native PostCompact boundary below is the next directly witnessed opportunity:
# it must retry the standing request before peer mail, without waiting for a
# cooperative patrol or an operator paste.
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_copy_pane" "$GANG" hook >/dev/null
# The worker's own completion event lands after its lock and dispatching marker
# are released. It proves the native retry reached a terminal decision, unlike
# a command-local marker that could fire before the worker's cleanup.
tmux wait-for "gang-self-compact-$tick_copy_request"
equal "PostCompact retries the compact deferred behind copy-mode" present \
  "$([ -e "$tick_compacted" ] && printf present || printf absent)"
excludes "the native retry clears its standing self-compaction request" \
  "$("$GANG" status tick-copy)" "self-compaction requested"
tick_cross_rc=0
TMUX_PANE="$tick_caller_pane" GANG_TEST_TICK_MODE=sync \
  "$GANG" whoami >/dev/null || tick_cross_rc=$?
equal "a command from another window keeps its own successful result" 0 "$tick_cross_rc"
# The native boundary owned the compaction. The next cooperative pass may now
# spend the older peer message; it must not rediscover a request that boundary
# already completed.
equal "the post-compaction pass drains the formerly copy-mode-held message" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-copy" { print $4 }')"
equal "one global pass also drains the other hitched window" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-false" { print $4 }')"
contains "the closed native turn beats false occupied paint for delivery" \
  "$(pane_all tick-false)" "TICK_FALSE_OCCUPIED_MESSAGE"
equal "the completed pass retires every tick delivery owner marker" absent \
  "$(if [ -n "$(tmux show-options -wqv -t "$tick_copy_id" @gl_tick_delivery)$(tmux show-options -wqv -t "$tick_false_id" @gl_tick_delivery)" ]; then printf present; else printf absent; fi)"
contains "the copy-mode message reached the recipient after native retry" \
  "$(pane_all tick-copy)" "TICK_COPY_MESSAGE"

# tick visits collars in one shell. A preceding recap-aware collar must not
# lend its optional callback to the following deferred collar, or the latter
# strands its continuation in a spool for a recap boundary it cannot raise.
tick_follow_compacted="$RUN_ROOT/tick-follow-compacted"
cat > "$RUN_ROOT/collars/tick-recap-first.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_recap_boundary() { return 1; }
SH
cat > "$RUN_ROOT/collars/tick-recap-follow.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/tick-native.sh"
GANG_COMPACT_CMD=": > '$tick_follow_compacted'"
SH
"$HITCH" tick-recap-first -c tick-recap-first -d /tmp >/dev/null
"$HITCH" tick-recap-follow -c tick-recap-follow -d /tmp >/dev/null
tick_follow_id="$(window_id tick-recap-follow)"
tick_follow_pane="$(tmux list-panes -t "$tick_follow_id" -F '#{pane_id}')"
TMUX_PANE="$tick_follow_pane" "$GANG" compact --resume 'TICK_FOLLOW_CONTINUATION' >/dev/null
"$GANG" tick >/dev/null
equal "a non-recap collar compacts after a recap-aware collar in one tick" present \
  "$([ -e "$tick_follow_compacted" ] && printf present || printf absent)"
equal "the following non-recap collar takes its immediate continuation path" 0 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "tick-recap-follow" { print $4 }')"
contains "the following non-recap collar receives its continuation without a recap" \
  "$(pane_all tick-recap-follow)" "TICK_FOLLOW_CONTINUATION"
"$GANG" drop tick-recap-first >/dev/null
"$GANG" drop tick-recap-follow >/dev/null

# A CLEARED CONDITION LEAVES THE STATUS BAR WITHIN ONE TICK. A permission
# request paints !name! and may be the last event its dialog ever sends: a
# person who answers or declines it raises nothing further. The composer coming
# back is the clearing evidence, so the cooperative pass must read it and
# repaint the window, and the transition journal must name both edges by the
# path that wrote them. The startup prompt stays free of hook work, exactly as
# for the fixtures above, until hitch has verified its contract.
rm -f -- "$tick_prompt_enable"
"$HITCH" tick-glyph -c tick-native -d /tmp >/dev/null
tick_glyph_id="$(window_id tick-glyph)"
tick_glyph_pane="$(tmux list-panes -t "$tick_glyph_id" -F '#{pane_id}')"
: > "$tick_prompt_enable"
tmux wait-for "gang-tick-prompt-${tick_glyph_pane#%}" &
tick_glyph_prompt_waiter=$!
tmux send-keys -t "$tick_glyph_id" Enter
wait "$tick_glyph_prompt_waiter"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "the closed native turn paints the glyph fixture idle" '~tick-glyph~' \
  "$(tmux display-message -p -t "$tick_glyph_id" '#{window_name}')"
printf '%s' '{"hook_event_name":"PermissionRequest"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "a native permission request paints the window occupied" '!tick-glyph!' \
  "$(tmux display-message -p -t "$tick_glyph_id" '#{window_name}')"
contains "and raises the event-tier occupied fact" \
  "$(tmux show-options -wqv -t "$tick_glyph_id" @gl_occupied)" "open "
tick_glyph_rc=0
"$GANG" tick >/dev/null || tick_glyph_rc=$?
equal "the refreshing pass itself succeeds" 0 "$tick_glyph_rc"
equal "one cooperative tick repaints the answered window idle" '~tick-glyph~' \
  "$(tmux display-message -p -t "$tick_glyph_id" '#{window_name}')"
equal "the same pass retires the raise the live composer answered" "" \
  "$(tmux show-options -wqv -t "$tick_glyph_id" @gl_occupied)"

# The journal holds fixed state words only. A parenthetical detail can quote
# pane text or a native session identity, so any field outside the grammar is
# a leak, and the whole team's journal is checked, not only this fixture's.
tick_glyph_digest="$(python3 -c 'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')" "$GANG_SESSION")"
tick_glyph_journal="$XDG_STATE_HOME/gangline/tick/$tick_glyph_digest/transitions"
tick_glyph_edges="no journal"
tick_glyph_bad="no journal"
if [ -f "$tick_glyph_journal" ]; then
  tick_glyph_edges="$(awk -F '\t' '$2 == "tick-glyph" { print $3, $4, $5 }' \
    "$tick_glyph_journal" | tail -n 2)"
  tick_glyph_bad="$(awk -F '\t' '
    function word(s) {
      return s ~ /^(none|-busy-|~wait~|~idle~|!occupied!|!dead!|!bricked!|!blocked!|!session-lost!|!harness-lost!|[?]unknown[?])$/
    }
    !(NF == 5 \
      && $1 ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z$/ \
      && $2 ~ /^[A-Za-z0-9._-]+$/ && word($3) && word($4) && $3 != $4 \
      && $5 ~ /^[a-z-]+$/) { bad++ }
    END { print bad + 0 }' "$tick_glyph_journal")"
fi
equal "the journal records the hook's occupied edge, then the tick's idle edge" \
  $'~idle~ !occupied! hook\n!occupied! ~idle~ tick' "$tick_glyph_edges"
equal "every journal line is a five-field record of fixed state words" 0 \
  "$tick_glyph_bad"
contains "explain shows the agent's recent transitions with their source" \
  "$("$GANG" explain tick-glyph)" '!occupied! -> ~idle~ (tick)'

# A JOURNAL AT ITS BOUND ROTATES AND KEEPS ITS OLDER GENERATION READABLE. The
# bound is read from the script so the fixture crosses exactly the size it
# enforces, and blank filler lines carry no record for explain to show. The
# rotation is decided under the team's journal claim: while a live process
# holds that claim, the writer that crosses the bound appends its line and
# leaves the move to the holder, and the first edge after the claim is gone
# makes the move.
tick_glyph_bound="$(awk -F= '$1 == "GLYPH_JOURNAL_BOUND" { print $2; exit }' "$GANG")"
tick_glyph_claim="$GANG_LOCK_DIR/journal-${tick_glyph_digest}_rotate.claim"
head -c "$tick_glyph_bound" /dev/zero | tr '\0' '\n' >> "$tick_glyph_journal"
ln -s "$$" "$tick_glyph_claim"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "a writer that finds the journal claim held leaves the rotation to its holder" \
  'not rotated' \
  "$([ -f "$tick_glyph_journal.1" ] || [ ! -f "$tick_glyph_journal" ] \
      && printf rotated || printf 'not rotated')"
equal "and still appends the edge that crossed the bound" \
  '~idle~ -busy- hook' \
  "$(tail -n 1 "$tick_glyph_journal" | awk -F '\t' '{ print $3, $4, $5 }')"
equal "a rotation left to the claim's holder marks no journal failure" "" \
  "$(tmux show-options -wqv -t "$tick_glyph_id" @gl_journal_failed)"
rm -f -- "$tick_glyph_claim"
printf '%s' '{"hook_event_name":"PermissionRequest"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "the first edge after the claim is released moves the journal to its older generation" \
  rotated \
  "$([ -f "$tick_glyph_journal.1" ] && [ ! -e "$tick_glyph_journal" ] \
      && printf rotated || printf 'not rotated')"
equal "a rotation that completed marks no journal failure" "" \
  "$(tmux show-options -wqv -t "$tick_glyph_id" @gl_journal_failed)"
contains "explain reads the crossing edge back from the older generation" \
  "$("$GANG" explain tick-glyph)" '-busy- -> !occupied! (hook)'

# A LINE THAT COULD NOT BE WRITTEN LEAVES A MARK THAT OUTLIVES LATER LINES. A
# directory standing where the journal belongs makes the next append fail. The
# glyph still changes and the window is marked; a later edge that does land must
# not erase the only evidence of the gap, which status and roster both report.
# The explain above repaints the window, and a write of the state a window
# already shows changes nothing, so a hook first paints it occupied and the busy
# edge below is a real change. Nothing reads state between the hooks, so each
# edge is exact.
printf '%s' '{"hook_event_name":"PermissionRequest"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "the window shows occupied before the journal is replaced" '!tick-glyph!' \
  "$(tmux display-message -p -t "$tick_glyph_id" '#{window_name}')"
rm -f -- "$tick_glyph_journal"
mkdir -- "$tick_glyph_journal"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "an edge the journal refused still paints the glyph" '-tick-glyph-' \
  "$(tmux display-message -p -t "$tick_glyph_id" '#{window_name}')"
contains "and marks the window with the edge that was lost" \
  "$(tmux show-options -wqv -t "$tick_glyph_id" @gl_journal_failed)" \
  '-> -busy- transition was NOT journaled'
rmdir -- "$tick_glyph_journal"
printf '%s' '{"hook_event_name":"PermissionRequest"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$tick_glyph_pane" "$GANG" hook >/dev/null
equal "the next edge starts a fresh journal from the recorded word" \
  '-busy- !occupied! hook' \
  "$(awk -F '\t' '$2 == "tick-glyph" { print $3, $4, $5 }' "$tick_glyph_journal")"
contains "and the mark of the earlier gap survives that success" \
  "$(tmux show-options -wqv -t "$tick_glyph_id" @gl_journal_failed)" 'NOT journaled'
contains "roster shows the journal failure in the agent's row" \
  "$("$GANG" roster | awk 'index($0, "tick-glyph")')" journal-failed
contains "status reports the journal failure" \
  "$("$GANG" status tick-glyph)" 'transition journal: '
"$GANG" drop tick-glyph >/dev/null

# A PASS THAT SPENDS ITS BUDGET STOPS, RECORDS A CURSOR, AND REPORTS PARTIAL.
# Three hitched windows and a budget of zero seconds: every pass makes the one
# visit it always owes, then stops. The passes are driven here, and the visit
# order shows the roster rotating from the cursor. The budget under test is the worker's own soft share of its
# deadline, read once per visit from the shell's whole-second clock, so a zero
# budget is spent by the first visit and nothing here waits.
tick_part_ledger="$RUN_ROOT/tick-partial-visits"
tick_part_digest="$(python3 -c 'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')" "$GANG_SESSION")"
tick_part_dir="$XDG_STATE_HOME/gangline/tick/$tick_part_digest"
for tick_part_n in 1 2 3; do
  tmux new-window -d -t "=$GANG_SESSION" -n "tick-part-$tick_part_n" "PS1='❯ ' bash --norc"
  "$GANG" adopt "tick-part-$tick_part_n" -c bash >/dev/null
done
tick_part_window_of() { # $1 agent name -> its window id
  tmux list-windows -t "=$GANG_SESSION" -F '#{window_id} #{@gl_agent}' \
    | awk -v a="$1" '$2 == a { printf "%s", $1; exit }'
}
tick_part_rc=0
: > "$tick_part_ledger"
GANG_TEST_TICK_MODE=manual GANG_TEST_TICK_SOFT_BUDGET_S=0 \
  GANG_TEST_TICK_VISIT_LEDGER="$tick_part_ledger" \
  "$GANG" tick > "$RUN_ROOT/tick-partial-1.out" 2>&1 || tick_part_rc=$?
equal "a pass that spends its budget after one visit still returns success" 0 "$tick_part_rc"
tick_part_first="$(sed -n '1p' "$tick_part_ledger")"
tick_part_total="$("$GANG" roster --porcelain | wc -l | tr -d ' ')"
equal "the budgeted pass made exactly the one visit it always owes" 1 \
  "$(wc -l < "$tick_part_ledger" | tr -d ' ')"
equal "the partial pass leaves the last visited window as its cursor" \
  "$(tick_part_window_of "$tick_part_first")" "$(<"$tick_part_dir/cursor")"
contains "health stays ok and names the partial pass" "$(<"$tick_part_dir/health")" \
  $'ok\t'
contains "the ok note counts what the pass visited against the roster" \
  "$(<"$tick_part_dir/health")" "partial pass: 1 of $tick_part_total agents visited"
excludes "a partial pass is not a failed tick" "$("$GANG" status 2>&1)" 'tick failed:'
equal "the partial marker does not outlive the worker that read it" absent \
  "$([ -e "$tick_part_dir/partial" ] && printf present || printf absent)"
GANG_TEST_TICK_MODE=manual GANG_TEST_TICK_SOFT_BUDGET_S=0 \
  GANG_TEST_TICK_VISIT_LEDGER="$tick_part_ledger" "$GANG" tick >/dev/null
GANG_TEST_TICK_MODE=manual GANG_TEST_TICK_SOFT_BUDGET_S=0 \
  GANG_TEST_TICK_VISIT_LEDGER="$tick_part_ledger" "$GANG" tick >/dev/null
equal "three budgeted passes visit three distinct agents in roster order from the cursor" 3 \
  "$(sort -u "$tick_part_ledger" | wc -l | tr -d ' ')"
tick_part_third="$(sed -n '3p' "$tick_part_ledger")"
equal "the cursor follows the latest visit" \
  "$(tick_part_window_of "$tick_part_third")" "$(<"$tick_part_dir/cursor")"
# A full pass starts after the cursor and, having reached everyone, removes it.
: > "$tick_part_ledger"
GANG_TEST_TICK_MODE=manual GANG_TEST_TICK_VISIT_LEDGER="$tick_part_ledger" \
  "$GANG" tick >/dev/null
equal "an unbudgeted pass visits the whole roster" "$tick_part_total" \
  "$(wc -l < "$tick_part_ledger" | tr -d ' ')"
equal "the pass after a cursor starts with the agent after it" \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#{window_id} #{@gl_agent}' \
     | awk -v c="$(tick_part_window_of "$tick_part_third")" '$2 != "" { l[++n] = $0; if ($1 == c) at = n } END { split(l[at % n + 1], f, " "); print f[2] }')" \
  "$(sed -n '1p' "$tick_part_ledger")"
equal "a complete pass removes the cursor" absent \
  "$([ -e "$tick_part_dir/cursor" ] && printf present || printf absent)"
contains "a complete pass records plain ok health" "$(<"$tick_part_dir/health")" $'ok\t'
excludes "a complete pass leaves no partial note" "$(<"$tick_part_dir/health")" 'partial pass'
for tick_part_n in 1 2 3; do "$GANG" drop "tick-part-$tick_part_n" >/dev/null; done
unset -f tick_part_window_of

# ISSUE #254: A ROSTER SNAPSHOT CAN NAME A WINDOW `gang drop` REMOVES WHILE THE
# PASS IS STILL WALKING IT. The seam sits right after tick_full_pass_once has
# read a roster entry as present (past launch_dead) and before any of its
# later pane reads/writes, so the drop lands deterministically inside that
# window rather than racing a real clock. The pass must treat the vanished
# entry as gone — no unreadable-pane alert, no activity or delivery-ownership
# write against it — and must still finish the rest of its roster normally.
"$HITCH" tick-stale-victim -c tick-native -d /tmp >/dev/null
"$HITCH" tick-stale-live -c tick-native -d /tmp >/dev/null
tick_stale_ready_fifo="$RUN_ROOT/tick-stale-ready"
tick_stale_release_fifo="$RUN_ROOT/tick-stale-release"
tick_stale_ledger="$RUN_ROOT/tick-stale-ledger"
mkfifo "$tick_stale_ready_fifo" "$tick_stale_release_fifo"
GANG_TEST_TICK_VICTIM=tick-stale-victim \
GANG_TEST_TICK_VICTIM_READY_FIFO="$tick_stale_ready_fifo" \
GANG_TEST_TICK_VICTIM_RELEASE_FIFO="$tick_stale_release_fifo" \
GANG_TEST_TICK_VISIT_LEDGER="$tick_stale_ledger" \
  "$GANG" tick > "$RUN_ROOT/tick-stale.out" 2> "$RUN_ROOT/tick-stale.err" &
tick_stale_pid=$!
IFS= read -r -N 1 _ < "$tick_stale_ready_fifo"
tick_stale_drop_rc=0
"$GANG" drop tick-stale-victim > "$RUN_ROOT/tick-stale-drop.out" 2>&1 || tick_stale_drop_rc=$?
printf '\n' > "$tick_stale_release_fifo"
tick_stale_rc=0
wait "$tick_stale_pid" || tick_stale_rc=$?
equal "the normal drop path still succeeds while a pass is mid-walk on it" 0 "$tick_stale_drop_rc"
equal "the pass carrying the vanished entry still completes cleanly" 0 "$tick_stale_rc"
excludes "no unreadable-pane alert names the vanished entry" \
  "$(<"$RUN_ROOT/tick-stale.err")" "cannot read pane"
excludes "no activity-bound alert names the vanished entry" \
  "$(<"$RUN_ROOT/tick-stale.err")" "cannot clear the activity-only bound"
excludes "no delivery-ownership alert names the vanished entry" \
  "$(<"$RUN_ROOT/tick-stale.err")" "could not mark delivery ownership"
contains "the still-live roster entry is visited in the same pass" \
  "$(<"$tick_stale_ledger")" "tick-stale-live"
"$GANG" drop tick-stale-live >/dev/null
# ISSUE #264: THE WINDOW MAY DISAPPEAR INSIDE occupied(), after the tick has
# already accepted its roster entry. Pause immediately before occupied's first
# pane read, remove that real window through the ordinary drop path, and then
# resume the pass. The stale ID is gone, not an unreadable live pane: it must
# be skipped without an occupancy alert or any later write, while the pass
# still reaches another roster entry. The refused-live-pane control remains in
# integration-readiness.sh and must keep failing loudly.
"$HITCH" tick-occupied-victim -c tick-occupied-gone -d /tmp >/dev/null
"$HITCH" tick-occupied-live -c tick-native -d /tmp >/dev/null
tick_occupied_ready_fifo="$RUN_ROOT/tick-occupied-ready"
tick_occupied_release_fifo="$RUN_ROOT/tick-occupied-release"
tick_occupied_ledger="$RUN_ROOT/tick-occupied-ledger"
mkfifo "$tick_occupied_ready_fifo" "$tick_occupied_release_fifo"
GANG_TEST_OCCUPIED_VICTIM=tick-occupied-victim \
GANG_TEST_OCCUPIED_READY_FIFO="$tick_occupied_ready_fifo" \
GANG_TEST_OCCUPIED_RELEASE_FIFO="$tick_occupied_release_fifo" \
GANG_TEST_TICK_VISIT_LEDGER="$tick_occupied_ledger" \
  "$GANG" tick > "$RUN_ROOT/tick-occupied.out" \
    2> "$RUN_ROOT/tick-occupied.err" &
tick_occupied_pid=$!
IFS= read -r -N 1 _ < "$tick_occupied_ready_fifo"
tick_occupied_drop_rc=0
"$GANG" drop tick-occupied-victim \
  > "$RUN_ROOT/tick-occupied-drop.out" 2>&1 || tick_occupied_drop_rc=$?
printf '\n' > "$tick_occupied_release_fifo"
tick_occupied_rc=0
wait "$tick_occupied_pid" || tick_occupied_rc=$?
equal "the normal drop path removes a window paused inside occupied" \
  0 "$tick_occupied_drop_rc"
equal "the pass whose occupancy target vanished still completes cleanly" \
  0 "$tick_occupied_rc"
excludes "a gone occupancy target raises no unreadable-pane alert" \
  "$(<"$RUN_ROOT/tick-occupied.err")" "refusing to guess occupancy"
excludes "a gone occupancy target receives no later tick write" \
  "$(<"$RUN_ROOT/tick-occupied.err")" "tick-occupied-victim"
contains "the same pass continues to its still-live roster entry" \
  "$(<"$tick_occupied_ledger")" "tick-occupied-live"
tick_occupied_cleanup_rc=0
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null \
  || tick_occupied_cleanup_rc=$?
equal "a clean pass clears any failed health left by the vanished-window fixture" \
  0 "$tick_occupied_cleanup_rc"
"$GANG" drop tick-occupied-live >/dev/null

# The final occupied fallback negates busy_painted's ordinary "not busy"
# answer. Drive the narrower race where the composer absence is read while the
# window is live, then drop it immediately before busy_painted's own capture;
# a gone result must not be negated into "occupied" for state_now to spend.
"$HITCH" tick-occupied-late -c tick-occupied-late -d /tmp >/dev/null
"$HITCH" tick-occupied-late-live -c tick-native -d /tmp >/dev/null
tick_occupied_late_id="$(window_id tick-occupied-late)"
tick_occupied_late_box_ready="$RUN_ROOT/tick-occupied-late-box-ready"
tick_occupied_late_box_hold="$RUN_ROOT/tick-occupied-late-box-hold"
mkfifo "$tick_occupied_late_box_ready" "$tick_occupied_late_box_hold"
tmux send-keys -l -t "$tick_occupied_late_id" \
  "printf '\\033[2J\\033[H'; printf 'TICK_OCCUPIED_LATE\\n'; printf x > '$tick_occupied_late_box_ready'; IFS= read -r _ < '$tick_occupied_late_box_hold'"
tmux send-keys -t "$tick_occupied_late_id" Enter
IFS= read -r -N 1 _ < "$tick_occupied_late_box_ready"
tick_occupied_late_ready_fifo="$RUN_ROOT/tick-occupied-late-ready"
tick_occupied_late_release_fifo="$RUN_ROOT/tick-occupied-late-release"
tick_occupied_late_ledger="$RUN_ROOT/tick-occupied-late-ledger"
mkfifo "$tick_occupied_late_ready_fifo" "$tick_occupied_late_release_fifo"
GANG_TEST_OCCUPIED_VICTIM=tick-occupied-late \
GANG_TEST_OCCUPIED_PHASE=busy \
GANG_TEST_OCCUPIED_READY_FIFO="$tick_occupied_late_ready_fifo" \
GANG_TEST_OCCUPIED_RELEASE_FIFO="$tick_occupied_late_release_fifo" \
GANG_TEST_TICK_VISIT_LEDGER="$tick_occupied_late_ledger" \
  "$GANG" tick > "$RUN_ROOT/tick-occupied-late.out" \
    2> "$RUN_ROOT/tick-occupied-late.err" &
tick_occupied_late_pid=$!
IFS= read -r -N 1 _ < "$tick_occupied_late_ready_fifo"
tick_occupied_late_drop_rc=0
"$GANG" drop tick-occupied-late \
  > "$RUN_ROOT/tick-occupied-late-drop.out" 2>&1 \
  || tick_occupied_late_drop_rc=$?
printf '\n' > "$tick_occupied_late_release_fifo"
tick_occupied_late_rc=0
wait "$tick_occupied_late_pid" || tick_occupied_late_rc=$?
equal "the normal drop path removes a window between occupancy reads" \
  0 "$tick_occupied_late_drop_rc"
equal "the pass whose final occupancy target vanished completes cleanly" \
  0 "$tick_occupied_late_rc"
excludes "a gone final occupancy target is not mislabeled occupied" \
  "$(<"$RUN_ROOT/tick-occupied-late.err")" "tick-occupied-late"
contains "the late-race pass continues to its live roster entry" \
  "$(<"$tick_occupied_late_ledger")" "tick-occupied-late-live"
"$GANG" drop tick-occupied-late-live >/dev/null

# THE DEADLINE IS AN OPERATOR SETTING, VALIDATED BEFORE THE WORKER STARTS, AND
# THE WORKER ACCEPTS ONLY THE NUMBER ITS CONTROLLER ENFORCES.
tick_deadline_rc=0
GANG_TICK_DEADLINE=abc "$GANG" tick > "$RUN_ROOT/tick-deadline-word.out" 2>&1 || tick_deadline_rc=$?
equal "a non-numeric GANG_TICK_DEADLINE refuses the tick" 1 "$tick_deadline_rc"
contains "the refusal names the setting and its unit" "$(<"$RUN_ROOT/tick-deadline-word.out")" \
  "GANG_TICK_DEADLINE must be a whole number of seconds, got 'abc'"
tick_deadline_rc=0
GANG_TICK_DEADLINE=30 "$GANG" tick > "$RUN_ROOT/tick-deadline-short.out" 2>&1 || tick_deadline_rc=$?
equal "a GANG_TICK_DEADLINE below the shipped budget refuses the tick" 1 "$tick_deadline_rc"
contains "the refusal names the floor" "$(<"$RUN_ROOT/tick-deadline-short.out")" \
  "GANG_TICK_DEADLINE must be at least 60 seconds, got 30"
excludes "a refused deadline writes no failed health" "$(<"$tick_part_dir/health")" $'failed\t'
tick_deadline_rc=0
GANG_TICK_DEADLINE=3601 "$GANG" tick > "$RUN_ROOT/tick-deadline-ceiling.out" 2>&1 || tick_deadline_rc=$?
equal "a GANG_TICK_DEADLINE above an hour refuses the tick" 1 "$tick_deadline_rc"
contains "the refusal names the ceiling" "$(<"$RUN_ROOT/tick-deadline-ceiling.out")" \
  "GANG_TICK_DEADLINE must be at most 3600 seconds, got 3601"
tick_deadline_rc=0
GANG_TICK_DEADLINE=9223372037 "$GANG" tick > "$RUN_ROOT/tick-deadline-overflow.out" 2>&1 \
  || tick_deadline_rc=$?
equal "a deadline that would overflow the deadline arithmetic refuses the tick" 1 "$tick_deadline_rc"
contains "the overflowing value is refused by the ceiling" \
  "$(<"$RUN_ROOT/tick-deadline-overflow.out")" \
  "GANG_TICK_DEADLINE must be at most 3600 seconds, got 9223372037"
tick_deadline_rc=0
GANG_TICK_DEADLINE=99999999999999999999 "$GANG" tick > "$RUN_ROOT/tick-deadline-wide.out" 2>&1 \
  || tick_deadline_rc=$?
equal "a deadline wider than a machine word refuses the tick" 1 "$tick_deadline_rc"
contains "the wide value is refused by the ceiling, not by the shell" \
  "$(<"$RUN_ROOT/tick-deadline-wide.out")" \
  "GANG_TICK_DEADLINE must be at most 3600 seconds, got 99999999999999999999"
excludes "the shell never saw the wide value as a number" \
  "$(<"$RUN_ROOT/tick-deadline-wide.out")" "integer expression expected"
tick_deadline_rc=0
GANG_TICK_DEADLINE=0090 "$GANG" tick > "$RUN_ROOT/tick-deadline-octal.out" 2>&1 || tick_deadline_rc=$?
equal "a deadline with a leading zero refuses the tick" 1 "$tick_deadline_rc"
contains "the leading zero is refused as not a whole number" \
  "$(<"$RUN_ROOT/tick-deadline-octal.out")" \
  "GANG_TICK_DEADLINE must be a whole number of seconds, got '0090'"
tick_deadline_rc=0
GANG_TICK_DEADLINE=90 GANG_TEST_TICK_MODE=manual "$GANG" tick \
  > "$RUN_ROOT/tick-deadline-long.out" 2>&1 || tick_deadline_rc=$?
equal "a longer GANG_TICK_DEADLINE reaches the worker through its controller" 0 "$tick_deadline_rc"
equal "the longer deadline's tick prints nothing" "" "$(<"$RUN_ROOT/tick-deadline-long.out")"
tick_deadline_rc=0
GANG_TICK_DEADLINE=90 GANG_TICK_INTERNAL=1 \
  "$ROOT/libexec/gang-tick-deadline" --clock-helper "$ROOT/libexec/gang-clock" \
  sh -c 'printf "%s\n" "$GANG_TICK_DEADLINE_SECONDS"' > "$RUN_ROOT/tick-deadline-export.out" 2>&1 \
  || tick_deadline_rc=$?
equal "the controller exports the configured deadline to its worker" 90 \
  "$(<"$RUN_ROOT/tick-deadline-export.out")"
# A LAUNCH DURING A PASS GETS ONE PASS AFTER IT, AND NO MORE. FIFO edges make
# each crossing exact: the first launch queues behind the parked owner, and
# the queued pass holds the queue lock until it holds the run lock, so every
# launch before that point is already served and forks nothing. A launch after
# it queues the next pass. Taking the queue lock and then the run lock waits
# out whatever is queued.
tick_ready_fifo="$RUN_ROOT/tick-ready"
tick_release_fifo="$RUN_ROOT/tick-release"
tick_ready2_fifo="$RUN_ROOT/tick-ready2"
tick_release2_fifo="$RUN_ROOT/tick-release2"
tick_ledger="$RUN_ROOT/tick-ledger"
tick_events="$XDG_DATA_HOME/gangline/events/events.jsonl"
mkfifo "$tick_ready_fifo" "$tick_release_fifo" "$tick_ready2_fifo" "$tick_release2_fifo"
tick_event_lines() { # tick events so far; an absent log has none
  if [ -e "$tick_events" ]; then grep -c '"tick\.' "$tick_events" || :; else printf '0\n'; fi
}
GANG_TEST_TICK_READY_FIFO="$tick_ready_fifo" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_release_fifo" \
GANG_TEST_TICK_LEDGER="$tick_ledger" \
  "$GANG" tick > "$RUN_ROOT/tick-owner.out" 2> "$RUN_ROOT/tick-owner.err" &
tick_owner_pid=$!
IFS= read -r -N 1 _ < "$tick_ready_fifo"
# Earlier teams leave their free lock files behind; the parked owner's run
# lock is the one a non-blocking attempt is refused.
tick_run_lock=""
for tick_lock_file in "$GANG_LOCK_DIR"/tick/*.run; do
  flock -n "$tick_lock_file" true || tick_run_lock+="$tick_lock_file"$'\n'
done
tick_run_lock="${tick_run_lock%$'\n'}"
tick_queue_lock="${tick_run_lock%.run}.queue"
equal "the parked owner's run lock is the only one held" 1 \
  "$(printf '%s\n' "$tick_run_lock" | grep -c '\.run$')"
GANG_TEST_TICK_MODE=async \
GANG_TEST_TICK_READY_FIFO="$tick_ready2_fifo" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_release2_fifo" \
GANG_TEST_TICK_LEDGER="$tick_ledger" \
  "$GANG" roster >/dev/null
tick_queued_rc=0
flock -n "$tick_queue_lock" true || tick_queued_rc=$?
equal "a launch during a parked pass queues one pass" 1 "$tick_queued_rc"
# A launch forks before it returns, and anything it forks inherits the queue
# descriptor and waits behind the parked owner, so holders counted right after
# the launches returned are complete. The queued pass starts children of its
# own that inherit it too, so only holders without a holder ancestor count.
tick_queue_holders() { # pids with the queue lock file open, one per line
  local fd pid
  for fd in /proc/[0-9]*/fd/*; do
    [ "$(readlink "$fd")" = "$tick_queue_lock" ] || continue
    pid="${fd#/proc/}"
    printf '%s\n' "${pid%%/*}"
  done 2>/dev/null | sort -u
}
tick_queue_roots() { # holders with no holder among their ancestors
  local holders pid up stat
  holders="$(tick_queue_holders)"
  while IFS= read -r pid; do
    [ -n "$pid" ] || continue
    up="$pid"
    while :; do
      stat="$(<"/proc/$up/stat")" 2>/dev/null || { up=""; break; }
      stat="${stat##*) }"
      up="$(printf '%s\n' "$stat" | awk '{print $2}')"
      [ "$up" -gt 1 ] || { up=""; break; }
      ! grep -qx "$up" <<<"$holders" || break
    done
    [ -n "$up" ] || printf '%s\n' "$pid"
  done <<<"$holders"
}
tick_queued_pid="$(tick_queue_roots)"
[ "$(grep -c . <<<"$tick_queued_pid")" = 1 ] \
  || fail "the queued pass is the queue lock's one root holder" "$tick_queued_pid"
tick_events_before="$(tick_event_lines)"
GANG_TEST_TICK_MODE=async GANG_TEST_TICK_LEDGER="$tick_ledger" "$GANG" roster >/dev/null
GANG_TEST_TICK_MODE=async GANG_TEST_TICK_LEDGER="$tick_ledger" "$GANG" roster >/dev/null
equal "launches behind a queued pass start no process" "$tick_queued_pid" \
  "$(tick_queue_roots)"
equal "launches behind a queued pass record no tick event" "$tick_events_before" \
  "$(tick_event_lines)"
# The queued pass is the only record of those launches, so the hangup that
# ends the pane of the command that queued it must not end it.
while IFS= read -r tick_holder_pid; do
  kill -HUP "$tick_holder_pid" || :
done < <(tick_queue_holders)
exec {tick_ready2_fd}<>"$tick_ready2_fifo"
printf '\n' > "$tick_release_fifo"
wait "$tick_owner_pid"
tick_hup_rc=0
IFS= read -r -N 1 -t 30 -u "$tick_ready2_fd" _ || tick_hup_rc=$?
exec {tick_ready2_fd}<&-
equal "a queued pass survives the hangup of its launcher's pane" 0 "$tick_hup_rc"
tick_queued_rc=0
flock -n "$tick_queue_lock" true || tick_queued_rc=$?
equal "the queued pass frees the queue before its pass starts" 0 "$tick_queued_rc"
GANG_TEST_TICK_MODE=async GANG_TEST_TICK_LEDGER="$tick_ledger" "$GANG" roster >/dev/null
printf '\n' > "$tick_release2_fifo"
flock "$tick_queue_lock" true
flock "$tick_run_lock" true
equal "three launches around two passes run exactly one pass each after them" "1 1 1 " \
  "$(tr '\n' ' ' < "$tick_ledger")"
equal "the served launches leave both tick locks free" "0 0" \
  "$(rc=0; flock -n "$tick_queue_lock" true || rc=$?; printf '%s ' "$rc"
     rc=0; flock -n "$tick_run_lock" true || rc=$?; printf '%s' "$rc")"

# NOTHING UNDER A PASS HOLDS A TICK LOCK. The pass starts its controller with
# the run descriptor closed and closes the queue descriptor before that, so a subprocess that outlives the pass cannot wedge later
# launches. A tmux shim run by the worker lists its own open descriptors.
tick_fd_probe_bin="$RUN_ROOT/tick-fd-probe-bin"
tick_fd_probe="$RUN_ROOT/tick-fd-probe"
mkdir -p "$tick_fd_probe_bin"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v tmux)"
  printf 'RUN=%q\n' "$tick_run_lock"
  printf 'QUEUE=%q\n' "$tick_queue_lock"
  printf 'PROBE=%q\n' "$tick_fd_probe"
  cat <<'SH'
. "$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard "$REAL" "$0" tmux || exit $?
if [ -n "${GANG_TICK_DEADLINE_SECONDS:-}" ] && [ ! -e "$PROBE" ]; then
  held=""
  for fd in /proc/$$/fd/*; do
    case "$(readlink "$fd")" in "$RUN"|"$QUEUE") held="$held ${fd##*/}" ;; esac
  done
  printf 'probed%s\n' "$held" > "$PROBE"
fi
exec "$REAL" "$@"
SH
} > "$tick_fd_probe_bin/tmux"
chmod +x "$tick_fd_probe_bin/tmux"
GANG_TEST_TICK_MODE=sync PATH="$tick_fd_probe_bin:$PATH" "$GANG" roster >/dev/null
equal "a queued pass's subprocess holds neither tick lock open" probed \
  "$(<"$tick_fd_probe")"

# A CATCHABLE CONTROLLER DEATH MUST NOT ORPHAN ITS NEW-SESSION WORKER. The
# worker blocks reading a FIFO and writes nothing to the controller pipes, so
# EPIPE cannot end it before controller cleanup is observed. Resolve the whole
# relationship from one completed process listing before signalling this
# exact controller.
tick_controller_ready="$RUN_ROOT/tick-controller-ready"
tick_controller_release="$RUN_ROOT/tick-controller-release"
tick_controller_ps="$RUN_ROOT/tick-controller-ps"
mkfifo "$tick_controller_ready" "$tick_controller_release"
GANG_TEST_TICK_READY_FIFO="$tick_controller_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_controller_release" \
  "$GANG" tick > "$RUN_ROOT/tick-controller-owner.out" 2>&1 &
tick_controller_owner=$!
IFS= read -r -N 1 _ < "$tick_controller_ready"
ps -e -o pid=,ppid=,pgid=,stat=,args= > "$tick_controller_ps"
# The controller descends from the owner this shell started; the worker is
# its child and leads its own process group.
tick_controller_pid="$(awk -v root="$tick_controller_owner" -v ctl="$ROOT/libexec/gang-tick-deadline" '
  { parent[$1] = $2; line[$1] = $0 }
  END {
    for (p in line) {
      if (index(line[p], ctl) == 0) continue
      for (q = parent[p]; q > 1; q = parent[q]) if (q == root) { print p; break }
    }
  }' "$tick_controller_ps")"
tick_controller_worker="$(awk -v ctl="$tick_controller_pid" '$2 == ctl { print $1 }' "$tick_controller_ps")"
tick_controller_pgrp="$(awk -v w="$tick_controller_worker" '$1 == w { print $3 }' "$tick_controller_ps")"
equal "the controller-death fixture resolved one worker under its own controller" 1 \
  "$(printf '%s\n' "$tick_controller_worker" | grep -c '^[0-9][0-9]*$')"
equal "the controller-death fixture's worker leads its own process group" \
  "$tick_controller_worker" "$tick_controller_pgrp"
if [ -n "$tick_controller_pid" ] && [ "$tick_controller_pgrp" = "$tick_controller_worker" ]; then
  kill -TERM "$tick_controller_pid"
fi
tick_controller_owner_rc=0
wait "$tick_controller_owner" || tick_controller_owner_rc=$?
equal "controller TERM remains a surfaced tick failure" 1 \
  "$tick_controller_owner_rc"
equal "controller TERM leaves no live process in the worker's group" 0 \
  "$(ps -e -o pgid=,stat= | awk -v g="$tick_controller_pgrp" '$1 == g && $2 !~ /^Z/' | wc -l | tr -d ' ')"
tick_controller_next_rc=0
"$GANG" tick >/dev/null || tick_controller_next_rc=$?
equal "the next tick passes after a killed controller" 0 "$tick_controller_next_rc"
excludes "the deadline controller ignores an ambient clock executable" \
  "$(<"$ROOT/libexec/gang-tick-deadline")" "GANGLINE_CLOCK_HELPER"

tick_deadline_bound_probe="$(GANG_TICK_DEADLINE=60 python3 - "$ROOT/libexec/gang-tick-deadline" \
  "$ROOT/libexec/gang-clock" 2>/dev/null <<'PY'
import runpy
import subprocess
import sys

scope = runpy.run_path(sys.argv[1], run_name="gang_tick_deadline_bound_probe")
main = scope["main"]
runtime = main.__globals__


class Boundary(Exception):
    pass


setattr(runtime["subprocess"], "Time" + "outExpired", Boundary)


class Worker:
    pid = 424242
    returncode = None
    timed_calls = 0

    def communicate(self, **options):
        if options:
            self.timed_calls += 1
            if self.timed_calls > 1:
                raise AssertionError("deadline wait was restarted")
            raise Boundary
        self.returncode = -9
        return b"", b""


child = Worker()
runtime["subprocess"].Popen = lambda *_args, **_kwargs: child
runtime["os"].killpg = lambda _pid, _signal: None
runtime["clock_now_ns"] = lambda: 1
runtime["clock_elapsed"] = lambda _started, _duration: False
sys.argv = [sys.argv[1], "--clock-helper", sys.argv[2], "worker"]
result = main()
print(f"{result}:{child.timed_calls}")
PY
)"
equal "the tick controller spends one independently bounded child wait" \
  "124:1" "$tick_deadline_bound_probe"

tick_deadline_reaped_probe="$(python3 - "$ROOT/libexec/gang-tick-deadline" <<'PY'
import runpy
import sys

scope = runpy.run_path(sys.argv[1], run_name="gang_tick_deadline_probe")
called = []

class ExitedOwnedLeader:
    pid = 424242
    returncode = None

    @staticmethod
    def poll():
        return 0

class ReapedLeader:
    pid = 434343
    returncode = 0

cleanup = scope["kill_worker_group"]
cleanup.__globals__["os"].killpg = lambda pid, signum: called.append((pid, signum))
cleanup.__globals__["worker"] = ExitedOwnedLeader()
cleanup()
cleanup.__globals__["worker"] = ReapedLeader()
cleanup()
print(f"{len(called)}:{called[0][0] if called else 0}")
PY
)"
equal "deadline cleanup signals an owned zombie group but not a reaped PGID" \
  "1:424242" "$tick_deadline_reaped_probe"

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
"$GANG" adopt tick-restart -c tick-codex-adopt >/dev/null
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
tick_failed_note="$(cut -f3 "$tick_health_file")"
contains "status surfaces the last tick failure with its note" \
  "$("$GANG" status tick-restart 2>/dev/null)" "tick failed: $tick_failed_note"
tick_failed_roster="$("$GANG" roster 2>/dev/null)"
equal "roster prints exactly one tick-failure line" 1 \
  "$(printf '%s\n' "$tick_failed_roster" | grep -c 'tick failed: ' || :)"
contains "the roster's tick-failure line carries the recorded note" \
  "$tick_failed_roster" "tick failed: $tick_failed_note"

tick_next_err="$RUN_ROOT/tick-next.err"
"$GANG" teams >/dev/null 2> "$tick_next_err"
excludes "an ordinary command's stderr carries no tick-failure line" \
  "$(<"$tick_next_err")" "tick failed"
tick_isolation_rc=0
GANG_TEST_TICK_MODE=sync "$GANG" teams >/dev/null 2>&1 || tick_isolation_rc=$?
equal "a detached tick failure never changes its spawning command result" 0 "$tick_isolation_rc"

"$GANG" drop tick-restart >/dev/null 2>&1
"$GANG" tick >/dev/null
excludes "a later successful pass clears the health failure" \
  "$(<"$tick_health_file")" $'failed\t'
equal "the clean pass records an ok log fixture" ok \
  "$(case "$(<"$tick_log_file")" in $'ok\t'*) printf ok ;; *) printf other ;; esac)"
excludes "a clean pass clears the roster's tick-failure line" \
  "$("$GANG" roster 2>/dev/null)" "tick failed:"

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
"$GANG" adopt tick-fresh -c tick-codex-adopt >/dev/null
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
"$GANG" adopt tick-wrong -c tick-codex-adopt >/dev/null
"$GANG" tick >/dev/null
equal "a rollout naming another thread refuses rather than trusting the lock" "" \
  "$(tmux show-options -wqv -t "$tick_wrong_id" @gl_session_live_id)"
contains "and that refusal is recorded where status and roster read it" \
  "$(tmux show-options -wqv -t "$tick_wrong_id" @gl_session_probe_failed)" \
  "live harness session id could not be read"

# A SESSION'S OWN SUB-THREADS ARE NOT A SECOND SESSION. Codex 0.151.0 runs each
# sub-agent as a thread in the same process, holding that thread's lock and
# rollout open beside the primary's while it is live, and each child rollout's
# session_meta names its parent in parent_thread_id. Counting pairs left a
# healthy session unverified whenever it had a sub-agent open. The one
# thread with no parent is the session; every other pair must reach it through
# witnessed parents, or the reading stays refused.
tick_tree_meta() { # $1 rollout path, $2 id, $3 parent id, empty or EMPTY, $4 session id, none or null
  python3 - "$@" <<'PY'
import json
import sys

path, thread, parent, session = sys.argv[1:5]
payload = {"id": thread, "thread_source": "user"}
if session != "none":
    payload["session_id"] = None if session == "null" else session
if parent:
    payload.update(parent_thread_id="" if parent == "EMPTY" else parent,
                   thread_source="subagent")
with open(path, "w") as out:
    out.write(json.dumps({"type": "session_meta", "payload": payload}) + "\n")
    out.write(json.dumps({"type": "turn_context", "payload": {}}) + "\n")
PY
}
tick_tree_root="$RUN_ROOT/tick-codex-tree"
mkdir -p "$tick_tree_root/thread-writer-locks" "$tick_tree_root/sessions"
# A pair spec is id:parent:session[:metadata id]; session "lock" holds the lock
# alone and "-" an empty rollout.
tick_tree_hold() { # $1 name, then pair specs; prints "window pid"
  local name="$1" ready="$RUN_ROOT/tick-tree-$1-ready" spec id parent session meta
  local rollout held="" window
  shift
  for spec in "$@"; do
    IFS=: read -r id parent session meta <<< "$spec"
    rollout="$tick_tree_root/sessions/rollout-2026-09-16T00-00-00-$id.jsonl"
    : > "$tick_tree_root/thread-writer-locks/$id.lock"
    held="$held $(printf '%q' "$tick_tree_root/thread-writer-locks/$id.lock")"
    [ "$session" != lock ] || continue
    if [ "$session" = - ]; then
      : > "$rollout"
    else
      tick_tree_meta "$rollout" "${meta:-$id}" "$parent" "$session"
    fi
    held="$held $(printf '%q' "$rollout")"
  done
  mkfifo "$ready"
  window="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
    -n "tick-tree-$name" \
    "exec python3 $(printf '%q %q' "$RUN_ROOT/tick-codex-holder.py" "$ready")$held")"
  IFS= read -r -N 1 _ < "$ready"
  printf '%s %s' "$window" "$(tmux display-message -p -t "$window" '#{pane_pid}')"
}
tick_tree_probe() { # $1 name, then pair specs; prints rc and the id read
  local held out rc=0
  held="$(tick_tree_hold "$@")"
  out="$("$ROOT/libexec/gang-codex-live-id" "${held#* }")" || rc=$?
  tmux kill-window -t "${held%% *}"
  printf '%s %s' "$rc" "$out"
}

equal "a primary with nested sub-threads reads as the primary" "0 tree-primary-600" \
  "$(tick_tree_probe nested \
    tree-primary-600::tree-primary-600 \
    tree-child-601:tree-primary-600:tree-primary-600 \
    tree-grandchild-602:tree-child-601:tree-primary-600)"
equal "the witness order does not choose the session" "0 tree-primary-610" \
  "$(tick_tree_probe reordered \
    tree-grandchild-612:tree-child-611:tree-primary-610 \
    tree-child-611:tree-primary-610:tree-primary-610 \
    tree-primary-610::tree-primary-610)"
equal "two parentless threads in one process stay ambiguous" "1 " \
  "$(tick_tree_probe two-roots \
    tree-primary-620::tree-primary-620 tree-other-621::tree-other-621)"
equal "a thread whose parent is not witnessed is not proven a descendant" "1 " \
  "$(tick_tree_probe orphan \
    tree-primary-630::tree-primary-630 \
    tree-stray-631:tree-absent-639:tree-primary-630)"
equal "a parent cycle beside a root is not a descendant of it" "1 " \
  "$(tick_tree_probe cycle \
    tree-primary-640::tree-primary-640 \
    tree-loop-641:tree-loop-642:tree-primary-640 \
    tree-loop-642:tree-loop-641:tree-primary-640)"
equal "a child carrying another session's id is not this session's child" "1 " \
  "$(tick_tree_probe foreign-child \
    tree-primary-650::tree-primary-650 \
    tree-child-651:tree-primary-650:tree-elsewhere-659)"
equal "a rollout whose metadata names another thread contradicts its pair" "1 " \
  "$(tick_tree_probe renamed \
    tree-primary-660::tree-primary-660 \
    tree-child-661:tree-primary-660:tree-primary-660:tree-impostor-669)"
equal "a sibling pair without readable metadata settles nothing" "1 " \
  "$(tick_tree_probe unreadable \
    tree-primary-670::tree-primary-670 tree-child-671::-)"
equal "an unreadable primary is not settled by its readable child" "1 " \
  "$(tick_tree_probe unreadable-root \
    tree-primary-675::- tree-child-676:tree-primary-675:tree-primary-675)"
# The held set changes as threads open and close, so the pairs present are not
# the whole evidence: a lock without its rollout leaves the reading open.
equal "a child pair beside its root's bare lock is not the session" "1 " \
  "$(tick_tree_probe bare-root \
    tree-primary-700::lock tree-child-701:tree-primary-700:tree-primary-700)"
equal "a root pair beside an unexplained lock is not settled" "1 " \
  "$(tick_tree_probe extra-lock \
    tree-primary-710::tree-primary-710 tree-other-711::lock)"
# The root's session_id is its own id and every descendant carries it.
equal "threads naming no session settle nothing" "1 " \
  "$(tick_tree_probe no-session \
    tree-primary-720::none tree-child-721:tree-primary-720:none)"
equal "a root with a null session settles nothing" "1 " \
  "$(tick_tree_probe null-session \
    tree-primary-725::null tree-child-726:tree-primary-725:tree-primary-725)"
equal "a tree naming another session is not this one" "1 " \
  "$(tick_tree_probe foreign-tree \
    tree-primary-730::tree-elsewhere-739 \
    tree-child-731:tree-primary-730:tree-elsewhere-739)"
equal "an empty parent id does not make a root" "1 " \
  "$(tick_tree_probe empty-parent \
    tree-primary-740:EMPTY:tree-primary-740 \
    tree-child-741:tree-primary-740:tree-primary-740)"

# The same shape seen through the tick: roster, status and explain carry no
# identity-unverified verdict, while a root other than the registered session
# is still the contradiction that blocks delivery.
tick_tree_id="$(tick_tree_hold healthy \
  tree-primary-680::tree-primary-680 \
  tree-child-681:tree-primary-680:tree-primary-680)"
tick_tree_id="${tick_tree_id%% *}"
"$GANG" adopt tick-tree-healthy -c tick-codex-adopt >/dev/null
tmux set-option -w -t "$tick_tree_id" @gl_session_id tree-primary-680
"$GANG" tick > "$RUN_ROOT/tick-tree.out" 2>&1 || :
equal "the tick records the primary behind its sub-threads" tree-primary-680 \
  "$(tmux show-options -wqv -t "$tick_tree_id" @gl_session_live_id)"
equal "and leaves no unread-identity verdict behind" "" \
  "$(tmux show-options -wqv -t "$tick_tree_id" @gl_session_probe_failed)"
excludes "status does not call a sub-threaded session unverified" \
  "$("$GANG" status tick-tree-healthy 2>&1)" "identity-unverified"
excludes "explain does not call a sub-threaded session unverified" \
  "$("$GANG" explain tick-tree-healthy 2>&1)" "identity-unverified"
tick_tree_roster="$("$GANG" roster 2>&1 | grep -F tick-tree-healthy || :)"
contains "roster lists the sub-threaded session" "$tick_tree_roster" tick-tree-healthy
excludes "roster does not call a sub-threaded session unverified" \
  "$tick_tree_roster" "identity-unverified"

tick_tree_lost_id="$(tick_tree_hold lost \
  tree-primary-690::tree-primary-690 \
  tree-child-691:tree-primary-690:tree-primary-690)"
tick_tree_lost_id="${tick_tree_lost_id%% *}"
"$GANG" adopt tick-tree-lost -c tick-codex-adopt >/dev/null
tmux set-option -w -t "$tick_tree_lost_id" @gl_session_id tree-registered-699
"$GANG" tick > "$RUN_ROOT/tick-tree.out" 2>&1 || :
equal "a sub-threaded root that is not the registered session is read exactly" \
  tree-primary-690 \
  "$(tmux show-options -wqv -t "$tick_tree_lost_id" @gl_session_live_id)"
contains "and is session-lost rather than unverified" \
  "$("$GANG" status tick-tree-lost 2>&1)" "!session-lost!"
"$GANG" drop tick-tree-lost > "$RUN_ROOT/tick-tree-drop.out" 2>&1
"$GANG" drop tick-tree-healthy >> "$RUN_ROOT/tick-tree-drop.out" 2>&1

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
"$GANG" adopt tick-bare -c tick-codex-adopt >/dev/null

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

# A QUIET HOOKLESS HARNESS MAY APPEAR AFTER ADOPTION. The native hook retry
# cannot cover it, so one ordinary tick may read the declared root witness.
# The fixture's reader ledger distinguishes that bounded backfill from the
# normal verification of an already-recorded root: an absent or unreadable
# result must never make later ticks read again.
tick_root_backfill_reads="$RUN_ROOT/tick-root-backfill-reads"
cat > "$RUN_ROOT/collars/tick-root-backfill.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_STOP_HOOK=
collar_harness_identity() {
  local mode=""
  mode="\$(tmux show-options -wqv -t "\$1" @gl_tick_backfill_fixture_mode 2>/dev/null)" \
    || mode=""
  printf '%s\\n' "\$1" >> '$tick_root_backfill_reads'
  case "\$mode" in
    positive) printf '4242\\t7'; return 0 ;;
    unreadable) printf 'fixture root reading is unreadable'; return 2 ;;
    *) return 1 ;;
  esac
}
SH
tick_root_backfill_read_count() {
  awk -v id="$1" '$0 == id { count++ } END { print count + 0 }' \
    "$tick_root_backfill_reads"
}

tick_root_absent_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-root-absent "PS1='❯ ' exec bash --norc")"
"$GANG" adopt tick-root-absent -c tick-root-backfill >/dev/null
equal "the hookless root fixture starts with exactly its adoption read" 1 \
  "$(tick_root_backfill_read_count "$tick_root_absent_id")"
"$GANG" tick >/dev/null
equal "a quiet absent root records its one tick attempt" absent \
  "$(tmux show-options -wqv -t "$tick_root_absent_id" @gl_harness_identity_tick_attempted)"
equal "a quiet absent root leaves no invented witness" "" \
  "$(tmux show-options -wqv -t "$tick_root_absent_id" @gl_harness_identity)"
equal "the first quiet tick reads the declared root once" 2 \
  "$(tick_root_backfill_read_count "$tick_root_absent_id")"
"$GANG" tick >/dev/null
equal "a quiet absent root is not polled by later ticks" 2 \
  "$(tick_root_backfill_read_count "$tick_root_absent_id")"

tick_root_positive_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-root-positive "PS1='❯ ' exec bash --norc")"
"$GANG" adopt tick-root-positive -c tick-root-backfill >/dev/null
tmux set-option -w -t "$tick_root_positive_id" @gl_harness_identity_unreadable \
  'the hitch-time reader was unreadable'
tmux set-option -w -t "$tick_root_positive_id" @gl_tick_backfill_fixture_mode positive
"$GANG" tick >/dev/null
equal "a quiet positive root is recorded by its one tick attempt" $'4242\t7' \
  "$(tmux show-options -wqv -t "$tick_root_positive_id" @gl_harness_identity)"
equal "a positive root records the completed tick attempt" recorded \
  "$(tmux show-options -wqv -t "$tick_root_positive_id" @gl_harness_identity_tick_attempted)"
equal "a tick backfill never clears an existing unreadable verdict" \
  'the hitch-time reader was unreadable' \
  "$(tmux show-options -wqv -t "$tick_root_positive_id" @gl_harness_identity_unreadable)"

tick_root_recorded_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-root-recorded "PS1='❯ ' exec bash --norc")"
"$GANG" adopt tick-root-recorded -c tick-root-backfill >/dev/null
tmux set-option -w -t "$tick_root_recorded_id" @gl_harness_identity $'4242\t7'
tmux set-option -w -t "$tick_root_recorded_id" @gl_tick_backfill_fixture_mode positive
"$GANG" tick >/dev/null
equal "a recorded root is preserved instead of backfilled" $'4242\t7' \
  "$(tmux show-options -wqv -t "$tick_root_recorded_id" @gl_harness_identity)"
equal "a recorded root never gains a tick-backfill attempt" "" \
  "$(tmux show-options -wqv -t "$tick_root_recorded_id" @gl_harness_identity_tick_attempted)"

tick_root_lost_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-root-lost "PS1='❯ ' exec bash --norc")"
"$GANG" adopt tick-root-lost -c tick-root-backfill >/dev/null
tmux set-option -w -t "$tick_root_lost_id" @gl_harness_lost 'the prior root was lost'
tmux set-option -w -t "$tick_root_lost_id" @gl_tick_backfill_fixture_mode positive
tick_root_lost_reads="$(tick_root_backfill_read_count "$tick_root_lost_id")"
"$GANG" tick >/dev/null
equal "a lost root verdict is preserved instead of backfilled" \
  'the prior root was lost' \
  "$(tmux show-options -wqv -t "$tick_root_lost_id" @gl_harness_lost)"
equal "a lost root never gains a tick-backfill attempt" "" \
  "$(tmux show-options -wqv -t "$tick_root_lost_id" @gl_harness_identity_tick_attempted)"
equal "a lost root skips the declared reader entirely" "$tick_root_lost_reads" \
  "$(tick_root_backfill_read_count "$tick_root_lost_id")"

tick_root_unreadable_id="$(tmux new-window -d -P -F '#{window_id}' \
  -t "=$GANG_SESSION" -n tick-root-unreadable "PS1='❯ ' exec bash --norc")"
"$GANG" adopt tick-root-unreadable -c tick-root-backfill >/dev/null
tmux set-option -w -t "$tick_root_unreadable_id" @gl_tick_backfill_fixture_mode unreadable
"$GANG" tick >/dev/null
equal "an unreadable quiet root records the failed tick attempt" unreadable \
  "$(tmux show-options -wqv -t "$tick_root_unreadable_id" @gl_harness_identity_tick_attempted)"
equal "an unreadable quiet root preserves its diagnostic" \
  'fixture root reading is unreadable' \
  "$(tmux show-options -wqv -t "$tick_root_unreadable_id" @gl_harness_identity_unreadable)"
equal "the unreadable quiet root is read once by tick" 2 \
  "$(tick_root_backfill_read_count "$tick_root_unreadable_id")"
"$GANG" tick >/dev/null
equal "an unreadable quiet root is not retried by later ticks" 2 \
  "$(tick_root_backfill_read_count "$tick_root_unreadable_id")"

# Team teardown retires the ephemeral health files with the session that gave
# them meaning. A file down does not own keeps their directory in place: the
# team still ends, and the leftover is named and fails the teardown instead of
# passing it silently.
tick_state_foreign="${tick_health_file%/health}/foreign"
: > "$tick_state_foreign"
tick_down_rc=0
tick_down_err="$("$GANG" down "$GANG_SESSION" 2>&1 >/dev/null)" || tick_down_rc=$?
equal "a tick state directory down could not remove fails the teardown" 1 \
  "$tick_down_rc"
contains "and down names the directory it left behind" "$tick_down_err" \
  "tick state directory ${tick_health_file%/health} NOT removed"
excludes "and does not claim its own files were left there" "$tick_down_err" \
  "files NOT all removed"
rm -f -- "$tick_state_foreign"
rmdir -- "${tick_health_file%/health}"
equal "tick test teardown ends only its exact disposable session" absent \
  "$(if tmux has-session -t "=$GANG_SESSION" 2>/dev/null; then printf present; else printf absent; fi)"
equal "team teardown removes its ephemeral tick health file" absent \
  "$([ ! -e "$tick_health_file" ] && printf absent || printf present)"
equal "team teardown removes both generations of its transition journal" absent \
  "$([ ! -e "$tick_glyph_journal" ] && [ ! -e "$tick_glyph_journal.1" ] \
      && printf absent || printf present)"
equal "team teardown removes both generations of its cache-compaction journal" absent \
  "$([ ! -e "$tick_cache_journal" ] && [ ! -e "$tick_cache_journal.1" ] \
      && printf absent || printf present)"

export GANG_SESSION="$tick_original_session"
if [ -n "$tick_original_collars" ]; then
  export GANG_COLLARS="$tick_original_collars"
else
  unset GANG_COLLARS
fi
