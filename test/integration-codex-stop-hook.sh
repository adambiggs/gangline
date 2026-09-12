#!/usr/bin/env bash
# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Verified peer reply obligations and the native Stop adapter.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file and
# supplies the private tmux server, helpers, counters, and cleanup.

reply_original_collars="${GANG_COLLARS:-}"
reply_original_path="$PATH"
mkdir -p "$RUN_ROOT/collars"
cat > "$RUN_ROOT/collars/replyable.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="sh -c 'PS1=\"❯ \" exec bash --norc' fixture"
GANG_STOP_HOOK=1
collar_submitted_prompt() { # target unused, native UserPromptSubmit payload
  printf '%s' "\$2" | python3 -c '
import json, sys
value = json.load(sys.stdin).get("prompt")
if not isinstance(value, str) or not value:
    raise SystemExit(2)
print(value, end="")
'
}
collar_context() {
  tmux show-options -wqv -t "\$1" @test_context
}
SH
cat > "$RUN_ROOT/collars/reply-advisory.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$RUN_ROOT/collars/replyable.sh"
collar_dismiss_advisory() { return 1; }
SH
export GANG_COLLARS="$RUN_ROOT/collars"

# Deferred-reply timers are observed without leaving hundreds of transient
# host units behind this correlation suite. The callback tests below read the
# exact argv written here, then advance only this fixture's wall clock.
reply_timer_bin="$RUN_ROOT/reply-timer-bin"
reply_timer_args="$RUN_ROOT/reply-timer-args"
reply_real_date="$(command -v date)"
mkdir -p "$reply_timer_bin"
: > "$reply_timer_args"
cat > "$reply_timer_bin/systemd-run" <<SH
#!/bin/sh
printf '%s\n' "\$@" > "$reply_timer_args"
# A hidden entry already present here would expose a process-death interval
# with accepted mail but no recovery clock. The harmless inverse ordering is
# required: arm a callback that finds nothing until the atomic commit follows.
for arg do
  case "\$arg" in
    .deferred-*)
      for entry in "\$GANG_LOCK_DIR"/spool/*/"\$arg"; do
        [ ! -f "\$entry" ] || exit 91
      done
      ;;
  esac
done
exit \${GANG_TEST_DEFER_ARM_FAIL:-0}
SH
cat > "$reply_timer_bin/systemctl" <<'SH'
#!/bin/sh
case "$*" in
  *is-active*)
    if [ -n "${GANG_TEST_DEFER_SERVICE_GONE:-}" ]; then
      printf 'inactive\n'
      exit 3
    fi
    if [ -n "${GANG_TEST_DEFER_SERVICE_UNREADABLE:-}" ]; then
      exit 1
    fi
    printf 'active\n'
    exit 0
    ;;
  *) exit 0 ;;
esac
SH
cat > "$reply_timer_bin/date" <<SH
#!/bin/sh
if [ "\${1:-}" = +%s ] && [ -n "\${GANG_TEST_DEFER_NOW:-}" ]; then
  printf '%s\n' "\$GANG_TEST_DEFER_NOW"
else
  exec "$reply_real_date" "\$@"
fi
SH
chmod +x "$reply_timer_bin/systemd-run" "$reply_timer_bin/systemctl" \
  "$reply_timer_bin/date"
export PATH="$reply_timer_bin:$PATH"

reply_stop_hook="$ROOT/collars/plugins/codex-stop-hook.py"
reply_stop_payload='{"hook_event_name":"Stop","stop_hook_active":false}'
reply_stop_active_payload='{"hook_event_name":"Stop","stop_hook_active":true}'
reply_stop_stderr="$RUN_ROOT/reply-stop-stderr"

reply_prompt_event() { # $1 pane, $2 exact native prompt
  python3 -c 'import json,sys; print(json.dumps({"hook_event_name":"UserPromptSubmit","prompt":sys.argv[1]}))' "$2" \
    | TMUX_PANE="$1" "$GANG" hook >/dev/null
}

reply_stop_run() { # $1 pane, optional $2 payload; sets reply_stop_output
  : > "$reply_stop_stderr"
  reply_stop_output="$(printf '%s' "${2:-$reply_stop_payload}" \
    | TMUX_PANE="$1" python3 "$reply_stop_hook" "$GANG" 2> "$reply_stop_stderr")"
}

reply_nonce_from() { # $1 window, $2 witnessed sender -> its one nonce
  local key value nonce marker _token witness mode _digest _replies extra
  while read -r key value; do
    case "$key" in @gl_reply_*) ;; *) continue ;; esac
    nonce="${key#@gl_reply_}"
    IFS=: read -r marker _token witness mode _digest _replies extra <<<"$value"
    [ "$marker" = message ] && [ "$witness" = "$2" ] && [ -z "$extra" ] || continue
    case "$mode" in
      request|waived) [ -z "$(tmux show-options -wqv -t "$1" "@gl_rsettled_$nonce")" ] || continue ;;
      reply) [ -z "$(tmux show-options -wqv -t "$1" "@gl_rprompt_$nonce")" ] || continue ;;
      *) continue ;;
    esac
    printf '%s' "$nonce"
    return 0
  done <<<"$(tmux show-options -w -t "$1")"
  return 1
}

reply_nonce_for_body() { # target window, sender, mode, body -> correlated nonce
  local key value nonce marker _token witness mode digest replies extra wire seen
  while read -r key value; do
    case "$key" in @gl_reply_*) ;; *) continue ;; esac
    nonce="${key#@gl_reply_}"
    IFS=: read -r marker _token witness mode digest replies extra <<<"$value"
    [ "$marker" = message ] && [ "$witness" = "$2" ] \
      && [ "$mode" = "$3" ] && [ -z "$extra" ] || continue
    if [ "$mode" = request ]; then
      wire="$(reply_request_envelope "$witness" "$nonce" "$4")"
    else
      wire="$(reply_response_envelope "$witness" "$nonce" "$replies" "$4")"
    fi
    seen="$(printf '%s' "$wire" | sha256sum)"
    [ "${seen%% *}" = "$digest" ] || continue
    printf '%s' "$nonce"
    return 0
  done <<<"$(tmux show-options -w -t "$1")"
  return 1
}

reply_record_wire() { # window nonce body -> the exact envelope its record digests
  local value _marker _token witness mode _digest replies _extra
  value="$(tmux show-options -wqv -t "$1" "@gl_reply_$2")"
  IFS=: read -r _marker _token witness mode _digest replies _extra <<<"$value"
  if [ "$mode" = request ]; then
    reply_request_envelope "$witness" "$2" "$3"
  else
    reply_response_envelope "$witness" "$2" "$replies" "$3"
  fi
}

reply_request_envelope() { # sender nonce body
  printf '[gang:%s#%s] %s [/gang:%s#%s]' "$1" "$2" "$3" "$1" "$2"
}

reply_waived_envelope() { # sender nonce body
  printf '[gang:%s#%s no-reply] %s [/gang:%s#%s]' "$1" "$2" "$3" "$1" "$2"
}

reply_response_envelope() { # sender nonce reply-to body
  printf '[gang:%s#%s reply-to=%s] %s [/gang:%s#%s]' \
    "$1" "$2" "$3" "$4" "$1" "$2"
}

"$HITCH" reply-a -c replyable -d /tmp >/dev/null
reply_a_id="$(window_id reply-a)"
reply_a_pane="$(tmux list-panes -t "$reply_a_id" -F '#{pane_id}')"
# An agent hitches B, deliberately: lifecycle ancestry must not create debt.
TMUX_PANE="$reply_a_pane" "$HITCH" reply-b -c replyable -d /tmp >/dev/null
reply_b_id="$(window_id reply-b)"
reply_b_pane="$(tmux list-panes -t "$reply_b_id" -F '#{pane_id}')"
"$HITCH" reply-c -c replyable -d /tmp >/dev/null
reply_c_id="$(window_id reply-c)"
reply_c_pane="$(tmux list-panes -t "$reply_c_id" -F '#{pane_id}')"
# This remains a valid peer name even though older internal mail used the same
# author string. Legacy parsing must not let that namespace collision erase a
# possible peer obligation.
"$HITCH" auto-resume -c replyable -d /tmp >/dev/null

equal "an agent-created window starts with no peer reply debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_prompt_event "$reply_b_pane" "operator-only prompt"
equal "operator/session-keyboard input creates no peer reply debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "operator-only work is allowed to become idle" "{}" "$reply_stop_output"

# A SENDER MAY WAIVE THE REPLY. --no-reply stamps the envelope and the record,
# so the recipient owes nothing for the message and explain keeps the waiver as
# audit. The thread still closes at the reading turn's boundary, so a later
# message to the sender is a fresh request rather than an answer.
reply_waived_out="$(printf '%s' WAIVED_ONE \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --no-reply --stdin)"
contains "a waived send to an idle recipient is delivered at once" \
  "$reply_waived_out" "delivered to reply-b"
contains "a waived send says it owes no reply" "$reply_waived_out" "(owes no reply)"
reply_waived_one="$(reply_nonce_from "$reply_b_id" reply-a)" || reply_waived_one=""
equal "the waived send leaves its recipient one record" 16 "${#reply_waived_one}"
contains "the waiver travels in the envelope" "$(pane_all reply-b)" \
  "[gang:reply-a#$reply_waived_one no-reply] WAIVED_ONE"
reply_waived_one_digest="$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_waived_one" | cut -d: -f5)"
equal "the waived record carries its envelope digest" 64 "${#reply_waived_one_digest}"
reply_prompt_event "$reply_b_pane" \
  "$(reply_waived_envelope reply-a "$reply_waived_one" WAIVED_ONE)"
equal "a waived message the recipient has read owes no reply" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
equal "the read waived message has its prompt proof" "$reply_waived_one_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rprompt_$reply_waived_one")"
excludes "status names no debt for a waived message" "$($GANG status reply-b)" \
  "reply owed to reply-a"
reply_stop_run "$reply_b_pane"
equal "a waived message never refuses idle" "{}" "$reply_stop_output"
equal "the reading turn's boundary closes the waived thread" \
  "$reply_waived_one_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_waived_one")"
contains "explain keeps the waiver as audit" "$($GANG explain reply-b)" \
  "reply waived by reply-a (message $reply_waived_one; nothing owed; read)"

# An answer to a waived message is allowed, correlated, delivered at once, and
# opens no reciprocal debt.
contains "a second waived send is delivered at once" \
  "$(printf '%s' WAIVED_TWO \
    | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --no-reply --stdin)" \
  "delivered to reply-b"
reply_waived_two="$(reply_nonce_from "$reply_b_id" reply-a)" || reply_waived_two=""
equal "the second waived send leaves its own record" 16 "${#reply_waived_two}"
reply_waived_two_digest="$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_waived_two" | cut -d: -f5)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_waived_envelope reply-a "$reply_waived_two" WAIVED_TWO)"
contains "an answer to a waived message is delivered at once" \
  "$(printf '%s' WAIVED_ANSWER | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin)" \
  "delivered to reply-a"
reply_waived_answer="$(reply_nonce_for_body "$reply_a_id" reply-b reply WAIVED_ANSWER)" \
  || reply_waived_answer=""
equal "the answer leaves its recipient one record" 16 "${#reply_waived_answer}"
equal "the answer is correlated to the waived message" "$reply_waived_two" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_waived_answer" | cut -d: -f6)"
equal "the answer settles the waived record" "$reply_waived_two_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_waived_two")"
reply_prompt_event "$reply_a_pane" \
  "$(reply_response_envelope reply-b "$reply_waived_answer" "$reply_waived_two" WAIVED_ANSWER)"
equal "an answer to a waived message opens no reciprocal debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
reply_stop_run "$reply_b_pane"
for reply_waived_prefix in @gl_reply_ @gl_rprompt_ @gl_rdelivery_ @gl_rsettled_; do
  tmux set-option -uqw -t "$reply_b_id" "$reply_waived_prefix$reply_waived_one"
  tmux set-option -uqw -t "$reply_b_id" "$reply_waived_prefix$reply_waived_two"
  tmux set-option -uqw -t "$reply_a_id" "$reply_waived_prefix$reply_waived_answer"
done
printf '%s' OPERATOR_ENVELOPE \
  | "$GANG" send --to reply-b --from operator --stdin >/dev/null
equal "a verified self-declared operator envelope creates no peer debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"

# A REPLY THAT CREATES NO RECIPROCAL DEBT DOES NOT EARN A TURN OF ITS OWN.
# Requests remain immediate because their native prompt proof is what opens the
# recipient's obligation; replies wait for the next request that would wake the
# recipient anyway. The unrelated request already owed by the reply recipient
# is the safety discriminator: holding must not settle, shadow, or delay it.
"$HITCH" defer-a -c replyable -d "$RUN_ROOT" >/dev/null
"$HITCH" defer-b -c replyable -d "$RUN_ROOT" >/dev/null
"$HITCH" defer-c -c replyable -d "$RUN_ROOT" >/dev/null
defer_a_id="$(window_id defer-a)"
defer_a_pane="$(tmux list-panes -t "$defer_a_id" -F '#{pane_id}')"
defer_b_id="$(window_id defer-b)"
defer_b_pane="$(tmux list-panes -t "$defer_b_id" -F '#{pane_id}')"
defer_c_id="$(window_id defer-c)"
defer_c_pane="$(tmux list-panes -t "$defer_c_id" -F '#{pane_id}')"

# Prepare the only reply subtype that may be quiet: B asks A, A's answer wakes
# B immediately, B reads it, and B's next message merely acknowledges that
# reply. Direct answers and pure acknowledgements are distinguishable from the
# same immutable mode records the production classifier reads.
DEFER_ACK_ANSWER_META="" DEFER_ACK_ANSWER_PROMPT=""
prepare_defer_ack_chain() { # $1 unique fixture prefix
  local request="${1}_REQUEST" answer="${1}_ANSWER" request_nonce answer_nonce
  printf '%s' "$request" \
    | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
  request_nonce="$(reply_nonce_for_body \
    "$defer_a_id" defer-b request "$request")"
  reply_prompt_event "$defer_a_pane" \
    "$(reply_request_envelope defer-b "$request_nonce" "$request")"
  DEFER_DIRECT_OUT="$(printf '%s' "$answer" \
    | TMUX_PANE="$defer_a_pane" "$GANG" send --to defer-b --stdin)"
  answer_nonce="$(reply_nonce_for_body \
    "$defer_b_id" defer-a reply "$answer")"
  reply_prompt_event "$defer_b_pane" \
    "$(reply_response_envelope defer-a "$answer_nonce" "$request_nonce" "$answer")"
  DEFER_ACK_ANSWER_META="$(tmux show-options -wqv -t "$defer_b_id" "@gl_reply_$answer_nonce")"
  DEFER_ACK_ANSWER_PROMPT="$(tmux show-options -wqv -t "$defer_b_id" "@gl_rprompt_$answer_nonce")"
}

prepare_defer_ack_chain DEFER_INITIAL
contains "an answer to the recipient's own request stays immediate" \
  "$DEFER_DIRECT_OUT" "delivered to defer-b"
# source-guard: producer@b8242f807df4: DEFER_INITIAL_ANSWER is unique to the direct answer whose immediate send verdict is asserted above
contains "the awaited answer wakes its recipient" \
  "$(pane_all defer-b)" "DEFER_INITIAL_ANSWER"
# A answered the request, but its native turn remains open until Stop. Close
# that settled turn so defer-c's unrelated request exercises an idle recipient
# instead of entering the ordinary busy-recipient queue.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
printf '%s' DEFER_OWED_REQUEST \
  | TMUX_PANE="$defer_c_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_owed_request="$(reply_nonce_for_body \
  "$defer_a_id" defer-c request DEFER_OWED_REQUEST)"
reply_prompt_event "$defer_a_pane" \
  "$(reply_request_envelope defer-c "$defer_owed_request" DEFER_OWED_REQUEST)"
# source-guard: producer@dc07cf123c5d: DEFER_OWED_REQUEST is minted only by defer-c's immediately preceding verified send and prompt witness
equal "a request that opens reply debt wakes its recipient immediately" yes \
  "$([[ "$(pane_all defer-a)" == *DEFER_OWED_REQUEST* ]] \
      && printf yes || printf no)"
defer_owed_before="$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
equal "the immediate request leaves its reply obligation standing" \
  $'owed\t'"$defer_owed_request"$'\tdefer-c\tlive' "$defer_owed_before"
# End the native turn without forgiving the standing obligation. This makes
# defer-a idle before the correlated reply arrives, so unfixed live delivery
# necessarily wakes it and turns the negative assertion below red.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
equal "ending the recipient's turn preserves its unrelated obligation" \
  "$defer_owed_before" \
  "$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
defer_reply_out="$(printf '%s' DEFERRED_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin)"
excludes "a pure acknowledgement is accepted without waking its recipient" \
  "$(pane_all defer-a)" "DEFERRED_REPLY"
contains "the accepted reply says it is deliberately deferred" \
  "$defer_reply_out" "held for defer-a"
contains "status exposes the bounded no-reply hold" \
  "$($GANG status defer-a)" "deferred delivery: 1 envelope owes no reply"
contains "roster separates deferred envelopes from waking queue depth" \
  "$($GANG roster | grep '^defer-a ')" "deferred=1"
contains "mail exposes the complete held envelope without consuming it" \
  "$($GANG mail defer-a)" "DEFERRED_REPLY"
equal "holding a correlated reply cannot hide an obligation already owed" \
  "$defer_owed_before" \
  "$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
defer_a_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"
defer_hidden_before="$(find "$defer_a_spool" -maxdepth 1 -type f -name '.deferred-*' -printf '%f\n')"
reply_prompt_event "$defer_a_pane" "operator prompt while acknowledgement remains held"
excludes "a native prompt alone does not deliver the held acknowledgement" \
  "$(pane_all defer-a)" "DEFERRED_REPLY"
equal "a native prompt leaves the deferred entry hidden" \
  "$defer_hidden_before" \
  "$(find "$defer_a_spool" -maxdepth 1 -type f -name '.deferred-*' -printf '%f\n')"
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
tmux set-option -w -t "$defer_a_id" @gl_collar reply-advisory
defer_tick_pane_before="$(pane_all defer-a)"
GANG_TEST_TICK_MODE=manual "$GANG" tick >/dev/null
# source-guard: whole-surface@518a167ba3cf: an advisory-only tick must leave every byte of this dedicated recipient pane unchanged because it has no ordinary delivery to submit
equal "an advisory tick with no ordinary mail types no standalone acknowledgement" \
  "$defer_tick_pane_before" "$(pane_all defer-a)"
equal "an advisory tick with no ordinary mail leaves the deferred entry hidden" \
  "$defer_hidden_before" \
  "$(find "$defer_a_spool" -maxdepth 1 -type f -name '.deferred-*' -printf '%f\n')"
tmux set-option -w -t "$defer_a_id" @gl_collar replyable
printf '%s' DEFER_WAKE_REQUEST \
  | TMUX_PANE="$defer_c_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_bundle="$(pane_all defer-a)"
# source-guard: producer@9153b7e0f59e: the two unique markers bind this combined pane read to the held reply and the immediately preceding waking send
equal "the next waking request carries the held reply ahead of itself" \
  "DEFERRED_REPLY DEFER_WAKE_REQUEST " \
  "$(printf '%s\n' "$defer_bundle" \
      | grep -oE 'DEFERRED_REPLY|DEFER_WAKE_REQUEST' | awk '!seen[$0]++' | tr '\n' ' ')"
# source-guard: producer@21fa8863e6ac: the accumulated marker is emitted only by the deferred reply whose two unique delivery markers were just ordered on this pane
contains "the delivered reply is marked as accumulated context" \
  "$defer_bundle" "accumulated context; held because this envelope owed no reply"

# The reply settled defer-b's debt but did not itself end defer-b's native
# turn. Close that turn before using the next request as an immediate delivery
# witness; otherwise the established spool path quite correctly parks it.
defer_b_stop_rc=0
reply_stop_run "$defer_b_pane" || defer_b_stop_rc=$?
defer_b_stop_why="$(tr '\n' ' ' < "$reply_stop_stderr")"
equal "the sender can end its turn after its deferred reply is accepted" 0 \
  "$defer_b_stop_rc${defer_b_stop_why:+: $defer_b_stop_why}"
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
prepare_defer_ack_chain DEFER_NATIVE
defer_native_older_out="$(printf '%s' DEFER_NATIVE_OLDER_REQUEST \
  | TMUX_PANE="$defer_c_pane" "$GANG" send --to defer-a --stdin)"
contains "an older request waits while the recipient's turn is live" \
  "$defer_native_older_out" "queued for defer-a"
printf '%s' DEFER_NATIVE_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_native_obligations_before="$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
defer_native_drain_channel="gang-spool-drain-$defer_a_id"
tmux wait-for "$defer_native_drain_channel" &
defer_native_drain_wait=$!
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
wait "$defer_native_drain_wait"
defer_native_bundle="$(pane_all defer-a)"
# source-guard: producer@c7fe7240e1a5: both markers are unique to the ordinary request and held acknowledgement that the immediately preceding Stop must submit in one bundle
equal "one waking Stop preserves global order across ordinary and deferred mail" \
  "DEFER_NATIVE_OLDER_REQUEST DEFER_NATIVE_REPLY " \
  "$(printf '%s\n' "$defer_native_bundle" \
      | grep -oE 'DEFER_NATIVE_OLDER_REQUEST|DEFER_NATIVE_REPLY' \
      | awk '!seen[$0]++' | tr '\n' ' ')"
equal "drain-time promotion cannot change the recipient's obligation set" \
  "$defer_native_obligations_before" \
  "$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
equal "the waking Stop removes the hidden entry exactly once" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"

reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
defer_b_stop_rc=0
reply_stop_run "$defer_b_pane" || defer_b_stop_rc=$?
defer_b_stop_why="$(tr '\n' ' ' < "$reply_stop_stderr")"
equal "the sender can end its turn after bundled reply delivery" 0 \
  "$defer_b_stop_rc${defer_b_stop_why:+: $defer_b_stop_why}"
prepare_defer_ack_chain DEFER_SUPERSEDE
printf '%s' DEFER_SUPERSEDED_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_replacement_out="$(printf '%s' DEFER_REPLACEMENT_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --supersede --stdin)"
defer_superseded_mail="$($GANG mail defer-a)"
excludes "supersession retires an older reply from the deferred namespace" \
  "$defer_superseded_mail" "DEFER_SUPERSEDED_REPLY"
contains "the replacement inherits the retired reply's correlation" \
  "$defer_superseded_mail" "DEFER_REPLACEMENT_REPLY"
# Acceptance of the first acknowledgement closes its thread. The replacement
# inherits correlation so it opens no debt, but it matches no still-open reply
# record and therefore cannot inherit quiet-delivery eligibility. Because A is
# live, ordinary immediate delivery means the replacement waits in the waking
# queue until A's native boundary drains it.
contains "a replacement after the acknowledgement stays on the ordinary path" \
  "$defer_replacement_out" "queued for defer-a"
contains "the ordinary replacement remains visible in the waking queue" \
  "$($GANG roster | grep '^defer-a ')" "spooled=1"
equal "supersession moves the replacement out of the hidden namespace" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
excludes "the superseded body never reaches that context" \
  "$(pane_all defer-a)" "DEFER_SUPERSEDED_REPLY"

# Supersession may inherit an old correlation without inheriting quiet-delivery
# eligibility. Once B's reply-reading turn is closed, its new instructions are
# a fresh wake even when they replace a still-held acknowledgement.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
reply_stop_run "$defer_b_pane"
prepare_defer_ack_chain DEFER_FRESH_SUPERSEDE
printf '%s' DEFER_OLD_ACK \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
reply_stop_run "$defer_b_pane"
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
defer_fresh_supersede_out="$(printf '%s' DEFER_FRESH_REQUEST \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --supersede --stdin)"
contains "a fresh request that supersedes a held acknowledgement stays immediate" \
  "$defer_fresh_supersede_out" "delivered to defer-a"
# source-guard: producer@ba50b5eba65f: DEFER_FRESH_REQUEST is unique to the immediate superseding send whose verdict is asserted above
contains "the superseding request wakes its idle recipient" \
  "$(pane_all defer-a)" "DEFER_FRESH_REQUEST"
excludes "the retired acknowledgement never reaches the recipient" \
  "$(pane_all defer-a)" "DEFER_OLD_ACK"
equal "fresh supersession leaves no hidden acknowledgement" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"

# The bound is executable recovery, not status prose. Open another request at
# defer-b, accept its reply into the quiet queue, and fire the exact callback
# recorded by the fake transient timer after that entry's own due epoch.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
reply_stop_run "$defer_b_pane"
prepare_defer_ack_chain DEFER_BOUND
# The deadline is allowed to create a turn only when the recipient is idle.
# Close A's settled answer turn before B submits the quiet acknowledgement.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
defer_bound_before="$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
defer_bound_out="$(printf '%s' DEFER_BOUND_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin)"
contains "the second correlated reply is held behind a forced wake" \
  "$defer_bound_out" "retrying deadline starts in 30m"
excludes "the bounded hold still creates no immediate recipient turn" \
  "$(pane_all defer-a)" "DEFER_BOUND_REPLY"
defer_bound_entry="$(awk '/^\.deferred-[0-9]/ { print; exit }' "$reply_timer_args")"
defer_bound_unit="$(awk -F= '/^--unit=/ { print $2; exit }' "$reply_timer_args")"
defer_bound_due="${defer_bound_entry#.deferred-}"
defer_bound_due="${defer_bound_due%%-*}"
contains "the deadline service retries a callback that meets lock contention" \
  "$(cat "$reply_timer_args")" "--property=Restart=on-failure"
contains "deadline retries are paced rather than spun" \
  "$(cat "$reply_timer_args")" "--property=RestartSec=5s"
defer_bound_lock="$GANG_LOCK_DIR/$(printf '%s' "$defer_a_id" | tr -c 'A-Za-z0-9' '_').lock"
ln -s "$$" "$defer_bound_lock"
defer_bound_retry_rc=0
defer_bound_retry_err="$(GANG_TEST_DEFER_NOW="$((10#$defer_bound_due + 1))" \
  "$GANG" at --fire "$defer_bound_entry" --to defer-a \
    --unit "$defer_bound_unit" 2>&1)" || defer_bound_retry_rc=$?
equal "a deadline callback reports lock contention as retryable failure" 3 \
  "$defer_bound_retry_rc"
contains "the retryable callback names the competing delivery" \
  "$defer_bound_retry_err" "another Gangline process is delivering to defer-a"
contains "lock contention leaves the exact hidden envelope for the service retry" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-*)" \
  "$defer_bound_entry"
contains "status names an overdue envelope with live retry authority" \
  "$(GANG_TEST_DEFER_NOW="$((10#$defer_bound_due + 1))" "$GANG" status defer-a)" \
  "passed its deadline by 1s; its retry service is live"
rm -f -- "$defer_bound_lock"
GANG_TEST_DEFER_NOW="$((10#$defer_bound_due + 1))" \
  "$GANG" at --fire "$defer_bound_entry" --to defer-a \
    --unit "$defer_bound_unit" >/dev/null
# source-guard: producer@6f02d1a96735: DEFER_BOUND_REPLY is unique to the correlated send whose recorded timer callback was fired immediately above
contains "the deadline callback forces the held reply into the recipient" \
  "$(pane_all defer-a)" "DEFER_BOUND_REPLY"
equal "the forced wake also leaves the unrelated obligation visible" \
  "$defer_bound_before" \
  "$(TMUX_PANE="$defer_a_pane" "$GANG" reply-obligations)"
equal "the forced delivery leaves no hidden entry behind" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"

reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
reply_stop_run "$defer_b_pane"
prepare_defer_ack_chain DEFER_NOARM
# With no deadline service available, the optimization falls back through the
# ordinary immediate path. Make that path's live-delivery outcome observable.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
defer_noarm_out="$(printf '%s' DEFER_NOARM_REPLY \
  | GANG_TEST_DEFER_ARM_FAIL=1 TMUX_PANE="$defer_b_pane" \
    "$GANG" send --to defer-a --stdin 2>&1)"
contains "a host that cannot arm the bound says deferral is unavailable" \
  "$defer_noarm_out" "preserving immediate delivery"
contains "and falls back to a verified immediate delivery" \
  "$defer_noarm_out" "delivered to defer-a"
# source-guard: producer@2449e5abf204: DEFER_NOARM_REPLY is unique to the timer-refused send whose verified fallback verdict is asserted immediately above
contains "the fallback reply reached the recipient rather than disappearing" \
  "$(pane_all defer-a)" "DEFER_NOARM_REPLY"
equal "timer-arm failure leaves no hidden envelope behind" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"

# A superseding message may match a newly read reply while inheriting the
# request correlation of an older parked answer. The inherited request makes
# quiet delivery unsafe even though the fresh classifier saw only replies.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
reply_stop_run "$defer_b_pane"
prepare_defer_ack_chain DEFER_INHERITED_REQUEST
printf '%s' DEFER_INHERITED_OLD_ACK \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_inherited_clone=f8f8f8f8f8f8f8f8
tmux set-option -w -t "$defer_b_id" "@gl_reply_$defer_inherited_clone" \
  "$DEFER_ACK_ANSWER_META"
tmux set-option -w -t "$defer_b_id" "@gl_rprompt_$defer_inherited_clone" \
  "$DEFER_ACK_ANSWER_PROMPT"
defer_inherited_out="$(printf '%s' DEFER_INHERITED_REPLACEMENT \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --supersede --stdin)"
contains "inherited request correlation keeps a superseding acknowledgement waking" \
  "$defer_inherited_out" "queued for defer-a"
excludes "the inherited-request replacement is never accepted as a quiet hold" \
  "$defer_inherited_out" "held for defer-a"
equal "inherited request correlation leaves no hidden replacement" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"
contains "the waking replacement remains readable in the ordinary queue" \
  "$($GANG mail defer-a)" "DEFER_INHERITED_REPLACEMENT"
TMUX_PANE="$defer_a_pane" "$GANG" mail >/dev/null
tmux set-option -uw -t "$defer_b_id" "@gl_reply_$defer_inherited_clone"
tmux set-option -uw -t "$defer_b_id" "@gl_rprompt_$defer_inherited_clone"
tmux set-option -uw -t "$defer_b_id" "@gl_rsettled_$defer_inherited_clone"

# A first deadline invocation delayed by suspend still gets one real attempt.
# Its wall-clock lateness limits only repeat failures; it is not a precondition
# that can abandon a deliverable envelope before touching the pane lock.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
reply_stop_run "$defer_b_pane"
prepare_defer_ack_chain DEFER_LATE_FIRST
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
printf '%s' DEFER_LATE_FIRST_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_late_entry="$(awk '/^\.deferred-[0-9]/ { print; exit }' "$reply_timer_args")"
defer_late_unit="$(awk -F= '/^--unit=/ { print $2; exit }' "$reply_timer_args")"
defer_late_due="${defer_late_entry#.deferred-}"
defer_late_due="${defer_late_due%%-*}"
GANG_TEST_DEFER_NOW="$((10#$defer_late_due + 301))" \
  "$GANG" at --fire "$defer_late_entry" --to defer-a \
    --unit "$defer_late_unit" >/dev/null
# source-guard: producer@dca9bfd40f76: DEFER_LATE_FIRST_REPLY is unique to the held acknowledgement whose deliberately late first callback fires immediately above
contains "a suspend-delayed first callback still attempts and delivers" \
  "$(pane_all defer-a)" "DEFER_LATE_FIRST_REPLY"
equal "a successful late first attempt leaves no hidden envelope" "" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-* \
      | grep -v '\*$' || :)"

# Persistent callback failure has a finite retry budget. Once it is spent the
# service exits successfully, the hidden envelope stays readable, and status
# hands the stopped recovery to the operator instead of spawning forever.
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
reply_stop_run "$defer_b_pane"
prepare_defer_ack_chain DEFER_RETRY_CAP
reply_stop_run "$defer_a_pane" "$reply_stop_active_payload"
printf '%s' DEFER_RETRY_CAP_REPLY \
  | TMUX_PANE="$defer_b_pane" "$GANG" send --to defer-a --stdin >/dev/null
defer_cap_entry="$(awk '/^\.deferred-[0-9]/ { print; exit }' "$reply_timer_args")"
defer_cap_unit="$(awk -F= '/^--unit=/ { print $2; exit }' "$reply_timer_args")"
defer_cap_due="${defer_cap_entry#.deferred-}"
defer_cap_due="${defer_cap_due%%-*}"
defer_cap_lock="$GANG_LOCK_DIR/$(printf '%s' "$defer_a_id" | tr -c 'A-Za-z0-9' '_').lock"
ln -s "$$" "$defer_cap_lock"
defer_cap_rc=0
defer_cap_err="$(GANG_TEST_DEFER_NOW="$((10#$defer_cap_due + 301))" \
  "$GANG" at --fire "$defer_cap_entry" --to defer-a \
    --unit "$defer_cap_unit" 2>&1)" || defer_cap_rc=$?
equal "the deadline retry process stops after its bounded recovery window" 0 \
  "$defer_cap_rc"
contains "retry exhaustion names the retained envelope and operator handoff" \
  "$defer_cap_err" "exhausted its 5m retry window after a failed promotion attempt"
contains "retry exhaustion leaves the hidden envelope readable" \
  "$(printf '%s\n' "$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"/.deferred-*)" \
  "$defer_cap_entry"
contains "status reports that capped retry authority is gone" \
  "$(GANG_TEST_DEFER_NOW="$((10#$defer_cap_due + 301))" \
      GANG_TEST_DEFER_SERVICE_GONE=1 "$GANG" status defer-a)" \
  "retry budget was exhausted after a failed promotion attempt"
contains "the exhaustion marker explains the handoff when service state is unreadable" \
  "$(GANG_TEST_DEFER_NOW="$((10#$defer_cap_due + 301))" \
      GANG_TEST_DEFER_SERVICE_UNREADABLE=1 "$GANG" status defer-a)" \
  "retry budget was exhausted after a failed promotion attempt"

# A broken spool cannot record the detailed exhaustion marker. The callback
# still stops at its finite bound, and status names the marker-less GONE state
# instead of letting systemd respawn a process that cannot repair the disk.
defer_cap_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$defer_a_id" @gl_spool)"
defer_cap_marker="$defer_cap_spool/.retry-exhausted-$defer_cap_entry"
rm -f -- "$defer_cap_marker"
chmod 500 "$defer_cap_spool"
defer_marker_rc=0
defer_marker_err="$(GANG_TEST_DEFER_NOW="$((10#$defer_cap_due + 301))" \
  "$GANG" at --fire "$defer_cap_entry" --to defer-a \
    --unit "$defer_cap_unit" 2>&1)" || defer_marker_rc=$?
chmod 700 "$defer_cap_spool"
equal "an unwritable exhaustion handoff stops the retry process" 0 \
  "$defer_marker_rc"
contains "marker failure says the callback stopped rather than remaining retryable" \
  "$defer_marker_err" "stopping the callback so it cannot spin"
contains "status reports marker-less exhaustion as a GONE service of unknown cause" \
  "$(GANG_TEST_DEFER_NOW="$((10#$defer_cap_due + 301))" \
      GANG_TEST_DEFER_SERVICE_GONE=1 "$GANG" status defer-a)" \
  "retry service is GONE for an unknown reason"

# Hold the callback immediately after its failed attempt, then model the pane
# lock holder promoting the same entry. No exhausted marker may be written for
# the envelope that left the hidden namespace in that window.
defer_race_ready="$RUN_ROOT/defer-attempt-ready"
defer_race_release="$RUN_ROOT/defer-attempt-release"
defer_race_rc_file="$RUN_ROOT/defer-attempt-rc"
defer_race_out="$RUN_ROOT/defer-attempt.out"
defer_race_err="$RUN_ROOT/defer-attempt.err"
mkfifo "$defer_race_ready" "$defer_race_release"
(
  defer_race_rc=0
  GANG_TEST_DEFER_NOW="$((10#$defer_cap_due + 301))" \
    GANG_TEST_DEFER_ATTEMPT_READY_FIFO="$defer_race_ready" \
    GANG_TEST_DEFER_ATTEMPT_RELEASE_FIFO="$defer_race_release" \
    "$GANG" at --fire "$defer_cap_entry" --to defer-a \
      --unit "$defer_cap_unit" >"$defer_race_out" 2>"$defer_race_err" \
      || defer_race_rc=$?
  printf '%s\n' "$defer_race_rc" > "$defer_race_rc_file"
) &
defer_race_pid=$!
IFS= read -r _ < "$defer_race_ready" || true
defer_race_original="${defer_cap_entry#".deferred-${defer_cap_due}-"}"
mv -- "$defer_cap_spool/$defer_cap_entry" \
  "$defer_cap_spool/$defer_race_original"
printf x > "$defer_race_release"
wait "$defer_race_pid"
equal "a concurrent promotion ends the failed callback cleanly" 0 \
  "$(cat "$defer_race_rc_file")"
equal "a concurrently promoted envelope gets no orphan exhaustion marker" no \
  "$([ -e "$defer_cap_marker" ] && printf yes || printf no)"
equal "the simulated lock holder moved the envelope into the waking queue" yes \
  "$([ -f "$defer_cap_spool/$defer_race_original" ] && printf yes || printf no)"
rm -f -- "$defer_cap_spool/$defer_race_original"
rm -f -- "$defer_cap_lock"
tmux set-option -uw -t "$defer_a_id" "@gl_reply_$defer_owed_request"
tmux set-option -uw -t "$defer_a_id" "@gl_rprompt_$defer_owed_request"
tmux set-option -uw -t "$defer_a_id" "@gl_rdelivery_$defer_owed_request"
"$GANG" drop defer-a >/dev/null
"$GANG" drop defer-b >/dev/null
"$GANG" drop defer-c >/dev/null

# Force native prompt proof to land while transport verification is paused
# after reading the same immutable record. The suite tmux shim supplies the
# event barrier; there is no polling or timing claim in this race proof.
reply_race_gate="reply-proof-race-$$"
reply_race_state="$RUN_ROOT/reply-proof-race-state"
reply_race_out="$RUN_ROOT/reply-proof-race.out"
reply_race_err="$RUN_ROOT/reply-proof-race.err"
printf '%s' REQ_PROMPT_FIRST \
  | GANG_TEST_REPLY_PROOF_GATE="$reply_race_gate" \
    GANG_TEST_REPLY_GATE_STATE="$reply_race_state" \
    TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin \
      >"$reply_race_out" 2>"$reply_race_err" &
reply_race_pid=$!
tmux wait-for "$reply_race_gate-ready"
reply_race_nonce="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_race_nonce" REQ_PROMPT_FIRST)"
equal "the native prompt proof alone arms the debt while delivery is in flight" \
  $'owed\t'"$reply_race_nonce"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "a prompt-witnessed request asks for the reply that clears it" \
  "$reply_stop_output" "reply to reply-a"
printf '%s' ACK_PROMPT_FIRST \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_race_ack_nonce="$(reply_nonce_for_body \
  "$reply_a_id" reply-b reply ACK_PROMPT_FIRST)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_response_envelope reply-b "$reply_race_ack_nonce" \
    "$reply_race_nonce" ACK_PROMPT_FIRST)"
reply_stop_run "$reply_a_pane"
equal "the prompt-first reply recipient may idle without reciprocal debt" \
  "{}" "$reply_stop_output"
equal "a correlated reply discharges prompt-only provenance" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the answered debtor may idle while its delivery proof is in flight" \
  "{}" "$reply_stop_output"
reply_race_meta="$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_race_nonce")"
IFS=: read -r _ _ _ _ reply_race_digest _ <<<"$reply_race_meta"
equal "the discharged record carries no delivery proof yet" "" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rdelivery_$reply_race_nonce")"
tmux wait-for -S "$reply_race_gate-release"
wait "$reply_race_pid"
equal "the released delivery proof completes the discharged audit record" \
  "$reply_race_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rdelivery_$reply_race_nonce")"
equal "the missing delivery proof completes the already correlated settlement" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the prompt-first sender may idle after both proofs complete" \
  "{}" "$reply_stop_output"

# Direct verified request. Delivery proof means the harness accepted the paste,
# not that the request reached the agent: a harness that queues typed input
# mid-turn submits it only at a later boundary. Demanding the reply here blocked
# the debtor, boundary after boundary, for a message its pane still listed as
# queued.
printf '%s' REQ_DELIVERY_ONLY \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_delivery_only="$(reply_nonce_for_body \
  "$reply_b_id" reply-a request REQ_DELIVERY_ONLY)"
reply_delivery_only_wire="$(reply_request_envelope reply-a \
  "$reply_delivery_only" REQ_DELIVERY_ONLY)"
equal "delivery without its native prompt witness is audit, not debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "a request the debtor has not yet seen permits idle" "{}" "$reply_stop_output"
# A message that crosses an unread request does not answer it. Correlation
# needs the sender's own prompt proof of the request: without it the outbound
# is fresh business, the request stays audit until it is read, and it is owed
# then. Auto-correlating here settled a request its debtor never saw. The
# crossed request is answered before its recipient reads the crossing message:
# a prompt opens the reader's native turn, and mail sent into an open turn
# parks in the spool instead of landing as a record.
printf '%s' CROSSED_DELIVERY_ONLY \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_crossed_nonce="$(reply_nonce_for_body \
  "$reply_a_id" reply-b request CROSSED_DELIVERY_ONLY || true)"
equal "a message crossing an unread request is a request, not its reply" \
  request \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_crossed_nonce" | cut -d: -f4)"
equal "the crossing message settles nothing its sender has not read" "" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_delivery_only")"
equal "the unread request stays audit after the crossing message" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_prompt_event "$reply_b_pane" "$reply_delivery_only_wire"
equal "the late prompt proof arms the crossed request" \
  $'owed\t'"$reply_delivery_only"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "the crossed request asks for its reply once read" \
  "$reply_stop_output" "reply to reply-a"
printf '%s' ACK_DELIVERY_ONLY \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_delivery_ack_nonce="$(reply_nonce_for_body \
  "$reply_a_id" reply-b reply ACK_DELIVERY_ONLY || true)"
equal "a reply sent after reading the request settles it" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the answered debtor may idle" "{}" "$reply_stop_output"
reply_prompt_event "$reply_a_pane" \
  "$(reply_request_envelope reply-b "$reply_crossed_nonce" CROSSED_DELIVERY_ONLY)"
equal "the crossing message is owed a reply by its recipient" \
  $'owed\t'"$reply_crossed_nonce"$'\treply-b\tlive' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_delivery_ack_nonce" ACK_DELIVERY_ONLY)"
equal "the reply opens no reciprocal debt beside the crossed request" \
  $'owed\t'"$reply_crossed_nonce"$'\treply-b\tlive' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
printf '%s' ACK_CROSSED \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_crossed_ack_nonce="$(reply_nonce_for_body \
  "$reply_b_id" reply-a reply ACK_CROSSED || true)"
equal "answering the crossed request clears its recipient" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_record_wire "$reply_b_id" "$reply_crossed_ack_nonce" ACK_CROSSED)"
equal "both sides of a crossing are clear once each request is answered" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
equal "the crossed request's recipient may idle once it answered" "{}" "$reply_stop_output"
reply_stop_run "$reply_b_pane"
equal "the crossing sender may idle once both answers landed" "{}" "$reply_stop_output"

# A settlement proof beside neither arrival witness is corrupt evidence rather
# than a discharged obligation: no delivery path can produce that record.
reply_unwitnessed_nonce=0f1e2d3c4b5a6978
reply_unwitnessed_digest=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
reply_unwitnessed_token="$(tmux show-options -wqv -t "$reply_a_id" @gl_spool)"
tmux set-option -w -t "$reply_b_id" "@gl_reply_$reply_unwitnessed_nonce" \
  "message:$reply_unwitnessed_token:reply-a:request:$reply_unwitnessed_digest:-"
tmux set-option -w -t "$reply_b_id" "@gl_rsettled_$reply_unwitnessed_nonce" \
  "$reply_unwitnessed_digest"
equal "a settlement proof with no arrival witness stays ambiguous" \
  $'unknown\t'"$reply_unwitnessed_nonce"$'\treply-a\tprovenance-unwitnessed-request' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "an unwitnessed settlement fails closed at Stop" \
  "$reply_stop_output" '"decision": "block"'
excludes "an unwitnessed settlement demands no impossible reply" \
  "$reply_stop_output" "reply to reply-a"
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_unwitnessed_nonce"
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_unwitnessed_nonce"
equal "removing the corrupt record restores a clear window" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"

# A reply record is audit rather than debt however incomplete its arrival
# evidence is: nothing its holder can send settles a record that was never a
# request, so failing closed on one named no action. A read reply is a
# correlation target only for the turn that read it: the boundary closes it
# with the same settlement proof an acknowledgement would have written.
reply_partial_reply=2b3c4d5e6f708192
reply_partial_reply_digest=89abcdef0123456789abcdef0123456789abcdef0123456789abcdef01234567
tmux set-option -w -t "$reply_b_id" "@gl_reply_$reply_partial_reply" \
  "message:00000000:reply-a:reply:$reply_partial_reply_digest:-"
tmux set-option -w -t "$reply_b_id" "@gl_rprompt_$reply_partial_reply" \
  "$reply_partial_reply_digest"
equal "a reply record with one arrival witness is not a debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "a partial reply record cannot wedge its holder" "{}" "$reply_stop_output"
equal "the boundary closes the read reply record it saw" \
  "$reply_partial_reply_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply")"
equal "a closed reply record stays audit" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply"
tmux set-option -uw -t "$reply_b_id" "@gl_rprompt_$reply_partial_reply"
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_partial_reply"

# The close fails closed. A boundary whose settlement proof cannot be written
# leaves the reply open and refuses to end the turn, so the reply stays
# answerable in it; an allowed Stop with the reply open would correlate the
# next turn's fresh work to it. The close is also the last fact of the
# boundary: every other bookkeeping ran before it.
tmux set-option -w -t "$reply_b_id" "@gl_reply_$reply_partial_reply" \
  "message:00000000:reply-a:reply:$reply_partial_reply_digest:-"
tmux set-option -w -t "$reply_b_id" "@gl_rprompt_$reply_partial_reply" \
  "$reply_partial_reply_digest"
GANG_TEST_REPLY_SETTLE_FAIL=1 reply_stop_run "$reply_b_pane"
contains "a boundary that cannot close a read reply refuses idle" \
  "$reply_stop_output" '"decision": "block"'
contains "the refusal names the boundary, not the query" \
  "$reply_stop_output" "native Stop boundary could not be closed"
equal "the reply stays open when its close was not written" "" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply")"
reply_stop_run "$reply_b_pane"
equal "the next boundary closes it" "{}" "$reply_stop_output"
equal "and writes the settlement proof the failed one could not" \
  "$reply_partial_reply_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply")"
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply"
# An interrupted turn never reaches Stop and leaves no turn bracket. The
# prompt that begins the next turn closes the replies the ended turn read
# before it observes its own message; a prompt inside a live turn is steering
# and leaves them answerable.
tmux set-option -uw -t "$reply_b_id" @gl_turn
reply_prompt_event "$reply_b_pane" "operator prompt after an interrupt"
equal "a prompt after an interrupted turn closes the replies that turn read" \
  "$reply_partial_reply_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply")"
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply"
reply_prompt_event "$reply_b_pane" "operator prompt inside the live turn"
equal "a prompt inside a live turn leaves the replies it read answerable" "" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply")"
reply_stop_run "$reply_b_pane"
equal "the live turn's boundary closes them" \
  "$reply_partial_reply_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply")"
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_partial_reply"
tmux set-option -uw -t "$reply_b_id" "@gl_rprompt_$reply_partial_reply"
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_partial_reply"

# A request with incomplete arrival evidence asks the same question about its
# sender as a complete one. Naming a reply to a sender no inventory can find is
# an instruction the debtor cannot carry out.
reply_gone_partial=1a2b3c4d5e6f7081
reply_gone_partial_digest=fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210
tmux set-option -w -t "$reply_b_id" "@gl_reply_$reply_gone_partial" \
  "message:00000000:reply-a:request:$reply_gone_partial_digest:-"
tmux set-option -w -t "$reply_b_id" "@gl_rprompt_$reply_gone_partial" \
  "$reply_gone_partial_digest"
equal "a partial request from a vanished sender retires" \
  $'retired\t'"$reply_gone_partial"$'\treply-a\tsender-gone' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
equal "retiring a partial request writes its own monotonic proof" \
  "$reply_gone_partial_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rretired_$reply_gone_partial")"
reply_stop_run "$reply_b_pane"
equal "an unanswerable partial request cannot wedge its debtor" \
  "{}" "$reply_stop_output"
tmux set-option -uw -t "$reply_b_id" "@gl_rretired_$reply_gone_partial"
tmux set-option -uw -t "$reply_b_id" "@gl_rprompt_$reply_gone_partial"
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_gone_partial"

# Metadata lands before the paste, so a boundary can observe a request record
# with no proof at all while its send is in flight. That record is a message
# not yet in the debtor's context: it is not debt, and it is not an ambiguity
# to escalate; the same record read as owed once the paste and prompt landed.
reply_bare_candidate=3c4d5e6f70819a2b
reply_bare_candidate_digest=456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123
reply_bare_candidate_token="$(tmux show-options -wqv -t "$reply_a_id" @gl_spool)"
tmux set-option -w -t "$reply_b_id" "@gl_reply_$reply_bare_candidate" \
  "message:$reply_bare_candidate_token:reply-a:request:$reply_bare_candidate_digest:-"
equal "a request record with no arrival proof is a message in flight, not debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "a boundary that sees only in-flight metadata permits idle" "{}" "$reply_stop_output"
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_bare_candidate"

printf '%s' REQ_A_ONE \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_a_one="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_a_one_wire="$(reply_request_envelope reply-a "$reply_a_one" REQ_A_ONE)"
reply_prompt_event "$reply_b_pane" "$reply_a_one_wire"
equal "a verified peer request arms an obligation to its observed sender" \
  $'owed\t'"$reply_a_one"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
contains "status names the exact peer reply debt" "$($GANG status reply-b)" \
  "reply owed to reply-a (message $reply_a_one; sender live)"
contains "roster carries a compact peer debt alarm" "$($GANG roster)" "reply-owed"
reply_stop_run "$reply_b_pane"
contains "an outstanding peer request refuses idle" "$reply_stop_output" '"decision": "block"'
# ONE REFUSAL PER TURN. The re-Stop the harness marks as continuing after a
# block is released with the debt recorded, said by status and roster, and told
# to whoever watches the team; the record itself is untouched.
reply_stop_run "$reply_b_pane" "$reply_stop_active_payload"
equal "a re-Stop the harness marks as continuing after a block is released" \
  "{}" "$reply_stop_output"
equal "the release forgives nothing: the same debt is still owed" \
  $'owed\t'"$reply_a_one"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
contains "status stamps the release with what stood at the boundary" \
  "$($GANG status reply-b)" \
  "ago with a reply owed to reply-a (message $reply_a_one) standing; the next delivery raises it again"
contains "roster keeps the debt alarm after the release" "$($GANG roster)" "reply-owed"
contains "with no notify target and no lead, the undelivered alert is said on the window" \
  "$($GANG status reply-b)" "stop release alert NOT delivered: no notify target is declared and no agent is named lead"
"$GANG" notify reply-c >/dev/null
reply_stop_run "$reply_b_pane" "$reply_stop_active_payload"
equal "a later re-Stop with the debt still standing is released the same way" \
  "{}" "$reply_stop_output"
excludes "a stop alert does not wake its notify target on its own" \
  "$(pane_all reply-c)" \
  "stop alert (stop): reply-b went idle with a reply owed to reply-a (message $reply_a_one) standing"
contains "status exposes the deferred stop alert and its forced wake" \
  "$($GANG status reply-c)" "deferred delivery: 1 envelope owes no reply"
equal "the alert is Gangline's own and leaves its target nothing to owe" \
  $'clear\t-\t-\t-' "$(TMUX_PANE="$reply_c_pane" "$GANG" reply-obligations)"
excludes "a delivered alert retires the undelivered note" \
  "$($GANG status reply-b)" "stop release alert NOT delivered"
# A TIMED-OUT QUERY IS RECORDED BEFORE ANYTHING IS READ AGAIN, and told even
# when the second reading finds nothing standing: the fault is the timeout.
reply_auto_pane="$(tmux list-panes -t "$(window_id auto-resume)" -F '#{pane_id}')"
TMUX_PANE="$reply_auto_pane" "$GANG" reply-released query-timeout
contains "a query-timeout release with nothing standing still stamps the window" \
  "$($GANG status auto-resume)" \
  "ago after its reply query timed out; nothing readable stood, and the next delivery raises whatever does"
excludes "a query-timeout alert also waits without waking its notify target" \
  "$(pane_all reply-c)" \
  "stop alert (query-timeout): auto-resume went idle after its reply query timed out; nothing readable stands now"
contains "the quiet-alert queue accumulates both machine envelopes" \
  "$($GANG status reply-c)" "deferred delivery: 1 envelope owes no reply"
printf '%s' ALERT_WAKE \
  | "$GANG" send --to reply-c --from operator --stdin >/dev/null
reply_c_alert_bundle="$(pane_all reply-c)"
# source-guard: producer@8238b7b6eeca: both alert texts name their distinct raising windows and ALERT_WAKE is unique to the immediately preceding operator send
equal "the next operator wake carries both alerts ahead of current input" \
  "stop alert (stop) stop alert (query-timeout) ALERT_WAKE " \
  "$(printf '%s\n' "$reply_c_alert_bundle" \
      | grep -oE 'stop alert \(stop\)|stop alert \(query-timeout\)|ALERT_WAKE' \
      | awk '!seen[$0]++' | tr '\n' ' ')"
reply_prompt_event "$reply_auto_pane" "the next native prompt retires a query-timeout stamp"
excludes "a new native prompt retires the query-timeout stamp" \
  "$($GANG status auto-resume)" "Stop released"

# A legacy notify window can still be live-addressable before it owns durable
# spool identity. Deferral is unavailable there, but the alert must retain its
# established immediate fallback instead of dying inside spool establishment.
reply_stop_run "$reply_c_pane"
reply_c_spool_token="$(tmux show-options -wqv -t "$reply_c_id" @gl_spool)"
reply_c_timeout_before="$(pane_all reply-c | grep -o 'stop alert (query-timeout): auto-resume went idle' | wc -l)"
tmux set-option -uw -t "$reply_c_id" @gl_spool
TMUX_PANE="$reply_auto_pane" "$GANG" reply-released query-timeout
reply_c_timeout_after="$(pane_all reply-c | grep -o 'stop alert (query-timeout): auto-resume went idle' | wc -l)"
# source-guard: producer@f2fe94e7a052: the before/after delta binds this combined-pane count to the one spool-less fallback emitted immediately above
equal "a spool-less notify target retains immediate alert delivery" \
  "$((reply_c_timeout_before + 1))" "$reply_c_timeout_after"
tmux set-option -w -t "$reply_c_id" @gl_spool "$reply_c_spool_token"
reply_prompt_event "$reply_auto_pane" "retire the spool-less fallback release stamp"
TMUX_PANE="$reply_b_pane" "$GANG" reply-released query-timeout
contains "a query-timeout release names what the second reading found standing" \
  "$($GANG status reply-b)" \
  "ago after its reply query timed out, with a reply owed to reply-a (message $reply_a_one) standing; the next delivery raises it again"
"$GANG" notify clear >/dev/null

# None of the later native events owns reply state. An operator prompt is the
# discriminating interleave: old last-prompt logic lost the peer provenance.
reply_prompt_event "$reply_b_pane" "operator prompt between peer work and tools"
printf '%s' '{"hook_event_name":"PostToolUse"}' \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"Notification","notification_type":"agent_completed"}' \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"PreCompact"}' \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
printf '%s' "$reply_stop_payload" \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
equal "operator prompts, tools, background notices, compaction, and a later boundary preserve debt" \
  $'owed\t'"$reply_a_one"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
excludes "a new native prompt retires the release stamp" \
  "$($GANG status reply-b)" "Stop released"
reply_stop_run "$reply_b_pane"
contains "the next turn's first Stop refuses idle for the same debt again" \
  "$reply_stop_output" '"decision": "block"'

# Any genuine correlated reply satisfies the debt; its wording is not policy.
reply_ack_one='Waiting on background work; I will report later.'
printf '%s' "$reply_ack_one" \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
equal "an arbitrary concise acknowledgement clears the sender's obligation" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_ack_one_nonce="$(reply_nonce_from "$reply_a_id" reply-b)"
reply_ack_one_wire="$(reply_response_envelope reply-b "$reply_ack_one_nonce" \
  "$reply_a_one" "$reply_ack_one")"
reply_prompt_event "$reply_a_pane" "$reply_ack_one_wire"
equal "a correlated reply does not create reciprocal acknowledgement debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
equal "the reply recipient may idle without an acknowledgement loop" "{}" "$reply_stop_output"
reply_stop_run "$reply_b_pane"
equal "the acknowledging agent may idle while background work continues" "{}" "$reply_stop_output"

# A thread closes on any acknowledgement. A message sent to a reply's sender in
# the turn that read the reply is correlated to it and opens no debt, so an ack
# of an ack costs its recipient nothing; every ack in a chain was a request
# before, and the chain could not end without one unanswered message. Each
# side ends its reading turn before the peer writes to it again: a prompt
# opens the native turn, and mail sent into an open turn parks in the spool.
printf '%s' THREAD_REQ \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_thread_req="$(reply_nonce_for_body "$reply_a_id" reply-b request THREAD_REQ || true)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_request_envelope reply-b "$reply_thread_req" THREAD_REQ)"
equal "a thread opens with a request owed" \
  $'owed\t'"$reply_thread_req"$'\treply-b\tlive' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
printf '%s' THREAD_REPLY \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_ack_one_nonce="$(reply_nonce_for_body "$reply_b_id" reply-a reply THREAD_REPLY || true)"
reply_stop_run "$reply_a_pane"
equal "the answered request lets its debtor idle" "{}" "$reply_stop_output"
reply_prompt_event "$reply_b_pane" \
  "$(reply_record_wire "$reply_b_id" "$reply_ack_one_nonce" THREAD_REPLY)"
printf '%s' ACK_OF_ACK \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_ack_two_nonce="$(reply_nonce_for_body "$reply_a_id" reply-b reply ACK_OF_ACK || true)"
equal "an acknowledgement of a correlated reply is correlated to that reply" \
  "$reply_ack_one_nonce" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_ack_two_nonce" | cut -d: -f6)"
equal "acknowledging a reply closes it as a correlation target" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_ack_one_nonce" | cut -d: -f5)" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_ack_one_nonce")"
reply_stop_run "$reply_b_pane"
equal "the acknowledging agent may idle after acknowledging" "{}" "$reply_stop_output"
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_ack_two_nonce" ACK_OF_ACK)"
equal "an acknowledged acknowledgement opens no debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
printf '%s' ACK_CHAIN_END \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_ack_three_nonce="$(reply_nonce_for_body "$reply_b_id" reply-a reply ACK_CHAIN_END || true)"
equal "a further acknowledgement is correlated to the ack it answers" \
  "$reply_ack_two_nonce" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_ack_three_nonce" | cut -d: -f6)"
reply_stop_run "$reply_a_pane"
equal "the chain's sender may idle after its acknowledgement" "{}" "$reply_stop_output"
# A boundary does not close a reply its holder has not read; the reply is
# still answerable in the turn that reads it.
reply_stop_run "$reply_b_pane"
equal "a boundary before the reply is read leaves it open" "" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_ack_three_nonce")"
reply_prompt_event "$reply_b_pane" \
  "$(reply_record_wire "$reply_b_id" "$reply_ack_three_nonce" ACK_CHAIN_END)"
printf '%s' ACK_CHAIN_MORE \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_ack_four_nonce="$(reply_nonce_for_body "$reply_a_id" reply-b reply ACK_CHAIN_MORE || true)"
equal "a reply read after that boundary is still answerable in the turn that read it" \
  "$reply_ack_three_nonce" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_ack_four_nonce" | cut -d: -f6)"
reply_stop_run "$reply_b_pane"
# The chain ends on an unanswered acknowledgement: reading it costs nothing,
# and the boundary that ends the reading turn closes it.
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_ack_four_nonce" ACK_CHAIN_MORE)"
equal "the last acknowledgement in a chain opens no debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
equal "the acknowledged agent may idle without answering the ack" "{}" "$reply_stop_output"
equal "the boundary closes the reply that turn read" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_ack_four_nonce" | cut -d: -f5)" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_rsettled_$reply_ack_four_nonce")"
# A message sent in a later turn is fresh business. The turn that read the
# reply ended at its Stop, so the next message to that peer is a request and
# is owed like any other.
printf '%s' REQ_AFTER_THREAD \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_after_thread_nonce="$(reply_nonce_for_body \
  "$reply_b_id" reply-a request REQ_AFTER_THREAD || true)"
equal "a message sent in a later turn is a request, not a reply" request \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_after_thread_nonce" | cut -d: -f4)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_after_thread_nonce" REQ_AFTER_THREAD)"
equal "the later request is owed by its recipient" \
  $'owed\t'"$reply_after_thread_nonce"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
printf '%s' ACK_AFTER_THREAD \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_after_thread_ack="$(reply_nonce_for_body \
  "$reply_a_id" reply-b reply ACK_AFTER_THREAD || true)"
reply_stop_run "$reply_b_pane"
equal "the later request's recipient may idle once it answered" "{}" "$reply_stop_output"
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_after_thread_ack" ACK_AFTER_THREAD)"
reply_stop_run "$reply_a_pane"
equal "the later request's sender may idle after the answer" "{}" "$reply_stop_output"
# A turn ended by an interrupt closes what it read as a Stop would. The
# interrupt leaves no turn bracket; the prompt that begins the next turn closes
# the reply before it observes the operator's message, so fresh work sent to
# the reply's sender is a request and is owed.
printf '%s' REQ_INTERRUPTED \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_interrupted_req="$(reply_nonce_for_body "$reply_b_id" reply-a request REQ_INTERRUPTED || true)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_interrupted_req" REQ_INTERRUPTED)"
printf '%s' REPLY_INTERRUPTED \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_interrupted_reply="$(reply_nonce_for_body "$reply_a_id" reply-b reply REPLY_INTERRUPTED || true)"
reply_stop_run "$reply_b_pane"
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_interrupted_reply" REPLY_INTERRUPTED)"
tmux set-option -uw -t "$reply_a_id" @gl_turn
reply_prompt_event "$reply_a_pane" "operator prompt that begins the next turn"
equal "the next turn's prompt closes the reply the interrupted turn read" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_interrupted_reply" | cut -d: -f5)" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_rsettled_$reply_interrupted_reply")"
printf '%s' REQ_AFTER_INTERRUPT \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_after_interrupt="$(reply_nonce_for_body "$reply_b_id" reply-a request REQ_AFTER_INTERRUPT || true)"
equal "fresh work after an interrupted turn is a request, not a reply" request \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_after_interrupt" | cut -d: -f4)"
reply_stop_run "$reply_a_pane"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_after_interrupt" REQ_AFTER_INTERRUPT)"
equal "and is owed by its recipient" \
  $'owed\t'"$reply_after_interrupt"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
printf '%s' ACK_AFTER_INTERRUPT \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_after_interrupt_ack="$(reply_nonce_for_body "$reply_a_id" reply-b reply ACK_AFTER_INTERRUPT || true)"
reply_stop_run "$reply_b_pane"
equal "the interrupted agent's peer may idle once it answered" "{}" "$reply_stop_output"
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_after_interrupt_ack" ACK_AFTER_INTERRUPT)"
reply_stop_run "$reply_a_pane"
equal "the interrupted agent may idle after reading the answer" "{}" "$reply_stop_output"

# A correlated reply whose delivery proof never lands still settles once the
# creditor's native prompt witnesses it. The sending process can die between
# typing and verification: a tick killed at its deadline, or a live send that
# lost the screen. The barrier holds the delivery proof after the metadata is
# written, exactly the state such a death leaves; the debtor was then asked
# for a reply it had already given at every Stop until it sent another.
printf '%s' REQ_LATE_WITNESS \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_late_req="$(reply_nonce_for_body "$reply_b_id" reply-a request REQ_LATE_WITNESS || true)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_late_req" REQ_LATE_WITNESS)"
equal "the request to be answered late is owed" \
  $'owed\t'"$reply_late_req"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_late_gate="reply-late-witness-$$"
reply_late_state="$RUN_ROOT/reply-late-witness-state"
printf '%s' REPLY_LATE_WITNESS \
  | GANG_TEST_REPLY_PROOF_GATE="$reply_late_gate" \
    GANG_TEST_REPLY_GATE_STATE="$reply_late_state" \
    TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --live-only --stdin \
      >"$RUN_ROOT/reply-late-witness.out" 2>"$RUN_ROOT/reply-late-witness.err" &
reply_late_pid=$!
tmux wait-for "$reply_late_gate-ready"
reply_late_reply="$(reply_nonce_for_body "$reply_a_id" reply-b reply REPLY_LATE_WITNESS || true)"
equal "the held reply carries no delivery proof" "" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_rdelivery_$reply_late_reply")"
equal "without any arrival witness the debt still stands" \
  $'owed\t'"$reply_late_req"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_record_wire "$reply_a_id" "$reply_late_reply" REPLY_LATE_WITNESS)"
equal "the creditor's prompt witness settles the debt the delivery proof missed" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the debtor may idle on the creditor's witness alone" "{}" "$reply_stop_output"
reply_late_settled="$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_late_req")"
tmux wait-for -S "$reply_late_gate-release"
wait "$reply_late_pid"
equal "the released delivery proof completes the reply record" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_late_reply" | cut -d: -f5)" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_rdelivery_$reply_late_reply")"
equal "the late delivery proof rewrites the same settlement" "$reply_late_settled" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_late_req")"
equal "the late-witnessed debt stays settled after the retried proof" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
equal "the creditor owes nothing for the late-witnessed reply" "{}" "$reply_stop_output"

# A request parked during a live turn retains its stable sender and nonce until
# the next native boundary drains it. The event barrier is the worker's own
# completion signal, not a poll or timing assertion.
tmux set-option -w -t "$reply_b_id" @gl_turn "open $(date +%s)"
reply_spooled_out="$(printf '%s' REQ_SPOOLED \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin)"
contains "a peer request can wait durably behind a live turn" "$reply_spooled_out" \
  "queued for reply-b"
reply_b_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$reply_b_id" @gl_spool)"
reply_spool_entry="$(find "$reply_b_spool" -maxdepth 1 -type f -name '[0-9]*' -print)"
IFS=$'\t' read -r _ _ _ reply_spool_mode _ reply_spool_nonce \
  < "$reply_spool_entry"
equal "the durable entry retains request provenance" request "$reply_spool_mode"
reply_spool_wire="$(tail -n +3 "$reply_spool_entry")"
reply_drain_channel="gang-spool-drain-$reply_b_id"
tmux wait-for "$reply_drain_channel" &
reply_drain_waiter=$!
printf '%s' "$reply_stop_payload" \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
wait "$reply_drain_waiter"
# The production completion channel is intentionally reused. A signal from an
# earlier drain can remain latched when no fixture waiter owned it, so reject
# that stale completion once by waiting for the worker that must create this
# exact immutable record. The suite's tmux wait ceiling keeps a real missing-
# record defect loud instead of polling it into an eventual pass.
if [ -z "$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_spool_nonce")" ]; then
  tmux wait-for "$reply_drain_channel" &
  reply_drain_waiter=$!
  wait "$reply_drain_waiter"
fi
equal "a drained spool entry is not debt before its native prompt witness" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
equal "the drained request keeps its nonce for the prompt witness to complete" \
  request \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_spool_nonce" | cut -d: -f4)"
reply_prompt_event "$reply_b_pane" "$reply_spool_wire"
equal "the native witness completes the durable request obligation" \
  $'owed\t'"$reply_spool_nonce"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
printf '%s' ACK_SPOOLED \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_spool_ack_nonce="$(reply_nonce_from "$reply_a_id" reply-b)"
reply_spool_ack_wire="$(reply_response_envelope reply-b "$reply_spool_ack_nonce" \
  "$reply_spool_nonce" ACK_SPOOLED)"
reply_prompt_event "$reply_a_pane" "$reply_spool_ack_wire"
reply_stop_run "$reply_a_pane"
reply_stop_run "$reply_b_pane"
equal "a reply to durable mail restores an idle-safe clear state" "{}" "$reply_stop_output"

# A correlated answer parked behind the creditor's live turn is already the
# debtor's whole answer. Answers remain immediate-delivery mail rather than
# quiet acknowledgements, but durable acceptance still settles the debt; the
# ordinary drain then completes the audit record with its delivery proof.
printf '%s' REQ_PARKED_REPLY \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_parked_req="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_parked_req" REQ_PARKED_REPLY)"
equal "the creditor's request is owed before its reply is parked" \
  $'owed\t'"$reply_parked_req"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
tmux set-option -w -t "$reply_a_id" @gl_turn "open $(date +%s)"
reply_parked_out="$(printf '%s' ACK_PARKED_REPLY \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin)"
contains "a correlated reply can wait behind the creditor's live turn" \
  "$reply_parked_out" "queued for reply-a"
reply_parked_meta="$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_parked_req")"
IFS=: read -r _ _ _ _ reply_parked_digest _ <<<"$reply_parked_meta"
equal "acceptance into the spool writes the settlement proof" \
  "$reply_parked_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rsettled_$reply_parked_req")"
equal "a parked correlated reply settles the debt before delivery" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the debtor may idle while its reply waits for the creditor's boundary" \
  "{}" "$reply_stop_output"
# The debtor replaces its waiting reply. The replacement stands in for what it
# retires, so it carries the retired reply's correlation: the record on the
# debtor is already settled and would not correlate it again.
reply_a_spool="$GANG_LOCK_DIR/spool/$(tmux show-options -wqv -t "$reply_a_id" @gl_spool)"
reply_super_expected=""
for reply_super_entry in "$reply_a_spool"/[0-9]* "$reply_a_spool"/.deferred-[0-9]*; do
  [ -f "$reply_super_entry" ] || continue
  IFS=$'\t' read -r _ reply_super_sender _ reply_super_mode reply_super_reply_to _ \
    < "$reply_super_entry"
  [ "$reply_super_sender" = reply-b ] && [ "$reply_super_mode" = reply ] || continue
  reply_super_rest="$reply_super_reply_to"
  while :; do
    reply_super_nonce="${reply_super_rest%%,*}"
    case ",$reply_super_expected," in
      *",$reply_super_nonce,"*) ;;
      *) reply_super_expected="${reply_super_expected:+$reply_super_expected,}$reply_super_nonce" ;;
    esac
    [ "$reply_super_rest" != "$reply_super_nonce" ] || break
    reply_super_rest="${reply_super_rest#*,}"
  done
done
equal "the parked answer is among the replies the replacement will supersede" \
  "$reply_parked_req" \
  "$(printf '%s\n' "$reply_super_expected" | tr ',' '\n' | grep -Fx "$reply_parked_req")"
reply_super_out="$(printf '%s' ACK_PARKED_SUPERSEDE \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --supersede --stdin)"
contains "a replacement for a parked reply is accepted" \
  "$reply_super_out" "queued for reply-a"
reply_super_meta=""
for reply_super_entry in "$reply_a_spool"/[0-9]*; do
  reply_super_meta="$reply_super_meta$(head -n 1 "$reply_super_entry" | cut -f2,4,5)"$'\n'
done
equal "the replacement inherits every retired reply correlation" \
  $'reply-b\treply\t'"$reply_super_expected"$'\n' "$reply_super_meta"
equal "superseding a settled reply leaves the debt settled" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_a_drain_channel="gang-spool-drain-$reply_a_id"
tmux wait-for "$reply_a_drain_channel" &
reply_parked_waiter=$!
reply_stop_run "$reply_a_pane"
wait "$reply_parked_waiter"
equal "the creditor's boundary drains the parked reply" "{}" "$reply_stop_output"
reply_parked_ack_nonce="$(reply_nonce_for_body \
  "$reply_a_id" reply-b reply ACK_PARKED_SUPERSEDE)"
equal "the drained reply completes the audit record with its delivery proof" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_parked_ack_nonce" | cut -d: -f5)" \
  "$(tmux show-options -wqv -t "$reply_a_id" "@gl_rdelivery_$reply_parked_ack_nonce")"
reply_prompt_event "$reply_a_pane" \
  "$(reply_response_envelope reply-b "$reply_parked_ack_nonce" \
    "$reply_parked_req" ACK_PARKED_SUPERSEDE)"
equal "the drained reply opens no reciprocal debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
equal "the creditor may idle after reading the drained reply" "{}" "$reply_stop_output"
equal "the delivered reply leaves the settled debt settled" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"

# Upgrade compatibility cannot invent the stable identity and nonce absent
# from the old three-line spool shape. Deliver the body, but retain a loud
# legacy ambiguity instead of silently treating peer mail as control traffic.
tmux set-option -w -t "$reply_b_id" @gl_turn "open $(date +%s)"
reply_legacy_entry="$reply_b_spool/00000000000000000001-legacy"
printf '%s\n%s\n%s\n' reply-a legacy-peer \
  '[gang:reply-a] LEGACY_PEER_REQUEST [/gang:reply-a]' > "$reply_legacy_entry"
tmux wait-for "$reply_drain_channel" &
reply_legacy_waiter=$!
printf '%s' "$reply_stop_payload" \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
wait "$reply_legacy_waiter"
reply_legacy_query="$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
contains "legacy peer mail fails closed instead of losing its obligation" \
  "$reply_legacy_query" $'unknown\t'
contains "legacy ambiguity retains the witnessed peer name" \
  "$reply_legacy_query" $'\treply-a\tprovenance-legacy-legacy'
reply_stop_run "$reply_b_pane"
contains "legacy peer provenance refuses idle" "$reply_stop_output" '"decision": "block"'
reply_legacy_nonce="$(cut -f2 <<<"$reply_legacy_query")"
# Fixture retirement only: production intentionally has no guessed correlation
# capable of clearing an upgraded legacy record.
tmux set-option -uw -t "$reply_b_id" "@gl_rprompt_$reply_legacy_nonce" 2>/dev/null || true
tmux set-option -uw -t "$reply_b_id" "@gl_rdelivery_$reply_legacy_nonce" 2>/dev/null || true
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_legacy_nonce" 2>/dev/null || true
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_legacy_nonce"

# `auto-resume` is also a valid live agent name. A pre-upgrade entry cannot
# distinguish that peer from the historical internal author string, so the
# only safe interpretation is the same legacy ambiguity.
reply_legacy_auto_entry="$reply_b_spool/00000000000000000002-legacy-auto"
printf '%s\n%s\n%s\n' auto-resume legacy-auto-peer \
  '[gang:auto-resume] LEGACY_AUTO_RESUME_REQUEST [/gang:auto-resume]' \
  > "$reply_legacy_auto_entry"
tmux wait-for "$reply_drain_channel" &
reply_legacy_auto_waiter=$!
printf '%s' "$reply_stop_payload" \
  | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
wait "$reply_legacy_auto_waiter"
reply_legacy_auto_query="$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
contains "legacy control-name collision stays fail-closed for a valid peer" \
  "$reply_legacy_auto_query" $'\tauto-resume\tprovenance-legacy-legacy'
reply_legacy_auto_nonce="$(cut -f2 <<<"$reply_legacy_auto_query")"
tmux set-option -uw -t "$reply_b_id" "@gl_rprompt_$reply_legacy_auto_nonce" 2>/dev/null || true
tmux set-option -uw -t "$reply_b_id" "@gl_rdelivery_$reply_legacy_auto_nonce" 2>/dev/null || true
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_legacy_auto_nonce" 2>/dev/null || true
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_legacy_auto_nonce"

# One peer may have more than one message outstanding. The acknowledgement's
# correlated reply list settles all of that peer's proved requests at once;
# no last-writer slot may hide either record.
printf '%s' REQ_SAME_ONE \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_same_one="$(reply_nonce_for_body \
  "$reply_b_id" reply-a request REQ_SAME_ONE)"
reply_same_one_wire="$(reply_request_envelope \
  reply-a "$reply_same_one" REQ_SAME_ONE)"
printf '%s' REQ_SAME_TWO \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_same_two="$(reply_nonce_for_body \
  "$reply_b_id" reply-a request REQ_SAME_TWO)"
reply_same_two_wire="$(reply_request_envelope \
  reply-a "$reply_same_two" REQ_SAME_TWO)"
reply_prompt_event "$reply_b_pane" \
  "$reply_same_one_wire"$'\n'"$reply_same_two_wire"
reply_same_query="$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
contains "two messages from one peer retain the first obligation" "$reply_same_query" \
  $'owed\t'"$reply_same_one"$'\treply-a\tlive'
contains "two messages from one peer retain the second obligation" "$reply_same_query" \
  $'owed\t'"$reply_same_two"$'\treply-a\tlive'
printf '%s' ACK_BOTH_FROM_A \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_same_ack="$(reply_nonce_for_body \
  "$reply_a_id" reply-b reply ACK_BOTH_FROM_A)"
reply_same_ack_meta="$(tmux show-options -wqv -t "$reply_a_id" "@gl_reply_$reply_same_ack")"
IFS=: read -r _ _ _ _ _ reply_same_correlations <<<"$reply_same_ack_meta"
equal "one correlated acknowledgement settles both messages from its peer" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_response_envelope reply-b "$reply_same_ack" \
    "$reply_same_correlations" ACK_BOTH_FROM_A)"
equal "the multi-correlation reply remains acknowledgement-loop free" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_a_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_a_pane"
equal "the multi-correlation recipient may idle without reciprocal debt" \
  "{}" "$reply_stop_output"
reply_stop_run "$reply_b_pane"
equal "the multiple-message sender may idle after its acknowledgement" \
  "{}" "$reply_stop_output"

# Two peers arm two independent records. A response to one cannot overwrite or
# settle the other, and each correlated response remains reply-only at target.
printf '%s' REQ_MULTI_A \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_multi_a="$(reply_nonce_for_body \
  "$reply_b_id" reply-a request REQ_MULTI_A)"
reply_multi_a_wire="$(reply_request_envelope \
  reply-a "$reply_multi_a" REQ_MULTI_A)"
printf '%s' REQ_MULTI_C \
  | TMUX_PANE="$reply_c_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_multi_c="$(reply_nonce_for_body \
  "$reply_b_id" reply-c request REQ_MULTI_C)"
reply_multi_c_wire="$(reply_request_envelope \
  reply-c "$reply_multi_c" REQ_MULTI_C)"
reply_prompt_event "$reply_b_pane" \
  "$reply_multi_a_wire"$'\n'"$reply_multi_c_wire"
reply_multi_query="$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
contains "multiple peer senders retain A's obligation" "$reply_multi_query" \
  $'owed\t'"$reply_multi_a"$'\treply-a\tlive'
contains "multiple peer senders retain C's obligation" "$reply_multi_query" \
  $'owed\t'"$reply_multi_c"$'\treply-c\tlive'
printf '%s' ACK_ONLY_A \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
equal "replying to one peer leaves the other peer's obligation intact" \
  $'owed\t'"$reply_multi_c"$'\treply-c\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_multi_a_ack="$(reply_nonce_from "$reply_a_id" reply-b)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_response_envelope reply-b "$reply_multi_a_ack" "$reply_multi_a" ACK_ONLY_A)"
reply_stop_run "$reply_a_pane"
printf '%s' ACK_ONLY_C \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-c --stdin >/dev/null
reply_multi_c_ack="$(reply_nonce_from "$reply_c_id" reply-b)"
reply_prompt_event "$reply_c_pane" \
  "$(reply_response_envelope reply-b "$reply_multi_c_ack" "$reply_multi_c" ACK_ONLY_C)"
reply_stop_run "$reply_c_pane"
equal "one acknowledgement per peer clears multiple-sender debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"

# Completed metadata is retained as an audit trail. Build a substantial history
# through one tmux source transaction and prove its semantics stay clear; the
# production reader joins this set in one pass instead of rescanning it once per
# proof. This is an accumulation check, not a timing assertion.
reply_history_config="$RUN_ROOT/reply-history.conf"
reply_history_digest=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
reply_history_token="$(tmux show-options -wqv -t "$reply_a_id" @gl_spool)"
reply_history_index=1
while [ "$reply_history_index" -le 256 ]; do
  reply_history_nonce="$(printf 'feedfeed%08x' "$reply_history_index")"
  printf 'set-option -w -t %s @gl_reply_%s message:%s:reply-a:request:%s:-\n' \
    "$reply_b_id" "$reply_history_nonce" "$reply_history_token" "$reply_history_digest"
  printf 'set-option -w -t %s @gl_rprompt_%s %s\n' \
    "$reply_b_id" "$reply_history_nonce" "$reply_history_digest"
  printf 'set-option -w -t %s @gl_rdelivery_%s %s\n' \
    "$reply_b_id" "$reply_history_nonce" "$reply_history_digest"
  printf 'set-option -w -t %s @gl_rsettled_%s %s\n' \
    "$reply_b_id" "$reply_history_nonce" "$reply_history_digest"
  reply_history_index=$((reply_history_index + 1))
done > "$reply_history_config"
tmux source-file "$reply_history_config"
equal "retained settled history remains semantically clear" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"

# Corrupt evidence is never repaired into absence. A vanished sender remains
# named in the audit trail, but no longer leaves an impossible obligation.
tmux set-option -w -t "$reply_b_id" @gl_reply_bad malformed
contains "malformed provenance is a loud unknown" \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)" \
  $'unknown\t-\t-\treply-obligation option @gl_reply_bad is malformed'
contains "status reports malformed provenance as Stop-blocking" "$($GANG status reply-b)" \
  "reply obligation UNKNOWN"
reply_stop_run "$reply_b_pane"
contains "malformed provenance fails closed at Stop" "$reply_stop_output" '"decision": "block"'
tmux set-option -uw -t "$reply_b_id" @gl_reply_bad

reply_orphan_nonce=deadbeefdeadbeef
reply_orphan_digest=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
tmux set-option -w -t "$reply_b_id" "@gl_rprompt_$reply_orphan_nonce" "$reply_orphan_digest"
contains "orphaned proof is not mistaken for absent provenance" \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)" \
  "reply-obligation proof option @gl_rprompt_$reply_orphan_nonce is orphaned"
reply_stop_run "$reply_b_pane"
contains "orphaned proof fails closed at Stop" "$reply_stop_output" '"decision": "block"'
tmux set-option -uw -t "$reply_b_id" "@gl_rprompt_$reply_orphan_nonce"

printf '%s' REQ_GONE \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_gone_nonce="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_gone_nonce" REQ_GONE)"
tmux set-option -w -t "$reply_a_id" @gl_agent invalid/name
equal "an unreadable sender identity remains Stop-blocking ambiguity" \
  $'unknown\t'"$reply_gone_nonce"$'\treply-a\tsender-identity-unreadable' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "unreadable sender identity fails closed at Stop" \
  "$reply_stop_output" '"decision": "block"'
tmux set-option -w -t "$reply_a_id" @gl_agent reply-a
equal "repairing sender identity restores the live obligation" \
  $'owed\t'"$reply_gone_nonce"$'\treply-a\tlive' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_gone_meta="$(tmux show-options -wqv -t "$reply_b_id" "@gl_reply_$reply_gone_nonce")"
IFS=: read -r _ reply_gone_token _ _ reply_gone_digest _ <<<"$reply_gone_meta"
"$GANG" drop reply-a >/dev/null
equal "a vanished sender retires its original message debt with an audit note" \
  $'retired\t'"$reply_gone_nonce"$'\treply-a\tsender-gone' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
equal "sender retirement writes one monotonic proof" "$reply_gone_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rretired_$reply_gone_nonce")"
tmux set-option -w -t "$reply_b_id" "@gl_rsettled_$reply_gone_nonce" "$reply_gone_digest"
contains "conflicting reply and retirement proofs fail closed" \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)" \
  "conflicting settlement and retirement proofs"
reply_stop_run "$reply_b_pane"
contains "conflicting lifecycle proofs block Stop" \
  "$reply_stop_output" '"decision": "block"'
tmux set-option -uw -t "$reply_b_id" "@gl_rsettled_$reply_gone_nonce"
reply_c_token="$(tmux show-options -wqv -t "$reply_c_id" @gl_spool)"
tmux set-option -w -t "$reply_c_id" @gl_spool "$reply_gone_token"
tmux set-option -w -t "$reply_c_id" @gl_agent invalid/name
equal "the retirement proof avoids resolving the gone sender again" \
  $'retired\t'"$reply_gone_nonce"$'\treply-a\tsender-gone' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
tmux set-option -w -t "$reply_c_id" @gl_agent reply-c
tmux set-option -w -t "$reply_c_id" @gl_spool "$reply_c_token"
contains "status retains the vanished sender retirement history" \
  "$("$GANG" status reply-b)" \
  "reply obligation retired for reply-a (message $reply_gone_nonce; sender gone)"
reply_b_roster_row="$("$GANG" roster | awk '$1 == "reply-b" { print }')"
contains "roster retains the debtor after sender retirement" \
  "$reply_b_roster_row" "reply-b"
excludes "roster does not carry a blocking alarm for retired history" \
  "$reply_b_roster_row" "reply-owed"
reply_stop_run "$reply_b_pane"
equal "a vanished sender cannot wedge its debtor at Stop" "{}" "$reply_stop_output"

# THE NATIVE BOUNDARY CLOSES ITS OWN PIPE. A hook launches a detached
# cooperative tick on its way out, and the adapter reads the hook through a
# pipe it waits on until EOF. A detached descendant that inherits the write end
# holds a completed boundary hostage until the adapter times out (#195). Hold
# the tick at its first pass and list the pipe's holders once the hook itself
# has exited: only the reader may remain. The tick is then walked to its commit
# so its one pass has finished before anything later reads the team.
reply_tick_ready="$RUN_ROOT/reply-tick-ready"
reply_tick_release="$RUN_ROOT/reply-tick-release"
reply_tick_commit_ready="$RUN_ROOT/reply-tick-commit-ready"
reply_tick_commit_release="$RUN_ROOT/reply-tick-commit-release"
reply_tick_pipe="$RUN_ROOT/reply-tick-pipe"
reply_tick_hook_rc=0
mkfifo "$reply_tick_ready" "$reply_tick_release" "$reply_tick_pipe" \
  "$reply_tick_commit_ready" "$reply_tick_commit_release"
cat "$reply_tick_pipe" > "$RUN_ROOT/reply-tick-query.out" &
reply_tick_reader=$!
GANG_TEST_TICK_MODE='' \
GANG_TEST_TICK_READY_FIFO="$reply_tick_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$reply_tick_release" \
GANG_TEST_TICK_COMMIT_READY_FIFO="$reply_tick_commit_ready" \
GANG_TEST_TICK_COMMIT_RELEASE_FIFO="$reply_tick_commit_release" \
TMUX_PANE="$reply_b_pane" "$GANG" hook > "$reply_tick_pipe" \
  <<<"$reply_stop_payload" || reply_tick_hook_rc=$?
IFS= read -r -N 1 _ < "$reply_tick_ready"
# find reports the fd tables it may not read on the way past; the holders it
# could read are the whole answer, and the reader itself is one of them.
reply_tick_holders="$({ find /proc/[0-9]*/fd -maxdepth 1 -lname "$reply_tick_pipe" 2>/dev/null || true; } \
  | awk -F/ -v reader="$reply_tick_reader" '$3 != reader')"
equal "a detached tick inherits no descriptor of the query it followed" \
  "" "$reply_tick_holders"
printf '\n' > "$reply_tick_release"
IFS= read -r -N 1 _ < "$reply_tick_commit_ready"
printf '\n' > "$reply_tick_commit_release"
wait "$reply_tick_reader"
equal "the hook whose pipe closed returned successfully" 0 "$reply_tick_hook_rc"
equal "the successful hook recorded its Stop boundary" closed \
  "$(case "$(tmux show-options -wqv -t "$reply_b_id" @gl_turn)" in
       'closed v2:'*) printf closed ;;
       *) printf other ;;
     esac)"

# A clear native Stop has one cooperative retry edge. reply-obligations is the
# adapter's private, read-only first stage; if it launches its own tick, that
# Stop produces two whole-team passes. Synchronous test mode is safe here
# because the stages run directly, with no adapter timeout around the tick:
# the query produces no pass and the terminal gang hook produces one.
reply_stop_tick_ledger="$RUN_ROOT/reply-stop-tick-ledger"
: > "$reply_stop_tick_ledger"
reply_tick_query="$(GANG_TEST_TICK_MODE=sync \
  GANG_TEST_TICK_LEDGER="$reply_stop_tick_ledger" \
  TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
equal "the private Stop query still returns its retired audit" \
  $'retired\t'"$reply_gone_nonce"$'\treply-a\tsender-gone' "$reply_tick_query"
equal "the private Stop query launches no cooperative pass" 0 \
  "$(wc -l < "$reply_stop_tick_ledger" | tr -d ' ')"
: > "$reply_stop_tick_ledger"
reply_tick_hook_rc=0
GANG_TEST_TICK_MODE=sync GANG_TEST_TICK_LEDGER="$reply_stop_tick_ledger" \
  TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null \
    <<<"$reply_stop_payload" || reply_tick_hook_rc=$?
equal "the terminal Stop hook succeeds outside an adapter timeout" 0 \
  "$reply_tick_hook_rc"
equal "the terminal Stop hook launches the clear boundary's one pass" 1 \
  "$(wc -l < "$reply_stop_tick_ledger" | tr -d ' ')"

# Adapter failures are ambiguity, never permission. Its one clear arm delegates
# generic Stop bookkeeping; malformed or failed query paths do not.
reply_fake_root="$RUN_ROOT/reply-fake"
mkdir -p "$reply_fake_root"
cat > "$reply_fake_root/gang" <<'SH'
#!/bin/sh
case "$1" in
  reply-obligations) printf 'query\n' >> "${FAKE_QUERY_LOG:-/dev/null}"
                     [ -z "${FAKE_REPLY_HOLD:-}" ] || exec cat "$FAKE_REPLY_HOLD"
                     printf '%b' "$FAKE_REPLY_QUERY"; exit "${FAKE_REPLY_RC:-0}" ;;
  reply-released) printf 'released%s\n' "${2:+ $2}" >> "$FAKE_REPLY_LOG"
                  [ -z "${FAKE_RELEASE_HOLD:-}" ] || exec cat "$FAKE_RELEASE_HOLD"
                  exit "${FAKE_RELEASE_RC:-0}" ;;
  hook) printf 'hook\n' >> "$FAKE_REPLY_LOG"; cat >/dev/null
        [ -z "${FAKE_HOOK_DELAY:-}" ] || /bin/sleep "$FAKE_HOOK_DELAY"
        [ -z "${FAKE_HOOK_HOLD:-}" ] || exec cat "$FAKE_HOOK_HOLD"
        exit "${FAKE_HOOK_RC:-0}" ;;
  *) exit 99 ;;
esac
SH
chmod +x "$reply_fake_root/gang"
reply_fake_log="$reply_fake_root/log"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='broken\n' FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@922c3940adda: the complete fake-adapter stdout is the malformed query verdict, so any producer is valid evidence
contains "a malformed adapter query fails closed" "$reply_fake_out" '"decision": "block"'
# source-guard: whole-surface@1e0590192391: the complete fake hook log must stay empty on a malformed query, so any producer is valid evidence
equal "a malformed query cannot close the native boundary" "" "$(cat "$reply_fake_log")"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@2a5a693ab8a4: the complete fake-adapter stdout is the proved-clear verdict, so any producer is valid evidence
equal "the adapter allows only a proved clear query" "{}" "$reply_fake_out"
# source-guard: whole-surface@606d1f11d4cc: the complete fake hook log records the only delegated boundary, so any producer is valid evidence
equal "a proved clear query delegates ordinary Stop bookkeeping" "hook" \
  "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='retired\t1111111111111111\tretired-peer\tsender-gone\nowed\t2222222222222222\tlive-peer\tlive\n' \
    FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@7d5922ca40f6: the complete fake-adapter stdout is the mixed-verdict decision, so any producer is valid evidence
contains "a retired sender does not hide another peer's live obligation" \
  "$reply_fake_out" '"decision": "block"'
# source-guard: whole-surface@0024ed8cba27: the complete fake-adapter stdout is the mixed-verdict reason, so any producer is valid evidence
contains "the mixed verdict asks only for the live peer" \
  "$reply_fake_out" "reply to live-peer"
excludes "the mixed verdict never asks for the retired peer" \
  "$reply_fake_out" "retired-peer"
# source-guard: whole-surface@f037d514ecc3: the complete fake hook log must stay empty for a mixed blocking verdict, so any producer is valid evidence
equal "a mixed retired and owed query cannot close the native boundary" "" \
  "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='unknown\t3333333333333333\tstuck-peer\tsender-identity-unreadable\n' \
    FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@f788fc80dc85: the complete fake-adapter stdout is the unreachable-ambiguity decision, so any producer is valid evidence
contains "an unreachable ambiguity fails closed" \
  "$reply_fake_out" '"decision": "block"'
# source-guard: whole-surface@517a2ffc9bae: the complete fake-adapter stdout is the unreachable-ambiguity reason, so any producer is valid evidence
contains "an unreachable ambiguity names the evidence it cannot read" \
  "$reply_fake_out" "sender-identity-unreadable"
excludes "an unreachable ambiguity demands no reply" "$reply_fake_out" "reply to"
reply_fake_out="$(printf '%s' 'not-a-native-stop-payload' \
  | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@59580f6f1db8: the complete fake-adapter stdout is the unreadable-payload verdict, so any producer is valid evidence
contains "an unreadable Stop payload fails closed under its own name" \
  "$reply_fake_out" "native Stop payload could not be read"
excludes "an unreadable payload is not reported as an unreadable query" \
  "$reply_fake_out" "repair the query path"
# source-guard: whole-surface@e6bef153582c: the complete fake hook log must stay empty when no payload parsed, so any producer is valid evidence
equal "an unreadable payload closes no native boundary" "" "$(cat "$reply_fake_log")"
reply_fake_out="$(printf '\377\376' \
  | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@dd969a4ce663: the complete fake-adapter stdout is the undecodable-payload verdict, so any producer is valid evidence
contains "a Stop payload that is not text still answers with a verdict" \
  "$reply_fake_out" '"decision": "block"'
# source-guard: whole-surface@541f537de224: the complete fake-adapter stdout is the undecodable-payload reason, so any producer is valid evidence
contains "undecodable payload bytes fail closed under the payload name" \
  "$reply_fake_out" "native Stop payload could not be read"
# source-guard: whole-surface@f4b4fd06657d: the complete fake hook log must stay empty when no payload decoded, so any producer is valid evidence
equal "a payload that never decoded closes no native boundary" "" \
  "$(cat "$reply_fake_log")"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
    FAKE_HOOK_RC=9 python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@5185de53a929: the complete fake-adapter stdout is the failed-boundary verdict, so any visible producer is valid evidence
contains "failed Stop bookkeeping fails closed" "$reply_fake_out" '"decision": "block"'
excludes "failed Stop bookkeeping never emits an allow verdict" "$reply_fake_out" '{}'
# source-guard: whole-surface@e2d8f5f84857: the complete fake-adapter stdout is the failed-boundary reason, so any producer is valid evidence
contains "a failed boundary is reported as bookkeeping, not as an unread query" \
  "$reply_fake_out" "ordinary Stop bookkeeping failed"
excludes "a failed boundary does not send the agent to the query path" \
  "$reply_fake_out" "repair the query path"
# source-guard: whole-surface@aa46cab9286d: the complete fake hook log records the only delegated boundary attempted by this invocation, so any visible producer is valid evidence
equal "the failed boundary was attempted exactly once" "hook" "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_QUERY='owed\t2222222222222222\tlive-peer\tlive\n' FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@5557affec9d8: the complete fake-adapter stdout is the released verdict, so any producer is valid evidence
equal "a re-Stop with debt standing is released, not held" "{}" "$reply_fake_out"
# source-guard: whole-surface@8d591dac2c95: the complete fake hook log orders the release before the boundary, so any producer is valid evidence
equal "the release is reported to Gangline before the boundary closes" \
  $'released\nhook' "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_QUERY='owed\t2222222222222222\tlive-peer\tlive\n' FAKE_REPLY_LOG="$reply_fake_log" \
    FAKE_RELEASE_RC=7 python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@fbbe604c0c37: the complete fake-adapter stdout is the released verdict, so any producer is valid evidence
equal "a release Gangline could not record still releases" "{}" "$reply_fake_out"
# source-guard: whole-surface@85ec03f4a6b7: the complete fake hook log orders the failed release before the boundary, so any producer is valid evidence
equal "the boundary is closed even when the release report failed" \
  $'released\nhook' "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_QUERY='owed\t2222222222222222\tlive-peer\tlive\n' FAKE_REPLY_LOG="$reply_fake_log" \
    FAKE_HOOK_RC=9 python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@9b4301c3b50b: the complete fake-adapter stdout is the failed-boundary reason after a release, so any producer is valid evidence
contains "a failed boundary after a release names the debt still standing" \
  "$reply_fake_out" "released with a reply still owed"
excludes "a failed boundary after a release does not call the provenance clear" \
  "$reply_fake_out" "provenance is clear"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_QUERY='unknown\t3333333333333333\tstuck-peer\tsender-identity-unreadable\n' FAKE_REPLY_LOG="$reply_fake_log" \
    FAKE_HOOK_RC=9 python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@fdf3ab0a7763: the complete fake-adapter stdout is the failed-boundary reason after releasing unresolved provenance, so any producer is valid evidence
contains "a failed boundary after releasing unresolved provenance says so" \
  "$reply_fake_out" "released with peer-reply provenance unresolved"
excludes "unresolved provenance is not reported as a reply owed" \
  "$reply_fake_out" "reply still owed"
# THE QUERY TIMEOUT IS THE BEHAVIOUR UNDER TEST, so its clock is scaled, not
# stopped. The fake never answers: it opens a FIFO nobody writes and stays
# there until the adapter kills it. Measured margin: the fake answers a quiet
# box in well under 50 ms; the fixture budget is 0.25 s per attempt inside a
# 0.45 s deadline; production is 5 s per attempt inside 9 s, under the
# collars' 15 s native fuse.
reply_fake_query_log="$reply_fake_root/queries"
reply_fake_hold="$reply_fake_root/hold"
mkfifo "$reply_fake_hold"
: > "$reply_fake_log"; : > "$reply_fake_query_log"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_HOLD="$reply_fake_hold" GANG_STOP_QUERY_ATTEMPT_SEC=0.25 GANG_STOP_QUERY_DEADLINE_SEC=0.45 \
    FAKE_REPLY_LOG="$reply_fake_log" FAKE_QUERY_LOG="$reply_fake_query_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>"$reply_stop_stderr")"
# source-guard: whole-surface@b36adaf24f9f: the complete fake-adapter stdout is the timed-out query verdict, so any producer is valid evidence
contains "a query that never answers inside its deadline is refused under its own name" \
  "$reply_fake_out" "could not read verified peer-reply provenance in time"
excludes "a timed-out query is not sent to repair the query path" \
  "$reply_fake_out" "repair the query path"
# source-guard: whole-surface@abe5dcec80a3: the complete fake query log counts every attempt inside the deadline, so any producer is valid evidence
equal "the query was retried inside its deadline and then given up" \
  $'query\nquery' "$(cat "$reply_fake_query_log")"
# source-guard: whole-surface@795f44d43b6b: the complete fake hook log must stay empty for a refused timeout, so any producer is valid evidence
equal "a refused query-timeout closes no native boundary" "" "$(cat "$reply_fake_log")"
: > "$reply_fake_log"; : > "$reply_fake_query_log"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_HOLD="$reply_fake_hold" GANG_STOP_QUERY_ATTEMPT_SEC=0.25 GANG_STOP_QUERY_DEADLINE_SEC=0.45 \
    FAKE_REPLY_LOG="$reply_fake_log" FAKE_QUERY_LOG="$reply_fake_query_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@03d993bf8f89: the complete fake-adapter stdout is the released timeout verdict, so any producer is valid evidence
equal "a query-timeout on the re-Stop is released rather than held" "{}" "$reply_fake_out"
# source-guard: whole-surface@bc2c69ae2232: the complete fake hook log names the timeout as the release's own state, so any producer is valid evidence
equal "the query-timeout is released under its own name before the boundary closes" \
  $'released query-timeout\nhook' "$(cat "$reply_fake_log")"
: > "$reply_fake_log"; : > "$reply_fake_query_log"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_HOLD="$reply_fake_hold" GANG_STOP_QUERY_ATTEMPT_SEC=0.25 GANG_STOP_QUERY_DEADLINE_SEC=0.45 \
    FAKE_REPLY_LOG="$reply_fake_log" FAKE_QUERY_LOG="$reply_fake_query_log" \
    FAKE_HOOK_RC=9 python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@cba200edd698: the complete fake-adapter stdout is the failed-boundary reason after a timed-out query, so any producer is valid evidence
contains "a failed boundary after a timed-out query names the timeout" \
  "$reply_fake_out" "released after its reply query timed out"
excludes "a timed-out query is not called clear provenance" \
  "$reply_fake_out" "provenance is clear"

# THE BOUNDARY SPENDS WHAT THE QUERY LEFT, ON A REAL CLOCK. A boundary slower
# than a few seconds but well inside the native fuse must close, and the number
# the adapter carried was near the idle cost of one Gangline call, so this case
# is measured against the wall clock the collars bound rather than a scaled one.
# It therefore names /bin/sleep: the suite's own `sleep` on PATH is the counting
# stub that returns at once, which would leave this fixture holding nothing.
# Measured margin: the fake answers a quiet box in well under 50 ms, the fixture
# holds the boundary for 4 s, and production leaves the boundary everything the
# 15 s fuse has left after a 1 s reserve.
reply_fake_hook_hold="$reply_fake_root/hook-hold"
mkfifo "$reply_fake_hook_hold"
: > "$reply_fake_log"
# source-guard: whole-surface@9bfe5b4dba00: the complete fake-adapter stdout is the slow-boundary verdict, so any producer is valid evidence
equal "a boundary slower than a few seconds still closes inside the native fuse" \
  "{}" "$(printf '%s' "$reply_stop_payload" \
    | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
      FAKE_HOOK_DELAY=4 python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@ababcc619f93: the complete fake hook log records every boundary this invocation attempted, so any producer is valid evidence
equal "a slow boundary is delegated once and not retried" "hook" \
  "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
    FAKE_HOOK_HOLD="$reply_fake_hook_hold" GANG_STOP_HOOK_BUDGET_SEC=1.5 \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@db456842115c: the complete fake-adapter stdout is the unclosed-boundary verdict, so any producer is valid evidence
contains "a boundary that never closes inside the budget is refused under its own name" \
  "$reply_fake_out" "did not close inside its time budget"
excludes "an unclosed boundary is not reported as a broken hook path" \
  "$reply_fake_out" "repair the Gangline hook path"
# source-guard: whole-surface@e870119f1e4e: the complete fake-adapter stdout is the unclosed-boundary remedy, so any producer is valid evidence
contains "an unclosed boundary asks for the wait that can clear it" \
  "$reply_fake_out" "wait for the load to fall"
# THE MUTATING BOUNDARY IS ATTEMPTED ONCE OR NOT AT ALL. `gang hook` closes the
# turn, records the boundary's facts, dispatches delivery, and closes reply
# threads last; a killed attempt leaves a prefix of that done and a second
# attempt would repeat it beside a child the kill did not reach.
# source-guard: whole-surface@7ca557c333e1: the complete fake hook log records every boundary this invocation attempted, so any producer is valid evidence
equal "an unclosed boundary is never attempted a second time" "hook" \
  "$(cat "$reply_fake_log")"
: > "$reply_fake_log"
# AN UNCLOSED BOUNDARY FAILS CLOSED ON THE RE-STOP TOO. Allowing there leaves
# the turn bracket open, and a prompt under an open bracket is steering, so the
# replies that turn read stay answerable: the next message out can be
# correlated to one of them and its recipient is owed nothing back. A refusal
# is loud and recoverable; that loss has no record at all.
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_QUERY='clear\t-\t-\t-\n' FAKE_REPLY_LOG="$reply_fake_log" \
    FAKE_HOOK_HOLD="$reply_fake_hook_hold" GANG_STOP_HOOK_BUDGET_SEC=1.5 \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@962ee1fd1e44: the complete fake-adapter stdout is the unclosed-boundary verdict on a re-Stop, so any producer is valid evidence
contains "an unclosed boundary on the re-Stop still fails closed" \
  "$reply_fake_out" '"decision": "block"'
excludes "the re-Stop refusal is not a broken hook path either" \
  "$reply_fake_out" "repair the Gangline hook path"
# A STAGE THAT CANNOT FIT DOES NOT START. Every bound is clamped to what the
# fuse has left, so a query that spends its deadline and a release report that
# never answers leave nothing for the boundary — and a boundary with nothing
# left is not begun, because beginning it would mutate a prefix the harness is
# about to kill. Here the query spends 0.45 s of a 1.5 s fuse and the release
# report is clamped to the rest of it.
reply_fake_release_hold="$reply_fake_root/release-hold"
mkfifo "$reply_fake_release_hold"
: > "$reply_fake_log"; : > "$reply_fake_query_log"
reply_fake_started="$(date +%s%N)"
reply_fake_out="$(printf '%s' "$reply_stop_active_payload" \
  | FAKE_REPLY_HOLD="$reply_fake_hold" FAKE_RELEASE_HOLD="$reply_fake_release_hold" \
    FAKE_HOOK_HOLD="$reply_fake_hook_hold" \
    GANG_STOP_QUERY_ATTEMPT_SEC=0.25 GANG_STOP_QUERY_DEADLINE_SEC=0.45 \
    GANG_STOP_HOOK_BUDGET_SEC=1.5 \
    FAKE_REPLY_LOG="$reply_fake_log" FAKE_QUERY_LOG="$reply_fake_query_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
reply_fake_elapsed_ms=$(( ( $(date +%s%N) - reply_fake_started ) / 1000000 ))
# source-guard: whole-surface@f9cb1e28dd21: the complete fake hook log records every stage this invocation started, so any producer is valid evidence
equal "a fuse spent by the earlier stages never starts the mutating boundary" \
  "released query-timeout" "$(cat "$reply_fake_log")"
# source-guard: whole-surface@d19a5ab7088c: the complete fake-adapter stdout is the verdict for a spent fuse, so any producer is valid evidence
contains "a spent fuse still prints a verdict rather than nothing" \
  "$reply_fake_out" '"decision": "block"'
# source-guard: whole-surface@8a544445bc2e: the complete fake-adapter stdout names what the query found before the boundary ran out, so any producer is valid evidence
contains "a spent fuse names the released query alongside the unclosed boundary" \
  "$reply_fake_out" "released after its reply query timed out"
# The ceiling is coarse on purpose: it is twice the scaled fuse, so it survives
# a loaded box while still failing an adapter whose stage bounds add past the
# fuse. Stage constants unclamped, this same fixture spends 3.45 s. What the
# scaled numbers prove is the ordering and the clamping, not the production
# bound: interpreter startup precedes the adapter's clock and does not scale
# with the fixture, so it eats a scaled reserve while leaving most of the
# production one. A fixture at the production budget would be a mandatory test
# spending 15 s of wall time, which this suite does not admit.
equal "no stage bound outlives the fuse it was granted" yes \
  "$([ "$reply_fake_elapsed_ms" -lt 3000 ] && printf yes || printf no)"
reply_hook_budget="$(python3 -c '
import re, sys
match = re.search(r"GANG_STOP_HOOK_BUDGET_SEC\", \"([0-9.]+)\"", open(sys.argv[1]).read())
print(match.group(1) if match else "")
' "$reply_stop_hook")"
equal "the adapter budgets exactly the native fuse the collars grant" 15 \
  "$reply_hook_budget"

codex_reply_launch="$(env GANG_TEST_COLLARS='' ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$ROOT/collars/codex.sh")"
contains "the Codex collar installs peer-reply Stop enforcement" \
  "$codex_reply_launch" "codex-stop-hook.py"
contains "the Codex peer-reply helper retains its native fuse" \
  "$codex_reply_launch" 'timeout = 15'
claude_reply_launch="$(env GANG_TEST_COLLARS='' ROOT="$ROOT" GANG_CONTEXT_LIGHTS=off bash -c \
  '. "$1"; printf "%s" "$GANG_LAUNCH"' fixture "$ROOT/collars/claude-code.sh")"
contains "the Claude collar installs the same peer-reply Stop enforcement" \
  "$claude_reply_launch" "codex-stop-hook.py"
excludes "the Claude launch leaves the native Stop-block cap in force" \
  "$claude_reply_launch" "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP"

# Dropping the disposable window retires its complete option-scoped audit trail.
"$GANG" drop reply-b >/dev/null
"$GANG" drop reply-c >/dev/null
"$GANG" drop auto-resume >/dev/null
if [ -n "$reply_original_collars" ]; then
  export GANG_COLLARS="$reply_original_collars"
else
  unset GANG_COLLARS
fi
export PATH="$reply_original_path"
