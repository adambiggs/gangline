#!/usr/bin/env bash
# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Verified peer reply obligations and the native Stop adapter.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file and
# supplies the private tmux server, helpers, counters, and cleanup.

reply_original_collars="${GANG_COLLARS:-}"
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
SH
export GANG_COLLARS="$RUN_ROOT/collars"

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
      request) [ -z "$(tmux show-options -wqv -t "$1" "@gl_rsettled_$nonce")" ] || continue ;;
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

reply_request_envelope() { # sender nonce body
  printf '[gang:%s#%s] %s [/gang:%s#%s]' "$1" "$2" "$3" "$1" "$2"
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
printf '%s' OPERATOR_ENVELOPE \
  | "$GANG" send --to reply-b --from operator --stdin >/dev/null
equal "a verified self-declared operator envelope creates no peer debt" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"

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
equal "prompt-first concurrent evidence is preserved while delivery is in flight" \
  $'unknown\t'"$reply_race_nonce"$'\treply-a\tprovenance-prompt-request' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "an answerable ambiguity asks for the reply that clears it" \
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

# Direct verified request. Delivery verification and the native prompt witness
# are independent facts and may arrive in either order.
printf '%s' REQ_DELIVERY_ONLY \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_delivery_only="$(reply_nonce_for_body \
  "$reply_b_id" reply-a request REQ_DELIVERY_ONLY)"
reply_delivery_only_wire="$(reply_request_envelope reply-a \
  "$reply_delivery_only" REQ_DELIVERY_ONLY)"
equal "delivery without its native prompt witness fails closed as ambiguous" \
  $'unknown\t'"$reply_delivery_only"$'\treply-a\tprovenance-verified-request' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "ambiguous peer provenance refuses idle" "$reply_stop_output" '"decision": "block"'
printf '%s' ACK_DELIVERY_ONLY \
  | TMUX_PANE="$reply_b_pane" "$GANG" send --to reply-a --stdin >/dev/null
reply_delivery_ack_nonce="$(reply_nonce_for_body \
  "$reply_a_id" reply-b reply ACK_DELIVERY_ONLY)"
reply_prompt_event "$reply_a_pane" \
  "$(reply_response_envelope reply-b "$reply_delivery_ack_nonce" \
    "$reply_delivery_only" ACK_DELIVERY_ONLY)"
reply_stop_run "$reply_a_pane"
equal "the delivery-first reply recipient may idle without reciprocal debt" \
  "{}" "$reply_stop_output"
equal "a correlated reply discharges delivery-only provenance" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the answered debtor may idle without its native prompt witness" \
  "{}" "$reply_stop_output"
reply_delivery_meta="$(tmux show-options -wqv -t "$reply_b_id" \
  "@gl_reply_$reply_delivery_only")"
IFS=: read -r _ _ _ _ reply_delivery_digest _ <<<"$reply_delivery_meta"
equal "the discharged record carries no prompt proof yet" "" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rprompt_$reply_delivery_only")"
reply_prompt_event "$reply_b_pane" "$reply_delivery_only_wire"
equal "the late prompt proof completes the discharged audit record" \
  "$reply_delivery_digest" \
  "$(tmux show-options -wqv -t "$reply_b_id" "@gl_rprompt_$reply_delivery_only")"
equal "the missing prompt proof completes the already correlated settlement" \
  $'clear\t-\t-\t-' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
equal "the delivery-first sender may idle after both proofs complete" \
  "{}" "$reply_stop_output"

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
# request, so failing closed on one named no action.
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
reply_stop_run "$reply_b_pane" "$reply_stop_active_payload"
contains "the native recursion marker does not erase an outstanding debt" \
  "$reply_stop_output" '"decision": "block"'

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
equal "a later turn boundary verifies the spooled peer request without losing its nonce" \
  $'unknown\t'"$reply_spool_nonce"$'\treply-a\tprovenance-verified-request' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
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

# Adapter failures are ambiguity, never permission. Its one clear arm delegates
# generic Stop bookkeeping; malformed or failed query paths do not.
reply_fake_root="$RUN_ROOT/reply-fake"
mkdir -p "$reply_fake_root"
cat > "$reply_fake_root/gang" <<'SH'
#!/bin/sh
case "$1" in
  reply-obligations) printf '%b' "$FAKE_REPLY_QUERY"; exit "${FAKE_REPLY_RC:-0}" ;;
  hook) printf 'hook\n' >> "$FAKE_REPLY_LOG"; cat >/dev/null; exit "${FAKE_HOOK_RC:-0}" ;;
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
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='unknown\t4444444444444444\tanswerable-peer\tprovenance-prompt-request\n' \
    FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@1d9038b9a1d5: the complete fake-adapter stdout is the answerable-ambiguity decision, so any producer is valid evidence
contains "an answerable ambiguity still refuses idle" \
  "$reply_fake_out" '"decision": "block"'
# source-guard: whole-surface@70a0bbcdfc2d: the complete fake-adapter stdout is the answerable-ambiguity reason, so any producer is valid evidence
contains "an answerable ambiguity names the peer to answer" \
  "$reply_fake_out" "reply to answerable-peer"
reply_fake_out="$(printf '%s' "$reply_stop_payload" \
  | FAKE_REPLY_QUERY='unknown\t4444444444444444\tanswerable-peer\tprovenance-prompt-request\nunknown\t5555555555555555\tstuck-peer\tsender-identity-unreadable\n' \
    FAKE_REPLY_LOG="$reply_fake_log" \
    python3 "$reply_stop_hook" "$reply_fake_root/gang" 2>/dev/null)"
# source-guard: whole-surface@083823472772: the complete fake-adapter stdout is the mixed-ambiguity reason, so any producer is valid evidence
contains "a mixed ambiguity reports the record no reply reaches" \
  "$reply_fake_out" "sender-identity-unreadable"
excludes "a mixed ambiguity does not reduce to its answerable half" \
  "$reply_fake_out" "reply to answerable-peer"
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
contains "the Claude launch carries the explicitly selected unlimited Stop-block lifecycle" \
  "$claude_reply_launch" "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=0"

# Dropping the disposable window retires its complete option-scoped audit trail.
"$GANG" drop reply-b >/dev/null
"$GANG" drop reply-c >/dev/null
"$GANG" drop auto-resume >/dev/null
if [ -n "$reply_original_collars" ]; then
  export GANG_COLLARS="$reply_original_collars"
else
  unset GANG_COLLARS
fi
