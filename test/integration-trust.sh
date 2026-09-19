# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# An attended native hook-review path: it must compose the same Codex hooks as a
# hitch, but it must omit the refusing preflight and leave every native choice to
# the person at the pane.

trust_bin="$RUN_ROOT/trust-bin"
trust_observer="$trust_bin/observe-input.py"
trust_args="$RUN_ROOT/trust.args"
trust_keys="$RUN_ROOT/trust.keys"
trust_tmux_bin="$RUN_ROOT/trust.tmux-bin"
trust_tmux_env="$RUN_ROOT/trust.tmux-env"
trust_session="$RUN_ROOT/trust.session"
trust_ready="gangline-trust-$RANDOM"
trust_key_seen="${trust_ready}-key"
mkdir -p "$trust_bin"
cat > "$trust_observer" <<'PY'
import os
import subprocess
import sys

subprocess.run(
    ["tmux", "-S", os.environ["GANG_TMUX_SOCKET"], "wait-for", "-S",
     os.environ["CODEX_TRUST_READY"]],
    check=True,
)
with open(sys.argv[1], "wb", buffering=0) as stream:
    while True:
        chunk = os.read(0, 1)
        if not chunk:
            break
        stream.write(chunk)
        subprocess.run(
            ["tmux", "-S", os.environ["GANG_TMUX_SOCKET"], "wait-for", "-S",
             os.environ["CODEX_TRUST_KEY_SEEN"]],
            check=True,
        )
PY
cat > "$trust_bin/codex" <<'SH'
#!/bin/sh
printf '%s\n' "$@" > "$CODEX_TRUST_ARGS"
command -v tmux > "$CODEX_TRUST_TMUX_BIN"
printf '%s\n' "${TMUX-unset}" > "$CODEX_TRUST_TMUX_ENV"
printf '%s\n' "${GANG_SESSION-unset}" > "$CODEX_TRUST_SESSION"
printf '%s\n' 'Hooks need review' '  1. Review hooks' '  2. Trust all and continue'
stty -icanon min 1 time 0 -echo
exec python3 "$CODEX_TRUST_OBSERVER" "$CODEX_TRUST_KEYS"
SH
chmod +x "$trust_bin/codex"

tmux set-environment -g CODEX_TRUST_ARGS "$trust_args"
tmux set-environment -g CODEX_TRUST_KEYS "$trust_keys"
tmux set-environment -g CODEX_TRUST_TMUX_BIN "$trust_tmux_bin"
tmux set-environment -g CODEX_TRUST_TMUX_ENV "$trust_tmux_env"
tmux set-environment -g CODEX_TRUST_SESSION "$trust_session"
tmux set-environment -g CODEX_TRUST_OBSERVER "$trust_observer"
tmux set-environment -g CODEX_TRUST_READY "$trust_ready"
tmux set-environment -g CODEX_TRUST_KEY_SEEN "$trust_key_seen"
PATH="$trust_bin:$PATH" "$GANG" trust codex -d "$RUN_ROOT" > "$RUN_ROOT/trust.out"
trust_output="$(<"$RUN_ROOT/trust.out")"
tmux wait-for "$trust_ready"
trust_window="$(printf '%s\n' "$trust_output" \
  | awk '{ for (i = 1; i <= NF; i++) if ($i ~ /^@[0-9]+$/) { print $i; exit } }')"
[ -n "$trust_window" ] \
  || fail "the attended trust command reports its native review window" "$trust_output"
trust_pane="$(tmux list-panes -t "$trust_window" -F '#{pane_id}')"
trust_capture="$(tmux capture-pane -p -J -S - -t "$trust_pane")"

contains "trust opens the native review in a named attended window" \
  "$trust_output" "$trust_window"
equal "the attended trust launch retains the suite's controlled tmux route" \
  "$RUN_ROOT/waitbin/tmux" "$(<"$trust_tmux_bin")"
equal "the attended trust launch clears tmux's implicit server route" \
  unset "$(<"$trust_tmux_env")"
equal "the attended trust launch supplies hooks the recorded team route" \
  "$GANG_SESSION" "$(<"$trust_session")"
contains "the attended window carries Codex's native hook-review prompt" \
  "$trust_capture" "Hooks need review"
contains "the attended trust launch retains the SessionStart hook" \
  "$(<"$trust_args")" "hooks.SessionStart="
contains "the attended trust launch retains the Stop hook" \
  "$(<"$trust_args")" "hooks.Stop="
for trust_event in SessionStart UserPromptSubmit PostToolUse PermissionRequest \
                   PreCompact PostCompact Stop; do
  contains "the attended trust launch retains the $trust_event hook" \
    "$(<"$trust_args")" "hooks.$trust_event="
done
excludes "the attended trust launch does not invoke the refusing preflight" \
  "$(tmux display-message -p -t "$trust_window" '#{pane_start_command}')" \
  "codex-hooks-preflight.py"
equal "the attended trust window has no Gangline agent registration" "" \
  "$(tmux show-options -wqv -t "$trust_window" @gl_agent)"
equal "no key reaches the attended native trust menu" "" "$(<"$trust_keys")"
# This must differ from Codex's trust-all shortcut. A hidden Gangline `2`
# signals the observer first, so the exact log below fails instead of racing
# the fixture's deliberate calibration input.
tmux send-keys -t "$trust_window" -l x
tmux wait-for "$trust_key_seen"
equal "the native-menu observer catches a bare key without masking trust" x "$(<"$trust_keys")"
tmux kill-window -t "$trust_window"
tmux set-environment -gu CODEX_TRUST_ARGS
tmux set-environment -gu CODEX_TRUST_KEYS
tmux set-environment -gu CODEX_TRUST_TMUX_BIN
tmux set-environment -gu CODEX_TRUST_TMUX_ENV
tmux set-environment -gu CODEX_TRUST_SESSION
tmux set-environment -gu CODEX_TRUST_OBSERVER
tmux set-environment -gu CODEX_TRUST_READY
tmux set-environment -gu CODEX_TRUST_KEY_SEEN
