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
  local key value
  while read -r key value; do
    case "$key:$value" in
      @gl_reply_*:*:"$2":*) printf '%s' "${key#@gl_reply_}"; return 0 ;;
    esac
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

reply_new_window() { # name -> id TAB pane
  local id pane_id
  id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
    -n "$1" "PS1='❯ ' bash --norc")"
  pane_id="$(tmux list-panes -t "$id" -F '#{pane_id}')"
  printf '%s\t%s\n' "$id" "$pane_id"
}

IFS=$'\t' read -r reply_a_id reply_a_pane <<<"$(reply_new_window reply-a)"
"$GANG" adopt reply-a -c replyable >/dev/null
IFS=$'\t' read -r reply_b_id reply_b_pane <<<"$(reply_new_window reply-b)"
# An agent adopts B, deliberately: lifecycle ancestry must not create debt.
TMUX_PANE="$reply_a_pane" "$GANG" adopt reply-b -c replyable >/dev/null
IFS=$'\t' read -r reply_c_id reply_c_pane <<<"$(reply_new_window reply-c)"
"$GANG" adopt reply-c -c replyable >/dev/null

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

# Direct verified request. Delivery verification and the native prompt witness
# are independent facts and may arrive in either order.
printf '%s' REQ_A_ONE \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_a_one="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_a_one_wire="$(reply_request_envelope reply-a "$reply_a_one" REQ_A_ONE)"
equal "delivery without its native prompt witness fails closed as ambiguous" \
  $'unknown\t'"$reply_a_one"$'\treply-a\tprovenance-verified-request' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "ambiguous peer provenance refuses idle" "$reply_stop_output" '"decision": "block"'
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

# Two peers arm two independent records. A response to one cannot overwrite or
# settle the other, and each correlated response remains reply-only at target.
printf '%s' REQ_MULTI_A \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_multi_a="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_multi_a" REQ_MULTI_A)"
printf '%s' "$reply_stop_payload" | TMUX_PANE="$reply_b_pane" "$GANG" hook >/dev/null
printf '%s' REQ_MULTI_C \
  | TMUX_PANE="$reply_c_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_multi_c="$(reply_nonce_from "$reply_b_id" reply-c)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-c "$reply_multi_c" REQ_MULTI_C)"
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

# Corrupt evidence is never repaired into absence. A vanished sender remains a
# named debt rather than being redirected to the hitcher or team lead.
tmux set-option -w -t "$reply_b_id" @gl_reply_bad malformed
contains "malformed provenance is a loud unknown" \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)" \
  $'unknown\t-\t-\treply-obligation option @gl_reply_bad is malformed'
contains "status reports malformed provenance as Stop-blocking" "$($GANG status reply-b)" \
  "reply obligation UNKNOWN"
reply_stop_run "$reply_b_pane"
contains "malformed provenance fails closed at Stop" "$reply_stop_output" '"decision": "block"'
tmux set-option -uw -t "$reply_b_id" @gl_reply_bad

printf '%s' REQ_GONE \
  | TMUX_PANE="$reply_a_pane" "$GANG" send --to reply-b --stdin >/dev/null
reply_gone_nonce="$(reply_nonce_from "$reply_b_id" reply-a)"
reply_prompt_event "$reply_b_pane" \
  "$(reply_request_envelope reply-a "$reply_gone_nonce" REQ_GONE)"
"$GANG" drop reply-a >/dev/null
equal "a vanished sender stays attached to its original message debt" \
  $'owed\t'"$reply_gone_nonce"$'\treply-a\tgone' \
  "$(TMUX_PANE="$reply_b_pane" "$GANG" reply-obligations)"
reply_stop_run "$reply_b_pane"
contains "stale sender provenance still refuses idle" "$reply_stop_output" '"decision": "block"'

# Adapter failures are ambiguity, never permission. Its one clear arm delegates
# generic Stop bookkeeping; malformed or failed query paths do not.
reply_fake_root="$RUN_ROOT/reply-fake"
mkdir -p "$reply_fake_root"
cat > "$reply_fake_root/gang" <<'SH'
#!/bin/sh
case "$1" in
  reply-obligations) printf '%b' "$FAKE_REPLY_QUERY"; exit "${FAKE_REPLY_RC:-0}" ;;
  hook) printf 'hook\n' >> "$FAKE_REPLY_LOG"; cat >/dev/null ;;
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

# The final gone-sender record is impossible to satisfy by design; remove only
# this fixture option before deleting its disposable receiver.
tmux set-option -uw -t "$reply_b_id" "@gl_reply_$reply_gone_nonce"
"$GANG" drop reply-b >/dev/null
"$GANG" drop reply-c >/dev/null
if [ -n "$reply_original_collars" ]; then
  export GANG_COLLARS="$reply_original_collars"
else
  unset GANG_COLLARS
fi
