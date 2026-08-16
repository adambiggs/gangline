# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# The spool: parked delivery, mail, archive, porcelain, queue age, preemption, and adoption.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# in order and it reads that shell's fixtures, helpers and counters.
# SPOOLED DELIVERY. A refused delivery is a live target that cannot take input
# yet, and every refusal happens before a keystroke — so the body is still the
# sender's and can be parked. Nothing in this world polls or schedules: the only
# thing that drains a spool is a native Stop event, which the world fires by
# hand exactly as a harness would.
cat > "$RUN_ROOT/collars/nodrain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
SH
"$HITCH" nodrain -c nodrain -d /tmp >/dev/null
nodrain_id="$(window_id nodrain)"
nodrain_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$nodrain_id" @gl_spool)"
tmux send-keys -l -t "$nodrain_id" 'HUMAN_DRAFT'
if nohook_out="$(printf 'MARK_NOHOOK' |
  "$GANG" send --to nodrain --from tester --stdin 2>&1)"; then
  fail "a target with no turn boundary does not park a refusal" \
    "send reported success"
else
  equal "a target with no turn boundary keeps refusal status" "3" "$?"
fi
contains "naming the declaration a drain would need" "$nohook_out" "GANG_STOP_HOOK"
contains "and says the message was not parked" "$nohook_out" "NOT parked"
excludes "the refusing target received no body" "$(pane nodrain)" "MARK_NOHOOK"
[ ! -d "$nodrain_spool" ] \
  && pass "nothing undrainable was put on disk" \
  || fail "nothing undrainable was put on disk" "$nodrain_spool exists"
"$GANG" drop nodrain >/dev/null
if super_out="$(printf 'MARK_LONE_SUPERSEDE' |
  "$GANG" send --to alpha --from tester --supersede --live-only --stdin 2>&1)"; then
  fail "superseding a live-only send is refused" "send accepted incompatible flags"
else
  pass "superseding a live-only send is refused"
fi
contains "because live-only never parks" "$super_out" "--live-only never parks"

cross_ready_one="test-cross-ready-one-$$"
cross_ready_two="test-cross-ready-two-$$"
cross_release="test-cross-release-$$"
cross_holder_claimed="test-cross-holder-claimed-$$"
cross_holder_release="test-cross-holder-release-$$"
cat > "$RUN_ROOT/collars/spoolable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
_gl_spool_real="\$(declare -f collar_input)"
eval "spool_real_input \${_gl_spool_real#collar_input}"
collar_input() { # once per armed drain, report what the spool and the lock look like
  local lock dir live=no holder="" waiting=0 f
  [ ! -f "$RUN_ROOT/unreadable-drain" ] || return 1
  lock="$GANG_LOCK_DIR/\$(printf '%s' "\$1" | tr -c 'A-Za-z0-9' '_').lock"
  if [ -f "$RUN_ROOT/cross-block" ]; then
    if mkdir "$RUN_ROOT/cross-slot-one" 2>/dev/null; then
      tmux wait-for -S "$cross_ready_one"
    else
      tmux wait-for -S "$cross_ready_two"
    fi
    tmux wait-for -L "$cross_release"
    tmux wait-for -U "$cross_release"
  fi
  if [ -f "$RUN_ROOT/claim-watch" ] && [ -L "\$lock" ]; then
    rm -f "$RUN_ROOT/claim-watch"
    dir="$GANG_LOCK_DIR/spool/\$(tmux show-options -wqv -t "\$1" @gl_spool)"
    for f in "\$dir"/sending-*; do [ -f "\$f" ] && waiting=\$((waiting + 1)); done
    holder="\$(readlink "\$lock" 2>/dev/null)" || holder=""
    [ -n "\$holder" ] && kill -0 "\$holder" 2>/dev/null && live=yes
    printf 'holder-alive=%s claimed=%s\n' "\$live" "\$waiting" \
      > "$RUN_ROOT/claim-observed"
  fi
  if [ -f "$RUN_ROOT/cross-holder" ] && [ -L "\$lock" ]; then
    rm -f "$RUN_ROOT/cross-holder"
    dir="$GANG_LOCK_DIR/spool/\$(tmux show-options -wqv -t "\$1" @gl_spool)"
    waiting=0
    for f in "\$dir"/sending-*; do [ -f "\$f" ] && waiting=\$((waiting + 1)); done
    printf 'claimed=%s\n' "\$waiting" > "$RUN_ROOT/cross-holder-observed"
    tmux wait-for -S "$cross_holder_claimed"
    tmux wait-for -L "$cross_holder_release"
    tmux wait-for -U "$cross_holder_release"
  fi
  spool_real_input "\$1"
}
SH
"$HITCH" parker -c spoolable -d /tmp >/dev/null
parker_id="$(window_id parker)"
parker_pane_id="$(tmux list-panes -t "$parker_id" -F '#{pane_id}')"
parker_token="$(tmux show-options -wqv -t "$parker_id" @gl_spool)"
if [ -n "$parker_token" ]; then
  pass "a hitched agent already has the spool identity a sender will need"
else
  fail "a hitched agent already has the spool identity a sender will need" \
    "@gl_spool is empty"
fi
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
spool_out="$(printf 'MARK_SPOOLED' |
  "$GANG" send --to parker --from tester --stdin)"
contains "a refused delivery is parked rather than lost" "$spool_out" "queued for parker"
# THE DECISION THIS GUARD RECORDS SURVIVES; ONLY ITS WORDING MOVES. It was
# written so a parked message can never be reported as delivered, and that is
# still enforced below. What changed is that "NOT delivered" was the whole
# headline, and read beside "refused" and "wait for it to become idle" it told
# a sender their message had failed when it had been accepted — one routed a
# report around gang to escape it. The old expectation was defensible clause by
# clause and wrong in composition, which is why it is the wording that moves
# and not the rule.
contains "and does not let it be read as delivered" "$spool_out" "not yet in the session"
excludes "and never calls an accepted message refused" "$spool_out" "refused"
contains "it says what the sender must now do, which is nothing" \
  "$spool_out" "nothing further is needed from you"
# The drain is conditional on a turn nobody has taken yet, so the promise is
# conditional too: an agent that takes no further turn never drains, and the
# message must not claim otherwise.
contains "and promises the drain only on a turn actually being completed" \
  "$spool_out" "next completes a turn"
excludes "nothing was typed into the refusing target" "$(pane parker)" "MARK_SPOOLED"
contains "status reports what is waiting for that target" \
  "$("$GANG" status parker)" "spooled: 1"
contains "roster carries the same count" "$("$GANG" roster)" "spooled=1"

if live_only_out="$(printf 'MARK_LIVE_ONLY' |
  "$GANG" send --to parker --from tester --live-only --stdin 2>&1)"; then
  fail "live-only refuses instead of parking" "send unexpectedly succeeded"
else
  pass "live-only refuses instead of parking"
fi
contains "live-only reports the original refusal" "$live_only_out" "draft"
contains "live-only leaves the waiting count unchanged" \
  "$("$GANG" status parker)" "spooled: 1"
excludes "live-only typed nothing into the target" "$(pane parker)" "MARK_LIVE_ONLY"

# The removed flag is refused as unknown before anything is read or parked.
spool_noop_out="$(printf 'MARK_ANNOUNCED' |
  "$GANG" send --to parker --from tester --spool --supersede --stdin 2>&1 || true)"
contains "the removed spool flag is an unknown argument" \
  "$spool_noop_out" "send: unknown argument '--spool'"
excludes "and the refused flag parked nothing" "$spool_noop_out" "queued for parker"
contains "the waiting count is unchanged" \
  "$("$GANG" status parker)" "spooled: 1"

# Two messages from one sender are two messages. Only the sender's explicit
# flag makes a newer one replace an older, and it replaces only its OWN.
printf 'MARK_OTHER_SENDER' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_STALE' | "$GANG" send --to parker --from tester --stdin >/dev/null
contains "a second message from one sender does not replace the first" \
  "$("$GANG" status parker)" "spooled: 3"
printf 'MARK_LATEST' |
  "$GANG" send --to parker --from tester --supersede --stdin >/dev/null
contains "until the sender says the newer one supersedes them" \
  "$("$GANG" status parker)" "spooled: 2"

# EVERY SENDER WRITES UNDER THE ONE IDENTITY. Minting on the way to parking a
# message would let two senders arriving together mint two, and the message that
# lost would sit in a directory nothing points at, reported as waiting and
# deleted by nothing.
equal "parking a message never re-mints the window's spool identity" \
  "$parker_token" "$(tmux show-options -wqv -t "$parker_id" @gl_spool)"
parker_spool_dir="$GANG_LOCK_DIR/spool/$parker_token"
parker_under_token=0
for parker_entry in "$parker_spool_dir"/[0-9]*; do
  [ -f "$parker_entry" ] && parker_under_token=$((parker_under_token + 1))
done
equal "so every parked message is reachable under it" "2" "$parker_under_token"

tmux send-keys -t "$parker_id" C-u
: > "$RUN_ROOT/claim-watch"
tmux wait-for "gang-spool-drain-$parker_id" &
parker_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  BASH_ENV="$RUN_ROOT/no-bashpid" TMUX_PANE="$parker_pane_id" \
    "$GANG" hook >/dev/null
wait "$parker_drain_waiter"
parker_drained="$(pane parker)"
contains "the target's own Stop drains what was parked for it" \
  "$parker_drained" "MARK_LATEST"
contains "including the message from the other sender" \
  "$parker_drained" "MARK_OTHER_SENDER"
contains "each drained message keeps its own sender's attribution" \
  "$parker_drained" "[gang:other#"
excludes "a superseded message is never delivered" "$parker_drained" "MARK_STALE"
excludes "nor the one it superseded" "$parker_drained" "MARK_SPOOLED"
excludes "nor the deprecated-form predecessor" "$parker_drained" "MARK_ANNOUNCED"
drain_order="$(printf '%s\n' "$parker_drained" |
  grep -oE 'MARK_OTHER_SENDER|MARK_LATEST' | awk '!seen[$0]++' | tr '\n' ' ')"
equal "and the spool drains oldest first" "MARK_OTHER_SENDER MARK_LATEST " \
  "$drain_order"
excludes "a drained spool reports nothing waiting" \
  "$("$GANG" status parker)" "spooled:"

# Supersession follows the replacement's settled outcome. A verified live B
# retires waiting A; a later Stop has no predecessor left to deliver.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_LIVE_PREDECESSOR' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
printf 'MARK_LIVE_REPLACEMENT' |
  "$GANG" send --to parker --from tester --supersede --stdin >/dev/null
contains "a live superseding replacement is delivered" \
  "$(pane parker)" "MARK_LIVE_REPLACEMENT"
excludes "its predecessor is no longer waiting" \
  "$("$GANG" status parker)" "spooled:"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
excludes "a retired predecessor never arrives after its replacement" \
  "$(pane parker)" "MARK_LIVE_PREDECESSOR"

# WHAT THE DRAIN LOOKED LIKE FROM INSIDE THE PANE, read by the fixture the first
# time gang asked it for the composer while holding the delivery lock. Both
# facts are invariants a background drain has to satisfy every time: the lock
# names a process that is actually alive, or the next sender reads it as stale,
# deletes it, and pastes into the same composer concurrently; and the entry is
# already claimed out of the live spool, or a second drain can deliver the same
# body again, and so can the next turn boundary if this one dies between the
# Enter and the retirement.
claim_observed="$(cat "$RUN_ROOT/claim-observed" 2>/dev/null)" || claim_observed=""
contains "the delivery lock a background drain holds names a live process" \
  "$claim_observed" "holder-alive=yes"
contains "and the entry it is delivering is already claimed out of the live spool" \
  "$claim_observed" "claimed=2"

# A larger queue is claimed as one delivery before the target's composer is read.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_BUNDLE_ONE' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
printf 'MARK_BUNDLE_TWO' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_BUNDLE_THREE' |
  "$GANG" send --to parker --from third --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
: > "$RUN_ROOT/claim-watch"
tmux wait-for "gang-spool-drain-$parker_id" &
bundle_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$bundle_drain_waiter"
bundle_claim_observed="$(cat "$RUN_ROOT/claim-observed")"
contains "one drain claims a three-message queue before reading the composer" \
  "$bundle_claim_observed" "claimed=3"
bundle_pane="$(pane parker)"
bundle_order="$(printf '%s\n' "$bundle_pane" |
  grep -oE 'MARK_BUNDLE_ONE|MARK_BUNDLE_TWO|MARK_BUNDLE_THREE' |
  awk '!seen[$0]++' | tr '\n' ' ')"
equal "the three-message bundle stays in stamp order" \
  "MARK_BUNDLE_ONE MARK_BUNDLE_TWO MARK_BUNDLE_THREE " "$bundle_order"

# TWO NATIVE BOUNDARIES CROSS BEFORE DRAIN READINESS. Both workers pass the
# dispatcher's non-empty precheck and are released together. The winner is held
# inside inject while the loser finishes, proving the loser touched no entry;
# an mv witness independently proves every claim happened under the pane lock.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_CROSS_ONE' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
printf 'MARK_CROSS_TWO' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_CROSS_THREE' |
  "$GANG" send --to parker --from third --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
cross_lock="$GANG_LOCK_DIR/$(printf '%s' "$parker_id" | tr -c 'A-Za-z0-9' '_').lock"
cross_log="$RUN_ROOT/cross-claims"
real_mv="$(command -v mv)"
cat > "$RUN_ROOT/bin/mv" <<SH
#!/bin/sh
dest=""
for arg do dest="\$arg"; done
case "\$dest" in
  "\${GANG_CROSS_SPOOL:-}"/sending-*)
    if [ -L "\${GANG_CROSS_LOCK:-}" ]; then
      printf 'locked\n' >> "\$GANG_CROSS_LOG"
    else
      printf 'unlocked\n' >> "\$GANG_CROSS_LOG"
    fi ;;
esac
exec "$real_mv" "\$@"
SH
chmod +x "$RUN_ROOT/bin/mv"
: > "$RUN_ROOT/cross-block"
: > "$RUN_ROOT/cross-holder"
rm -rf -- "$RUN_ROOT/cross-slot-one"
tmux wait-for -L "$cross_release"
tmux wait-for -L "$cross_holder_release"
tmux wait-for "$cross_ready_one" &
cross_ready_one_waiter=$!
tmux wait-for "$cross_ready_two" &
cross_ready_two_waiter=$!
tmux wait-for "$cross_holder_claimed" &
cross_holder_waiter=$!
tmux wait-for "gang-spool-drain-$parker_id" &
cross_loser_waiter=$!
cross_hook_env=(
  "GANG_CROSS_LOCK=$cross_lock"
  "GANG_CROSS_SPOOL=$parker_spool_dir"
  "GANG_CROSS_LOG=$cross_log"
  "TMUX_PANE=$parker_pane_id"
)
printf '%s' '{"hook_event_name":"Stop"}' |
  env "${cross_hook_env[@]}" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Stop"}' |
  env "${cross_hook_env[@]}" "$GANG" hook >/dev/null
wait "$cross_ready_one_waiter"
wait "$cross_ready_two_waiter"
rm -f -- "$RUN_ROOT/cross-block"
tmux wait-for -U "$cross_release"
wait "$cross_holder_waiter"
wait "$cross_loser_waiter"
contains "one crossed worker owns the whole queue before the loser finishes" \
  "$(cat "$RUN_ROOT/cross-holder-observed")" "claimed=3"
# source-guard: producer@9213715b1b14: the fixture mv shim records the pane-lock symlink at each claim rename, and no other path writes this log
equal "every crossed-dispatch claim occurs under the pane lock" "3" \
  "$(grep -c '^locked$' "$cross_log")"
excludes "no crossed worker claims before owning the pane lock" \
  "$(cat "$cross_log")" "unlocked"
tmux wait-for "gang-spool-drain-$parker_id" &
cross_winner_waiter=$!
tmux wait-for -U "$cross_holder_release"
wait "$cross_winner_waiter"
rm -f -- "$RUN_ROOT/bin/mv"
cross_pane="$(pane parker)"
# source-guard: producer@e10d2a56bcab: the three unique markers above exist only in the queue released through the crossed workers, whose pane lock and claims were observed directly
contains "the crossed workers submit the complete queue" \
  "$cross_pane" "MARK_CROSS_THREE"
cross_order="$(printf '%s\n' "$cross_pane" |
  grep -oE 'MARK_CROSS_ONE|MARK_CROSS_TWO|MARK_CROSS_THREE' |
  awk '!seen[$0]++' | tr '\n' ' ')"
# source-guard: producer@bc6b9765db17: the three unique queued markers are the only producers matched from the pane, and their committed spool stamps define the expected order
equal "the crossed dispatch preserves one oldest-first bundle" \
  "MARK_CROSS_ONE MARK_CROSS_TWO MARK_CROSS_THREE " "$cross_order"
excludes "the crossed dispatch leaves no second live bundle" \
  "$("$GANG" status parker)" "spooled:"

# A refused bundle returns every claim to its own live stamp.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_REFUSED_ONE' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
printf 'MARK_REFUSED_TWO' |
  "$GANG" send --to parker --from other --stdin >/dev/null
printf 'MARK_REFUSED_THREE' |
  "$GANG" send --to parker --from third --stdin >/dev/null
tmux wait-for "gang-spool-drain-$parker_id" &
refused_bundle_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$refused_bundle_waiter"
contains "a refused bundle returns every message to the waiting queue" \
  "$("$GANG" status parker)" "spooled: 3"
refused_sending=0
for refused_entry in "$parker_spool_dir"/sending-*; do
  [ -f "$refused_entry" ] && refused_sending=$((refused_sending + 1))
done
equal "a refused bundle leaves no entry claimed" "0" "$refused_sending"
tmux send-keys -t "$parker_id" C-u
tmux wait-for "gang-spool-drain-$parker_id" &
refused_cleanup_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$refused_cleanup_waiter"

# A READABLE OBSTRUCTION AFTER STOP is an ordinary retry, not a failed drain.
# GANG_BOOT_TIMEOUT=0 makes the final immediate stable-pane reading the whole
# readiness budget; no timing claim is under test.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_UNREADABLE_BOUNDARY' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
tmux wait-for "gang-spool-drain-$parker_id" &
readable_retry_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  GANG_BOOT_TIMEOUT=0 TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$readable_retry_waiter"
readable_retry_status="$("$GANG" status parker)"
contains "a readable obstruction after Stop leaves the message waiting" \
  "$readable_retry_status" "spooled: 1"
excludes "and remains an ordinary retry rather than a failed drain" \
  "$readable_retry_status" "spool drain NOT verified"

# A TURN BOUNDARY THAT STILL EXPOSES NO READABLE COMPOSER is not another
# healthy refusal. Nothing may be typed and every entry remains live, but the
# failed drain has to survive the hook process in status until a later verified
# drain clears it.
tmux send-keys -t "$parker_id" C-u
: > "$RUN_ROOT/unreadable-drain"
tmux wait-for "gang-spool-drain-$parker_id" &
unreadable_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  GANG_BOOT_TIMEOUT=0 TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$unreadable_drain_waiter"
unreadable_drain_status="$("$GANG" status parker)"
contains "an unreadable composer after Stop leaves the message waiting" \
  "$unreadable_drain_status" "spooled: 1"
contains "and records the failed drain instead of silently retrying forever" \
  "$unreadable_drain_status" "native delivery boundary did not expose a readable composer"
rm -f -- "$RUN_ROOT/unreadable-drain"
tmux wait-for "gang-spool-drain-$parker_id" &
unreadable_recovery_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$unreadable_recovery_waiter"
# source-guard: producer@70b679da62cf: the recovered drain verifies Enter, and the earlier refused send never put this marker in the target pane
contains "the next readable boundary delivers the waiting message" \
  "$(pane parker)" "MARK_UNREADABLE_BOUNDARY"
excludes "and a verified drain clears the prior failure" \
  "$("$GANG" status parker)" "spool drain NOT verified"

# An entry a drain claimed and never retired — what a killed worker leaves — is
# never picked up again, and never hides: the ones behind it still drain.
parker_inflight="$parker_spool_dir/sending-00000000000000000001-abadcafe"
printf '%s\n%s\n%s\n' tester MARK_INTERRUPTED \
  '[gang:tester#abadcafe] MARK_INTERRUPTED [/gang:tester#abadcafe]' \
  > "$parker_inflight"
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_BEHIND_IT' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
tmux send-keys -t "$parker_id" C-u
tmux wait-for "gang-spool-drain-$parker_id" &
parker_second_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$parker_pane_id" "$GANG" hook >/dev/null
wait "$parker_second_waiter"
parker_after_second="$(pane parker)"
excludes "a claimed entry is never delivered by a later drain" \
  "$parker_after_second" "MARK_INTERRUPTED"
contains "and the messages behind it are not lost with it" \
  "$parker_after_second" "MARK_BEHIND_IT"
[ -f "$parker_inflight" ] \
  && pass "it stays on disk where a person can read it" \
  || fail "it stays on disk where a person can read it" "$parker_inflight is gone"
parker_held_status="$("$GANG" status parker)"
contains "and status names it rather than losing it quietly" \
  "$parker_held_status" "held (delivery NOT verified — it may still have arrived): MARK_INTERRUPTED"
contains "naming the directory it is readable in, not an empty one" \
  "$parker_held_status" "read them under $parker_spool_dir"
rm -f "$parker_inflight"

# Everything gang parks has a deletion path, and this is it.
tmux send-keys -l -t "$parker_id" 'HUMAN_DRAFT'
printf 'MARK_DIES_WITH_WINDOW' |
  "$GANG" send --to parker --from tester --stdin >/dev/null
parker_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$parker_id" @gl_spool)"
[ -d "$parker_spool" ] \
  && pass "a spooled message is on disk beside the delivery locks" \
  || fail "a spooled message is on disk beside the delivery locks" "$parker_spool is absent"
parker_failed="$parker_spool/failed-00000000000000000002-deadbeef"
printf '%s\n%s\n%s\n' other MARK_ARCHIVED_HELD \
  '[gang:other#deadbeef] MARK_ARCHIVED_HELD [/gang:other#deadbeef]' \
  > "$parker_failed"
parker_archive_names="$(cd "$parker_spool" && ls)"
parker_live_entry=""
for parker_entry in "$parker_spool"/[0-9]*; do
  [ -f "$parker_entry" ] || continue
  parker_live_entry="$parker_entry"
  break
done
[ -n "$parker_live_entry" ] \
  && cp "$parker_live_entry" "$RUN_ROOT/pre-archive-body"
"$GANG" drop parker >/dev/null
[ ! -d "$parker_spool" ] \
  && pass "dropping an agent deletes its spool" \
  || fail "dropping an agent deletes its spool" "$parker_spool survived"
parker_archive_count=0
parker_archive_dir=""
for archive_dir in "$GANG_ARCHIVE_DIR"/*; do
  [ -d "$archive_dir" ] || continue
  parker_archive_count=$((parker_archive_count + 1))
  parker_archive_dir="$archive_dir"
done
equal "dropping mail creates exactly one teardown archive" "1" \
  "$parker_archive_count"
[ -d "$parker_archive_dir/parker" ] \
  && pass "the teardown archive groups entries under the agent name" \
  || fail "the teardown archive groups entries under the agent name" \
    "$parker_archive_dir/parker is absent"
equal "the teardown archive preserves every entry filename" \
  "$parker_archive_names" "$(cd "$parker_archive_dir/parker" && ls)"
if cmp "$RUN_ROOT/pre-archive-body" \
  "$parker_archive_dir/parker/${parker_live_entry##*/}"; then
  pass "the archived entry preserves the composed body byte for byte"
else
  fail "the archived entry preserves the composed body byte for byte" \
    "archived bytes differ"
fi
[ -f "$parker_archive_dir/parker/${parker_failed##*/}" ] \
  && pass "a held failed entry is archived with waiting mail" \
  || fail "a held failed entry is archived with waiting mail" \
    "${parker_failed##*/} is absent"

"$HITCH" archive-second -c spoolable -d /tmp >/dev/null
archive_second_id="$(window_id archive-second)"
tmux send-keys -l -t "$archive_second_id" 'HUMAN_DRAFT'
printf 'MARK_SECOND_ARCHIVE' |
  "$GANG" send --to archive-second --from tester --stdin >/dev/null
"$GANG" drop archive-second >/dev/null
archive_count=0
for archive_dir in "$GANG_ARCHIVE_DIR"/*; do
  [ -d "$archive_dir" ] && archive_count=$((archive_count + 1))
done
equal "a second teardown claims a distinct archive directory" "2" \
  "$archive_count"

"$HITCH" archive-empty -c spoolable -d /tmp >/dev/null
"$GANG" drop archive-empty >/dev/null
archive_count_after_empty=0
for archive_dir in "$GANG_ARCHIVE_DIR"/*; do
  [ -d "$archive_dir" ] && archive_count_after_empty=$((archive_count_after_empty + 1))
done
equal "dropping an empty queue creates no archive directory" \
  "$archive_count" "$archive_count_after_empty"

# A SPOOL DIRECTORY OUTLIVES THE WINDOW THAT NAMED IT. Only drop and down
# delete one, so a window killed any other way strands its directory under the
# spool root — and spool_establish adopts the directory its token names rather
# than insisting on creating it, so a token minted onto a stray hands the new
# agent the dead one's waiting mail. The stub below replaces the mint's entropy
# with a counter and pre-seeds a directory for EVERY value that counter can
# produce, so a mint that publishes without consulting the filesystem lands on
# a stray whichever draw it happens to take. Past the sequence the stub is the
# real od again, so the mint that does consult it reaches an unoccupied name —
# and so wait_hook_arm, which indexes a tmux hook by the nonce and retries a
# taken index only eight times, still gets the entropy it needs. The counter
# file is what arms the stub: removing it after the hitch returns od to the
# real one for every later check, and for the fixture pane that inherited this
# PATH.
mkdir -p "$RUN_ROOT/mintbin"
cat > "$RUN_ROOT/mintbin/od" <<SH
#!/bin/sh
REAL="$(command -v od)"
counter="$RUN_ROOT/mint-counter"
SH
cat >> "$RUN_ROOT/mintbin/od" <<'SH'
[ -f "$counter" ] || exec "$REAL" "$@"
n=$(( $(cat "$counter") + 1 ))
printf '%s' "$n" > "$counter"
[ "$n" -le 255 ] || exec "$REAL" "$@"
printf ' %02x 00 00 00\n' "$n"
SH
chmod +x "$RUN_ROOT/mintbin/od"
mint_seeded=255
mint_seed_n=1
while [ "$mint_seed_n" -le "$mint_seeded" ]; do
  mint_seed_dir="$GANG_LOCK_DIR/spool/$(printf '%02x000000' "$mint_seed_n")"
  mkdir -p "$mint_seed_dir"
  printf 'ghost\nstranded\n[gang:ghost#deadbeef] MARK_STRANDED [/gang:ghost#deadbeef]\n' \
    > "$mint_seed_dir/00000000000000000001-deadbeef"
  mint_seed_n=$((mint_seed_n + 1))
done
printf '0' > "$RUN_ROOT/mint-counter"
PATH="$RUN_ROOT/mintbin:$PATH" "$HITCH" collider -c spoolable -d /tmp >/dev/null
mint_calls="$(cat "$RUN_ROOT/mint-counter")"
rm -f "$RUN_ROOT/mint-counter"
collider_token="$(tmux show-options -wqv -t "$(window_id collider)" @gl_spool)"
case "$collider_token" in
  ??000000) fail "a mint skips past every identity already on disk" \
              "published '$collider_token', which names a directory that was already there" ;;
  *) pass "a mint skips past every identity already on disk" ;;
esac
# The counter is what proves the stub was reached at all: without it, a token
# outside the sequence is what an unstubbed run produces too.
[ "$mint_calls" -gt "$mint_seeded" ] \
  && pass "and it re-mints rather than publishing the first name it draws" \
  || fail "and it re-mints rather than publishing the first name it draws" \
    "the mint drew $mint_calls times over $mint_seeded occupied names"
# Read AFTER the drop, because that is where an adoption does its damage: a
# window holding a stray's token archives and deletes the stray on the way out,
# so the stranded mail is gone under the dead agent's name.
"$GANG" drop collider >/dev/null
mint_intact=1
mint_seed_n=1
while [ "$mint_seed_n" -le "$mint_seeded" ]; do
  mint_seed_dir="$GANG_LOCK_DIR/spool/$(printf '%02x000000' "$mint_seed_n")"
  grep -q MARK_STRANDED "$mint_seed_dir/00000000000000000001-deadbeef" 2>/dev/null \
    || mint_intact=0
  mint_seed_n=$((mint_seed_n + 1))
done
equal "and dropping it takes no stranded spool with it" "1" "$mint_intact"
mint_seed_n=1
while [ "$mint_seed_n" -le "$mint_seeded" ]; do
  rm -rf -- "$GANG_LOCK_DIR/spool/$(printf '%02x000000' "$mint_seed_n")"
  mint_seed_n=$((mint_seed_n + 1))
done

# WHAT TEARDOWN CANNOT NAME, TEARDOWN MUST NOT DESTROY. spool_write stages
# every entry as a dot-file and commits it by rename, so a write that dies in
# between leaves a fragment matching none of the names the archive used to look
# for — and the delete that followed erased it while reporting the teardown
# clean.
"$HITCH" strays -c spoolable -d /tmp >/dev/null
strays_id="$(window_id strays)"
strays_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$strays_id" @gl_spool)"
mkdir -p "$strays_spool"
printf 'tester\nhalf a message\n[gang:tester#deadbeef] MARK_FRAGMENT' \
  > "$strays_spool/.writing-1234-00000000000000000009"
"$GANG" drop strays >/dev/null
strays_archived=""
for strays_entry in "$GANG_ARCHIVE_DIR"/*/strays/*; do
  [ -f "$strays_entry" ] || continue
  strays_archived="$strays_archived ${strays_entry##*/}"
done
equal "a fragment teardown cannot account for is archived, not deleted" \
  " writing-1234-00000000000000000009" "$strays_archived"
[ ! -d "$strays_spool" ] \
  && pass "and the spool it was in is gone once nothing is left in it" \
  || fail "and the spool it was in is gone once nothing is left in it" \
    "$strays_spool survived"

# THE NAME THAT MAKES A FRAGMENT VISIBLE MUST NOT NAME SOMETHING ELSE. Stripping
# the leading dot maps `.foo` and `foo` onto one destination, and an ordinary mv
# replaces the first archive file with the second. The spool then empties, the
# rmdir succeeds, and teardown reports a clean archive with one message gone —
# the same silent loss the whole archive-every-child rule exists to stop, this
# time inside the rule.
"$HITCH" twinned -c spoolable -d /tmp >/dev/null
twinned_id="$(window_id twinned)"
twinned_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$twinned_id" @gl_spool)"
mkdir -p "$twinned_spool"
printf 'tester\nhidden half\n[gang:tester#deadbeef] MARK_HIDDEN_TWIN' \
  > "$twinned_spool/.writing-1234-00000000000000000011"
printf 'tester\nvisible half\n[gang:tester#deadbeef] MARK_VISIBLE_TWIN' \
  > "$twinned_spool/writing-1234-00000000000000000011"
"$GANG" drop twinned >/dev/null
twinned_kept=0
for twinned_entry in "$GANG_ARCHIVE_DIR"/*/twinned/*; do
  [ -f "$twinned_entry" ] && twinned_kept=$((twinned_kept + 1))
done
equal "two children wanting one archive name are both kept" "2" "$twinned_kept"
grep -rq MARK_VISIBLE_TWIN "$GANG_ARCHIVE_DIR" \
  && pass "the child that was already visible is not overwritten" \
  || fail "the child that was already visible is not overwritten" \
    "MARK_VISIBLE_TWIN is in no archived file"
grep -rq MARK_HIDDEN_TWIN "$GANG_ARCHIVE_DIR" \
  && pass "and the hidden one arrives beside it under a name of its own" \
  || fail "and the hidden one arrives beside it under a name of its own" \
    "MARK_HIDDEN_TWIN is in no archived file"
twinned_hidden=0
for twinned_entry in "$GANG_ARCHIVE_DIR"/*/twinned/.*; do
  [ -f "$twinned_entry" ] && twinned_hidden=$((twinned_hidden + 1))
done
equal "with nothing left hidden in a directory a person reads with ls" "0" \
  "$twinned_hidden"

# THE EDGES OF THAT SAME RULE, each of which can be broken on its own while the
# pair above stays green. `..foo` is a child the archive's own ..?* glob brings
# in and one stripped dot leaves it hidden; a name of nothing but dots strips to
# nothing at all, and an empty name is not a visible one either. And a name is
# taken by anything that answers to it: a relative symlink stops resolving the
# moment it is moved into the archive, after which an existence test that does
# not ask about links calls the name free and the next child is moved over it.
"$HITCH" edged -c spoolable -d /tmp >/dev/null
edged_id="$(window_id edged)"
edged_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$edged_id" @gl_spool)"
mkdir -p "$edged_spool" "$GANG_LOCK_DIR/linktarget"
printf 'tester\ndoubly hidden\n[gang:tester#deadbeef] MARK_DOUBLE_DOT' \
  > "$edged_spool/..writing-1234-00000000000000000021"
printf 'tester\nno name at all\n[gang:tester#deadbeef] MARK_ALL_DOTS' \
  > "$edged_spool/..."
# Resolves here, so the archive walk accepts it as a file; its target is
# reached by a relative path that no longer exists once it has been moved.
printf 'tester\nlink target\n[gang:tester#deadbeef] MARK_LINK_TARGET\n' \
  > "$GANG_LOCK_DIR/linktarget/realfile"
ln -s ../../linktarget/realfile "$edged_spool/danglepair"
printf 'tester\nsecond claimant\n[gang:tester#deadbeef] MARK_LINK_TWIN' \
  > "$edged_spool/.danglepair"
"$GANG" drop edged >/dev/null
edged_hidden=0
for edged_entry in "$GANG_ARCHIVE_DIR"/*/edged/.*; do
  [ -e "$edged_entry" ] || [ -L "$edged_entry" ] || continue
  case "${edged_entry##*/}" in .|..) continue ;; esac
  edged_hidden=$((edged_hidden + 1))
done
equal "a child behind two dots is archived visibly, not merely one dot shallower" \
  "0" "$edged_hidden"
grep -rq MARK_DOUBLE_DOT "$GANG_ARCHIVE_DIR" \
  && pass "and its body is what arrived under that visible name" \
  || fail "and its body is what arrived under that visible name" \
    "MARK_DOUBLE_DOT is in no archived file"
# Named exactly, so this stays red for the two-dot case alone even if the
# all-dots fallback beside it is what someone else changed.
edged_doubled=0
for edged_entry in "$GANG_ARCHIVE_DIR"/*/edged/writing-1234-00000000000000000021; do
  [ -f "$edged_entry" ] && edged_doubled=$((edged_doubled + 1))
done
equal "under the name that is left when every leading dot is gone" "1" \
  "$edged_doubled"
grep -rq MARK_ALL_DOTS "$GANG_ARCHIVE_DIR" \
  && pass "a child whose whole name is dots is archived under a name of its own" \
  || fail "a child whose whole name is dots is archived under a name of its own" \
    "MARK_ALL_DOTS is in no archived file"
grep -rq MARK_LINK_TWIN "$GANG_ARCHIVE_DIR" \
  && pass "and a name held by a link that no longer resolves is not treated as free" \
  || fail "and a name held by a link that no longer resolves is not treated as free" \
    "MARK_LINK_TWIN is in no archived file"
edged_links=0
for edged_entry in "$GANG_ARCHIVE_DIR"/*/edged/*; do
  [ -L "$edged_entry" ] && edged_links=$((edged_links + 1))
done
equal "with the link itself still there rather than replaced by its rival" "1" \
  "$edged_links"
# EVERY ONE OF THEM UNDER THE AGENT THAT OWNED IT, AND NONE OF THEM MERGED. The
# greps above answer "is this body anywhere in the archive", which stays true
# for a child that landed beside the wrong agent or on top of another one; the
# count is what says four children went in and four came out, in the directory
# a person reads to find that agent's mail.
edged_kept=0
for edged_entry in "$GANG_ARCHIVE_DIR"/*/edged/* "$GANG_ARCHIVE_DIR"/*/edged/.[!.]*; do
  [ -e "$edged_entry" ] || [ -L "$edged_entry" ] || continue
  edged_kept=$((edged_kept + 1))
done
equal "and all four children archived under the agent whose spool they were in" \
  "4" "$edged_kept"
rm -rf -- "$GANG_LOCK_DIR/linktarget"

# The fragment is the reachable case; the class is anything the archive cannot
# move. Teardown refuses on those rather than deleting a directory whose
# contents it never read.
"$HITCH" unaccountable -c spoolable -d /tmp >/dev/null
unaccountable_id="$(window_id unaccountable)"
unaccountable_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$unaccountable_id" @gl_spool)"
mkdir -p "$unaccountable_spool/debris"
printf 'MARK_DEBRIS' > "$unaccountable_spool/debris/note"
if unaccountable_out="$("$GANG" drop unaccountable 2>&1)"; then
  fail "teardown refuses a spool holding something it could not archive" \
    "drop reported success"
else
  pass "teardown refuses a spool holding something it could not archive"
fi
contains "naming the directory a person has to read" \
  "$unaccountable_out" "$unaccountable_spool"
grep -q MARK_DEBRIS "$unaccountable_spool/debris/note" \
  && pass "and leaving what it could not account for where it is" \
  || fail "and leaving what it could not account for where it is" \
    "$unaccountable_spool/debris/note is gone"
window_id unaccountable >/dev/null \
  && pass "and ending nothing else: the agent is still there to drop" \
  || fail "and ending nothing else: the agent is still there to drop" \
    "the window is gone"
rm -rf -- "$unaccountable_spool/debris"
"$GANG" drop unaccountable >/dev/null

# ACCEPTED IS WHAT LICENSES A RETIREMENT. --supersede used to unlink the
# sender's waiting entries before the replacement was written, and spool_write
# can still die three ways after that point — so the sender's queued traffic
# went and nothing took its place. The stamp is the clean injection: it fails
# after where the sweep used to run and before any entry exists.
"$HITCH" superseder -c spoolable -d /tmp >/dev/null
superseder_id="$(window_id superseder)"
superseder_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$superseder_id" @gl_spool)"
tmux send-keys -l -t "$superseder_id" 'HUMAN_DRAFT'
printf 'MARK_PREDECESSOR' |
  "$GANG" send --to superseder --from tester --stdin >/dev/null
mkdir -p "$RUN_ROOT/nostamp"
# Only the stamp fails. gang reads its config through python3 too, so a stub
# that failed everything would kill the send before it ever reached the spool
# and this case would pass without exercising the ordering at all.
cat > "$RUN_ROOT/nostamp/python3" <<SH
#!/bin/sh
case "\$*" in *time_ns*) exit 1 ;; esac
exec "$(command -v python3)" "\$@"
SH
chmod +x "$RUN_ROOT/nostamp/python3"
nostamp_rc=0
printf 'MARK_UNSTAMPED_REPLACEMENT' |
  PATH="$RUN_ROOT/nostamp:$PATH" "$GANG" send --to superseder --from tester \
    --supersede --stdin > "$RUN_ROOT/nostamp.out" 2> "$RUN_ROOT/nostamp.err" \
  || nostamp_rc=$?
nostamp_out="$(cat "$RUN_ROOT/nostamp.err" "$RUN_ROOT/nostamp.out")"
if [ "$nostamp_rc" -eq 0 ]; then
  fail "a replacement that cannot be stamped is not accepted" \
    "send reported success"
else
  pass "a replacement that cannot be stamped is not accepted"
fi
# Pins the injection: a send that died somewhere earlier would never have
# reached the retirement, and this case would say nothing about its ordering.
case "$nostamp_out" in
  *"cannot stamp a spool entry"*) pass "and the stamp is what it died on" ;;
  *) fail "and the stamp is what it died on" \
       "exit $nostamp_rc, said: [$nostamp_out]" ;;
esac
if grep -rq MARK_PREDECESSOR "$superseder_spool"; then
  pass "so the message it would have replaced is still waiting"
else
  fail "so the message it would have replaced is still waiting" \
    "$superseder_spool no longer holds it"
fi

# The other half: a retirement that fails AFTER the replacement is committed
# must lose nothing either, and must say so rather than reporting a clean send.
# A bodyless entry is what the sweep cannot read past, and its stamp puts it
# first in the oldest-first walk.
printf 'tester\n\n' > "$superseder_spool/00000000000000000001-deadbeef"
if halfway_out="$(printf 'MARK_COMMITTED_REPLACEMENT' |
  "$GANG" send --to superseder --from tester --supersede --stdin 2>&1)"; then
  fail "a retirement that fails after the commit is reported" \
    "send reported success"
else
  pass "a retirement that fails after the commit is reported"
fi
contains "saying the replacement is parked rather than lost" \
  "$halfway_out" "IS parked as"
if grep -rq MARK_COMMITTED_REPLACEMENT "$superseder_spool"; then
  pass "and the replacement really is on disk"
else
  fail "and the replacement really is on disk" \
    "$superseder_spool does not hold it"
fi
if grep -rq MARK_PREDECESSOR "$superseder_spool"; then
  pass "and nothing it could not retire was destroyed"
else
  fail "and nothing it could not retire was destroyed" \
    "$superseder_spool no longer holds the predecessor"
fi
rm -f -- "$superseder_spool/00000000000000000001-deadbeef"
"$GANG" drop superseder >/dev/null

# AN AGENT NAME BECOMES A PATH COMPONENT. spool_archive names the archive
# subdirectory after the agent whose mail it is holding, so a "name" carrying a
# parent reference moves pending messages out of the archive root entirely — on
# a plain gang drop, with no error, into a directory nobody chose. Identity is
# read from the registration now, so the registration is what gets validated
# before it is joined to a path: the check sits at the boundary that builds the
# destination, where no reader can route around it.
"$HITCH" traversal -c spoolable -d /tmp >/dev/null
traversal_id="$(window_id traversal)"
tmux send-keys -l -t "$traversal_id" 'HUMAN_DRAFT'
printf 'MARK_TRAVERSAL' |
  "$GANG" send --to traversal --from tester --stdin >/dev/null
traversal_escape="$RUN_ROOT/outside"
rm -rf -- "$traversal_escape"
tmux rename-window -t "$traversal_id" '../../outside'
tmux set-option -w -t "$traversal_id" @gl_agent '../../outside'
"$GANG" drop '../../outside' >/dev/null 2>&1 || true
traversal_archived=""
for archived_traversal in "$GANG_ARCHIVE_DIR"/*/*/[0-9]*; do
  [ -f "$archived_traversal" ] || continue
  grep -q MARK_TRAVERSAL "$archived_traversal" 2>/dev/null \
    && traversal_archived="$archived_traversal"
done
equal "a registration that is not a name never becomes an archive path" \
  "contained archived" \
  "$([ -e "$traversal_escape" ] && printf escaped || printf contained) $([ -n "$traversal_archived" ] && printf archived || printf lost)"
tmux kill-window -t "$traversal_id" 2>/dev/null || true

# A FOREIGN MAIL READ INSPECTS THE QUEUE AND NOTHING ELSE. It does not load the
# target's collar, claim entries, take the pane lock, or attempt delivery. A
# self-read below consumes, but archives before a byte reaches stdout.
"$HITCH" mailer -c spoolable -d /tmp >/dev/null
mailer_id="$(window_id mailer)"
tmux send-keys -l -t "$mailer_id" 'HUMAN_DRAFT'
printf 'MARK_MAIL_ONE' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
printf 'MARK_MAIL_TWO' |
  "$GANG" send --to mailer --from other --stdin >/dev/null
printf 'MARK_MAIL_THREE' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mailer_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$mailer_id" @gl_spool)"
mail_names_before="$(cd "$mailer_spool" && ls)"
mail_status_before="$("$GANG" status mailer)"
mail_out="$("$GANG" mail mailer)"
contains "mail prints the first waiting body" "$mail_out" "MARK_MAIL_ONE"
contains "mail prints the second waiting body" "$mail_out" "MARK_MAIL_TWO"
contains "mail prints the third waiting body" "$mail_out" "MARK_MAIL_THREE"
contains "mail names the first sender" "$mail_out" "from tester"
contains "mail names the other sender" "$mail_out" "from other"
mail_order="$(printf '%s\n' "$mail_out" |
  grep -oE 'MARK_MAIL_ONE|MARK_MAIL_TWO|MARK_MAIL_THREE' |
  awk '!seen[$0]++' | tr '\n' ' ')"
equal "mail prints waiting bodies in stamp order" \
  "MARK_MAIL_ONE MARK_MAIL_TWO MARK_MAIL_THREE " "$mail_order"
mail_status_after="$("$GANG" status mailer)"
contains "mail leaves the waiting count unchanged before its read" \
  "$mail_status_before" "spooled: 3"
contains "mail leaves the waiting count unchanged after its read" \
  "$mail_status_after" "spooled: 3"
equal "mail leaves every entry filename untouched" "$mail_names_before" \
  "$(cd "$mailer_spool" && ls)"

tmux set-option -w -t "$mailer_id" @gl_collar no-such-collar
mail_without_collar="$("$GANG" mail mailer)"
contains "mail reads bodies without loading the target collar" \
  "$mail_without_collar" "MARK_MAIL_ONE"
if missing_collar_status="$("$GANG" status mailer 2>&1)"; then
  fail "the same target proves its collar is not loadable" \
    "status unexpectedly succeeded"
else
  pass "the same target proves its collar is not loadable"
fi
contains "status fails specifically on the missing collar" \
  "$missing_collar_status" "unknown collar 'no-such-collar'"
tmux set-option -w -t "$mailer_id" @gl_collar spoolable

mailer_failed="$mailer_spool/failed-00000000000000000005-facefeed"
printf '%s\n%s\n%s\n' other MARK_MAIL_HELD \
  '[gang:other#facefeed] MARK_MAIL_HELD [/gang:other#facefeed]' \
  > "$mailer_failed"
mail_with_held="$("$GANG" mail mailer)"
contains "mail prints a held body" "$mail_with_held" "MARK_MAIL_HELD"
contains "mail labels held delivery as unverified" \
  "$mail_with_held" "held (delivery NOT verified"
# READING YOUR OWN QUEUE CONSUMES IT. A message the addressee has already read
# is delivered again at its next turn boundary — the same body twice, once by
# hand and once by the spool. Reading it IS its delivery, so the read retires
# what it printed. Only for its own agent: a read by anybody else is an
# inspection and stays non-destructive.
mailer_bodies() { # $1 = a mail rendering -> the marks it printed, in order
  printf '%s\n' "$1" | grep -oE 'MARK_MAIL_[A-Z]+' | tr '\n' ' ' || true
}
mailer_self_pane="$(tmux list-panes -t "$(window_id mailer)" -F '#{pane_id}')"
mail_reference="$("$GANG" mail mailer)"
mailer_self_rc=0
TMUX_PANE="$mailer_self_pane" "$GANG" mail > "$RUN_ROOT/mail-self.out" 2>&1 \
  || mailer_self_rc=$?
mailer_self_out="$(grep -v '^gang: WARNING: executing dirty ' \
  "$RUN_ROOT/mail-self.out" || true)"
equal "bare mail reads the calling agent's own queue" \
  "0|$(mailer_bodies "$mail_reference")" \
  "$mailer_self_rc|$(mailer_bodies "$mailer_self_out")"
contains "a self-read says what it consumed, not what waits" \
  "$mailer_self_out" "consumed"
equal "a self-read retires exactly the waiting entries it printed" \
  "failed-00000000000000000005-facefeed" "$(cd "$mailer_spool" && ls)"
mail_after_self="$("$GANG" mail mailer)"
excludes "the consumed bodies are gone from the queue" \
  "$mail_after_self" "MARK_MAIL_ONE"
excludes "every consumed body, not merely the first" \
  "$mail_after_self" "MARK_MAIL_THREE"
contains "a self-read never consumes a held entry" \
  "$mail_after_self" "MARK_MAIL_HELD"
contains "and the held entry keeps saying delivery was not verified" \
  "$mail_after_self" "held (delivery NOT verified"

# An entry that lands after the read is not one the read printed, so nothing
# retires it: the next read is where it appears.
printf 'MARK_MAIL_LATER' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mail_later_rc=0
TMUX_PANE="$mailer_self_pane" "$GANG" mail > "$RUN_ROOT/mail-later.out" 2>&1 \
  || mail_later_rc=$?
mail_later_out="$(grep -v '^gang: WARNING: executing dirty ' \
  "$RUN_ROOT/mail-later.out" || true)"
equal "a message that arrives after a self-read survives it" \
  "0|MARK_MAIL_LATER MARK_MAIL_HELD " \
  "$mail_later_rc|$(mailer_bodies "$mail_later_out")"
equal "and the second read retires only what the second read printed" \
  "failed-00000000000000000005-facefeed" "$(cd "$mailer_spool" && ls)"

# MALFORMED SPOOL BYTES FAIL BEFORE CLAIM. This entry cannot be produced by
# spool_write, but corruption must not turn its known undelivered state into a
# held verdict saying it may have arrived.
mail_malformed="$mailer_spool/00000000000000000006-malformed"
printf '%s\n%s\n' tester malformed-fragment > "$mail_malformed"
mail_malformed_before="$(cd "$mailer_spool" && ls)"
mail_malformed_rc=0
TMUX_PANE="$mailer_self_pane" "$GANG" mail \
  >"$RUN_ROOT/mail-malformed.out" 2>&1 || mail_malformed_rc=$?
[ "$mail_malformed_rc" -ne 0 ] \
  && pass "a bodyless spool entry refuses a self-read" \
  || fail "a bodyless spool entry refuses a self-read" \
    "mail unexpectedly returned zero"
equal "a bodyless entry stays waiting under its original name" \
  "$mail_malformed_before" "$(cd "$mailer_spool" && ls)"
excludes "a bodyless entry creates no false held verdict" \
  "$(cd "$mailer_spool" && ls)" "sending-00000000000000000006-malformed"
rm -f -- "$mail_malformed"

# ARCHIVE FAILURE PRECEDES CLAIM. A self-read cannot turn mail with a known
# undelivered fate into a sending-/held entry merely because its recovery
# destination is misconfigured.
printf 'MARK_MAIL_ARCHIVE_REFUSAL' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mail_archive_refusal_before="$(cd "$mailer_spool" && ls)"
mail_archive_blocker="$RUN_ROOT/archive-not-a-directory"
: > "$mail_archive_blocker"
mail_archive_refusal_rc=0
GANG_ARCHIVE_DIR="$mail_archive_blocker" TMUX_PANE="$mailer_self_pane" \
  "$GANG" mail >"$RUN_ROOT/mail-archive-refusal.out" 2>&1 \
  || mail_archive_refusal_rc=$?
[ "$mail_archive_refusal_rc" -ne 0 ] \
  && pass "an unwritable archive refuses a self-read" \
  || fail "an unwritable archive refuses a self-read" \
    "mail unexpectedly returned zero"
contains "the archive refusal names the unusable recovery path" \
  "$(<"$RUN_ROOT/mail-archive-refusal.out")" "$mail_archive_blocker"
equal "archive refusal leaves the waiting spool byte-for-byte named" \
  "$mail_archive_refusal_before" "$(cd "$mailer_spool" && ls)"
excludes "archive refusal creates no false held delivery verdict" \
  "$(cd "$mailer_spool" && ls)" "sending-"
rm -f -- "$mail_archive_blocker"
mail_archive_recovery="$(TMUX_PANE="$mailer_self_pane" "$GANG" mail 2>&1)"
contains "the untouched message remains readable after archive repair" \
  "$mail_archive_recovery" "MARK_MAIL_ARCHIVE_REFUSAL"

# A SHELL FILTER MAY HIDE OUTPUT, BUT IT CANNOT DESTROY THE ONLY COPY. This is
# the live incident shape: tail consumes all of gang mail's stdout and prints
# only its end. The body prefix is absent from the filtered view, while the
# archive named on unfiltered stderr keeps the exact complete envelope.
mail_filter_body="MARK_MAIL_FILTER_HEAD
$(printf 'filter filler %02d\n' {1..40})
MARK_MAIL_FILTER_TAIL"
printf '%s' "$mail_filter_body" |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
TMUX_PANE="$mailer_self_pane" "$GANG" mail \
  2>"$RUN_ROOT/mail-filter.err" | tail -20 >"$RUN_ROOT/mail-filter.out"
excludes "tail hides the head of a long self-read as the incident requires" \
  "$(<"$RUN_ROOT/mail-filter.out")" "MARK_MAIL_FILTER_HEAD"
contains "the destructive read names its archive outside the stdout pipe" \
  "$(<"$RUN_ROOT/mail-filter.err")" "self-mail is destructive"
contains "the archive notice carries its deletion path" \
  "$(<"$RUN_ROOT/mail-filter.err")" "delete this read archive after recovery with: rm -rf --"
mail_filter_archive=""
for mail_archived_entry in "$GANG_ARCHIVE_DIR"/*/mailer/[0-9]*; do
  [ -f "$mail_archived_entry" ] || continue
  grep -q MARK_MAIL_FILTER_HEAD "$mail_archived_entry" \
    && mail_filter_archive="$mail_archived_entry"
done
[ -n "$mail_filter_archive" ] \
  && pass "the filtered-away head survives in the named archive" \
  || fail "the filtered-away head survives in the named archive" \
    "no archived entry contains MARK_MAIL_FILTER_HEAD"
contains "the archived entry keeps the filtered message's tail too" \
  "$(<"$mail_filter_archive")" "MARK_MAIL_FILTER_TAIL"

# Somebody else's read is an inspection. lead reading a teammate's queue must
# leave every entry exactly where the teammate's own next turn will find it.
printf 'MARK_MAIL_FOREIGN' |
  "$GANG" send --to mailer --from tester --stdin >/dev/null
mail_foreign_before="$(cd "$mailer_spool" && ls)"
# shellcheck disable=SC2154  # set in test/integration-substrate.sh
mail_foreign_out="$(TMUX_PANE="$alpha_tmux_pane" "$GANG" mail mailer)"
contains "another agent's read still prints the waiting body" \
  "$mail_foreign_out" "MARK_MAIL_FOREIGN"
equal "another agent's read consumes nothing" \
  "$mail_foreign_before" "$(cd "$mailer_spool" && ls)"
equal "and an operator outside the team consumes nothing either" \
  "$mail_foreign_before" \
  "$("$GANG" mail mailer >/dev/null; cd "$mailer_spool" && ls)"
"$GANG" drop mailer >/dev/null

"$HITCH" empty-mailbox -c spoolable -d /tmp >/dev/null
empty_mail_out="$("$GANG" mail empty-mailbox)"
contains "mail exits cleanly on an empty queue" \
  "$empty_mail_out" "no mail waiting for empty-mailbox"
"$GANG" drop empty-mailbox >/dev/null

# PORCELAIN IS EXACT TSV, NOT THE HUMAN GLYPH TABLE. First spend the exact-row
# assertion against the default roster and require it to fail; only then use it
# as the instrument for the scripting output.
cat > "$RUN_ROOT/collars/porcelain.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_BUSY_REGEX='MARK_PORCELAIN_BUSY'
SH
"$HITCH" porcelain-busy -c porcelain -d /tmp >/dev/null
"$HITCH" porcelain-idle -c porcelain -d /tmp >/dev/null
porcelain_busy_id="$(window_id porcelain-busy)"
porcelain_painted="porcelain-painted-$$"
tmux send-keys -l -t "$porcelain_busy_id" \
  "printf MARK_PORCELAIN_BUSY\\n; tmux wait-for -S $porcelain_painted"
tmux send-keys -t "$porcelain_busy_id" Enter
tmux wait-for "$porcelain_painted"
tmux set-option -w -t "$porcelain_busy_id" @gl_session_id sid-porcelain
porcelain_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$porcelain_busy_id" @gl_spool)"
mkdir -p "$porcelain_spool"
printf '%s\n%s\n%s\n' tester MARK_PORCELAIN_QUEUE \
  '[gang:tester#abcd1234] MARK_PORCELAIN_QUEUE [/gang:tester#abcd1234]' \
  > "$porcelain_spool/00000000100000000000-abcd1234"
mkdir -p "$RUN_ROOT/porcelain-bin"
cat > "$RUN_ROOT/porcelain-bin/date" <<'SH'
#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
case "${1:-}" in
  +%s) printf '105\n' ;;
  *) exec /usr/bin/date "$@" ;;
esac
SH
chmod +x "$RUN_ROOT/porcelain-bin/date"
expected_porcelain="$(printf \
  'porcelain-busy\tporcelain\tbusy\t1\t5\tsid-porcelain\nporcelain-idle\tporcelain\tidle\t0\t-\tUNSTAMPED')"
default_porcelain_probe="$(PATH="$RUN_ROOT/porcelain-bin:$PATH" \
  GANG_ACTIVITY_WINDOW=0 "$GANG" roster | grep '^porcelain-')"
if [ "$default_porcelain_probe" = "$expected_porcelain" ]; then
  fail "the exact TSV instrument rejects the decorated human roster" \
    "the human roster unexpectedly matched porcelain bytes"
else
  pass "the exact TSV instrument rejects the decorated human roster"
fi
actual_porcelain="$(PATH="$RUN_ROOT/porcelain-bin:$PATH" \
  GANG_ACTIVITY_WINDOW=0 "$GANG" roster --porcelain | grep '^porcelain-')"
equal "porcelain roster prints the exact six-column rows" \
  "$expected_porcelain" "$actual_porcelain"
equal "porcelain names contain no window-state glyph bytes" \
  $'porcelain-busy\nporcelain-idle' \
  "$(printf '%s\n' "$actual_porcelain" | cut -f1)"
equal "porcelain roster is empty when its session is absent" "" \
  "$(GANG_SESSION="porcelain-absent-$$" "$GANG" roster --porcelain)"
"$GANG" drop porcelain-busy >/dev/null
"$GANG" drop porcelain-idle >/dev/null

# QUEUE AGE COMES FROM THE OLDEST LIVE ENTRY'S FIXED-WIDTH STAMP. The chosen
# time is immediate input, not a wall-clock wait; prefix matching tolerates the
# one second in which the command itself runs.
"$HITCH" agebox -c spoolable -d /tmp >/dev/null
agebox_id="$(window_id agebox)"
agebox_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$agebox_id" @gl_spool)"
mkdir -p "$agebox_spool"
age_now="$(date +%s)"
one_hour_stamp="$(printf '%011d%09d' "$(( age_now - 3600 ))" 0)"
printf '%s\n%s\n%s\n' tester MARK_AGE_ONE_HOUR \
  '[gang:tester#aaaabbbb] MARK_AGE_ONE_HOUR [/gang:tester#aaaabbbb]' \
  > "$agebox_spool/$one_hour_stamp-aaaabbbb"
age_roster_out="$("$GANG" roster 2> "$RUN_ROOT/age-roster.err")"
contains "roster reports how long the oldest message has waited" \
  "$age_roster_out" "oldest=1h"
equal "roster parses a real padded stamp without stderr noise" "" \
  "$(<"$RUN_ROOT/age-roster.err")"
contains "status reports the same oldest-message age" \
  "$("$GANG" status agebox)" "the oldest has waited 1h"
two_hour_stamp="$(printf '%011d%09d' "$(( age_now - 7200 ))" 0)"
printf '%s\n%s\n%s\n' other MARK_AGE_TWO_HOURS \
  '[gang:other#ccccdddd] MARK_AGE_TWO_HOURS [/gang:other#ccccdddd]' \
  > "$agebox_spool/$two_hour_stamp-ccccdddd"
contains "queue age follows the older of two waiting entries" \
  "$("$GANG" roster)" "oldest=2h"
"$GANG" drop agebox >/dev/null

"$HITCH" agebad -c spoolable -d /tmp >/dev/null
agebad_id="$(window_id agebad)"
agebad_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv \
  -t "$agebad_id" @gl_spool)"
mkdir -p "$agebad_spool"
printf '%s\n%s\n%s\n' tester MARK_BAD_STAMP \
  '[gang:tester#eeeeffff] MARK_BAD_STAMP [/gang:tester#eeeeffff]' \
  > "$agebad_spool/12345-abc"
agebad_row="$("$GANG" roster | grep '^agebad')"
contains "a malformed oldest stamp still reports known queue depth" \
  "$agebad_row" "spooled=1"
excludes "a malformed oldest stamp does not fabricate an age" \
  "$agebad_row" "oldest="
"$GANG" drop agebad >/dev/null

# PREEMPTION CARRIES ITS REASON THROUGH THE BOUNDARY IT CREATES. The fixture
# witnesses the collar-declared key independently and keeps a normal spool so
# backlog can prove it neither competes with nor absorbs the reason.
cat > "$RUN_ROOT/preempt-rc" <<RC
PS1='❯ '
bind -x '"\C-g": printf "%s\n" INTERRUPT_KEY_RECEIVED > "$RUN_ROOT/preempt-key"'
RC
cat > "$RUN_ROOT/collars/preemptible.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'ENV=$RUN_ROOT/preempt-rc exec bash --posix' fixture"
GANG_INTERRUPT_KEY='C-g'
GANG_STOP_HOOK=1
SH
"$HITCH" preempt -c preemptible -d /tmp >/dev/null
preempt_id="$(window_id preempt)"
tmux send-keys -l -t "$preempt_id" 'HUMAN_DRAFT'
printf 'MARK_PARKED_ONE' |
  "$GANG" send --to preempt --from tester --stdin >/dev/null
printf 'MARK_PARKED_TWO' |
  "$GANG" send --to preempt --from other --stdin >/dev/null
printf 'MARK_PARKED_THREE' |
  "$GANG" send --to preempt --from third --stdin >/dev/null
contains "the preemption world starts with three messages parked" \
  "$("$GANG" status preempt)" "spooled: 3"
tmux send-keys -t "$preempt_id" C-u
preempt_out="$("$GANG" interrupt preempt -m 'MARK_PREEMPT' --from tester)"
contains "a reasoned interrupt still reports the collar key" \
  "$preempt_out" "C-g"
[ -f "$RUN_ROOT/preempt-key" ] \
  && pass "the reasoned interrupt sends the collar-declared key" \
  || fail "the reasoned interrupt sends the collar-declared key" \
    "$RUN_ROOT/preempt-key is absent"
preempt_pane="$(pane preempt)"
contains "the reason reaches the boundary created by the interrupt" \
  "$preempt_pane" "MARK_PREEMPT"
contains "the interrupt reason carries sender attribution" \
  "$preempt_pane" "[gang:tester#"
contains "the reason never joins or consumes the existing queue" \
  "$("$GANG" status preempt)" "spooled: 3"
excludes "the reason lands before any parked backlog" \
  "$preempt_pane" "MARK_PARKED_ONE"
excludes "the preemption does not drain another sender's backlog" \
  "$preempt_pane" "MARK_PARKED_TWO"

# A STOP WITH A REASON SELF-TARGETS TOO. Bare `gang interrupt` already stops
# the calling window's own turn; the reason is the note that turn's author
# leaves for the next one, and it used to be the single self target that could
# not be spelled — the leading -m was read as an agent name and answered with a
# synopsis. The delivery has to be witnessed on the pane rather than in gang's
# own report, because a parser that resolved self and a self-send guard that
# refused afterwards both exit with the reason still in hand.
rm -f "$RUN_ROOT/preempt-key"
preempt_tmux_pane="$(tmux list-panes -t "$preempt_id" -F '#{pane_id}')"
self_reason_out="$(TMUX_PANE="$preempt_tmux_pane" \
  "$GANG" interrupt -m 'MARK_SELF_REASON' 2>&1)" || self_reason_out="REFUSED: $self_reason_out"
contains "a leading -m stops the calling window's own turn" \
  "$self_reason_out" "interrupted preempt with C-g"
[ -f "$RUN_ROOT/preempt-key" ] \
  && pass "the self-targeted stop sends the collar-declared key" \
  || fail "the self-targeted stop sends the collar-declared key" \
    "$RUN_ROOT/preempt-key is absent"
self_reason_pane="$(pane preempt)"
# source-guard: producer@1784c6b08bf6: the self-targeted interrupt above is the sole producer of MARK_SELF_REASON; no other sender, spool entry or fixture writes that literal
contains "the self-targeted reason reaches the boundary it created" \
  "$self_reason_pane" "MARK_SELF_REASON"
# source-guard: producer@ae27904d4c02: only a body whose sender is preempt itself carries this attribution, and the self-targeted reason is the one such body on this pane — every other envelope here is from tester, other or third
contains "and carries the calling agent as its author" \
  "$self_reason_pane" "[gang:preempt#"
contains "a self-targeted reason is never parked either" \
  "$("$GANG" status preempt)" "spooled: 3"

tmux send-keys -l -t "$preempt_id" 'HUMAN_DRAFT'
if preempt_refused="$("$GANG" interrupt preempt \
  -m 'MARK_UNDELIVERED' --from tester 2>&1)"; then
  fail "a reasoned interrupt refuses an occupied draft after stopping" \
    "interrupt unexpectedly succeeded"
else
  pass "a reasoned interrupt refuses an occupied draft after stopping"
fi
contains "an undelivered interrupt reason is handed back in full" \
  "$preempt_refused" "MARK_UNDELIVERED"
contains "a refused interrupt reason is never added to the queue" \
  "$("$GANG" status preempt)" "spooled: 3"
refuses "interrupt rejects an empty reason" \
  "interrupt: -m needs a non-empty message" \
  "$GANG" interrupt preempt -m '' --from tester
refuses "interrupt accepts only one reason" \
  "interrupt: -m may be passed only once" \
  "$GANG" interrupt preempt -m one -m two --from tester
refuses "interrupt rejects a sender when there is no message" \
  "--from names the author of a message" \
  "$GANG" interrupt preempt --from tester
contains "interrupt help names its reason option" \
  "$("$GANG" interrupt --help)" '-m "reason"'
tmux send-keys -t "$preempt_id" C-u
"$GANG" drop preempt >/dev/null

# A window with no spool identity is refused rather than given one here. Minting
# at the moment a message needs parking is exactly the race the identity exists
# to avoid, so gang says so instead of narrowing the window.
"$HITCH" identityless -c spoolable -d /tmp >/dev/null
tmux set-option -uw -t "$(window_id identityless)" @gl_spool
tmux send-keys -l -t "$(window_id identityless)" 'HUMAN_DRAFT'
if identityless_out="$(printf 'MARK_NO_IDENTITY' |
  "$GANG" send --to identityless --from tester --stdin 2>&1)"; then
  fail "a window with no spool identity refuses to park a message" \
    "send reported the message parked"
else
  pass "a window with no spool identity refuses to park a message"
fi
contains "and says what would have to happen instead" \
  "$identityless_out" "re-hitch or re-adopt"
# Refusing is only half of it. Minting on the way past is the race the identity
# exists to avoid, so the refusal must also leave nothing behind — a window that
# came out of this with a token would have been given one at exactly the moment
# two senders could each give it a different one.
equal "and the refusal mints nothing on its way out" "" \
  "$(tmux show-options -wqv -t "$(window_id identityless)" @gl_spool)"
"$GANG" drop identityless >/dev/null

# ADOPTION MINTS IT TOO, and nothing tested that. An adopted window is an agent
# by every other measure, so a spool identity it never received would make
# a refused send fail to park for a target the operator had just enrolled.
tmux new-window -d -t "=$GANG_SESSION" -n taken -c /tmp \
  "sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
"$GANG" adopt taken -c spoolable >/dev/null
taken_id="$(window_id taken)"
taken_token="$(tmux show-options -wqv -t "$taken_id" @gl_spool)"
if [ -n "$taken_token" ]; then
  pass "an adopted agent receives the spool identity a sender will need"
else
  fail "an adopted agent receives the spool identity a sender will need" \
    "@gl_spool is empty"
fi
# And it is a usable one, not merely a present one.
tmux send-keys -l -t "$taken_id" 'HUMAN_DRAFT'
# Tolerated rather than asserted here so a missing identity reports as the two
# red checks it is, instead of aborting the run under set -e and taking every
# later world with it.
printf 'MARK_ADOPTED' |
  "$GANG" send --to taken --from tester --stdin >/dev/null 2>&1 || true
contains "and it can park a refused message under it" \
  "$("$GANG" status taken)" "spooled: 1"
"$GANG" drop taken >/dev/null

# A body that was already typed has an unknown fate, so it is NOT parked: a
# second copy of a message that may have landed is worse than one that failed
# loudly. This composer never changes, so the paste is unverifiable.
cat > "$RUN_ROOT/collars/unverifiable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
collar_input() { printf ''; }
SH
if "$GANG" hitch unverified -c unverifiable -d /tmp >/dev/null 2>&1; then
  fail "a composer that never changes cannot complete a hitch" "hitch reported success"
else
  pass "a composer that never changes cannot complete a hitch"
fi
if unverified_out="$(printf 'MARK_UNVERIFIED' |
  "$GANG" send --to unverified --from tester --stdin 2>&1)"; then
  fail "an unverified delivery is not spooled" "send reported success"
else
  pass "an unverified delivery is not spooled"
fi
contains "it failed rather than refused" "$unverified_out" "delivery NOT verified"
unverified_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$(window_id unverified)" @gl_spool)"
unverified_parked=0
for unverified_entry in "$unverified_spool"/*; do
  [ -f "$unverified_entry" ] && unverified_parked=$((unverified_parked + 1))
done
equal "so nothing was parked for a body that may already have landed" "0" \
  "$unverified_parked"
"$GANG" drop unverified >/dev/null

# A drain that cannot verify its delivery quarantines that entry out of the
# glob and says so. It never sends it again on the chance the first attempt
# missed, and the entries behind it are not lost with it.
cat > "$RUN_ROOT/collars/wedging.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
_gl_wedge_real="\$(declare -f collar_input)"
eval "wedge_real_input \${_gl_wedge_real#collar_input}"
collar_input() { # the real box, then a draft that refuses, then one that never changes
  if [ -f "$RUN_ROOT/wedge-block" ]; then printf 'BLOCKING_DRAFT'; return 0; fi
  if [ -f "$RUN_ROOT/wedge-stuck" ]; then printf ''; return 0; fi
  wedge_real_input "\$1"
}
SH
"$HITCH" wedged -c wedging -d /tmp >/dev/null
: > "$RUN_ROOT/wedge-block"
wedged_id="$(window_id wedged)"
wedged_pane_id="$(tmux list-panes -t "$wedged_id" -F '#{pane_id}')"
printf 'MARK_WEDGED' | "$GANG" send --to wedged --from tester --stdin >/dev/null
printf 'MARK_HELD_TWO' | "$GANG" send --to wedged --from other --stdin >/dev/null
printf 'MARK_HELD_THREE' | "$GANG" send --to wedged --from third --stdin >/dev/null
contains "the blocked messages are waiting" "$("$GANG" status wedged)" "spooled: 3"
rm -f "$RUN_ROOT/wedge-block"
: > "$RUN_ROOT/wedge-stuck"
if hard_supersede_out="$(printf 'MARK_HARD_REPLACEMENT' |
  "$GANG" send --to wedged --from tester --supersede --stdin 2>&1)"; then
  fail "a hard-failed replacement does not report success" \
    "send unexpectedly succeeded"
else
  pass "a hard-failed replacement does not report success"
fi
contains "the replacement failed after typing" \
  "$hard_supersede_out" "delivery NOT verified"
contains "a hard failure supersedes nothing" \
  "$("$GANG" status wedged)" "spooled: 3"
tmux send-keys -t "$wedged_id" C-u
tmux wait-for "gang-spool-drain-$wedged_id" &
wedged_drain_waiter=$!
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$wedged_pane_id" "$GANG" hook >/dev/null
wait "$wedged_drain_waiter"
wedged_status="$("$GANG" status wedged)"
contains "an unverified drain is reported, not swallowed" \
  "$wedged_status" "spool drain NOT verified"
contains "roster carries that verdict too" "$("$GANG" roster)" "spool-held=3"
excludes "and the entry is not left where it would be sent a second time" \
  "$wedged_status" "spooled:"
wedged_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$wedged_id" @gl_spool)"
wedged_quarantined=0
for wedged_entry in "$wedged_spool"/failed-*; do
  [ -f "$wedged_entry" ] && wedged_quarantined=$((wedged_quarantined + 1))
done
equal "every body in an unverified bundle is kept where a person can read it" "3" \
  "$wedged_quarantined"
# READ IT. A file of the right name is not a kept message: the promise gang
# makes when it holds a body instead of re-sending it is that the body is still
# there, and only reading it says so.
wedged_body="$(cat "$wedged_spool"/failed-* 2>/dev/null)" || wedged_body=""
contains "the body itself, not just a file with the right name" \
  "$wedged_body" "MARK_WEDGED"
contains "the second body is named as held" "$wedged_status" "MARK_HELD_TWO"
contains "the third body is named as held" "$wedged_status" "MARK_HELD_THREE"
contains "and the sender it was parked under" "$wedged_body" "tester"
# READ OUT OF THE REPORT ITSELF, not recomputed beside it. Holding a message
# instead of re-sending it is only honest if the report says where it went, and
# a check that derives the path independently cannot see the report naming an
# empty one.
contains "and the report hands over the directory it is readable in" \
  "$wedged_status" "read them under $wedged_spool"

# A harness may accept the submission into its own queue after Gangline's
# verification failed and drain it later. The held record is therefore not a
# reason to send a second copy on another Stop event.
tmux send-keys -t "$wedged_id" Enter
rm -f "$RUN_ROOT/wedge-stuck"
wedged_arrived="$(pane wedged)"
wedged_before_count="$(printf '%s\n' "$wedged_arrived" | grep -o 'MARK_WEDGED' | wc -l | tr -d ' ')"
printf '%s' '{"hook_event_name":"Stop"}' |
  TMUX_PANE="$wedged_pane_id" "$GANG" hook >/dev/null
wedged_after_count="$(pane wedged | grep -o 'MARK_WEDGED' | wc -l | tr -d ' ')"
equal "a later native boundary never re-sends an unverified held entry" \
  "$wedged_before_count" "$wedged_after_count"
"$GANG" drop wedged >/dev/null

# One spool is deliberately left alive for the teardown below to account for.
"$HITCH" lingering -c spoolable -d /tmp >/dev/null
lingering_id="$(window_id lingering)"
tmux send-keys -l -t "$lingering_id" 'HUMAN_DRAFT'
printf 'MARK_LINGERS' |
  "$GANG" send --to lingering --from tester --stdin >/dev/null
# shellcheck disable=SC2034  # read in test/integration-hooks.sh, which accounts for this spool at teardown
lingering_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$lingering_id" @gl_spool)"
