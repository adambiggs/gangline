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

# A preflight refusal is a held, unregistered window, so the roster must still
# expose the single attended recovery line rather than flattening it to idle.
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

"$GANG" down "$GANG_SESSION" >/dev/null
export GANG_SESSION="$friction_original_session"
if [ -n "$friction_original_collars" ]; then
  export GANG_COLLARS="$friction_original_collars"
else
  unset GANG_COLLARS
fi
