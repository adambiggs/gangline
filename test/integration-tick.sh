# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# Cooperative tick: global retries, copy-mode recovery, native identity, and rails.
#
# A PART IS A FRAGMENT, NOT A SCRIPT. test/integration.sh sources this file
# after the ordinary substrate checks and supplies its fixtures and assertions.

tick_original_session="$GANG_SESSION"
tick_original_collars="${GANG_COLLARS:-}"

# The popup stores this word in a tmux option before `sh -c` reads it. Keep
# the byte proof adjacent to its consumer: a dollar must be encoded without
# relying on tmux's option rewrite, while ordinary shell-significant bytes
# still arrive exactly as they started.
alert_ui_option_quote_program="$(awk '
  /^shell_quote\(\) \{/ { keep=1 }
  /^gang_root\(\) \{/ { exit }
  keep { print }
' "$GANG")"
alert_ui_option_quote_program+=$'\nencoded="$(option_shell_quote "$OPTION_QUOTE_FIXTURE")" || exit 1\nOPTION_QUOTE_ENCODED="$encoded" sh -c \'sh -c "printf %s \\"${OPTION_QUOTE_ENCODED}\\""\'\n'
alert_ui_option_quote_fixture="path \$ quote ' backtick \` hash # slash \\"
alert_ui_option_quote_result="$(OPTION_QUOTE_FIXTURE="$alert_ui_option_quote_fixture" \
  bash -c "$alert_ui_option_quote_program")"
equal "the popup option word round-trips shell-significant bytes" \
  "$alert_ui_option_quote_fixture" "$alert_ui_option_quote_result"

# THE ALERT CENTER GETS ITS OWN TMUX SERVER. Key tables are server-global, so a
# binding-conflict fixture on the substrate server would rewrite configuration
# owned by the other integration parts. This exact private socket contains one
# Gangline team and one inert survivor that keeps the server readable after
# `down`, allowing the uninstall result itself to be observed.
alert_ui_root="$RUN_ROOT/alert-ui-server"
alert_ui_injected="/var/tmp/gangline-alert-injected-$$"
alert_ui_session="quote'\`# \$(touch $alert_ui_injected)'"
alert_ui_survivor="gang-alert-ui-survivor-$$"
alert_ui_observer="gang-alert-ui-observer-$$"
mkdir -p "$alert_ui_root"
alert_ui_tmux() { TMUX_TMPDIR="$alert_ui_root" tmux "$@"; }
alert_ui_gang_for() {
  local session="$1"
  shift
  TMUX_TMPDIR="$alert_ui_root" GANG_SESSION="$session" \
    GANG_LOCK_DIR="${ALERT_UI_LOCK_DIR:-$RUN_ROOT/alert-ui-locks}" \
    GANG_ARCHIVE_DIR="$RUN_ROOT/alert-ui-archive" \
    XDG_STATE_HOME="$RUN_ROOT/alert-ui-state" "$GANG" "$@"
}
alert_ui_gang() { alert_ui_gang_for "$alert_ui_session" "$@"; }

alert_ui_tmux new-session -d -s "$alert_ui_session" -n caller \
  "PS1='❯ ' exec bash --norc"
alert_ui_tmux new-session -d -s "$alert_ui_survivor" -n survivor \
  "PS1='❯ ' exec bash --norc"
alert_ui_tmux new-session -d -s "$alert_ui_observer" -n observer \
  "PS1='❯ ' exec bash --norc"
alert_ui_gang adopt caller -c bash >/dev/null
alert_ui_caller_id="$(alert_ui_tmux list-windows -t "=$alert_ui_session" \
  -F '#{window_id} #{@gl_agent}' | awk '$2 == "caller" { print $1 }')"
equal "the alert-center fixture has one readiness-proven adopted window" \
  caller "$(alert_ui_tmux show-options -wqv -t "$alert_ui_caller_id" @gl_agent)"

# Prefix+A belongs to the operator until Gangline proves it is free. The first
# pass must still install the status widget while recording the key conflict.
alert_ui_tmux set-option -t "=$alert_ui_session:" status-right \
  'operator-left operator-right'
alert_ui_tmux bind-key -T prefix A display-message operator-A
alert_ui_gang tick >/dev/null
alert_ui_user_binding="$(alert_ui_tmux list-keys -T prefix A)"
contains "alert-center install preserves a pre-existing Prefix+A binding" \
  "$alert_ui_user_binding" "display-message operator-A"
contains "the preserved key conflict remains inspectable" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" \
    @gl_alert_binding_conflict)" "left it unchanged"

# The binding claim locks the existing socket directory descriptor so callers
# with different Gangline lock roots still agree. The former guard-file checks
# protected against following a planted link; absence is the stronger contract
# now that the claim creates no path at all. The crossed claim fixtures below
# retain the evidence that this artifact-free lock still serializes callers.
alert_ui_socket="$(alert_ui_tmux display-message -p \
  -t "=$alert_ui_session" '#{socket_path}')"
alert_ui_binding_root="$alert_ui_socket.gangline-locks"
equal "the server-global binding claim creates no filesystem artifact" absent \
  "$([ ! -e "$alert_ui_binding_root" ] && [ ! -L "$alert_ui_binding_root" ] \
      && [ ! -e "$alert_ui_socket.gangline-alert-binding.guard" ] \
      && [ ! -L "$alert_ui_socket.gangline-alert-binding.guard" ] \
      && printf absent || printf present)"
alert_ui_gang tick >/dev/null

# Once the operator frees the proposal, the next ordinary pass owns it.
alert_ui_tmux unbind-key -T prefix A
alert_ui_gang tick >/dev/null
equal "the installed popup records its current binding version" 2 \
  "$(alert_ui_tmux show-options -gqv @gl_alert_binding_version)"

# A released server already records the broken 2.8.0 key as Gangline-owned.
# Recreate that exact upgrade state: a cooperative pass must replace its own
# obsolete key while the existing foreign-binding guard above remains intact.
alert_ui_tmux bind-key -T prefix A display-popup -E -w 80% -h 70% \
  "GANG_SESSION=#{q:session_name} #{@gl_alert_command}"
alert_ui_legacy_binding="$(alert_ui_tmux list-keys -T prefix A)"
alert_ui_tmux set-option -g @gl_alert_binding "$alert_ui_legacy_binding"
alert_ui_tmux set-option -gu @gl_alert_binding_version
alert_ui_gang tick >/dev/null
alert_ui_binding="$(alert_ui_tmux list-keys -T prefix A)"
excludes "an upgrade replaces the exact still-owned 2.8.0 popup" \
  "$alert_ui_binding" '#{q:session_name}'
equal "the upgraded popup records its current binding version" 2 \
  "$(alert_ui_tmux show-options -gqv @gl_alert_binding_version)"
# tmux 3.4 quotes percent-bearing arguments when list-keys renders them, while
# 3.2a prints the same popup dimensions bare. This display-only normalization
# leaves the raw command below to prove the session-scoped invocation.
alert_ui_binding_shape="${alert_ui_binding//\"/}"
contains "the free Prefix+A key opens a tmux-native popup" \
  "$alert_ui_binding_shape" "display-popup -E -h 70% -w 80%"
contains "the popup resolves its client session's recorded alert command" \
  "$alert_ui_binding" '@gl_alert_command'
contains "the popup executes only the resolved session command" \
  "$alert_ui_binding" 'sh -c'
excludes "the popup command is independent of tmux format expansion" \
  "$alert_ui_binding" '#{'
alert_ui_installed_command="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_session:" @gl_alert_command)"
contains "the session-local popup command contains the complete invocation" \
  "$alert_ui_installed_command" "alerts --open"
contains "the session-local command pins its shell-quoted session" \
  "$alert_ui_installed_command" "GANG_SESSION="
excludes "the session-local popup command leaves no dollar for tmux to rewrite" \
  "$alert_ui_installed_command" '$'
alert_ui_right_once="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_session:" status-right)"
alert_ui_gang tick >/dev/null
alert_ui_right_twice="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_session:" status-right)"
equal "reinstalling the alert center is status-right idempotent" \
  "$alert_ui_right_once" "$alert_ui_right_twice"
equal "a repeat pass records no conflict on its own key" "" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_binding_conflict)"
equal "the installed status line contains one owned widget reference" 1 \
  "$([[ "$alert_ui_right_twice" == *'#{E:@gl_alert_widget}'* \
       && "$alert_ui_right_twice" != *'#{E:@gl_alert_widget}'*'#{E:@gl_alert_widget}'* ]] \
      && printf 1 || printf other)"
excludes "the status widget spawns no command on tmux repaints" \
  "$alert_ui_right_twice" '#('

# Simulate an upgrade from the old UI with both its owned status command and
# its marked window still present. A retained failed health record is the old
# active condition; migration must not manufacture a new transition from it.
alert_ui_digest="$(python3 -c \
  'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$alert_ui_socket" "$alert_ui_session")"
alert_ui_health="$RUN_ROOT/alert-ui-state/gangline/tick/$alert_ui_digest/health"
printf 'failed\t100\tlegacy tick failure\n' > "$alert_ui_health"
alert_ui_legacy_segment="#('/stale/snapshot/gang-tick-health.sh' '/stale/health')"
alert_ui_tmux set-option -t "=$alert_ui_session:" status-right \
  "operator-left $alert_ui_legacy_segment operator-right"
alert_ui_tmux set-option -t "=$alert_ui_session:" \
  @gl_tick_health_segment "$alert_ui_legacy_segment"
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_status_segment
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_seen
alert_ui_legacy_id="$(alert_ui_tmux new-window -d -P -F '#{window_id}' \
  -t "=$alert_ui_session" -n gangline-alerts "exec bash --norc")"
alert_ui_tmux set-option -w -t "$alert_ui_legacy_id" @gl_tick_alerts 1
alert_ui_tmux set-option -w -t "$alert_ui_legacy_id" monitor-activity on
alert_ui_tmux set-option -w -t "$alert_ui_legacy_id" monitor-bell on
alert_ui_selected_before="$(alert_ui_tmux display-message -p \
  -t "=$alert_ui_session:" '#{window_id}')"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
alert_ui_legacy_rc=0
alert_ui_gang tick >/dev/null 2>&1 || alert_ui_legacy_rc=$?
equal "the retained legacy failure remains an active failing pass" 1 \
  "$alert_ui_legacy_rc"
equal "upgrade removes the exact marked legacy alert window" absent \
  "$(if alert_ui_tmux list-windows -a -F '#{window_id}' \
       | grep -Fx "$alert_ui_legacy_id" >/dev/null; then printf present; else printf absent; fi)"
equal "legacy-window migration does not select another normal window" \
  "$alert_ui_selected_before" \
  "$(alert_ui_tmux display-message -p -t "=$alert_ui_session:" '#{window_id}')"
alert_ui_migrated_right="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_session:" status-right)"
excludes "upgrade removes the obsolete repaint command" \
  "$alert_ui_migrated_right" '/stale/snapshot'
contains "upgrade preserves the operator's left status content" \
  "$alert_ui_migrated_right" operator-left
contains "upgrade preserves the operator's right status content" \
  "$alert_ui_migrated_right" operator-right
contains "upgrade installs the static alert widget" \
  "$alert_ui_migrated_right" '#{E:@gl_alert_widget}'
equal "legacy active state migrates as active and unseen" '1 1' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"

# Recovery is the only resolver. After it, a genuinely new transition supplies
# one short display-message, and another failing pass supplies none. The PATH
# seam logs the real tmux call without changing its result.
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
alert_ui_gang tick >/dev/null
equal "recovery clears the migrated active and unseen counts" '0 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"

alert_ui_tmux_bin="$RUN_ROOT/alert-ui-bin"
alert_ui_message_ledger="$RUN_ROOT/alert-ui-display-messages"
mkdir -p "$alert_ui_tmux_bin"
cat > "$alert_ui_tmux_bin/tmux" <<SH
#!/bin/sh
case "\$*" in
  *'gang: new alert: tick failed:'*) printf '%s\n' "\$*" >> '$alert_ui_message_ledger' ;;
esac
exec '$REAL_TMUX' "\$@"
SH
chmod +x "$alert_ui_tmux_bin/tmux"
alert_ui_selected_before="$(alert_ui_tmux display-message -p \
  -t "=$alert_ui_session:" '#{window_id}')"
alert_ui_window_count="$(alert_ui_tmux list-windows -t "=$alert_ui_session" | wc -l | tr -d ' ')"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
alert_ui_new_rc=0
PATH="$alert_ui_tmux_bin:$PATH" alert_ui_gang tick >/dev/null 2>&1 \
  || alert_ui_new_rc=$?
equal "a new failing condition fails its synchronous tick" 1 "$alert_ui_new_rc"
equal "a new transition sets active and unseen independently" '1 1' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "a new transition emits one short tmux message" 1 \
  "$(wc -l < "$alert_ui_message_ledger" | tr -d ' ')"
equal "a new alert creates no window" "$alert_ui_window_count" \
  "$(alert_ui_tmux list-windows -t "=$alert_ui_session" | wc -l | tr -d ' ')"
equal "a new alert does not change the selected window" \
  "$alert_ui_selected_before" \
  "$(alert_ui_tmux display-message -p -t "=$alert_ui_session:" '#{window_id}')"

alert_ui_row="$(alert_ui_gang alerts --porcelain)"
IFS=$'\t' read -r alert_ui_kind alert_ui_state alert_ui_visibility \
  alert_ui_at alert_ui_summary <<<"$alert_ui_row"
equal "porcelain identifies the active condition kind" tick "$alert_ui_kind"
equal "porcelain identifies unresolved lifecycle state" active "$alert_ui_state"
equal "porcelain distinguishes the unseen state" unseen "$alert_ui_visibility"
case "$alert_ui_at" in ''|*[!0-9]*) alert_ui_epoch=invalid ;; *) alert_ui_epoch=valid ;; esac
equal "porcelain carries the alert transition epoch" valid "$alert_ui_epoch"
contains "porcelain carries the failure summary" \
  "$alert_ui_summary" "missing-alert-collar"

# Opening holds the same short result guard as recovery/new-failure commits.
# The nonblocking kernel probe is immediate evidence that the seen mutation is
# serialized, rather than a timing guess about a background process.
alert_ui_open_ready="$RUN_ROOT/alert-ui-open-ready"
alert_ui_open_release="$RUN_ROOT/alert-ui-open-release"
alert_ui_open_wrong_locks="$RUN_ROOT/alert-ui-open-wrong-locks"
alert_ui_open_command="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_session:" @gl_alert_command)"
mkfifo "$alert_ui_open_ready" "$alert_ui_open_release"
TMUX_TMPDIR="$alert_ui_root" GANG_SESSION="$alert_ui_session" \
GANG_LOCK_DIR="$alert_ui_open_wrong_locks" \
GANG_TEST_ALERT_OPEN_READY_FIFO="$alert_ui_open_ready" \
GANG_TEST_ALERT_OPEN_RELEASE_FIFO="$alert_ui_open_release" \
  sh -c "$alert_ui_open_command" \
  > "$RUN_ROOT/alert-ui-open.out" &
alert_ui_open_pid=$!
IFS= read -r -N 1 _ < "$alert_ui_open_ready"
alert_ui_result_guard="$RUN_ROOT/alert-ui-locks/tick/$alert_ui_digest.result.guard"
exec {alert_ui_result_probe_fd}>>"$alert_ui_result_guard"
alert_ui_result_probe_rc=0
"$ROOT/libexec/gang-process-identity" --lock-fd \
  "$alert_ui_result_probe_fd" >/dev/null 2>&1 || alert_ui_result_probe_rc=$?
equal "opening owns the result transition guard before marking seen" \
  75 "$alert_ui_result_probe_rc"
exec {alert_ui_result_probe_fd}>&-
equal "the installed popup command overrides an unrelated ambient lock root" \
  absent \
  "$([ ! -e "$alert_ui_open_wrong_locks/tick/$alert_ui_digest.result.guard" ] \
      && [ ! -L "$alert_ui_open_wrong_locks/tick/$alert_ui_digest.result.guard" ] \
      && printf absent || printf present)"
equal "an in-flight open has not resolved or prematurely hidden the alert" '1 1' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
printf '\n' > "$alert_ui_open_release"
alert_ui_open_rc=0
wait "$alert_ui_open_pid" || alert_ui_open_rc=$?
equal "the serialized alert open completes" 0 "$alert_ui_open_rc"
equal "opening marks the alert seen without resolving it" '1 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
contains "opening leaves the failed health condition standing" \
  "$(<"$alert_ui_health")" $'failed\t'
alert_ui_seen_row="$(alert_ui_gang alerts --porcelain)"
IFS=$'\t' read -r _ _ alert_ui_seen_visibility _ _ <<<"$alert_ui_seen_row"
equal "the structured list reports an opened alert as seen" \
  seen "$alert_ui_seen_visibility"

# The installed key has to cross tmux's client command queue, its popup pty,
# and a second shell before the alert command can run. Executing the recorded
# command directly above does not cover that path: tmux 3.2a passes a popup's
# shell-command literally, so a format token in that argument becomes a shell
# comment and produces an empty popup that exits zero.
#
# script supplies a real attached client. Its input is a pipe kept open by this
# shell. Every synchronization point is a tmux event: client-attached proves the
# pty reached the server, the instrumented session option reports that the
# installed binding resolved it, and its final event fires only after the alert
# command returns. No delay, pane scrape, or polling stands in for completion.
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_seen
alert_ui_attached_reset_rc=0
PATH="$alert_ui_tmux_bin:$PATH" alert_ui_gang tick >/dev/null 2>&1 \
  || alert_ui_attached_reset_rc=$?
equal "the attached-client proof starts from an unresolved unseen alert" \
  '1 1 1' \
  "$alert_ui_attached_reset_rc $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"

alert_ui_client_input="$RUN_ROOT/alert-ui-client-input"
alert_ui_client_output="$RUN_ROOT/alert-ui-client-output"
alert_ui_client_status="$RUN_ROOT/alert-ui-client-status"
alert_ui_client_attached="alert-ui-client-attached-$$"
alert_ui_popup_ready="alert-ui-popup-ready-$$"
alert_ui_popup_release="alert-ui-popup-release-$$"
alert_ui_popup_render_ready="alert-ui-popup-render-ready-$$"
alert_ui_client_done="alert-ui-client-done-$$"
alert_ui_client_exited="alert-ui-client-exited-$$"
alert_ui_wrong_session_ledger="$RUN_ROOT/alert-ui-wrong-session-ran"
alert_ui_client_path=":$PATH:"
alert_ui_client_path="${alert_ui_client_path//":$RUN_ROOT/bin:"/:}"
alert_ui_client_path="${alert_ui_client_path#:}"
alert_ui_client_path="${alert_ui_client_path%:}"
printf -v alert_ui_client_target '%q' "=$alert_ui_session"
equal "the attached client excludes the suite's fake instant commands" absent \
  "$([[ ":$alert_ui_client_path:" == *":$RUN_ROOT/bin:"* ]] \
      && printf present || printf absent)"
mkfifo "$alert_ui_client_input"
exec 8<>"$alert_ui_client_input"
alert_ui_tmux set-hook -g client-attached \
  "wait-for -S $alert_ui_client_attached"
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_client_attached" &
alert_ui_client_attached_waiter=$!
(
  alert_ui_client_rc=0
  TERM=xterm PATH="$alert_ui_client_path" script -qefc \
    "stty rows 24 cols 80; TMUX_TMPDIR='$alert_ui_root' tmux attach-session -t $alert_ui_client_target" \
    /dev/null < "$alert_ui_client_input" > "$alert_ui_client_output" 2>&1 \
    || alert_ui_client_rc=$?
  printf '%s\n' "$alert_ui_client_rc" > "$alert_ui_client_status"
  TMUX_TMPDIR="$alert_ui_root" tmux wait-for -S "$alert_ui_client_exited"
  exit "$alert_ui_client_rc"
) &
alert_ui_client_pid=$!
wait "$alert_ui_client_attached_waiter"
alert_ui_tmux set-hook -gu client-attached
alert_ui_client_rows="$(alert_ui_tmux list-clients \
  -F '#{session_name} #{@gl_agent} #{client_flags} #{client_width}x#{client_height}')"
contains "the popup proof uses a real attached client with its own terminal" \
  "$alert_ui_client_rows" "$alert_ui_session caller attached"

# Bracket the session-local command that the product binding resolves with tmux
# events. The server-global key remains byte-for-byte installed code: ready
# proves its popup shell is alive before input is sent, and done can fire only
# after alerts --open has rendered and returned. The tmux server inherited the
# suite's bounded wait-for shim, so missing events fail loudly at its ceiling.
alert_ui_tmux set-option -t "=$alert_ui_observer:" @gl_alert_command \
  "printf wrong-session > '$alert_ui_wrong_session_ledger'; tmux wait-for -S $alert_ui_popup_ready; tmux wait-for $alert_ui_popup_release; tmux wait-for -S $alert_ui_client_done"
alert_ui_tmux set-option -t "=$alert_ui_session:" @gl_alert_command \
  "tmux wait-for -S $alert_ui_popup_ready; tmux wait-for $alert_ui_popup_release; GANG_TEST_ALERT_RENDER_READY_EVENT='$alert_ui_popup_render_ready' $alert_ui_installed_command; tmux wait-for -S $alert_ui_client_done"
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_popup_ready" &
alert_ui_popup_ready_waiter=$!
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_client_done" &
alert_ui_client_done_waiter=$!
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_popup_render_ready" &
alert_ui_popup_render_ready_waiter=$!
printf '\002A' >&8
alert_ui_popup_ready_rc=0
wait "$alert_ui_popup_ready_waiter" || alert_ui_popup_ready_rc=$?
equal "Prefix+A starts the installed popup shell on the attached client" \
  0 "$alert_ui_popup_ready_rc"
alert_ui_tmux wait-for -S "$alert_ui_popup_release"
alert_ui_popup_render_ready_rc=0
wait "$alert_ui_popup_render_ready_waiter" || alert_ui_popup_render_ready_rc=$?
equal "Prefix+A reaches the popup's rendered close gate before its close key" \
  0 "$alert_ui_popup_render_ready_rc"
printf 'x' >&8
alert_ui_client_done_rc=0
wait "$alert_ui_client_done_waiter" || alert_ui_client_done_rc=$?
alert_ui_tmux set-option -t "=$alert_ui_session:" @gl_alert_command \
  "$alert_ui_installed_command"
alert_ui_tmux set-option -u -t "=$alert_ui_observer:" @gl_alert_command
equal "the attached popup runs its session-local alert command to completion" \
  0 "$alert_ui_client_done_rc"
equal "the attached popup does not resolve another session's command" absent \
  "$([ ! -e "$alert_ui_wrong_session_ledger" ] \
      && printf absent || printf present)"
equal "Prefix+A on the attached client marks the active alert seen" '1 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"

TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_client_exited" &
alert_ui_client_exited_waiter=$!
printf '\002d' >&8
wait "$alert_ui_client_exited_waiter"
alert_ui_client_rc="$(<"$alert_ui_client_status")"
wait "$alert_ui_client_pid" || true
exec 8>&-
equal "the attached popup client detaches cleanly" 0 "$alert_ui_client_rc"
contains "the attached client renders the active alert inside the popup" \
  "$(<"$alert_ui_client_output")" "1 active alert (seen)"
contains "the attached popup stays open for its documented close key" \
  "$(<"$alert_ui_client_output")" "Press any key to close."
equal "the real popup executes none of the hostile session name" absent \
  "$([ ! -e "$alert_ui_injected" ] && printf absent || printf present)"
rm -f -- "$alert_ui_injected"

# An unrelated session has no recorded command. Drive the installed popup from
# a second real client and append only an inside-the-popup completion event to
# its already asserted shell command. Even with an executable named alerts
# on the server's PATH, the empty option must remain a no-op.
alert_ui_unrelated_ledger="$RUN_ROOT/alert-ui-unrelated-alerts-ran"
cat > "$RUN_ROOT/bin/alerts" <<SH
#!/bin/sh
printf called > '$alert_ui_unrelated_ledger'
SH
chmod +x "$RUN_ROOT/bin/alerts"
alert_ui_unrelated_input="$RUN_ROOT/alert-ui-unrelated-client-input"
alert_ui_unrelated_output="$RUN_ROOT/alert-ui-unrelated-client-output"
alert_ui_unrelated_status="$RUN_ROOT/alert-ui-unrelated-client-status"
alert_ui_unrelated_attached="alert-ui-unrelated-attached-$$"
alert_ui_unrelated_done="alert-ui-unrelated-done-$$"
alert_ui_unrelated_exited="alert-ui-unrelated-exited-$$"
printf -v alert_ui_unrelated_target '%q' "=$alert_ui_observer"
mkfifo "$alert_ui_unrelated_input"
exec 9<>"$alert_ui_unrelated_input"
alert_ui_tmux set-hook -g client-attached \
  "wait-for -S $alert_ui_unrelated_attached"
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_unrelated_attached" &
alert_ui_unrelated_attached_waiter=$!
(
  alert_ui_unrelated_rc=0
  TERM=xterm PATH="$alert_ui_client_path" script -qefc \
    "stty rows 24 cols 80; TMUX_TMPDIR='$alert_ui_root' tmux attach-session -t $alert_ui_unrelated_target" \
    /dev/null < "$alert_ui_unrelated_input" \
    > "$alert_ui_unrelated_output" 2>&1 \
    || alert_ui_unrelated_rc=$?
  printf '%s\n' "$alert_ui_unrelated_rc" > "$alert_ui_unrelated_status"
  TMUX_TMPDIR="$alert_ui_root" tmux wait-for -S "$alert_ui_unrelated_exited"
  exit "$alert_ui_unrelated_rc"
) &
alert_ui_unrelated_pid=$!
wait "$alert_ui_unrelated_attached_waiter"
alert_ui_tmux set-hook -gu client-attached
# The completion event rides inside the popup's own shell argument, and that
# argument cannot be recovered from list-keys: tmux 3.4 renders the product's
# $command as \\$command, which its parser reads back as one backslash and an
# expansion of the empty variable, so re-sourcing that rendering installs a
# popup that never evaluates the option. This fixture carries its own copy of
# the popup shell and binds it through argv, which no tmux version expands.
# Binding the copy unchanged first proves it byte-for-byte against the
# installed product key on whichever tmux runs the suite.
alert_ui_popup_shell='command="$(tmux show-options -qv -t "" @gl_alert_command 2>/dev/null)" || command=; [ -z "$command" ] || sh -c "$command"'
alert_ui_popup_bind() {
  alert_ui_tmux bind-key -T prefix A display-popup -E -w 80% -h 70% "$1"
}
alert_ui_popup_bind "$alert_ui_popup_shell"
equal "the fixture's popup shell is the installed product binding" \
  "$alert_ui_binding" "$(alert_ui_tmux list-keys -T prefix A)"
alert_ui_popup_bind "$alert_ui_popup_shell; tmux wait-for -S $alert_ui_unrelated_done"
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_unrelated_done" &
alert_ui_unrelated_done_waiter=$!
printf '\002A' >&9
wait "$alert_ui_unrelated_done_waiter"
alert_ui_popup_bind "$alert_ui_popup_shell"
alert_ui_restored_binding="$(alert_ui_tmux list-keys -T prefix A)"
equal "temporary popup instrumentation restores the exact owned binding" \
  "$alert_ui_binding" "$alert_ui_restored_binding"
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_unrelated_exited" &
alert_ui_unrelated_exited_waiter=$!
printf '\002d' >&9
wait "$alert_ui_unrelated_exited_waiter"
alert_ui_unrelated_rc="$(<"$alert_ui_unrelated_status")"
wait "$alert_ui_unrelated_pid" || true
exec 9>&-
rm -f -- "$RUN_ROOT/bin/alerts"
equal "the unrelated popup client detaches cleanly" 0 \
  "$alert_ui_unrelated_rc"
equal "the installed popup executes no PATH fallback in an unrelated session" \
  absent \
  "$([ ! -e "$alert_ui_unrelated_ledger" ] \
      && printf absent || printf present)"

alert_ui_repeat_rc=0
PATH="$alert_ui_tmux_bin:$PATH" alert_ui_gang tick >/dev/null 2>&1 \
  || alert_ui_repeat_rc=$?
equal "an unresolved repeat remains a failing tick" 1 "$alert_ui_repeat_rc"
equal "a repeat does not make a seen active alert unseen again" '1 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "a repeat failure emits no additional tmux message" 1 \
  "$(wc -l < "$alert_ui_message_ledger" | tr -d ' ')"

# Missing or malformed producer state is unknown, never recovery. Preserve the
# last tmux counts and require the inspectable command to fail loudly until the
# producer writes a valid record again.
alert_ui_health_saved="$RUN_ROOT/alert-ui-health-saved"
mv -- "$alert_ui_health" "$alert_ui_health_saved"
alert_ui_missing_rc=0
alert_ui_gang alerts > "$RUN_ROOT/alert-ui-missing.out" 2>&1 \
  || alert_ui_missing_rc=$?
equal "missing health cannot manufacture alert recovery" 1 \
  "$alert_ui_missing_rc"
contains "missing active health names the refused false recovery" \
  "$(<"$RUN_ROOT/alert-ui-missing.out")" "refusing to report recovery"
equal "missing health preserves the last active and seen counts" '1 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
mv -- "$alert_ui_health_saved" "$alert_ui_health"
cp -- "$alert_ui_health" "$alert_ui_health_saved"
printf 'not-a-health-record\n' > "$alert_ui_health"
alert_ui_malformed_rc=0
alert_ui_gang alerts > "$RUN_ROOT/alert-ui-malformed.out" 2>&1 \
  || alert_ui_malformed_rc=$?
equal "malformed health cannot manufacture alert recovery" 1 \
  "$alert_ui_malformed_rc"
contains "malformed health is named as unreadable state" \
  "$(<"$RUN_ROOT/alert-ui-malformed.out")" "unreadable or malformed"
equal "malformed health preserves the last active and seen counts" '1 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
printf 'ok\t123\t\t\n' > "$alert_ui_health"
alert_ui_empty_ticket_rc=0
alert_ui_gang alerts > "$RUN_ROOT/alert-ui-empty-ticket.out" 2>&1 \
  || alert_ui_empty_ticket_rc=$?
equal "an explicitly empty ordered ticket cannot manufacture recovery" \
  1 "$alert_ui_empty_ticket_rc"
contains "an empty ordered ticket is named as malformed health" \
  "$(<"$RUN_ROOT/alert-ui-empty-ticket.out")" "unreadable or malformed"
printf 'ok\t123\t\t999\njunk\n' > "$alert_ui_health"
alert_ui_multiline_health_rc=0
alert_ui_gang alerts > "$RUN_ROOT/alert-ui-multiline-health.out" 2>&1 \
  || alert_ui_multiline_health_rc=$?
equal "trailing health records cannot hide behind a clean prefix" \
  1 "$alert_ui_multiline_health_rc"
contains "a trailing health record is named as malformed health" \
  "$(<"$RUN_ROOT/alert-ui-multiline-health.out")" "unreadable or malformed"
printf 'ok\t123\t\t999\0\n' > "$alert_ui_health"
alert_ui_nul_health_rc=0
alert_ui_gang alerts > "$RUN_ROOT/alert-ui-nul-health.out" 2>&1 \
  || alert_ui_nul_health_rc=$?
equal "a NUL-corrupted clean record cannot manufacture recovery" \
  1 "$alert_ui_nul_health_rc"
contains "NUL-corrupted health is named as malformed" \
  "$(<"$RUN_ROOT/alert-ui-nul-health.out")" "unreadable or malformed"

# Corruption does not erase the last trustworthy active lifecycle. A failing
# producer repairs its record, but must neither reopen the seen transition nor
# emit a duplicate new-alert message.
alert_ui_corrupt_repeat_rc=0
PATH="$alert_ui_tmux_bin:$PATH" alert_ui_gang tick >/dev/null 2>&1 \
  || alert_ui_corrupt_repeat_rc=$?
equal "a failed tick repairs corrupt active health as a failure" \
  1 "$alert_ui_corrupt_repeat_rc"
equal "repairing corrupt active health preserves its seen lifecycle" '1 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "repairing corrupt active health emits no duplicate transition" 1 \
  "$(wc -l < "$alert_ui_message_ledger" | tr -d ' ')"
contains "the corrupt active record is repaired to inspectable failure" \
  "$(<"$alert_ui_health")" $'failed\t'
rm -f -- "$alert_ui_health_saved"

alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
alert_ui_gang tick >/dev/null
equal "a clean pass resolves the active alert" '0 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "the resolved alert disappears from the structured list" 0 \
  "$(alert_ui_gang alerts --porcelain | wc -l | tr -d ' ')"

alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
PATH="$alert_ui_tmux_bin:$PATH" alert_ui_gang tick >/dev/null 2>&1 || true
equal "failure after recovery is a new unseen transition" '1 1' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "the post-recovery transition emits exactly one new message" 2 \
  "$(wc -l < "$alert_ui_message_ledger" | tr -d ' ')"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
alert_ui_gang tick >/dev/null

# A pass owns the tick lock through its health commit. Hold a failing pass at
# that exact seam, request another pass after repairing the condition, and
# require the same owner to consume the dirty edge before its result returns.
alert_ui_commit_ready="$RUN_ROOT/alert-ui-commit-ready"
alert_ui_commit_release="$RUN_ROOT/alert-ui-commit-release"
alert_ui_commit_ledger="$RUN_ROOT/alert-ui-commit-ledger"
mkfifo "$alert_ui_commit_ready" "$alert_ui_commit_release"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
GANG_TEST_TICK_COMMIT_READY_FIFO="$alert_ui_commit_ready" \
GANG_TEST_TICK_COMMIT_RELEASE_FIFO="$alert_ui_commit_release" \
GANG_TEST_TICK_LEDGER="$alert_ui_commit_ledger" \
  alert_ui_gang tick > "$RUN_ROOT/alert-ui-commit-owner.out" 2>&1 &
alert_ui_commit_owner=$!
IFS= read -r -N 1 _ < "$alert_ui_commit_ready"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
alert_ui_cross_rc=0
alert_ui_gang tick >/dev/null 2>&1 || alert_ui_cross_rc=$?
equal "a contender cannot pass while the older health result is uncommitted" \
  0 "$alert_ui_cross_rc"
printf '\n' > "$alert_ui_commit_release"
alert_ui_commit_owner_rc=0
wait "$alert_ui_commit_owner" || alert_ui_commit_owner_rc=$?
equal "the committing owner reruns the crossed recovery before returning" \
  0 "$alert_ui_commit_owner_rc"
equal "the serialized health owner consumes one dirty rerun" '1 2 ' \
  "$(tr '\n' ' ' < "$alert_ui_commit_ledger")"
equal "the newer recovery is the final alert state" '0 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
contains "the newer recovery is the final health record" \
  "$(<"$alert_ui_health")" $'ok\t'

# Deadline/controller failures return after their worker is gone. Hold the
# older parent at its failure commit, let a later clean invocation commit, then
# release the old parent: its lower result ticket must not overwrite recovery.
alert_ui_bad_clock="$RUN_ROOT/alert-ui-bad-clock"
cat > "$alert_ui_bad_clock" <<'SH'
#!/bin/sh
case "${1:-}" in
  now) printf '1\n'; exit 0 ;;
  elapsed) exit 2 ;;
  *) exit 2 ;;
esac
SH
chmod +x "$alert_ui_bad_clock"
alert_ui_parent_ready="$RUN_ROOT/alert-ui-parent-ready"
alert_ui_parent_release="$RUN_ROOT/alert-ui-parent-release"
mkfifo "$alert_ui_parent_ready" "$alert_ui_parent_release"
GANG_TEST_CLOCK="$alert_ui_bad_clock" \
GANG_TEST_TICK_PARENT_COMMIT_READY_FIFO="$alert_ui_parent_ready" \
GANG_TEST_TICK_PARENT_COMMIT_RELEASE_FIFO="$alert_ui_parent_release" \
  alert_ui_gang tick > "$RUN_ROOT/alert-ui-parent-failure.out" 2>&1 &
alert_ui_parent_owner=$!
IFS= read -r -N 1 _ < "$alert_ui_parent_ready"
alert_ui_gang tick >/dev/null
printf '\n' > "$alert_ui_parent_release"
alert_ui_parent_rc=0
wait "$alert_ui_parent_owner" || alert_ui_parent_rc=$?
equal "the older controller failure still returns its own failure" \
  1 "$alert_ui_parent_rc"
contains "the older controller failure retains its diagnostic" \
  "$(<"$RUN_ROOT/alert-ui-parent-failure.out")" \
  "cannot compare the shared monotonic deadline"
equal "an older controller failure cannot overwrite newer alert recovery" '0 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
contains "an older controller failure cannot overwrite newer health" \
  "$(<"$alert_ui_health")" $'ok\t'

# The binding table is server-global. Hold the first team's last-team snapshot
# while a second team tries to configure itself; the binding claim must keep
# that second team unconfigured until the first teardown finishes, after which
# one retry installs a live binding rather than leaving a configured orphan.
alert_ui_gang_for "$alert_ui_survivor" adopt survivor -c bash >/dev/null
alert_ui_binding_ready="$RUN_ROOT/alert-ui-binding-ready"
alert_ui_binding_release="$RUN_ROOT/alert-ui-binding-release"
mkfifo "$alert_ui_binding_ready" "$alert_ui_binding_release"
GANG_TEST_ALERT_BINDING_READY_FIFO="$alert_ui_binding_ready" \
GANG_TEST_ALERT_BINDING_RELEASE_FIFO="$alert_ui_binding_release" \
  alert_ui_gang down "$alert_ui_session" \
  > "$RUN_ROOT/alert-ui-down.out" 2>&1 &
alert_ui_down_owner=$!
IFS= read -r -N 1 _ < "$alert_ui_binding_ready"
alert_ui_cross_binding_rc=0
ALERT_UI_LOCK_DIR="$RUN_ROOT/alert-ui-other-locks" \
  alert_ui_gang_for "$alert_ui_survivor" tick >/dev/null 2>&1 \
  || alert_ui_cross_binding_rc=$?
equal "a team cannot configure across another team's binding teardown" \
  1 "$alert_ui_cross_binding_rc"
equal "the losing team publishes no command behind the teardown snapshot" "" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_command)"
equal "binding contention still surfaces its committed active alert" '1 1' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_unseen)"
contains "binding contention keeps the static status widget visible" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" status-right)" \
  '#{E:@gl_alert_widget}'
printf '\n' > "$alert_ui_binding_release"
alert_ui_down_rc=0
wait "$alert_ui_down_owner" || alert_ui_down_rc=$?
equal "the serialized first-team teardown completes" 0 "$alert_ui_down_rc"
alert_ui_gang_for "$alert_ui_survivor" tick >/dev/null
alert_ui_survivor_binding="$(alert_ui_tmux list-keys -T prefix A)"
alert_ui_survivor_command="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_command)"
contains "the surviving team installs the popup after the teardown seam" \
  "${alert_ui_survivor_binding//\"/}" \
  "display-popup -E -h 70% -w 80%"
equal "the surviving team's encoded command resolves to this gang binary" \
  "$("$ROOT/bin/gang" --version)" \
  "$(sh -c "${alert_ui_survivor_command% alerts --open} --version")"
contains "the surviving team's command includes the complete alert invocation" \
  "$alert_ui_survivor_command" \
  "alerts --open"

# A teardown that loses the same server-global claim must mutate nothing. Hold
# a harmless tick inside the claim, attempt `down` from a different team lock
# root, and compare every alert-center surface before allowing the tick out.
alert_ui_claim_ready="$RUN_ROOT/alert-ui-claim-ready"
alert_ui_claim_release="$RUN_ROOT/alert-ui-claim-release"
mkfifo "$alert_ui_claim_ready" "$alert_ui_claim_release"
alert_ui_survivor_right="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" status-right)"
alert_ui_survivor_counts="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_active) $(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_unseen)"
alert_ui_survivor_command="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_command)"
GANG_TEST_ALERT_BINDING_CLAIM_READY_FIFO="$alert_ui_claim_ready" \
GANG_TEST_ALERT_BINDING_CLAIM_RELEASE_FIFO="$alert_ui_claim_release" \
  alert_ui_gang_for "$alert_ui_survivor" tick \
  > "$RUN_ROOT/alert-ui-claim-owner.out" 2>&1 &
alert_ui_claim_owner=$!
IFS= read -r -N 1 _ < "$alert_ui_claim_ready"
alert_ui_losing_down_rc=0
ALERT_UI_LOCK_DIR="$RUN_ROOT/alert-ui-third-locks" \
  alert_ui_gang_for "$alert_ui_survivor" down "$alert_ui_survivor" \
  > "$RUN_ROOT/alert-ui-losing-down.out" 2>&1 \
  || alert_ui_losing_down_rc=$?
equal "down refuses while another binding transaction owns the server" \
  1 "$alert_ui_losing_down_rc"
equal "a claim-refused down leaves its team live" present \
  "$(if alert_ui_tmux has-session -t "=$alert_ui_survivor" 2>/dev/null; then printf present; else printf absent; fi)"
equal "a claim-refused down preserves status-right byte-for-byte" \
  "$alert_ui_survivor_right" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" status-right)"
equal "a claim-refused down preserves active and unseen state" \
  "$alert_ui_survivor_counts" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_unseen)"
equal "a claim-refused down preserves its popup command" \
  "$alert_ui_survivor_command" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_command)"
printf '\n' > "$alert_ui_claim_release"
alert_ui_claim_owner_rc=0
wait "$alert_ui_claim_owner" || alert_ui_claim_owner_rc=$?
equal "the winning binding transaction completes after refused down" \
  0 "$alert_ui_claim_owner_rc"

# `down` is the alert-center uninstall path. The inert observer keeps the
# server alive so both exact binding removal and unrelated-session survival are
# immediate evidence after the last configured team leaves.
alert_ui_gang_for "$alert_ui_survivor" down "$alert_ui_survivor" >/dev/null
equal "alert-center uninstall leaves the unrelated tmux session live" present \
  "$(if alert_ui_tmux has-session -t "=$alert_ui_observer" 2>/dev/null; then printf present; else printf absent; fi)"
equal "the last Gangline team removes only its owned Prefix+A binding" absent \
  "$(if alert_ui_tmux list-keys -T prefix A >/dev/null 2>&1; then printf present; else printf absent; fi)"
equal "the last Gangline team removes its binding version marker" "" \
  "$(alert_ui_tmux show-options -gqv @gl_alert_binding_version)"
equal "the last Gangline team removes its temporary binding probe" "" \
  "$(alert_ui_tmux show-options -gqv @gl_alert_binding_probe)"
equal "alert-center teardown leaves no binding-guard filesystem state" absent \
  "$([ ! -e "$alert_ui_binding_root" ] && [ ! -L "$alert_ui_binding_root" ] \
      && [ ! -e "$alert_ui_socket.gangline-alert-binding.guard" ] \
      && [ ! -L "$alert_ui_socket.gangline-alert-binding.guard" ] \
      && printf absent || printf present)"
alert_ui_tmux kill-session -t "=$alert_ui_observer"
unset -f alert_ui_tmux alert_ui_gang alert_ui_gang_for

export GANG_SESSION="gangtick-test-$$"
export GANG_TICK_DEADLINE_SECONDS=60

tick_monotonic_ns() {
  "$ROOT/libexec/gang-clock" now
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

# The ownership guard protects only lock-metadata transactions. It must be
# closed before the worker enters its cooperative pass, or any subprocess that
# outlives the worker can inherit the flock and wedge every later contender.
# The first pass command opens the same guard independently; status zero means
# no inherited open description still owns it.
tick_guard_probe_bin="$RUN_ROOT/tick-guard-probe-bin"
tick_guard_probe="$RUN_ROOT/tick-guard-probe"
tick_guard_path="${tick_lock_path%.lock}.guard"
tick_dirty_path="${tick_lock_path%.lock}.dirty"
mkdir -p "$tick_guard_probe_bin"
{
  printf '#!/usr/bin/env bash\n'
  printf 'REAL=%q\n' "$(command -v rm)"
  printf 'HELPER=%q\n' "$ROOT/libexec/gang-process-identity"
  printf 'GUARD=%q\n' "$tick_guard_path"
  printf 'DIRTY=%q\n' "$tick_dirty_path"
  printf 'PROBE=%q\n' "$tick_guard_probe"
  cat <<'SH'
if [ "${1:-}" = -f ] && [ "${2:-}" = -- ] && [ "${3:-}" = "$DIRTY" ] \
   && [ ! -e "$PROBE" ]; then
  exec 9>"$GUARD"
  rc=0
  "$HELPER" --lock-fd 9 >/dev/null 2>&1 || rc=$?
  printf '%s\n' "$rc" > "$PROBE"
  exec 9>&-
fi
exec "$REAL" "$@"
SH
} > "$tick_guard_probe_bin/rm"
chmod +x "$tick_guard_probe_bin/rm"
tick_guard_probe_rc=0
GANG_TICK_INTERNAL=1 PATH="$tick_guard_probe_bin:$PATH" \
  "$GANG" __tick-worker > "$RUN_ROOT/tick-guard-probe.out" 2>&1 \
  || tick_guard_probe_rc=$?
equal "the guard probe worker completes its cooperative pass" 0 \
  "$tick_guard_probe_rc"
equal "a cooperative-pass subprocess inherits no tick ownership guard" 0 \
  "$(<"$tick_guard_probe")"

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

# A REAL KILLED GENERATION IS RECLAIMABLE. Resolve the exact process and its
# kernel generation before killing it, wait for that generation to disappear,
# then plant its v2 record at the real team lock path. Retirement is authorized
# only after the helper reports that exact generation dead.
tick_dead_ready="$RUN_ROOT/tick-dead-ready"
mkfifo "$tick_dead_ready"
setsid python3 - "$tick_dead_ready" <<'PY' &
import signal
import sys

with open(sys.argv[1], "w", encoding="utf-8") as ready:
    ready.write("x")
signal.pause()
PY
tick_dead_pid=$!
IFS= read -r -N 1 _ < "$tick_dead_ready"
tick_dead_identity="$("$ROOT/libexec/gang-process-identity" \
  --tick "$tick_dead_pid" "$GANG_SESSION")"
IFS=$'\t' read -r _ tick_dead_token tick_dead_pgrp _ _ _ _ tick_dead_namespace \
  <<<"$tick_dead_identity"
tick_dead_command="$(ps -o command= -p "$tick_dead_pid")"
contains "the killed-owner fixture resolves its unique process before signalling" \
  "$tick_dead_command" "$tick_dead_ready"
kill -KILL "$tick_dead_pid"
wait "$tick_dead_pid" 2>/dev/null || true
tick_dead_state=0
"$ROOT/libexec/gang-process-identity" --tick "$tick_dead_pid" "$GANG_SESSION" \
  "$tick_dead_namespace" "$tick_dead_token" \
  >/dev/null 2>&1 || tick_dead_state=$?
equal "the killed-owner fixture proves the exact generation is dead" 1 \
  "$tick_dead_state"
ln -s "v3:$tick_dead_pid:$tick_dead_token:$tick_dead_pgrp:1:$tick_dead_namespace" \
  "$tick_lock_path"
tmux set-option -t "=$GANG_SESSION:" status-right \
  "operator-left #('/stale/snapshot/gang-tick-health.sh' '/stale/health') operator-right"
tmux set-option -u -t "=$GANG_SESSION:" @gl_tick_health_segment
tick_dead_reclaim_rc=0
"$GANG" tick >/dev/null 2>&1 || tick_dead_reclaim_rc=$?
equal "a real dead generation is accepted as reclaimable evidence" \
  0 "$tick_dead_reclaim_rc"
equal "a real dead generation in the team tick lock is reclaimed" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"

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
IFS=$'\t' read -r _ tick_shell_token tick_shell_pgrp _ _ _ _ tick_shell_namespace \
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

ln -s "v3:$$:0:$tick_shell_pgrp:$(date +%s):$tick_shell_namespace" \
  "$tick_lock_path"
"$GANG" tick >/dev/null
equal "a generation mismatch is reclaimed without signalling the reused pid" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"

# A RECORD IS ONE LINE. A line-oriented read would take a valid first line and
# never see what followed it; the whole target is judged, not its prefix.
tick_multiline_record="$(printf 'v3:%s:%s:%s:%s:%s\nreplacement' \
  "$$" "$tick_shell_token" "$tick_shell_pgrp" "$(tick_monotonic_ns)" "$(cut -f 8 <<<"$tick_shell_identity")")"
ln -s "$tick_multiline_record" "$tick_lock_path"
tick_multiline_rc=0
GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
  > "$RUN_ROOT/tick-multiline.out" 2>&1 || tick_multiline_rc=$?
equal "a lock record with a second line is a loud lock fault" 1 "$tick_multiline_rc"
contains "the multi-line record is named as an unreadable owner" \
  "$(<"$RUN_ROOT/tick-multiline.out")" "has an unreadable owner"
equal "the multi-line record is retained unchanged" "$tick_multiline_record" \
  "$(readlink "$tick_lock_path")"
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"

# A PID ANOTHER USER OWNS IS NOT AN UNKNOWN OWNER. Every tick worker runs as
# this user and proved its own /proc readable when it acquired, so a recorded
# pid whose /proc now belongs to someone else was reused and cannot be a live
# owner. Such a lock is retired, and the worker says why: it is not the
# fail-closed "unreadable process identity" verdict, which stays for evidence
# that genuinely cannot be read.
tick_foreign_pid=""
for tick_candidate in /proc/1 /proc/2 /proc/[0-9]*; do
  tick_candidate="${tick_candidate#/proc/}"
  tick_candidate_uid="$(awk '/^Uid:/ { print $2 }' "/proc/$tick_candidate/status" 2>/dev/null)"
  [ -n "$tick_candidate_uid" ] || continue
  [ "$tick_candidate_uid" != "$(id -u)" ] || continue
  tick_foreign_pid="$tick_candidate"
  break
done
if [ -z "$tick_foreign_pid" ]; then
  unknown "a lock naming another user's pid is retired rather than retained" \
    "every process visible here belongs to this user, so no foreign pid exists to plant"
else
  tick_foreign_helper_rc=0
  "$ROOT/libexec/gang-process-identity" --tick "$tick_foreign_pid" "$GANG_SESSION" \
    >/dev/null 2>&1 || tick_foreign_helper_rc=$?
  equal "the identity helper classes another user's pid as foreign, not unknown" \
    3 "$tick_foreign_helper_rc"
  ln -s "$tick_foreign_pid" "$tick_lock_path"
  tick_foreign_legacy_rc=0
  GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
    > "$RUN_ROOT/tick-foreign-legacy.out" 2>&1 || tick_foreign_legacy_rc=$?
  equal "a legacy lock naming another user's pid is retired and the pass runs" \
    0 "$tick_foreign_legacy_rc"
  contains "the retired owner is named as another user's process" \
    "$(<"$RUN_ROOT/tick-foreign-legacy.out")" \
    "belongs to another user and cannot be a tick worker for this team; the stale lock was retired"
  excludes "another user's pid is not reported as unreadable identity" \
    "$(<"$RUN_ROOT/tick-foreign-legacy.out")" "unreadable process identity"
  equal "the legacy foreign-owner lock is gone after the pass" absent \
    "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
  tick_foreign_token="$(sed 's/^.*) //' "/proc/$tick_foreign_pid/stat" | awk '{ print $20 }')"
  ln -s "v2:$tick_foreign_pid:$tick_foreign_token:$tick_foreign_pid:1" "$tick_lock_path"
  tick_foreign_v2_rc=0
  "$GANG" tick >/dev/null 2>&1 || tick_foreign_v2_rc=$?
  equal "a v2 lock naming another user's live generation is reclaimed" 0 "$tick_foreign_v2_rc"
  equal "the v2 foreign-owner lock is gone after the pass" absent \
    "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
fi

# A PID IS ONLY A NAME INSIDE ITS PID NAMESPACE. A worker in a sandbox with its
# own pid table records the number it sees; on the host that number belongs to
# an unrelated process. The record carries the owner's namespace, and a
# contender in the initial namespace resolves it through /proc: a live owner is
# ordinary contention, an owner that died with its sandbox is reclaimed because
# no process in that namespace remains, and a contender inside a namespace that
# cannot see the owner retains the lock and names the namespace it cannot see.
tick_host_namespace="$(cut -f 8 <<<"$tick_shell_identity")"
tick_reader_at_root=0
[ "$tick_host_namespace" != "$((0xEFFFFFFC))" ] || tick_reader_at_root=1
tick_namespace_run() { # a fresh pid table under this same uid, as a harness sandbox is
  unshare --kill-child=SIGKILL -U --map-user="$(id -u)" --map-group="$(id -g)" \
    -pf --mount-proc "$@"
}
tick_namespace_host_pid() { # $1 namespace inode, $2 pid inside it -> the host pid, or nothing
  local status="" inner="" candidate=""
  for status in /proc/[0-9]*/status; do
    inner="$(awk '/^NSpid:/ { if (NF > 2) print $NF }' "$status" 2>/dev/null)" || continue
    [ "$inner" = "$2" ] || continue
    candidate="${status%/status}"
    [ "$(readlink "$candidate/ns/pid" 2>/dev/null)" = "pid:[$1]" ] || continue
    printf '%s' "${candidate#/proc/}"
    return 0
  done
  return 1
}
if ! tick_namespace_run true 2>/dev/null; then
  unknown "a tick owner in a child pid namespace is resolved by namespace" \
    "unprivileged pid namespaces are unavailable here (unshare -U --map-user -pf --mount-proc)"
else
  tick_ns_ready="$RUN_ROOT/tick-ns-ready"
  mkfifo "$tick_ns_ready"
  # The stand-in owner becomes a session leader before publishing the identity
  # it sees of itself, so every field the record carries is settled. It wears
  # the tick-worker role (argv tail and environment) so the reclaim path can
  # be driven to its signal against it.
  tick_namespace_run env GANG_TICK_INTERNAL=1 python3 - "$ROOT/libexec/gang-process-identity" \
    "$GANG_SESSION" "$tick_ns_ready" __tick-worker <<'PY' &
import os
import signal
import subprocess
import sys

os.setsid()
with open(sys.argv[3], "w", encoding="utf-8") as ready:
    subprocess.run(
        [sys.argv[1], "--tick", str(os.getpid()), sys.argv[2]],
        stdout=ready,
        check=True,
    )
signal.pause()
PY
  tick_ns_launcher=$!
  IFS=$'\t' read -r tick_ns_inner tick_ns_token tick_ns_pgrp _ _ tick_ns_role _ tick_ns_namespace \
    < "$tick_ns_ready"
  # Located through the host's own view rather than the job's process tree,
  # which differs between an exec'd and a forked background launcher.
  tick_ns_owner="$(tick_namespace_host_pid "$tick_ns_namespace" "$tick_ns_inner")" || tick_ns_owner=""
  equal "the host finds exactly the stand-in owner behind the recorded namespace pid" python3 \
    "$(cat "/proc/${tick_ns_owner:-0}/comm" 2>/dev/null)"
  equal "the stand-in owner presents as a tick worker of this team" 1 "$tick_ns_role"
  equal "the stand-in owner sees itself in a namespace other than the host's" different \
    "$([ -n "$tick_ns_namespace" ] && [ "$tick_ns_namespace" != "$tick_host_namespace" ] && printf different || printf same)"
  equal "the host resolves the namespaced owner to the identity it sees of itself" \
    "$tick_ns_inner	$tick_ns_token	$tick_ns_pgrp" \
    "$("$ROOT/libexec/gang-process-identity" --tick "$tick_ns_inner" "$GANG_SESSION" \
        "$tick_ns_namespace" 2>/dev/null | cut -f 1-3)"
  ln -s "v3:$tick_ns_inner:$tick_ns_token:$tick_ns_pgrp:$(tick_monotonic_ns):$tick_ns_namespace" \
    "$tick_lock_path"
  tick_ns_live_rc=0
  GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
    > "$RUN_ROOT/tick-ns-live.out" 2>&1 || tick_ns_live_rc=$?
  equal "a live owner in a child pid namespace is contention, not a steal" 75 "$tick_ns_live_rc"
  equal "the live namespaced owner keeps its lock" present \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  equal "the live namespaced owner is marked dirty like any live owner" present \
    "$([ -e "${tick_lock_path%.lock}.dirty" ] && printf present || printf absent)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
  # An older worker inside the same kind of namespace wrote v2 records with no
  # namespace at all. Read against the host table its pid is another user's
  # process, or another generation; the start token still identifies it among
  # the namespaces the host can see, so a live owner is contention, not a
  # retired stale lock.
  ln -s "v2:$tick_ns_inner:$tick_ns_token:$tick_ns_pgrp:$(tick_monotonic_ns)" \
    "$tick_lock_path"
  tick_ns_v2_rc=0
  GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
    > "$RUN_ROOT/tick-ns-v2.out" 2>&1 || tick_ns_v2_rc=$?
  equal "a live v2 owner recorded from inside a child pid namespace is contention" \
    75 "$tick_ns_v2_rc"
  equal "the live namespaced v2 owner keeps its lock" present \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
  # Past the reclaim edge the same v2 owner must be killable: the pidfd is
  # opened on the host pid the token resolved, and the record re-read from
  # that pid must be judged in the owner's namespace, not the host's, or the
  # expired worker is reported dead while it lives on unkillable.
  tick_ns_kill_rc=0
  "$ROOT/libexec/gang-process-identity" --kill "$tick_ns_inner" "$tick_ns_token" \
    "$GANG_SESSION" "$ROOT/libexec/gang-clock" >/dev/null 2>&1 || tick_ns_kill_rc=$?
  equal "an expired v2 owner located through a child namespace is signalled and confirmed dead" \
    0 "$tick_ns_kill_rc"
  equal "the signalled namespaced owner is gone from the host table" absent \
    "$([ -d "/proc/${tick_ns_owner:-0}" ] && printf present || printf absent)"
  ln -s "v3:$tick_ns_inner:$tick_ns_token:$tick_ns_pgrp:$(tick_monotonic_ns):$tick_ns_namespace" \
    "$tick_lock_path"
  # The observed production shape: the sandbox dies and takes its worker with
  # it, leaving the namespace pid in the shared lock. Prove the exact
  # generation gone the way the contender will, then reclaim.
  kill -KILL "$tick_ns_owner" "$tick_ns_launcher" 2>/dev/null || true
  wait "$tick_ns_launcher" 2>/dev/null || true
  tick_ns_dead_rc=0
  "$ROOT/libexec/gang-process-identity" --tick "$tick_ns_inner" "$GANG_SESSION" \
    "$tick_ns_namespace" >/dev/null 2>&1 || tick_ns_dead_rc=$?
  tick_ns_reclaim_rc=0
  "$GANG" tick > "$RUN_ROOT/tick-ns-dead.out" 2>&1 || tick_ns_reclaim_rc=$?
  if [ "$tick_reader_at_root" -eq 1 ]; then
    equal "no process remains in the dead namespace, which the host reads as death" \
      1 "$tick_ns_dead_rc"
    equal "an owner that died with its pid namespace is reclaimed by a host contender" \
      0 "$tick_ns_reclaim_rc"
    equal "the dead namespaced owner's lock is gone after the pass" absent \
      "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
  else
    unknown "no process remains in the dead namespace, which the host reads as death" \
      "this contender is itself outside the initial PID namespace"
    unknown "an owner that died with its pid namespace is reclaimed by a host contender" \
      "this contender is itself outside the initial PID namespace"
    unknown "the dead namespaced owner's lock is gone after the pass" \
      "this contender is itself outside the initial PID namespace"
    equal "a non-initial reader cannot call an unseen namespace owner dead" \
      4 "$tick_ns_dead_rc"
    equal "a non-initial contender fails closed on the unseen namespace" \
      1 "$tick_ns_reclaim_rc"
    contains "the non-initial contender names the namespace it cannot see" \
      "$(<"$RUN_ROOT/tick-ns-dead.out")" \
      "in pid namespace $tick_ns_namespace, which this process cannot see; lock was retained"
    equal "a non-initial contender retains the unseen namespace owner's lock" present \
      "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  fi
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"

  # The other direction: a contender inside a fresh pid table sees none of the
  # host's processes, so a host-owned lock must be retained, not read as dead.
  ln -s "v3:$$:$tick_shell_token:$tick_shell_pgrp:$(tick_monotonic_ns):$tick_host_namespace" \
    "$tick_lock_path"
  tick_ns_blind_rc=0
  tick_namespace_run env GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
    > "$RUN_ROOT/tick-ns-blind.out" 2>&1 || tick_ns_blind_rc=$?
  equal "a contender that cannot see the owner's namespace fails closed" 1 "$tick_ns_blind_rc"
  contains "the blind contender names the namespace it cannot see" \
    "$(<"$RUN_ROOT/tick-ns-blind.out")" \
    "in pid namespace $tick_host_namespace, which this process cannot see; lock was retained"
  equal "the host-owned lock survives a blind contender" present \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
fi

# ABSENCE IS DEATH ONLY FROM A COMPLETE /proc. Self showing its own pid proves
# which table procfs presents, not that every process of this uid is listed:
# hidepid=ptraceable filters same-uid entries, and something other than procfs
# at /proc enumerates nothing. Read through such a view, a namespace with no
# visible process stays unresolvable rather than reclaimed.
tick_proc_view_root="$RUN_ROOT/tick-proc-view"
mkdir -p "$tick_proc_view_root"
printf '26 31 0:24 / /proc rw,nosuid,nodev,noexec,relatime shared:13 - proc proc rw\n' \
  > "$tick_proc_view_root/plain"
printf '26 31 0:24 / /proc rw,nosuid,nodev,noexec,relatime shared:13 - proc proc rw,hidepid=invisible\n' \
  > "$tick_proc_view_root/invisible"
printf '26 31 0:24 / /proc rw,nosuid,nodev,noexec,relatime,hidepid=2 shared:13 - proc proc rw\n' \
  > "$tick_proc_view_root/invisible-mount"
printf '26 31 0:24 / /proc rw,nosuid,nodev,noexec,relatime shared:13 - proc proc rw,hidepid=4\n' \
  > "$tick_proc_view_root/ptraceable"
printf '26 31 0:24 / /proc rw,nosuid,nodev,noexec,relatime shared:13 - proc proc rw\n80 26 0:50 / /proc rw,relatime - tmpfs none rw\n' \
  > "$tick_proc_view_root/overmounted"
printf '26 31 0:24 / /sys rw,nosuid,nodev,noexec,relatime shared:13 - sysfs sysfs rw\n' \
  > "$tick_proc_view_root/missing"
tick_proc_view_probe="$(python3 - "$ROOT/libexec/gang-process-identity" \
  "$tick_proc_view_root" 2>/dev/null <<'PY'
import os
import runpy
import sys

scope = runpy.run_path(sys.argv[1], run_name="gang_tick_proc_view_probe")
runtime = scope["main"].__globals__
complete = runtime["proc_view_complete"]
verdicts = [
    name
    for name in ("plain", "invisible", "invisible-mount", "ptraceable", "overmounted", "missing")
    if complete(os.path.join(sys.argv[2], name))
]
verdicts.append("absent" if not complete(os.path.join(sys.argv[2], "no-such-file")) else "read")

# A reader at the initial namespace whose view is incomplete: a namespace
# holding no visible process must not read as dead.
runtime["proc_view_complete"] = lambda mountinfo="": False
runtime["own_pid_namespace"] = lambda: runtime["INIT_PID_NAMESPACE"]
try:
    runtime["linux_locate"](os.getpid(), expected_namespace=1)
    verdicts.append("resolved")
except runtime["UnresolvableProcess"]:
    verdicts.append("unresolvable")
except runtime["DeadProcess"]:
    verdicts.append("dead")
runtime["proc_view_complete"] = lambda mountinfo="": True
try:
    runtime["linux_locate"](os.getpid(), expected_namespace=1)
    verdicts.append("resolved")
except runtime["UnresolvableProcess"]:
    verdicts.append("unresolvable")
except runtime["DeadProcess"]:
    verdicts.append("dead")
print(",".join(verdicts))
PY
)"
equal "only procfs without a same-uid hidepid filter counts as a complete view, and an unreadable mount table does not" \
  "plain,invisible,invisible-mount,absent,unresolvable,dead" "$tick_proc_view_probe"

# THE KERNEL GUARD SERIALIZES RETIREMENT. The interpreter shim blocks one
# contender after it holds the per-team flock and has read the stale record.
# A second real worker must return contention without claiming or deleting the
# same lock, which must remain unchanged until the first transaction completes.
tick_reclaim_guard_race() { # $1 suffix, $2 planted record, $3 observed pid
  local suffix="$1" planted="$2" observed_pid="$3"
  local race_bin="$RUN_ROOT/tick-retire-$suffix-bin"
  local identity_ready="$RUN_ROOT/tick-retire-$suffix-identity-ready"
  local identity_release="$RUN_ROOT/tick-retire-$suffix-identity-release"
  local wrapper_once="$RUN_ROOT/tick-retire-$suffix-once"
  local first_pid first_rc=0 second_rc=0 ledger="$RUN_ROOT/tick-retire-$suffix-ledger"
  mkdir -p "$race_bin"
  mkfifo "$identity_ready" "$identity_release"
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
  first_pid=$!
  IFS= read -r -N 1 _ < "$identity_ready"
  GANG_TEST_TICK_LEDGER="$ledger" GANG_TICK_INTERNAL=1 \
    "$GANG" __tick-worker > "$RUN_ROOT/tick-retire-$suffix-second.out" 2>&1 \
    || second_rc=$?
  equal "$suffix retirement guard rejects a simultaneous second owner" \
    75 "$second_rc"
  equal "$suffix retirement guard leaves the observed metadata unchanged" \
    "$planted" "$(readlink "$tick_lock_path" 2>/dev/null || true)"
  equal "$suffix retirement guard prevents the second worker pass" absent \
    "$([ -e "$ledger" ] && printf present || printf absent)"
  printf '\n' > "$identity_release"
  wait "$first_pid" || first_rc=$?
  equal "$suffix guarded retirement lets the first contender finish" 0 "$first_rc"
  equal "$suffix guarded retirement releases its lock" absent \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
}

tick_reclaim_guard_race dead-owner \
  "v3:$tick_dead_pid:$tick_dead_token:$tick_dead_pgrp:1:$tick_dead_namespace" "$tick_dead_pid"
tick_reclaim_guard_race replaced-generation \
  "v3:$$:0:$tick_shell_pgrp:1:$tick_shell_namespace" "$$"

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
# shellcheck disable=SC2016
tick_identity_shape_probe missing-helper-field '
if [ "${1:-}" = "$HELPER" ] && [ "${2:-}" = --tick ] && [ "${3:-}" = "$TARGET" ]; then
  out="$("$REAL" "$@")" || exit $?
  TAB="$(printf "\t")"
  printf "%s\n" "${out%"$TAB"*}"
  exit 0
fi
exec "$REAL" "$@"'

# THE DEADLINE IS AN INTERNAL CONTROLLER CONTRACT, NOT ARITHMETIC INPUT. The
# public controller always publishes 60; noncanonical, invalid-octal, and
# overflowing direct-worker values must fail before they can age a live lock.
tick_bad_budget_probe() { # $1 value
  local value="$1" rc=0 output="$RUN_ROOT/tick-budget-$1.out"
  ln -s "v2:$$:$tick_shell_token:$tick_shell_pgrp:1" "$tick_lock_path"
  GANG_TICK_DEADLINE_SECONDS="$value" GANG_TICK_INTERNAL=1 \
    "$GANG" __tick-worker > "$output" 2>&1 || rc=$?
  equal "deadline value $value is rejected before lock arithmetic" 1 "$rc"
  contains "deadline value $value names the fixed production contract" \
    "$(<"$output")" "production 60-second deadline budget"
  equal "deadline value $value retains the observed live lock" present \
    "$([ -L "$tick_lock_path" ] && printf present || printf absent)"
  rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
}
tick_bad_budget_probe 060
tick_bad_budget_probe 08
tick_bad_budget_probe 13836000000

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
  tick_bystander_session _ tick_bystander_role tick_bystander_safe _ \
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
contains "the shell authorization gate emits its own refusal before the helper" \
  "$(<"$RUN_ROOT/tick-bystander-contender.out")" \
  "cannot be reclaimed safely: exact leader generation, tick-worker role, or session leadership is unknown"
tick_bystander_helper_rc=0
"$ROOT/libexec/gang-process-identity" --kill "$tick_bystander" \
  "$tick_bystander_token" "$GANG_SESSION" "$ROOT/libexec/gang-clock" \
  >/dev/null 2>&1 \
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
IFS=: read -r _ tick_token_worker tick_token_value _ _ tick_token_namespace \
  <<<"$tick_token_record"
tick_wrong_token_rc=0
"$ROOT/libexec/gang-process-identity" --kill "$tick_token_worker" \
  "${tick_token_value}x" "$GANG_SESSION" "$ROOT/libexec/gang-clock" \
  "$tick_token_namespace" \
  >/dev/null 2>&1 \
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
tick_clock_helper="$RUN_ROOT/tick-monotonic-clock"
mkdir -p "$tick_clock_bin"
mkfifo "$tick_clock_ready" "$tick_clock_release"
GANG_TEST_TICK_READY_FIFO="$tick_clock_ready" \
GANG_TEST_TICK_RELEASE_FIFO="$tick_clock_release" \
  "$GANG" tick > "$RUN_ROOT/tick-clock-owner.out" 2>&1 &
tick_clock_owner=$!
IFS= read -r -N 1 _ < "$tick_clock_ready"
tick_clock_record="$(readlink "$tick_lock_path")"
IFS=: read -r _ tick_clock_worker _ _ tick_clock_acquired _ \
  <<<"$tick_clock_record"
cat > "$tick_clock_helper" <<'SH'
#!/bin/sh
now="${GANG_TEST_CLOCK_NOW_NS:?}"
[ -z "${GANG_TEST_CLOCK_WITNESS:-}" ] \
  || printf '%s\n' "$now" >> "$GANG_TEST_CLOCK_WITNESS"
case "${1:-}" in
  now) [ "$#" -eq 1 ] || exit 2; printf '%s\n' "$now" ;;
  elapsed)
    [ "$#" -eq 3 ] || exit 2
    [ "$now" -ge "$2" ] || exit 2
    [ $(( now - $2 )) -ge "$3" ]
    ;;
  *) exit 2 ;;
esac
SH
chmod +x "$tick_clock_helper"
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
  GANG_TEST_CLOCK="$tick_clock_helper" \
  GANG_TEST_CLOCK_WITNESS="$RUN_ROOT/tick-clock-witness" \
  GANG_TEST_CLOCK_NOW_NS=$(( tick_clock_acquired + 30000000000 )) \
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
contains "the lock decision reads the suite-only monotonic seam" \
  "$(<"$RUN_ROOT/tick-clock-witness")" \
  "$(( tick_clock_acquired + 30000000000 ))"

if [ "$tick_clock_worker_state" -eq 0 ]; then
  tick_clock_edge_rc=0
  GANG_TICK_INTERNAL=1 GANG_TEST_CLOCK="$tick_clock_helper" \
    GANG_TEST_CLOCK_NOW_NS=$(( tick_clock_acquired + 61000000000 )) \
    "$GANG" __tick-worker \
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
  tick_expired_pgrp _ tick_expired_namespace <<<"$tick_expired_record"
equal "the expiry fixture owns a generation-bearing production lock" v3 \
  "$tick_expired_version"
equal "the expiry fixture worker is its own process-group leader" \
  "$tick_expired_pid" "$tick_expired_pgrp"
tick_expired_old=$(( $(tick_monotonic_ns) - 120000000000 ))
rm -f -- "$tick_lock_path"
ln -s "v3:$tick_expired_pid:$tick_expired_token:$tick_expired_pgrp:$tick_expired_old:$tick_expired_namespace" \
  "$tick_lock_path"
tick_expired_rc=0
"$GANG" tick > "$RUN_ROOT/tick-expired-reclaim.out" 2>&1 \
  || tick_expired_rc=$?
equal "an expired exact worker generation is reclaimed as a surfaced failure" \
  1 "$tick_expired_rc"
equal "pidfd-confirmed expiry retires the worker lock" absent \
  "$([ ! -e "$tick_lock_path" ] && [ ! -L "$tick_lock_path" ] && printf absent || printf present)"
tick_expired_generation_state=0
"$ROOT/libexec/gang-process-identity" \
  --tick "$tick_expired_pid" "$GANG_SESSION" \
  "$tick_expired_namespace" "$tick_expired_token" >/dev/null 2>&1 \
  || tick_expired_generation_state=$?
equal "expiry leaves the real owner generation dead" \
  1 "$tick_expired_generation_state"
# Once expiry signals the owner, this branch is unreachable. If signalling does
# not occur, restore the exact owner record and release its FIFO so the owned
# worker exits and the later tick can retire the stale lock.
if [ "$tick_expired_generation_state" -eq 0 ]; then
  equal "an owner surviving expiry retains its exact record" \
    "v3:$tick_expired_pid:$tick_expired_token:$tick_expired_pgrp:$tick_expired_old:$tick_expired_namespace" \
    "$(readlink "$tick_lock_path" 2>/dev/null || true)"
  rm -f -- "$tick_lock_path"
  ln -s "$tick_expired_record" "$tick_lock_path"
  printf '\n' > "$tick_expired_release"
fi
tick_expired_owner_rc=0
wait "$tick_expired_owner" || tick_expired_owner_rc=$?
equal "the expired worker owner reports the surfaced recovery failure" \
  1 "$tick_expired_owner_rc"
rm -f -- "$tick_lock_path" "${tick_lock_path%.lock}.dirty"
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
  tick_controller_token tick_controller_pgrp _ tick_controller_namespace \
  <<<"$tick_controller_record"
tick_controller_identity="$("$ROOT/libexec/gang-process-identity" \
  --tick "$tick_controller_worker" "$GANG_SESSION" \
  "$tick_controller_namespace" "$tick_controller_token")"
IFS=$'\t' read -r _ _ _ tick_controller_session _ tick_controller_role _ \
  <<<"$tick_controller_identity"
tick_controller_pid="$(ps -o ppid= -p "$tick_controller_worker" | tr -d ' ')"
tick_controller_command="$(ps -o command= -p "$tick_controller_pid")"
equal "the controller-death fixture owns a generation-bearing lock" v3 \
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
  --tick "$tick_controller_worker" "$GANG_SESSION" \
  "$tick_controller_namespace" "$tick_controller_token" >/dev/null 2>&1 \
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
contains "status repair installs the static alert widget" \
  "$tick_repaired_right" '#{E:@gl_alert_widget}'
excludes "status repair removes every tick command from repaint" \
  "$tick_repaired_right" '#('
contains "the deadline controller fixes the production budget at sixty seconds" \
  "$(<"$ROOT/libexec/gang-tick-deadline")" "DEADLINE_SECONDS = 60"
excludes "the deadline controller ignores an ambient clock executable" \
  "$(<"$ROOT/libexec/gang-tick-deadline")" "GANGLINE_CLOCK_HELPER"
excludes "the generation killer ignores an ambient clock executable" \
  "$(<"$ROOT/libexec/gang-process-identity")" "GANGLINE_CLOCK_HELPER"

tick_death_bound_probe="$(python3 - "$ROOT/libexec/gang-process-identity" \
  "$ROOT/libexec/gang-clock" <<'PY'
import runpy
import sys

scope = runpy.run_path(sys.argv[1], run_name="gang_process_identity_probe")
kill_generation = scope["kill_generation"]
runtime = kill_generation.__globals__
poll_calls = []
signals = []


class OnePoll:
    def register(self, _fd, _events):
        pass

    def poll(self, _milliseconds):
        poll_calls.append(1)
        if len(poll_calls) > 1:
            raise AssertionError("death confirmation polled more than once")
        return []


runtime["os"].pidfd_open = lambda pid: opened_pids.append(pid) or 99
runtime["os"].close = lambda _fd: None
runtime["signal"].pidfd_send_signal = lambda pidfd, signum: signals.append((pidfd, signum))
runtime["select"].poll = OnePoll
located_pids = []
opened_pids = []


def fake_locate(pid, namespace=None, token=None):
    located_pids.append(pid)
    return 77, 5, True


def fake_record(pid, session, namespace=None, token=None, proc_pid=None):
    # The kill path may resolve an owner once; after that only the pid its
    # pidfd was opened on may be read, so a second resolution is a defect.
    assert proc_pid == 77, f"record read without the pidfd's pid: {proc_pid}"
    return ("python", "S", pid, pid, "token", 1, 1, 1, 5)


runtime["linux_locate"] = fake_locate
runtime["linux_record"] = fake_record
runtime["clock_now_ns"] = lambda _path: 1
runtime["clock_elapsed"] = lambda _path, _started, _duration: False
bounded = kill_generation(41, "token", "team", sys.argv[2])


def broken_clock(_path, _started, _duration):
    raise runtime["ClockError"]


runtime["clock_elapsed"] = broken_clock
clock_failed = kill_generation(41, "token", "team", sys.argv[2])
print(f"{bounded}:{len(poll_calls)}:{clock_failed}:{len(signals)}")
print(f"{len(located_pids)}:{sorted(set(opened_pids))}")
PY
)"
equal "death confirmation has one kernel fallback and names a post-signal clock fault" \
  "3:1:4:2" "$(head -1 <<<"$tick_death_bound_probe")"
equal "each kill resolves its owner once and signals only the pid its pidfd was opened on" \
  "2:[77]" "$(tail -1 <<<"$tick_death_bound_probe")"

tick_deadline_bound_probe="$(python3 - "$ROOT/libexec/gang-tick-deadline" \
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
contains "the attached-human status-right contains the static alert widget" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" status-right)" \
  '#{E:@gl_alert_widget}'
equal "failure records one active unseen alert" '1 1' \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_alert_active) $(tmux show-options -qv -t "=$GANG_SESSION:" @gl_alert_unseen)"
# The old expectation created a permanent normal window and raised activity and
# bell on it. That behavior was the focus-stealing defect: an alert transition
# now changes tmux options and one message, so any marked window is regression.
equal "failure creates no dedicated alert window" 0 \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#{@gl_tick_alerts}' | grep -c '^1$' || :)"
excludes "roster has no alert pseudo-agent to filter" \
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
equal "a clean tick resolves active and unseen alert state" '0 0' \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" @gl_alert_active) $(tmux show-options -qv -t "=$GANG_SESSION:" @gl_alert_unseen)"
# The former success-race assertions drove a one-shot alert body inside a
# disposable window. Removing that process/window path eliminates the race
# itself; these immediate native-state checks fail if either old artifact
# returns, without constructing the defective surface as their fixture.
equal "a clean tick has no alert body window to race" 0 \
  "$(tmux list-windows -t "=$GANG_SESSION" -F '#{@gl_tick_alerts}' | grep -c '^1$' || :)"
excludes "a clean tick keeps command substitution out of status repaint" \
  "$(tmux show-options -qv -t "=$GANG_SESSION:" status-right)" '#('

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

# Team teardown uninstalls the session's alert-center options and retires the
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
