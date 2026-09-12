# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
# An attended native hook-review path: it must compose the same Codex hooks as a
# hitch, but it must omit the refusing preflight and leave every native choice to
# the person at the pane.

trust_bin="$RUN_ROOT/trust-bin"
trust_args="$RUN_ROOT/trust.args"
trust_hold="$RUN_ROOT/trust-hold"
trust_ready="gangline-trust-$RANDOM"
mkdir -p "$trust_bin"
mkfifo "$trust_hold"
cat > "$trust_bin/codex" <<'SH'
#!/bin/sh
printf '%s\n' "$@" > "$CODEX_TRUST_ARGS"
printf '%s\n' 'Hooks need review' '  1. Review hooks' '  2. Trust all and continue'
tmux -S "$GANG_TMUX_SOCKET" wait-for -S "$CODEX_TRUST_READY"
exec 9<>"$CODEX_TRUST_HOLD"
cat <&9 >/dev/null
SH
chmod +x "$trust_bin/codex"

trust_output="$(env PATH="$trust_bin:$PATH" CODEX_TRUST_ARGS="$trust_args" \
  CODEX_TRUST_HOLD="$trust_hold" CODEX_TRUST_READY="$trust_ready" \
  "$GANG" trust codex)"
tmux wait-for "$trust_ready"
trust_window="$(window_id trust-codex)"
trust_pane="$(tmux list-panes -t "$trust_window" -F '#{pane_id}')"
trust_capture="$(tmux capture-pane -p -J -S - -t "$trust_pane")"

contains "trust opens the native review in a named attended window" \
  "$trust_output" "trust-codex"
# source-guard: producer@06ad90b31a3a: the fixture codex wrapper is the only process this window runs and it prints the native-menu sentinel before signalling the test barrier
contains "the attended window carries Codex's native hook-review prompt" \
  "$trust_capture" "Hooks need review"
contains "the attended trust launch retains the SessionStart hook" \
  "$(<"$trust_args")" "hooks.SessionStart="
contains "the attended trust launch retains the Stop hook" \
  "$(<"$trust_args")" "hooks.Stop="
excludes "the attended trust launch does not invoke the refusing preflight" \
  "$(tmux display-message -p -t "$trust_window" '#{pane_start_command}')" \
  "codex-hooks-preflight.py"
excludes "the attended trust window is not registered as an agent" \
  "$("$GANG" roster --porcelain)" $'trust-codex\t'
tmux kill-window -t "$trust_window"
