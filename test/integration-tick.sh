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
alert_ui_contender="gang-alert-ui-contender-$$"
mkdir -p "$alert_ui_root"
alert_ui_tmux() { env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux "$@"; }
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
alert_ui_tmux new-session -d -s "$alert_ui_contender" -n contender \
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
contains "the roster names why the alert center has no key" \
  "$(alert_ui_gang roster)" "open key unavailable:"

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
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$REAL_TMUX' "\$0" tmux || exit \$?
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

# The alert list keeps what the tick raised and cleared, so a failure a later
# pass resolved can still be read after the fact. Porcelain stays the active
# conditions only.
alert_ui_listing="$(alert_ui_gang alerts)"
contains "the alert list carries a recent-alerts history" \
  "$alert_ui_listing" "recent alerts:"
alert_ui_history="${alert_ui_listing#*recent alerts:}"
contains "the history records the raised failure" \
  "$alert_ui_history" "raised: "
contains "the raised history row carries the failure summary" \
  "$alert_ui_history" "missing-alert-collar"
equal "porcelain lists no history rows" 1 \
  "$(alert_ui_gang alerts --porcelain | wc -l | tr -d ' ')"

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
#
# The popup is a scrolling pty: a note that wraps past its rows pushes the
# headline off its screen, and tmux repaints the popup from that screen. A
# full suite already ships enough collars for the note to overflow; a long
# collar name makes every part selection render the same overflowing note.
# Its first characters are two-byte so that a row boundary can fall inside
# one of them.
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_seen
printf -v alert_ui_e_acute '\303\251'
printf -v alert_ui_long_collar_head '%040d' 0
printf -v alert_ui_long_collar_tail '%0400d' 0
alert_ui_long_collar="missing-alert-collar-${alert_ui_long_collar_head//0/$alert_ui_e_acute}${alert_ui_long_collar_tail//0/x}"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar "$alert_ui_long_collar"
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
# The client's terminal bytes pass through a reader on their way to the
# capture file. It signals a tmux event the moment a named marker has been
# written to the file, so a wait on that event is a wait on the file's
# contents, not on time. A second program replays the capture up to a marker
# as an 80x24 terminal and prints the popup's interior rows: what the
# client's screen showed inside the popup at that point, and nothing outside
# it. It refuses any control sequence it does not model instead of guessing.
alert_ui_client_reader="$RUN_ROOT/alert-ui-capture-reader"
alert_ui_client_screen="$RUN_ROOT/alert-ui-capture-screen"
cat > "$alert_ui_client_screen" <<'PY'
import re
import sys
import unicodedata

ROWS, COLS = 24, 80
TOP, BOTTOM, LEFT, RIGHT = 4, 17, 8, 69  # popup interior, 0-based, inclusive
# tmux erases the panes before it draws the overlay, so the marker's own bytes
# land on a screen the popup has been erased from and not yet drawn back onto.
# The replay therefore runs to the end of the repaint that follows the marker,
# whose last cell is the popup's bottom-right corner.
CORNER = "┘"
# Modes that change no cell: cursor keys, cursor visibility and shape, mouse
# reporting, bracketed paste, and the application-escape mode.
NEUTRAL_MODES = {1, 12, 25, 1000, 1002, 1003, 1004, 1005, 1006, 2004, 7727}

raw = open(sys.argv[1], "rb").read()
marker = sys.argv[2].encode()
found = raw.find(marker)
cut = raw.find(CORNER.encode(), found + len(marker)) if found >= 0 else -1
if cut < 0:
    print("marker-missing")
    sys.exit(0)
unsupported = []
head = raw[:cut + len(CORNER.encode())]
try:
    text = head.decode("utf-8")
except UnicodeDecodeError as bad:
    unsupported.append("utf-8@%d" % bad.start)
    text = head.decode("utf-8", "replace")
screen = [[" "] * COLS for _ in range(ROWS)]
row = col = 0
top, bottom = 0, ROWS - 1
saved = (0, 0)
alternate = None
pending = False


def scroll_up(n):
    for _ in range(n):
        del screen[top]
        screen.insert(bottom, [" "] * COLS)


def scroll_down(n):
    for _ in range(n):
        del screen[bottom]
        screen.insert(top, [" "] * COLS)


def linefeed():
    global row, pending
    pending = False
    if row == bottom:
        scroll_up(1)
    elif row < ROWS - 1:
        row += 1


CSI = re.compile(r"\x1b\[([0-9;?>]*)([ -/]*)([@-~])")
OSC = re.compile(r"\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)")
i = 0
while i < len(text):
    ch = text[i]
    if ch == "\x1b":
        m = CSI.match(text, i)
        if m:
            params, inter, final = m.groups()
            private = params[:1] if params[:1] in "?>" else ""
            digits = params[len(private):]
            nums = [int(x) if x else 0 for x in digits.split(";")] if digits else []
            n = nums[0] if nums and nums[0] else 1
            if inter or private == ">" and final not in "cq":
                unsupported.append(m.group(0))
            elif private == ">":
                pass  # a request for the terminal's identity writes no cell
            elif private == "?":
                if final not in "hl" or not nums:
                    unsupported.append(m.group(0))
                elif nums == [1049]:
                    if final == "h":
                        alternate = ([line[:] for line in screen], row, col)
                        for r in range(ROWS):
                            screen[r] = [" "] * COLS
                    elif alternate is not None:
                        screen[:], row, col = alternate
                        alternate = None
                        pending = False
                elif not all(mode in NEUTRAL_MODES for mode in nums):
                    unsupported.append(m.group(0))
            elif final in "Hf":
                row = min(ROWS - 1, max(0, (nums[0] if nums else 1) - 1))
                col = min(COLS - 1, max(0, (nums[1] if len(nums) > 1 else 1) - 1))
                pending = False
            elif final == "A":
                row = max(0, row - n)
                pending = False
            elif final == "B":
                row = min(ROWS - 1, row + n)
                pending = False
            elif final == "C":
                col = min(COLS - 1, col + n)
                pending = False
            elif final == "D":
                col = max(0, col - n)
                pending = False
            elif final == "G":
                col = min(COLS - 1, n - 1)
                pending = False
            elif final == "d":
                row = min(ROWS - 1, n - 1)
                pending = False
            elif final == "X":
                for c in range(col, min(COLS, col + n)):
                    screen[row][c] = " "
            elif final == "K":
                mode = nums[0] if nums else 0
                rng = range(col, COLS) if mode == 0 else \
                    range(0, col + 1) if mode == 1 else range(COLS)
                for c in rng:
                    screen[row][c] = " "
            elif final == "J":
                mode = nums[0] if nums else 0
                if mode == 0:
                    for c in range(col, COLS):
                        screen[row][c] = " "
                    for r in range(row + 1, ROWS):
                        screen[r] = [" "] * COLS
                elif mode == 1:
                    for c in range(0, col + 1):
                        screen[row][c] = " "
                    for r in range(0, row):
                        screen[r] = [" "] * COLS
                else:
                    for r in range(ROWS):
                        screen[r] = [" "] * COLS
            elif final == "r":
                top = max(0, (nums[0] if nums and nums[0] else 1) - 1)
                bottom = min(ROWS - 1, (nums[1] if len(nums) > 1 and nums[1] else ROWS) - 1)
                row = col = 0
                pending = False
            elif final == "L":
                if top <= row <= bottom:
                    for _ in range(n):
                        del screen[bottom]
                        screen.insert(row, [" "] * COLS)
            elif final == "M":
                if top <= row <= bottom:
                    for _ in range(n):
                        del screen[row]
                        screen.insert(bottom, [" "] * COLS)
            elif final == "S":
                scroll_up(n)
            elif final == "T":
                scroll_down(n)
            elif final == "P":
                line = screen[row]
                del line[col:col + n]
                line.extend([" "] * (COLS - len(line)))
            elif final == "@":
                line = screen[row]
                line[col:col] = [" "] * n
                del line[COLS:]
            elif final == "m":
                pass  # colour and attributes leave both the cells and the
                # pending wrap alone
            elif final == "c":
                pass  # a request for the terminal's identity writes no cell
            elif final == "t" and nums and nums[0] in (22, 23):
                pass  # the window title stack holds no cell
            else:
                unsupported.append(m.group(0))
            i = m.end()
            continue
        m = OSC.match(text, i)
        if m:
            i = m.end()
            continue
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if nxt in "()*+":
            i += 3
        elif nxt == "7":
            saved = (row, col)
            i += 2
        elif nxt == "8":
            row, col = saved
            pending = False
            i += 2
        elif nxt in "=>":
            i += 2
        elif nxt == "M":
            if row == top:
                scroll_down(1)
            elif row > 0:
                row -= 1
            pending = False
            i += 2
        elif nxt == "D":
            linefeed()
            i += 2
        else:
            unsupported.append(text[i:i + 2])
            i += 2
        continue
    if ch == "\r":
        col = 0
        pending = False
    elif ch == "\n":
        linefeed()
    elif ch == "\b":
        col = max(0, col - 1)
        pending = False
    elif ch == "\t":
        col = min(COLS - 1, (col // 8 + 1) * 8)
        pending = False
    elif ch in "\x00\x07":
        pass
    elif ch < " " or ch == "\x7f":
        unsupported.append(ch)
    else:
        if unicodedata.category(ch) in ("Cf", "Mn", "Me") \
                or unicodedata.east_asian_width(ch) in "WF":
            unsupported.append(ch)  # this screen models one cell per character
        if pending:
            col = 0
            linefeed()
        screen[row][col] = ch
        if col == COLS - 1:
            pending = True
        else:
            col += 1
    i += 1

for r in range(TOP, BOTTOM + 1):
    print("".join(screen[r][LEFT:RIGHT + 1]).rstrip())
print("unsupported=%d%s" % (len(unsupported), "".join(" " + repr(u) for u in unsupported[:5])))
print("frame=%s" % ("marker" if sys.argv[2] in "".join(screen[ROWS - 1]) else "other"))
PY
alert_ui_popup_marker_first="alert-ui-popup-witness-$$-first"
alert_ui_popup_marker_second="alert-ui-popup-witness-$$-second"
cat > "$alert_ui_client_reader" <<'PY'
import os
import subprocess
import sys

# A marker's event fires once the capture also holds the popup repaint that
# follows it. tmux erases the panes before it draws the overlay, so the marker
# alone names a screen the popup has been erased from; the corner is the last
# cell that repaint writes.
CORNER = "┘".encode()
markers = [m.encode() for m in sys.argv[1:]]
pending = [[m, False] for m in markers]
keep = max([len(m) for m in markers] + [len(CORNER)]) - 1
tail = b""
while True:
    chunk = os.read(0, 65536)
    if not chunk:
        break
    view = memoryview(chunk)
    while len(view):
        view = view[os.write(1, view):]
    window = tail + chunk
    for entry in list(pending):
        marker, seen = entry
        start = 0
        if not seen:
            at = window.find(marker)
            if at < 0:
                continue
            entry[1] = True
            start = at + len(marker)
        if window.find(CORNER, start) < 0:
            continue
        pending.remove(entry)
        subprocess.run(
            ["tmux", "wait-for", "-S", marker.decode() + "-seen"], check=True)
    tail = window[-keep:] if keep > 0 else b""
PY
alert_ui_tmux set-hook -g client-attached \
  "wait-for -S $alert_ui_client_attached"
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_client_attached" &
alert_ui_client_attached_waiter=$!
(
  alert_ui_client_rc=0
  TERM=xterm PATH="$alert_ui_client_path" script -qefc \
    "stty rows 24 cols 80; unset TMUX; TMUX_TMPDIR='$alert_ui_root' tmux attach-session -t $alert_ui_client_target" \
    /dev/null < "$alert_ui_client_input" 2>&1 \
    | TMUX_TMPDIR="$alert_ui_root" python3 "$alert_ui_client_reader" \
        "$alert_ui_popup_marker_first" "$alert_ui_popup_marker_second" \
        > "$alert_ui_client_output" \
    || alert_ui_client_rc=$?
  printf '%s\n' "$alert_ui_client_rc" > "$alert_ui_client_status"
  env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for -S "$alert_ui_client_exited"
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
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_popup_ready" &
alert_ui_popup_ready_waiter=$!
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_client_done" &
alert_ui_client_done_waiter=$!
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_popup_render_ready" &
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
if [ "$alert_ui_popup_render_ready_rc" -ne 0 ]; then
  # This barrier is the only ceiling on the popup's wait for its terminal, so
  # it also ends the client: a popup still waiting for an answer that never
  # came consumes the close key instead of ending on it, and the checks below
  # would then never reach their own verdicts.
  alert_ui_popup_abandoned_rc=0
  alert_ui_tmux kill-session -t "=$alert_ui_session:" \
    || alert_ui_popup_abandoned_rc=$?
  equal "the abandoned popup's client is torn down rather than waited on" \
    0 "$alert_ui_popup_abandoned_rc"
fi

# The capture holds every byte tmux ever sent, including the popup's transient
# incremental writes. The rendered state is what tmux repaints the popup from,
# and render-ready fired only after tmux had consumed the whole report into
# that screen. Changing a session option redraws every attached client in
# full: panes, then status, then the overlay. The marker therefore arrives
# mid-frame, on a screen whose panes have just been erased and whose popup has
# not been drawn again yet, so both the event and the replay run on to the end
# of that repaint. What the replayed screen then shows inside the popup is the
# overlay's own repaint, drawn from the popup screen as it stands while the
# popup waits for its key.
alert_ui_tmux set-option -t "=$alert_ui_session:" status-left-length 64
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_popup_marker_first-seen" &
alert_ui_popup_marker_first_waiter=$!
alert_ui_tmux set-option -t "=$alert_ui_session:" status-left \
  "$alert_ui_popup_marker_first"
alert_ui_popup_marker_first_rc=0
wait "$alert_ui_popup_marker_first_waiter" || alert_ui_popup_marker_first_rc=$?
TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_popup_marker_second-seen" &
alert_ui_popup_marker_second_waiter=$!
alert_ui_tmux set-option -t "=$alert_ui_session:" status-left \
  "$alert_ui_popup_marker_second"
alert_ui_popup_marker_second_rc=0
wait "$alert_ui_popup_marker_second_waiter" || alert_ui_popup_marker_second_rc=$?
alert_ui_tmux set-option -u -t "=$alert_ui_session:" status-left
alert_ui_tmux set-option -u -t "=$alert_ui_session:" status-left-length
equal "the attached client repaints on each popup witness marker" '0 0' \
  "$alert_ui_popup_marker_first_rc $alert_ui_popup_marker_second_rc"
# The binding's 80% by 70% popup on the 80x24 client has a 62x14 interior;
# the report reserves six of those rows, so the note gets seven rows and its
# pointer the eighth.
alert_ui_popup_screen="$(python3 "$alert_ui_client_screen" \
  "$alert_ui_client_output" "$alert_ui_popup_marker_second")"
alert_ui_popup_row() { printf '%s\n' "$alert_ui_popup_screen" | sed -n "${1}p"; }
equal "the replayed client screen used only modelled control sequences" \
  "unsupported=0" "$(alert_ui_popup_row 15)"
equal "the replayed popup rows come from the marked repaint" \
  "frame=marker" "$(alert_ui_popup_row 16)"
equal "the attached client renders the active alert inside the popup" \
  "1 active alert (seen)" "$(alert_ui_popup_row 1)"
equal "the attached popup separates its headline from the transition" \
  "" "$(alert_ui_popup_row 2)"
contains "the attached popup states the failing transition" \
  "$(alert_ui_popup_row 3)" "[seen] cooperative tick failed at "
contains "the attached popup keeps the failure note inside its rows" \
  "$(alert_ui_popup_row 4)" "gang: unknown collar 'missing-alert-collar-"
contains "the attached popup points at the full note it cannot show" \
  "$(alert_ui_popup_row 11)" "run gang alerts for the full note)"
equal "the attached popup stays open for its documented close key" \
  "Press any key to close." "$(alert_ui_popup_row 13)"
equal "the attached popup ends on the row after its close key" \
  "" "$(alert_ui_popup_row 14)"
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

env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_client_exited" &
alert_ui_client_exited_waiter=$!
printf '\002d' >&8
wait "$alert_ui_client_exited_waiter"
alert_ui_client_rc="$(<"$alert_ui_client_status")"
wait "$alert_ui_client_pid" || true
exec 8>&-
equal "the attached popup client detaches cleanly" 0 "$alert_ui_client_rc"
equal "the real popup executes none of the hostile session name" absent \
  "$([ ! -e "$alert_ui_injected" ] && printf absent || printf present)"
rm -f -- "$alert_ui_injected"

# The same report on a terminal the popup binding did not size. Narrow
# columns wrap the fixed lines too, and a two-byte character can straddle a
# row boundary; the report still has to end inside the terminal's rows with
# its pointer whole and every character intact.
alert_ui_direct_env="TMUX_TMPDIR=$(printf '%q' "$alert_ui_root") XDG_STATE_HOME=$(printf '%q' "$RUN_ROOT/alert-ui-state") GANG_ARCHIVE_DIR=$(printf '%q' "$RUN_ROOT/alert-ui-archive")"
alert_ui_narrow_output="$RUN_ROOT/alert-ui-narrow-output"
alert_ui_narrow_rc=0
printf 'x' | TERM=xterm PATH="$alert_ui_client_path" script -qefc \
  "stty rows 16 cols 20; $alert_ui_direct_env $alert_ui_installed_command" \
  /dev/null > "$alert_ui_narrow_output" 2>&1 || alert_ui_narrow_rc=$?
equal "the open report on a narrow terminal returns cleanly" 0 "$alert_ui_narrow_rc"
alert_ui_narrow_shape="$(python3 - "$alert_ui_narrow_output" <<'PY'
import sys
raw = open(sys.argv[1], "rb").read()
try:
    text = raw.decode("utf-8")
except UnicodeDecodeError:
    print("split-character")
    sys.exit(0)
rows = 0
for line in text.split("\r\n"):
    width = sum(2 if ord(c) > 127 else 1 for c in line)
    rows += max(1, -(-width // 20))
print("fits" if rows <= 16 else "overflows by %d" % (rows - 16))
PY
)"
equal "the narrow report keeps whole characters and ends inside the terminal" \
  fits "$alert_ui_narrow_shape"
alert_ui_narrow_flat="$(tr -d '\r\n' < "$alert_ui_narrow_output")"
contains "the narrow report keeps its headline" \
  "$alert_ui_narrow_flat" "1 active alert (seen)"
contains "the narrow report keeps the note's start" \
  "$alert_ui_narrow_flat" "unknown collar 'missing-alert-collar-"
contains "the narrow report keeps its pointer whole" \
  "$alert_ui_narrow_flat" "run gang alerts for the full note)"
contains "the narrow report keeps its close key" \
  "$alert_ui_narrow_flat" "Press any key to close."

# A terminal that cannot be measured cannot bound the note, and a guessed
# size would let the note scroll the popup again. The report then carries
# only the pointer.
alert_ui_unsized_bin="$RUN_ROOT/alert-ui-unsized-bin"
mkdir -p "$alert_ui_unsized_bin"
printf '#!/bin/sh\nexit 1\n' > "$alert_ui_unsized_bin/stty"
chmod +x "$alert_ui_unsized_bin/stty"
alert_ui_unsized_output="$RUN_ROOT/alert-ui-unsized-output"
alert_ui_unsized_rc=0
printf 'x' | TERM=xterm PATH="$alert_ui_client_path" script -qefc \
  "stty rows 24 cols 80; PATH=$(printf '%q' "$alert_ui_unsized_bin"):\$PATH $alert_ui_direct_env $alert_ui_installed_command" \
  /dev/null > "$alert_ui_unsized_output" 2>&1 || alert_ui_unsized_rc=$?
equal "the open report on an unmeasurable terminal returns cleanly" 0 \
  "$alert_ui_unsized_rc"
alert_ui_unsized_flat="$(tr -d '\r\n' < "$alert_ui_unsized_output")"
contains "the unmeasured report keeps its headline" \
  "$alert_ui_unsized_flat" "1 active alert (seen)"
contains "the unmeasured report says why the note is absent" \
  "$alert_ui_unsized_flat" "(note hidden: size unknown; run gang alerts)"
excludes "the unmeasured report prints no note row it cannot bound" \
  "$alert_ui_unsized_flat" "unknown collar"
contains "the unmeasured report keeps its close key" \
  "$alert_ui_unsized_flat" "Press any key to close."

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
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_unrelated_attached" &
alert_ui_unrelated_attached_waiter=$!
(
  alert_ui_unrelated_rc=0
  TERM=xterm PATH="$alert_ui_client_path" script -qefc \
    "stty rows 24 cols 80; unset TMUX; TMUX_TMPDIR='$alert_ui_root' tmux attach-session -t $alert_ui_unrelated_target" \
    /dev/null < "$alert_ui_unrelated_input" \
    > "$alert_ui_unrelated_output" 2>&1 \
    || alert_ui_unrelated_rc=$?
  printf '%s\n' "$alert_ui_unrelated_rc" > "$alert_ui_unrelated_status"
  env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for -S "$alert_ui_unrelated_exited"
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
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_unrelated_done" &
alert_ui_unrelated_done_waiter=$!
printf '\002A' >&9
wait "$alert_ui_unrelated_done_waiter"
alert_ui_popup_bind "$alert_ui_popup_shell"
alert_ui_restored_binding="$(alert_ui_tmux list-keys -T prefix A)"
equal "temporary popup instrumentation restores the exact owned binding" \
  "$alert_ui_binding" "$alert_ui_restored_binding"
env -u TMUX TMUX_TMPDIR="$alert_ui_root" tmux wait-for "$alert_ui_unrelated_exited" &
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
# emit a duplicate new-alert message. With the prior record unreadable the
# transition is unknown: a history row would read as a second raised failure,
# so the repair appends none and marks the history incomplete instead.
alert_ui_alerts="${alert_ui_health%/health}/alerts"
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_history_lost
alert_ui_rows_before="$(wc -l < "$alert_ui_alerts" | tr -d ' ')"
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
equal "repairing unreadable health appends no history row" \
  "$alert_ui_rows_before" "$(wc -l < "$alert_ui_alerts" | tr -d ' ')"
equal "repairing unreadable health marks the history incomplete" 1 \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_history_lost)"
rm -f -- "$alert_ui_health_saved"

alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
alert_ui_gang tick >/dev/null
equal "a clean pass resolves the active alert" '0 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "the resolved alert disappears from the structured list" 0 \
  "$(alert_ui_gang alerts --porcelain | wc -l | tr -d ' ')"
alert_ui_resolved="$(alert_ui_gang alerts)"
contains "a resolved alert leaves its history readable" \
  "$alert_ui_resolved" "recent alerts:"
alert_ui_resolved_history="${alert_ui_resolved#*recent alerts:}"
contains "the history keeps the resolved failure" \
  "$alert_ui_resolved_history" "raised: "
contains "the history records the pass that cleared it" \
  "$alert_ui_resolved_history" " cleared"
contains "the roster names the alert center key with no active alert" \
  "$(alert_ui_gang roster)" "Prefix+A"

alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
PATH="$alert_ui_tmux_bin:$PATH" alert_ui_gang tick >/dev/null 2>&1 || true
equal "failure after recovery is a new unseen transition" '1 1' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
equal "the post-recovery transition emits exactly one new message" 2 \
  "$(wc -l < "$alert_ui_message_ledger" | tr -d ' ')"

# A clean pass over an unreadable prior record cannot tell a resolution from a
# pass that resolved nothing, so it too appends no row and marks the history
# incomplete. A pass refuses health it cannot read when it starts, so the
# record turns unreadable while the pass is held at its commit.
alert_ui_unread_ready="$RUN_ROOT/alert-ui-unread-ready"
alert_ui_unread_release="$RUN_ROOT/alert-ui-unread-release"
mkfifo "$alert_ui_unread_ready" "$alert_ui_unread_release"
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_history_lost
alert_ui_rows_before="$(wc -l < "$alert_ui_alerts" | tr -d ' ')"
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
GANG_TEST_TICK_COMMIT_READY_FIFO="$alert_ui_unread_ready" \
GANG_TEST_TICK_COMMIT_RELEASE_FIFO="$alert_ui_unread_release" \
  alert_ui_gang tick > "$RUN_ROOT/alert-ui-unread.out" 2>&1 &
alert_ui_unread_owner=$!
IFS= read -r -N 1 _ < "$alert_ui_unread_ready"
printf 'junk\n' > "$alert_ui_health"
printf '\n' > "$alert_ui_unread_release"
alert_ui_unread_rc=0
wait "$alert_ui_unread_owner" || alert_ui_unread_rc=$?
equal "a clean pass over health made unreadable at its commit completes" \
  0 "$alert_ui_unread_rc"
equal "a clean pass over unreadable health appends no history row" \
  "$alert_ui_rows_before" "$(wc -l < "$alert_ui_alerts" | tr -d ' ')"
equal "a clean pass over unreadable health marks the history incomplete" 1 \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_history_lost)"

# THE LISTING NAMES THE CURRENT GENERATION FIRST. A reader takes no lock, and
# the first rotation can land between its two existence checks; checked in the
# other order, neither file is seen and a journal with rows reads as "no alert
# history". The history-lost read is the one external command between the
# checks, so this shim rotates the journal there. These assertions show what a
# rotation at that read produces; they reach the window between the checks only
# while the read stays between them.
alert_ui_real_mv="$(command -v mv)"
alert_ui_rotate_bin="$RUN_ROOT/alert-ui-rotate-bin"
alert_ui_rotate_ledger="$RUN_ROOT/alert-ui-rotations"
mkdir -p "$alert_ui_rotate_bin"
cat > "$alert_ui_rotate_bin/tmux" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$REAL_TMUX' "\$0" tmux || exit \$?
case "\$*" in
  *@gl_alert_history_lost*)
    if [ -f '$alert_ui_alerts' ]; then
      '$alert_ui_real_mv' -f -- '$alert_ui_alerts' '$alert_ui_alerts.1'
      printf 'rotated\n' >> '$alert_ui_rotate_ledger'
    fi ;;
esac
exec '$REAL_TMUX' "\$@"
SH
chmod +x "$alert_ui_rotate_bin/tmux"
equal "the rotation fixture starts with one generation" absent \
  "$(if [ -e "$alert_ui_alerts.1" ]; then printf present; else printf absent; fi)"
alert_ui_rotated="$(PATH="$alert_ui_rotate_bin:$PATH" alert_ui_gang alerts)"
equal "the journal rotated at the history-lost read" 1 \
  "$(wc -l < "$alert_ui_rotate_ledger" | tr -d ' ')"
excludes "a rotation at the history-lost read cannot read as no history" \
  "$alert_ui_rotated" "no alert history"
contains "a rotation at the history-lost read reads as unknown" \
  "$alert_ui_rotated" "recent alerts: unknown"
mv -- "$alert_ui_alerts.1" "$alert_ui_alerts"

# A row with a surplus field, even an empty one, is not one this writer
# produced. It is shown as unreadable rather than as a failure whose summary
# swallowed the surplus. Tab is whitespace to read, so a split alone drops a
# trailing empty field.
cp -- "$alert_ui_alerts" "$RUN_ROOT/alert-ui-alerts-saved"
printf '123\tfailed\tnote\textra\n' > "$alert_ui_alerts"
alert_ui_surplus="$(alert_ui_gang alerts)"
contains "a history row with a surplus field is unreadable" \
  "$alert_ui_surplus" "unreadable history row"
excludes "a surplus field is not read into the failure summary" \
  "$alert_ui_surplus" "raised: note"
printf '123\tfailed\tnote\t\n' > "$alert_ui_alerts"
alert_ui_surplus="$(alert_ui_gang alerts)"
contains "a history row with an empty surplus field is unreadable" \
  "$alert_ui_surplus" "unreadable history row"
excludes "an empty surplus field does not read as a transition" \
  "$alert_ui_surplus" "raised: note"
mv -- "$RUN_ROOT/alert-ui-alerts-saved" "$alert_ui_alerts"

# ROTATION PRECEDES THE APPEND AND LANDS ONLY ON A FILE. A journal past its
# bound is rotated before the next row. A rotation that cannot land keeps every
# row in place, appends nothing and marks the history incomplete, so the bound
# holds and the gap is visible; mv onto a directory would move the journal
# inside it, where no reader looks.
alert_ui_pad_journal() {
  awk 'BEGIN { for (i = 0; i < 72000; i++) printf "%d\tok\t\n", 1000000000 + i }' \
    > "$alert_ui_alerts"
}
cp -- "$alert_ui_alerts" "$RUN_ROOT/alert-ui-alerts-saved"
alert_ui_pad_journal
alert_ui_padded="$(wc -c < "$alert_ui_alerts" | tr -d ' ')"
equal "the padded journal is past its bound" over \
  "$(if [ "$alert_ui_padded" -gt 1048576 ]; then printf over; else printf within; fi)"
mkdir -- "$alert_ui_alerts.1"
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_history_lost
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
alert_ui_dir_rotation_rc=0
alert_ui_gang tick >/dev/null 2>&1 || alert_ui_dir_rotation_rc=$?
equal "the transition over a directory generation is a failing tick" 1 \
  "$alert_ui_dir_rotation_rc"
equal "a rotation onto a directory keeps the journal in place" \
  "$alert_ui_padded" "$(wc -c < "$alert_ui_alerts" | tr -d ' ')"
equal "a rotation onto a directory moves nothing into it" "" \
  "$(ls -A -- "$alert_ui_alerts.1")"
equal "a rotation onto a directory marks the history incomplete" 1 \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_history_lost)"
rm -rf -- "$alert_ui_alerts.1"

alert_ui_mv_bin="$RUN_ROOT/alert-ui-mv-bin"
alert_ui_mv_ledger="$RUN_ROOT/alert-ui-mv-refusals"
mkdir -p "$alert_ui_mv_bin"
cat > "$alert_ui_mv_bin/mv" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$alert_ui_real_mv' "\$0" mv || exit \$?
case "\$*" in
  *alerts.1) printf '%s\n' "\$*" >> '$alert_ui_mv_ledger'; exit 1 ;;
esac
exec '$alert_ui_real_mv' "\$@"
SH
chmod +x "$alert_ui_mv_bin/mv"
alert_ui_pad_journal
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_history_lost
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar bash
PATH="$alert_ui_mv_bin:$PATH" alert_ui_gang tick >/dev/null
equal "the clean transition attempted one refused rotation" 1 \
  "$(wc -l < "$alert_ui_mv_ledger" | tr -d ' ')"
equal "a refused rotation keeps the journal at its size" \
  "$alert_ui_padded" "$(wc -c < "$alert_ui_alerts" | tr -d ' ')"
equal "a refused rotation marks the history incomplete" 1 \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_history_lost)"

# A SIZE THAT CANNOT BE READ IS A ROTATION THAT CANNOT LAND. Read as zero, a
# journal that cannot be read was never rotated, and every transition appended
# to it past its bound with no marker. It keeps its rows, gains none, and the
# history says it is incomplete.
chmod 200 -- "$alert_ui_alerts"
equal "the write-only journal cannot be read" unreadable \
  "$(if [ -r "$alert_ui_alerts" ]; then printf readable; else printf unreadable; fi)"
alert_ui_tmux set-option -u -t "=$alert_ui_session:" @gl_alert_history_lost
alert_ui_tmux set-option -w -t "$alert_ui_caller_id" @gl_collar missing-alert-collar
alert_ui_unsized_rc=0
alert_ui_gang tick >/dev/null 2>&1 || alert_ui_unsized_rc=$?
chmod 600 -- "$alert_ui_alerts"
equal "the transition over an unreadable journal is a failing tick" 1 \
  "$alert_ui_unsized_rc"
equal "a journal whose size cannot be read gains no row" \
  "$alert_ui_padded" "$(wc -c < "$alert_ui_alerts" | tr -d ' ')"
equal "a journal whose size cannot be read marks the history incomplete" 1 \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_history_lost)"
mv -- "$RUN_ROOT/alert-ui-alerts-saved" "$alert_ui_alerts"

# A PASS OWNS THE RUN LOCK THROUGH ITS HEALTH COMMIT, AND A LAUNCH DURING IT IS
# SERVED AFTERWARDS. Hold a failing pass at that exact seam, repair the
# condition, and launch: the launch queues behind the held pass instead of
# committing ahead of it, and its own pass then records the recovery. Taking
# the queue lock and then the run lock waits out the queued pass, which holds
# the first until it holds the second.
alert_ui_queue_lock="$RUN_ROOT/alert-ui-locks/tick/$alert_ui_digest.queue"
alert_ui_run_lock="$RUN_ROOT/alert-ui-locks/tick/$alert_ui_digest.run"
alert_ui_queued_wait() {
  flock "$alert_ui_queue_lock" true
  flock "$alert_ui_run_lock" true
}
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
GANG_TEST_TICK_MODE=async GANG_TEST_TICK_LEDGER="$alert_ui_commit_ledger" \
  alert_ui_gang roster >/dev/null
alert_ui_queued_rc=0
flock -n "$alert_ui_queue_lock" true || alert_ui_queued_rc=$?
equal "a launch while the older result is uncommitted queues a pass" \
  1 "$alert_ui_queued_rc"
printf '\n' > "$alert_ui_commit_release"
alert_ui_commit_owner_rc=0
wait "$alert_ui_commit_owner" || alert_ui_commit_owner_rc=$?
equal "the held pass commits its own failure" 1 "$alert_ui_commit_owner_rc"
alert_ui_queued_wait
equal "the queued launch runs one pass after the held one" '1 1 ' \
  "$(tr '\n' ' ' < "$alert_ui_commit_ledger")"
equal "the newer recovery is the final alert state" '0 0' \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_session:" @gl_alert_unseen)"
contains "the newer recovery is the final health record" \
  "$(<"$alert_ui_health")" $'ok\t'

# Deadline/controller failures return after their worker is gone. Hold the
# older parent at its failure commit and launch a later pass: it queues until
# the parent has committed, so the older failure cannot overwrite recovery.
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
GANG_TEST_TICK_MODE=async alert_ui_gang roster >/dev/null
alert_ui_parent_queued_rc=0
flock -n "$alert_ui_queue_lock" true || alert_ui_parent_queued_rc=$?
equal "a launch during an uncommitted controller failure queues a pass" \
  1 "$alert_ui_parent_queued_rc"
printf '\n' > "$alert_ui_parent_release"
alert_ui_parent_rc=0
wait "$alert_ui_parent_owner" || alert_ui_parent_rc=$?
alert_ui_queued_wait
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

# An alert-center install that fails must say why in the alert itself. A tick
# alert keeps only the last line of its pass's output, and the install's
# failure was that last line with no cause: the step that failed, and what it
# printed, appeared at most on earlier lines the alert drops. A live team whose
# health record is malformed fails the install at its widget refresh, which
# names that record as unreadable, while the rest of the pass succeeds.
alert_ui_cause="gang-alert-cause-$$"
alert_ui_tmux new-session -d -s "$alert_ui_cause" -n cause \
  "PS1='❯ ' exec bash --norc"
alert_ui_gang_for "$alert_ui_cause" adopt cause -c bash >/dev/null
alert_ui_cause_digest="$(python3 -c \
  'import hashlib,sys; print(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).hexdigest()[:24])' \
  "$alert_ui_socket" "$alert_ui_cause")"
alert_ui_cause_health="$RUN_ROOT/alert-ui-state/gangline/tick/$alert_ui_cause_digest/health"
mkdir -p "${alert_ui_cause_health%/health}"
printf 'malformed\n' > "$alert_ui_cause_health"
alert_ui_cause_rc=0
alert_ui_gang_for "$alert_ui_cause" tick > "$RUN_ROOT/alert-ui-cause.out" 2>&1 \
  || alert_ui_cause_rc=$?
equal "a tick that cannot install the alert center fails" 1 "$alert_ui_cause_rc"
alert_ui_cause_note="$(cut -f3 "$alert_ui_cause_health")"
equal "the alert names the failed install step and what that step printed" \
  "tick could not install the tmux alert center: could not refresh the alert widget (alert health is unreadable or malformed; refusing to change active alert state)" \
  "$alert_ui_cause_note"
alert_ui_tmux kill-session -t "=$alert_ui_cause"

# A successful binding claim protects the mutation after it returns, not merely
# the helper that acquired it. Pause a harmless tick at that exact seam: a
# second ordinary tick leaves the in-flight binding alone, while `down` from a
# separate team must still be refused rather than remove its own alert state.
alert_ui_post_claim_ready="$RUN_ROOT/alert-ui-post-claim-ready"
alert_ui_post_claim_release="$RUN_ROOT/alert-ui-post-claim-release"
mkfifo "$alert_ui_post_claim_ready" "$alert_ui_post_claim_release"
alert_ui_gang_for "$alert_ui_contender" adopt contender -c bash >/dev/null
alert_ui_survivor_right="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" status-right)"
alert_ui_survivor_counts="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_active) $(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_unseen)"
alert_ui_survivor_command="$(alert_ui_tmux show-options -qv \
  -t "=$alert_ui_survivor:" @gl_alert_command)"
GANG_TEST_ALERT_BINDING_POST_CLAIM_READY_FIFO="$alert_ui_post_claim_ready" \
GANG_TEST_ALERT_BINDING_POST_CLAIM_RELEASE_FIFO="$alert_ui_post_claim_release" \
  alert_ui_gang_for "$alert_ui_survivor" tick \
  > "$RUN_ROOT/alert-ui-claim-owner.out" 2>&1 &
alert_ui_claim_owner=$!
IFS= read -r -N 1 _ < "$alert_ui_post_claim_ready"
# A second ordinary tick sees the server-global claim held, but that is not a
# broken alert center: the first tick is actively installing it. The contender
# must leave the binding to that owner without publishing a false health
# failure, which is the path the recorder's status probes exercise.
alert_ui_losing_tick_rc=0
ALERT_UI_LOCK_DIR="$RUN_ROOT/alert-ui-fourth-locks" \
  alert_ui_gang_for "$alert_ui_survivor" tick \
  > "$RUN_ROOT/alert-ui-losing-tick.out" 2>&1 \
  || alert_ui_losing_tick_rc=$?
equal "a tick leaves an in-flight alert binding to its owner" \
  0 "$alert_ui_losing_tick_rc"
excludes "an in-flight alert binding is not a tick failure" \
  "$(<"$RUN_ROOT/alert-ui-losing-tick.out")" \
  "could not claim the Prefix+A binding"
excludes "an in-flight alert binding emits no terminal diagnostic" \
  "$(<"$RUN_ROOT/alert-ui-losing-tick.out")" \
  "another alert binding update is in flight"
alert_ui_losing_down_rc=0
ALERT_UI_LOCK_DIR="$RUN_ROOT/alert-ui-third-locks" \
  alert_ui_gang_for "$alert_ui_contender" down "$alert_ui_contender" \
  > "$RUN_ROOT/alert-ui-losing-down.out" 2>&1 \
  || alert_ui_losing_down_rc=$?
equal "down refuses while another binding transaction owns the server" \
  1 "$alert_ui_losing_down_rc"
equal "a claim-refused down leaves the contender team live" present \
  "$(if alert_ui_tmux has-session -t "=$alert_ui_contender" 2>/dev/null; then printf present; else printf absent; fi)"
equal "a claim-refused down preserves status-right byte-for-byte" \
  "$alert_ui_survivor_right" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" status-right)"
equal "a claim-refused down preserves active and unseen state" \
  "$alert_ui_survivor_counts" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_active) $(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_unseen)"
equal "a claim-refused down preserves its popup command" \
  "$alert_ui_survivor_command" \
  "$(alert_ui_tmux show-options -qv -t "=$alert_ui_survivor:" @gl_alert_command)"
printf '\n' > "$alert_ui_post_claim_release"
alert_ui_claim_owner_rc=0
wait "$alert_ui_claim_owner" || alert_ui_claim_owner_rc=$?
equal "the winning binding transaction completes after refused down" \
  0 "$alert_ui_claim_owner_rc"
alert_ui_gang_for "$alert_ui_contender" down "$alert_ui_contender" >/dev/null

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
tick_scope_stale="$RUN_ROOT/tick-scope-stale"
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
collar_waiting() {
  [ -e "$tick_scope_stale" ] || return 1
  printf 'stale bwrap witness'
}
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
# source-guard: producer@5362c5e07d45: the fake Codex fixture is the only process painting this dedicated pane, and the overflow glyph is emitted only after its three-row queue-preview truncation
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
tick_scope_bin="$RUN_ROOT/tick-scope-bin"
tick_real_systemctl="$(command -v systemctl)"
mkdir -p "$tick_scope_bin"
cat > "$tick_scope_bin/systemctl" <<SH
#!/bin/sh
. "\$GANG_TEST_PATH_SHIM_GUARD"
path_shim_guard '$tick_real_systemctl' "\$0" systemctl || exit \$?
case " \$* " in
  *' show --property=Version --value '*) printf 'fixture-manager\n' ;;
  *' is-active '*) printf 'inactive\n'; exit 3 ;;
  *) exit 1 ;;
esac
SH
chmod +x "$tick_scope_bin/systemctl"
# The original incident left a process witness after its cgroup was collected.
# Make that one stale record explicit: a fresh manager read must retire it even
# while copy-mode keeps the pane operator-owned.
tmux set-option -w -t "$tick_copy_id" @gl_scope \
  "gangline-$GANG_SESSION-tick-copy-0123456789abcdef.scope"
tmux set-option -w -t "$tick_copy_id" @gl_waiting $'waiting\tstale bwrap witness'
tmux set-option -w -t "$tick_copy_id" @gl_waiting_at 0
tmux set-option -w -t "$tick_copy_id" @gl_waiting_gen 1
: > "$tick_scope_stale"
tick_copy_scope_roster="$(PATH="$tick_scope_bin:$PATH" "$GANG" roster)"
contains "a gone registered scope retires the stale wait verdict" \
  "$tick_copy_scope_roster" "tick-copy        tick-native  ~idle~"
excludes "a gone registered scope does not retain its leaked process witness" \
  "$tick_copy_scope_roster" "stale bwrap witness"
contains "copy-mode is surfaced as generic operator-owned pane state" \
  "$tick_copy_scope_roster" "pane-mode=active (attach and leave it before continuing)"
excludes "copy-mode is not prescribed a mode-specific recovery key" \
  "$tick_copy_scope_roster" "send-keys -X cancel"
tmux set-option -uw -t "$tick_copy_id" @gl_scope
rm -f -- "$tick_scope_stale"
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
# source-guard: whole-surface@87a1cbbad112: the nonce-marked peer body is unique to this test and any transcript rendering proves the target consumed it
contains "the closed native turn beats false occupied paint for delivery" \
  "$(pane_all tick-false)" "TICK_FALSE_OCCUPIED_MESSAGE"
equal "the completed pass retires every tick delivery owner marker" absent \
  "$(if [ -n "$(tmux show-options -wqv -t "$tick_copy_id" @gl_tick_delivery)$(tmux show-options -wqv -t "$tick_false_id" @gl_tick_delivery)" ]; then printf present; else printf absent; fi)"
# source-guard: whole-surface@bfade6c1fd0a: the nonce-marked peer body is unique to this test and verified delivery may render it anywhere in the recipient transcript
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
# source-guard: producer@682932d22cdd: the unique continuation is injected only by the preceding same-tick deferred compact, so its visible echo binds the callback-scoping path
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
excludes "a partial pass is not a failed tick" "$("$GANG" status 2>&1)" 'last tick failed'
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
# The fail-closed path for a pane that stays unreadable on a window that
# still exists is unchanged code (window_gone must return false there and
# fall through to the original `die`); it is not independently re-exercised
# here. See test/evidence/tick254/NOTES.md.

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
GANG_TICK_DEADLINE=90 GANG_TICK_INTERNAL=1 "$GANG" __tick-worker \
  > "$RUN_ROOT/tick-deadline-mismatch.out" 2>&1 || tick_deadline_rc=$?
equal "a worker whose exported budget is not the configured deadline refuses" 1 "$tick_deadline_rc"
contains "the worker names both numbers" "$(<"$RUN_ROOT/tick-deadline-mismatch.out")" \
  "tick worker deadline budget 60 is not the configured GANG_TICK_DEADLINE of 90s"
tick_deadline_rc=0
GANG_TICK_DEADLINE=90 GANG_TICK_INTERNAL=1 \
  "$ROOT/libexec/gang-tick-deadline" --clock-helper "$ROOT/libexec/gang-clock" \
  sh -c 'printf "%s\n" "$GANG_TICK_DEADLINE_SECONDS"' > "$RUN_ROOT/tick-deadline-export.out" 2>&1 \
  || tick_deadline_rc=$?
equal "the controller exports the configured deadline to its worker" 90 \
  "$(<"$RUN_ROOT/tick-deadline-export.out")"
tick_deadline_rc=0
GANG_TICK_DEADLINE=abc "$ROOT/libexec/gang-tick-deadline" --clock-helper "$ROOT/libexec/gang-clock" \
  true > "$RUN_ROOT/tick-deadline-controller.out" 2>&1 || tick_deadline_rc=$?
equal "the controller refuses a malformed deadline on its own" 2 "$tick_deadline_rc"
contains "the controller's refusal names the setting" "$(<"$RUN_ROOT/tick-deadline-controller.out")" \
  "GANG_TICK_DEADLINE must be a whole number of seconds from 60 to 3600, got 'abc'"
tick_deadline_rc=0
GANG_TICK_DEADLINE=9223372037 "$ROOT/libexec/gang-tick-deadline" --clock-helper "$ROOT/libexec/gang-clock" \
  true > "$RUN_ROOT/tick-deadline-controller-ceiling.out" 2>&1 || tick_deadline_rc=$?
equal "the controller refuses a deadline above the ceiling on its own" 2 "$tick_deadline_rc"
contains "the controller's refusal names the ceiling" \
  "$(<"$RUN_ROOT/tick-deadline-controller-ceiling.out")" \
  "GANG_TICK_DEADLINE must be a whole number of seconds from 60 to 3600, got '9223372037'"

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

# THE DEADLINE IS AN INTERNAL CONTROLLER CONTRACT, NOT ARITHMETIC INPUT. The
# public controller publishes the validated GANG_TICK_DEADLINE, here its default
# of 60; noncanonical, invalid-octal, and overflowing direct-worker values must
# fail before the worker spends them.
tick_bad_budget_probe() { # $1 value
  local value="$1" rc=0 output="$RUN_ROOT/tick-budget-$1.out"
  GANG_TICK_DEADLINE_SECONDS="$value" GANG_TICK_INTERNAL=1 \
    "$GANG" __tick-worker > "$output" 2>&1 || rc=$?
  equal "deadline value $value is rejected before deadline arithmetic" 1 "$rc"
  contains "deadline value $value names the configured contract it missed" \
    "$(<"$output")" "tick worker deadline budget $value is not the configured GANG_TICK_DEADLINE of 60s"
}
tick_bad_budget_probe 060
tick_bad_budget_probe 08
tick_bad_budget_probe 13836000000

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
# The next tick also repairs a health segment an obsolete snapshot installed.
tmux set-option -t "=$GANG_SESSION:" status-right \
  "operator-left #('/stale/snapshot/gang-tick-health.sh' '/stale/health') operator-right"
tmux set-option -u -t "=$GANG_SESSION:" @gl_tick_health_segment
tick_controller_next_rc=0
"$GANG" tick >/dev/null || tick_controller_next_rc=$?
equal "the next tick passes after a killed controller" 0 "$tick_controller_next_rc"

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
# The warning lands on an unrelated command, so it has to say whether anything
# is stalled and where the caller goes next. Every ordinary command starts a
# tick as it exits, health or not, and a clean pass clears the record.
contains "the repeated failure says cooperative ticking continues" \
  "$(<"$tick_next_err")" "ticking continues"
contains "and names the command that reads the failure" \
  "$(<"$tick_next_err")" "gang alerts"
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

# Team teardown uninstalls the session's alert-center options and retires the
# ephemeral health files with the session that gave them meaning. A file down
# does not own keeps their directory in place: the team still ends, and the
# leftover is named and fails the teardown instead of passing it silently.
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
