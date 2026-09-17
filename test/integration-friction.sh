# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Native recovery seams: automatic recap, scope-confirmed copy-mode, and the
# attended hook-trust path. This part owns one separately named disposable
# team; every wait below is signalled by a fixture event, never a clock.

friction_original_session="$GANG_SESSION"
friction_original_collars="${GANG_COLLARS:-}"
export GANG_SESSION="gangfriction-$$"
friction_collars="$RUN_ROOT/friction-collars"
friction_recap_arm="$RUN_ROOT/friction-recap-arm"
friction_recap_channel="gang-friction-recap-$$"
mkdir -p "$friction_collars"
cat > "$RUN_ROOT/friction-bashrc" <<SH
PS1='❯ '
command_not_found_handle() {
  [ -e '$friction_recap_arm' ] || return 127
  tmux wait-for -S '$friction_recap_channel'
  return 127
}
SH
cat > "$friction_collars/friction-recap.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_LAUNCH="ENV='$RUN_ROOT/friction-bashrc' bash --posix"
GANG_STOP_HOOK=1
# The core transaction only needs the collar's positive boundary verdict. The
# shipped Codex reader is exercised below against the exact painted frame; this
# minimal collar keeps that parser assertion independent of core delivery.
collar_recap_boundary() {
  return 0
}
SH
export GANG_COLLARS="$friction_collars"
tmux new-session -d -s "$GANG_SESSION" -n caller "PS1='❯ ' exec bash --norc"
friction_socket="$(tmux display-message -p -t "=$GANG_SESSION" '#{socket_path}')"
"$GANG" adopt caller -c bash >/dev/null
friction_caller_id="$(window_id caller)"
friction_caller_pane="$(tmux list-panes -t "$friction_caller_id" -F '#{pane_id}')"
"$HITCH" recap -c friction-recap -d "$RUN_ROOT" >/dev/null
friction_recap_id="$(window_id recap)"
friction_recap_pane="$(tmux list-panes -t "$friction_recap_id" -F '#{pane_id}')"
# The collar's recap surface arrives after a native turn boundary. Close the
# launch turn first; otherwise the core correctly ranks the open turn over the
# recap frame and reports busy rather than inventing an idle boundary.
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_recap_pane" "$GANG" hook >/dev/null
: > "$friction_recap_arm"
# The first roster observation discovers the current recap frame and arms the
# ordinary delivery worker. Its injection barrier proves that discovery
# happened. A second observation is the roster promise while that durable
# continuation remains pending; it cannot be masked by the now-free composer.
friction_recap_trigger="$("$GANG" roster)"
contains "the initial recap roster observes its still-visible native frame" \
  "$friction_recap_trigger" "recap"
tmux wait-for "$friction_recap_channel"
# source-guard: whole-surface@4cbc280c1601: the dedicated fixture starts empty and the only producer of this sentence is the automatic recap continuation armed directly above
contains "the recap boundary submits one owned continuation" \
  "$(pane_all recap)" "Your context was just compacted."
friction_recap_pending="$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_pending)"
equal "the recap continuation keeps its exact durable marker before delivery completes" 16 \
  "${#friction_recap_pending}"
friction_recap_roster="$("$GANG" roster)"
contains "an empty native recap is visible as work pending continuation" \
  "$friction_recap_roster" "~wait~ (post-compaction continuation pending)"
equal "the recap continuation is recorded once while its old screen remains" 1 \
  "$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_handled)"
printf '%s' '{"hook_event_name":"UserPromptSubmit"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_recap_pane" "$GANG" hook >/dev/null
equal "the submitted continuation clears its parked-work marker" "" \
  "$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_pending)"
# A native build may send only PostCompact. That closing edge must make a
# future automatic recap observable even though the old recap paint's marker
# deliberately survives the continuation submission above.
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_recap_pane" "$GANG" hook >/dev/null
equal "an unowned PostCompact opens the next recap episode" "" \
  "$(tmux show-options -wqv -t "$friction_recap_id" @gl_recap_handled)"

# Two independent state readers can arrive on the same painted native recap.
# The collar holds reader one after the claim; reader two either loses that
# claim and reports unknown, or (in the mutation) reaches its own barrier.
# The event race lets the test deterministically release both old unclaimed
# readers while the fixed code releases just its one claim holder.
friction_race_first="gang-friction-race-first-$$"
friction_race_entered="gang-friction-race-entered-$$"
friction_race_release_one="gang-friction-race-release-one-$$"
friction_race_release_two="gang-friction-race-release-two-$$"
cat > "$friction_collars/friction-race.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
collar_recap_boundary() {
  case "\${GANG_TEST_RECAP_OBSERVER:-}" in
    one)
      tmux -S '$friction_socket' wait-for -S '$friction_race_first'
      tmux -S '$friction_socket' wait-for '$friction_race_release_one' ;;
    two)
      tmux -S '$friction_socket' wait-for -S '$friction_race_entered'
      tmux -S '$friction_socket' wait-for '$friction_race_release_two' ;;
  esac
  return 0
}
SH
"$HITCH" recap-race -c friction-race -d "$RUN_ROOT" >/dev/null
friction_race_id="$(window_id recap-race)"
friction_race_pane="$(tmux list-panes -t "$friction_race_id" -F '#{pane_id}')"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_race_pane" "$GANG" hook >/dev/null
tmux copy-mode -t "$friction_race_id"
friction_race_events="$RUN_ROOT/friction-race-events"
friction_race_one="$RUN_ROOT/friction-race-one"
friction_race_two="$RUN_ROOT/friction-race-two"
mkfifo "$friction_race_events"
exec 9<>"$friction_race_events"
GANG_TEST_RECAP_OBSERVER=one "$GANG" roster > "$friction_race_one" 2>&1 &
friction_race_one_pid=$!
tmux wait-for "$friction_race_first"
(
  race_two_rc=0
  GANG_TEST_RECAP_OBSERVER=two "$GANG" roster > "$friction_race_two" 2>&1 || race_two_rc=$?
  printf 'returned:%s\n' "$race_two_rc" > "$friction_race_events"
) &
friction_race_two_pid=$!
( tmux wait-for "$friction_race_entered"; printf 'entered\n' > "$friction_race_events" ) &
friction_race_entered_pid=$!
IFS= read -r friction_race_event <&9
case "$friction_race_event" in
  entered)
    tmux wait-for -S "$friction_race_release_one"
    tmux wait-for -S "$friction_race_release_two" ;;
  returned:*)
    tmux wait-for -S "$friction_race_release_one"
    # The fixed observer loses the recap claim before it enters its collar.
    # Release the fixture's separate waiter rather than killing its shell: a
    # killed parent can leave the tmux wait-for child as an unaccounted barrier.
    tmux wait-for -S "$friction_race_entered"
    wait "$friction_race_entered_pid" ;;
  *) fail "the concurrent recap observer reports a determinate race event" \
       "event [$friction_race_event]" ;;
esac
wait "$friction_race_one_pid"
wait "$friction_race_two_pid"
equal "concurrent recap observers leave one durable continuation" 1 \
  "$("$GANG" roster --porcelain | awk -F '\t' '$1 == "recap-race" { print $4 }')"
friction_race_pending="$(tmux show-options -wqv -t "$friction_race_id" @gl_recap_pending)"
equal "concurrent recap observers leave one marker nonce" 16 \
  "${#friction_race_pending}"
tmux send-keys -t "$friction_race_id" -X cancel
"$GANG" drop recap-race >/dev/null
exec 9>&-
rm -f -- "$friction_race_events"

# A compact request records ownership before inject so its later native bracket
# does not arm a duplicate continuation. That marker belongs only to a command
# that could have reached Enter: peer copy-mode and a deferred self request
# forced into a draft both refuse before typing, so each must retire it.
cat > "$friction_collars/friction-rollback.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_COMPACT_CMD='printf ignored'
collar_recap_boundary() { return 0; }
SH
"$HITCH" recap-rollback -c friction-rollback -d "$RUN_ROOT" >/dev/null
friction_rollback_id="$(window_id recap-rollback)"
friction_rollback_pane="$(tmux list-panes -t "$friction_rollback_id" -F '#{pane_id}')"
tmux copy-mode -t "$friction_rollback_id"
friction_rollback_rc=0
if TMUX_PANE="$friction_caller_pane" "$GANG" compact recap-rollback; then
  friction_rollback_rc=0
else
  friction_rollback_rc=$?
fi
equal "a peer copy-mode refusal is proved pre-Enter" 3 "$friction_rollback_rc"
equal "a refused peer compaction retires its ownership marker" "" \
  "$(tmux show-options -wqv -t "$friction_rollback_id" @gl_compaction_issued)"
# The copy-mode refusal proved the command did not reach Enter. Leave that
# operator-owned mode before presenting the separate native recap episode, so
# its continuation can be observed and delivered rather than rightly blocked.
tmux send-keys -t "$friction_rollback_id" -X cancel
printf '%s' '{"hook_event_name":"PreCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_rollback_pane" "$GANG" hook >/dev/null
printf '%s' '{"hook_event_name":"PostCompact"}' \
  | GANG_TEST_TICK_MODE=manual TMUX_PANE="$friction_rollback_pane" "$GANG" hook >/dev/null
# PostCompact schedules its ordinary drain; the following state read owns the
# fresh automatic recap observation and arms the one durable continuation.
friction_rollback_recap="$("$GANG" roster)"
contains "an unowned native recap after a refused peer request is observed" \
  "$friction_rollback_recap" "~wait~ (post-compaction continuation pending)"
friction_rollback_pending="$(tmux show-options -wqv -t "$friction_rollback_id" @gl_recap_pending)"
equal "an unowned native recap after a refused peer request gets one continuation" 16 \
  "${#friction_rollback_pending}"
# `@gl_recap_handled` is written only after the one continuation is accepted
# into durable delivery. The worker can claim that entry before a second roster
# sees it, so its transient spool count is not evidence of how many episodes
# the observer armed.
equal "the unowned native recap keeps one continuation after the refused pane mode ends" 1 \
  "$(tmux show-options -wqv -t "$friction_rollback_id" @gl_recap_handled)"
"$GANG" drop recap-rollback >/dev/null

friction_self_ready="gang-friction-self-ready-$$"
friction_self_release="gang-friction-self-release-$$"
cat > "$friction_collars/friction-self-rollback.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_SELF_COMPACT=deferred
GANG_COMPACT_CMD='printf ignored'
_friction_base_input="\$(declare -f collar_input)"
eval "\${_friction_base_input/collar_input/friction_base_input}"
collar_input() {
  if [ "\${GANG_TEST_SELF_ROLLBACK_SYNC:-}" = 1 ] \
     && [ -z "\$(tmux show-options -wqv -t \"\$1\" @gl_compaction_issued 2>/dev/null)" ]; then
    tmux -S '$friction_socket' wait-for -S '$friction_self_ready'
    tmux -S '$friction_socket' wait-for '$friction_self_release'
  fi
  friction_base_input "\$1"
}
SH
"$HITCH" self-rollback -c friction-self-rollback -d "$RUN_ROOT" >/dev/null
friction_self_id="$(window_id self-rollback)"
friction_self_pane="$(tmux list-panes -t "$friction_self_id" -F '#{pane_id}')"
TMUX_PANE="$friction_self_pane" "$GANG" compact >/dev/null
friction_self_request="$(tmux show-options -wqv -t "$friction_self_id" @gl_self_compact_requested)"
printf '%s' '{"hook_event_name":"Stop"}' \
  | GANG_TEST_TICK_MODE=manual GANG_TEST_SELF_ROLLBACK_SYNC=1 \
    TMUX_PANE="$friction_self_pane" "$GANG" hook >/dev/null
tmux wait-for "$friction_self_ready"
tmux copy-mode -t "$friction_self_id"
tmux wait-for -S "$friction_self_release"
tmux wait-for "gang-self-compact-$friction_self_request"
equal "a deferred pre-Enter refusal retires its ownership marker" "" \
  "$(tmux show-options -wqv -t "$friction_self_id" @gl_compaction_issued)"
equal "the deferred pre-Enter refusal preserves its retry request" "$friction_self_request" \
  "$(tmux show-options -wqv -t "$friction_self_id" @gl_self_compact_requested)"
tmux send-keys -t "$friction_self_id" -X cancel
"$GANG" drop self-rollback >/dev/null

# The shipped parser must recognise the native current-screen shape itself,
# not merely the test collar used to exercise the dispatcher transaction. The
# fixture signals only after the full screen is painted; no timed look stands
# in for that native fact.
friction_reader_channel="gang-friction-reader-$$"
friction_reader_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n recap-reader -c "$RUN_ROOT" \
  "printf '%s\\n' '─ Conversation recap ─' 'unfinished lane is preserved' '› '
tmux wait-for -S '$friction_reader_channel'
exec cat")"
friction_reader_pane="$(tmux list-panes -t "$friction_reader_id" -F '#{pane_id}')"
tmux wait-for "$friction_reader_channel"
friction_reader_rc=0
env -u GANG_TMUX_GUARD_AGENT TMUX="$friction_socket,0,0" \
  bash -c '. "$1"; collar_recap_boundary "$2"' fixture \
  "$ROOT/collars/codex.sh" "$friction_reader_pane" || friction_reader_rc=$?
equal "the shipped recap reader recognises an empty native recap" 0 \
  "$friction_reader_rc"
friction_draft_channel="gang-friction-draft-$$"
friction_draft_id="$(tmux new-window -d -P -F '#{window_id}' -t "=$GANG_SESSION" \
  -n recap-draft -c "$RUN_ROOT" \
  "printf '%s\\n' '─ Conversation recap ─' 'unfinished lane is preserved' '› draft'
tmux wait-for -S '$friction_draft_channel'
exec cat")"
friction_draft_pane="$(tmux list-panes -t "$friction_draft_id" -F '#{pane_id}')"
tmux wait-for "$friction_draft_channel"
friction_draft_rc=0
env -u GANG_TMUX_GUARD_AGENT TMUX="$friction_socket,0,0" \
  bash -c '. "$1"; collar_recap_boundary "$2"' fixture \
  "$ROOT/collars/codex.sh" "$friction_draft_pane" >/dev/null || friction_draft_rc=$?
equal "the shipped recap reader rejects a nonempty post-recap draft" 1 \
  "$friction_draft_rc"

# `gang trust` has to open the collar's native hook configuration directly;
# a preflight prefix here would recreate the refusal instead of exposing the
# menu an operator must answer. The stub reports its received arguments before
# signalling the test, so the argument list is the native launch evidence.
friction_trust_bin="$RUN_ROOT/friction-trust-bin"
friction_trust_args="$RUN_ROOT/friction-trust-args"
friction_trust_socket="$RUN_ROOT/friction-trust-socket"
friction_trust_lock="$RUN_ROOT/friction-trust-lock"
friction_trust_channel="gang-friction-trust-$$"
mkdir -p "$friction_trust_bin"
cat > "$friction_trust_bin/codex" <<SH
#!/bin/sh
printf '%s\\n' "\$@" > '$friction_trust_args'
printf '%s\\n' "\$GANG_TMUX_SOCKET" > '$friction_trust_socket'
printf '%s\\n' "\$GANG_LOCK_DIR" > '$friction_trust_lock'
tmux -S '$friction_socket' wait-for -S '$friction_trust_channel'
exec bash --norc
SH
chmod +x "$friction_trust_bin/codex"
friction_trust_out="$(PATH="$friction_trust_bin:$PATH" "$GANG" trust codex -d "$RUN_ROOT")"
tmux wait-for "$friction_trust_channel"
friction_trust_window="$(printf '%s\n' "$friction_trust_out" \
  | awk '{ for (i = 1; i <= NF; i++) if ($i ~ /^@[0-9]+$/) { print $i; exit } }')"
[ -n "$friction_trust_window" ] \
  || { printf 'friction: attended trust command named no window [%s]\n' "$friction_trust_out" >&2; exit 1; }
contains "the attended trust path opens the native hook configuration" \
  "$(<"$friction_trust_args")" 'hooks.PreCompact='
excludes "the attended trust path does not put its preflight before the menu" \
  "$(<"$friction_trust_args")" 'codex-hooks-preflight.py'
equal "the attended trust hook path receives only the guarded tmux route" \
  "$friction_socket" "$(<"$friction_trust_socket")"
equal "the attended trust hook path keeps its durable lock root" \
  "$GANG_LOCK_DIR" "$(<"$friction_trust_lock")"
contains "the attended trust command says it sent no menu key" \
  "$friction_trust_out" 'sent no menu key'
equal "the attended trust review is not registered as an agent" "" \
  "$(tmux show-options -wqv -t "$friction_trust_window" @gl_agent)"

# A trust-review window is a transient owned by the command, not an unadopted
# agent. Its model and effort must therefore follow the refused hitch's choices
# and a cooperative tick must retire it once its own collar positively sees the
# composer that follows the operator's native menu answer.
friction_trust_choice_args="$RUN_ROOT/friction-trust-choice-args"
friction_trust_choice_launch="$RUN_ROOT/friction-trust-choice-launch.sh"
friction_trust_choice_channel="gang-friction-trust-choice-$$"
cat > "$friction_trust_choice_launch" <<SH
#!/bin/sh
printf '%s\\n' "\$@" > '$friction_trust_choice_args'
tmux -S '$friction_socket' wait-for -S '$friction_trust_choice_channel'
PS1='❯ ' exec bash --norc
SH
chmod +x "$friction_trust_choice_launch"
cat > "$friction_collars/trust-review.sh" <<SH
# shellcheck shell=bash
# shellcheck disable=SC2034
. "$ROOT/collars/bash.sh"
GANG_TRUST_LAUNCH="'$friction_trust_choice_launch'"
GANG_MODEL_OPT='--model'
GANG_EFFORT_OPT='--effort='
GANG_EFFORT_CMD="printf 'careful\\n'"
collar_model_check() {
  [ "\$1" = chosen-model ]
}
SH
friction_trust_choice_out="$("$GANG" trust trust-review -d "$RUN_ROOT" \
  -m chosen-model -e careful)"
tmux wait-for "$friction_trust_choice_channel"
friction_trust_choice_window="$(printf '%s\n' "$friction_trust_choice_out" \
  | awk '{ for (i = 1; i <= NF; i++) if ($i ~ /^@[0-9]+$/) { print $i; exit } }')"
[ -n "$friction_trust_choice_window" ] \
  || { printf 'friction: choice trust command named no window [%s]\n' "$friction_trust_choice_out" >&2; exit 1; }
equal "the attended trust launch receives the requested model and effort" \
  $'--model\nchosen-model\n--effort=careful' "$(<"$friction_trust_choice_args")"
equal "the transient choice review remains unregistered before its menu is answered" "" \
  "$(tmux show-options -wqv -t "$friction_trust_choice_window" @gl_agent)"
"$GANG" tick >/dev/null
equal "a tick retires the composer-ready trust review that gang launched" "" \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#{window_id}' | \
    awk -v wanted="$friction_trust_choice_window" '$1 == wanted { print $1 }')"

# A preflight refusal can die before or just after hitch records its identity,
# so its marker must keep the attended recovery visible in either state.
friction_hold_hooks=""
for friction_event in SessionStart UserPromptSubmit PostToolUse PermissionRequest PreCompact PostCompact Stop; do
  friction_hold_hooks="$friction_hold_hooks -c 'hooks.$friction_event=[{ hooks = [{ type = \"command\", command = \"/bin/true\" }] }]'"
done
friction_hold_ready="$RUN_ROOT/friction-hold-ready"
mkfifo "$friction_hold_ready"
friction_hold_pane="$(tmux new-window -d -P -F '#{pane_id}' -t "=$GANG_SESSION" -n hook-hold -c "$RUN_ROOT" \
  "exec 9<>$friction_hold_ready
exec env -u TMUX GANG_TMUX_SOCKET=\"\${TMUX%%,*}\" PATH=$CODEX_STUB/bin:\$PATH CODEX_UNTRUSTED=1 python3 $ROOT/collars/plugins/codex-hooks-preflight.py codex$friction_hold_hooks")"
exec 8<"$friction_hold_ready"
cat <&8 >/dev/null
exec 8<&-
equal "the held hook-trust pane remains unregistered" "" \
  "$(tmux show-options -wqv -t "$friction_hold_pane" @gl_agent)"
friction_hold_roster="$("$GANG" roster)"
contains "a held hook-trust refusal remains visible in roster" \
  "$friction_hold_roster" '!hook-trust!'
contains "the held hook-trust roster row gives the attended re-grant command" \
  "$friction_hold_roster" "gang trust codex -d $RUN_ROOT"

# Exercise the printed recovery end to end against its original requested
# window. The fixture's native stand-in reports the preflight hook list through
# the shipped app-server stub, then keeps a real Codex-shaped composer alive.
# The fixture writes its approval fact only after the separate review window is
# closed, representing the operator's native approval; Gangline never sends
# that approval itself.
friction_rehitch_bin="$RUN_ROOT/friction-rehitch-bin"
friction_rehitch_approval="$RUN_ROOT/friction-rehitch-approved"
friction_rehitch_composer="$RUN_ROOT/friction-rehitch-composer.py"
mkdir -p "$friction_rehitch_bin"
cat > "$friction_rehitch_composer" <<'PY'
#!/usr/bin/env python3
import os
import sys
import termios
import tty

fd = sys.stdin.fileno()
saved = termios.tcgetattr(fd)
body = bytearray()


def render():
    os.write(sys.stdout.fileno(), b"\r\033[2K\xe2\x80\xba " + bytes(body))


try:
    tty.setraw(fd)
    render()
    while True:
        char = os.read(fd, 1)
        if not char:
            break
        if char in (b"\r", b"\n"):
            body.clear()
        elif char in (b"\x15", b"\x03"):
            body.clear()
        elif char in (b"\x08", b"\x7f"):
            del body[-1:]
        else:
            body.extend(char)
        render()
finally:
    termios.tcsetattr(fd, termios.TCSADRAIN, saved)
PY
chmod +x "$friction_rehitch_composer"
cat > "$friction_rehitch_bin/codex" <<SH
#!/bin/sh
case " \$* " in
  *' app-server '*)
    if [ -e '$friction_rehitch_approval' ]; then
    exec '$CODEX_STUB/bin/codex' "\$@"
    fi
    exec env CODEX_UNTRUSTED=1 '$CODEX_STUB/bin/codex' "\$@" ;;
esac
exec '$friction_rehitch_composer'
SH
chmod +x "$friction_rehitch_bin/codex"
friction_rehitch_refusal_rc=0
friction_rehitch_refusal="$(PATH="$friction_rehitch_bin:$CODEX_STUB/bin:$PATH" \
  "$HITCH" trust-rehitch -c codex -d "$RUN_ROOT" 2>&1)" || friction_rehitch_refusal_rc=$?
if [ "$friction_rehitch_refusal_rc" -ne 0 ]; then
  pass "an untrusted Codex hitch leaves its original requested window held"
else
  fail "an untrusted Codex hitch leaves its original requested window held" \
    "hitch unexpectedly succeeded [$friction_rehitch_refusal]"
fi
contains "an untrusted hitch directs the attended review to tick after its composer" \
  "$friction_rehitch_refusal" "run gang tick after the composer appears"
excludes "an untrusted hitch no longer asks the operator to quit Codex" \
  "$friction_rehitch_refusal" "quit codex"
friction_rehitch_id="$(window_id trust-rehitch)"
equal "the original refused window carries its exact attended-trust marker" \
  $'codex\t'"$RUN_ROOT" \
  "$(tmux show-options -wqv -t "$friction_rehitch_id" @gl_hook_trust_pending)"
equal "the original held trust refusal retains only its requested identity before re-hitch" trust-rehitch \
  "$(tmux show-options -wqv -t "$friction_rehitch_id" @gl_agent)"
friction_rehitch_roster="$("$GANG" roster)"
contains "the original held trust refusal remains visible despite partial registration" \
  "$friction_rehitch_roster" '!hook-trust!'
friction_rehitch_trust="$(PATH="$friction_rehitch_bin:$CODEX_STUB/bin:$PATH" \
  "$GANG" trust codex -d "$RUN_ROOT")"
friction_rehitch_review="$(printf '%s\n' "$friction_rehitch_trust" \
  | awk '{ for (i = 1; i <= NF; i++) if ($i ~ /^@[0-9]+$/) { print $i; exit } }')"
[ -n "$friction_rehitch_review" ] \
  || { printf 'friction: trust recovery opened no review window [%s]\n' "$friction_rehitch_trust" >&2; exit 1; }
# This is the attended review's normal termination, not a Gangline cleanup of
# the held refusal. The following exact original hitch must repurpose its pane.
tmux kill-window -t "$friction_rehitch_review"
: > "$friction_rehitch_approval"
friction_rehitch_success="$(PATH="$friction_rehitch_bin:$CODEX_STUB/bin:$PATH" \
  "$HITCH" trust-rehitch -c codex -d "$RUN_ROOT")"
contains "the exact refused hitch succeeds after attended trust without drop" \
  "$friction_rehitch_success" "hitched trust-rehitch"
equal "the recovered original window is registered to its original name" trust-rehitch \
  "$(tmux show-options -wqv -t "$friction_rehitch_id" @gl_agent)"
equal "a successful re-hitch consumes only the held trust marker" "" \
  "$(tmux show-options -wqv -t "$friction_rehitch_id" @gl_hook_trust_pending)"
"$GANG" drop trust-rehitch >/dev/null

"$GANG" down "$GANG_SESSION" >/dev/null
export GANG_SESSION="$friction_original_session"
if [ -n "$friction_original_collars" ]; then
  export GANG_COLLARS="$friction_original_collars"
else
  unset GANG_COLLARS
fi
