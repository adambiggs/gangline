#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Fresh native Claude Code/Codex run, recorded by the VHS tape beside this file.
set -euo pipefail
repo=$(cd -- "$(dirname -- "$0")/../.." && pwd)
: "${DEMO_STATE:?Set DEMO_STATE to a new directory under ~/.local/state}"
case "$DEMO_STATE" in "$HOME/.local/state/"*) ;; *) echo 'DEMO_STATE must be under ~/.local/state' >&2; exit 1 ;; esac
mkdir "$DEMO_STATE"
export DEMO_STATE
unset TMUX TMUX_PANE GANG_TMUX GANG_COLLARS
export GANG_SESSION=gangline-demo-vhs
export GANG_TMUX_SOCKET="$DEMO_STATE/tmux.sock"
export GANG_STATE_ROOT="$DEMO_STATE/state"
export GANG_CONFIG_DIR="$DEMO_STATE/config"
export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false
# The throwaway demo agents need access to the peer harness process.
# Gang needs to observe the peer harness process when delivering the reply.
metadata=$(git -C "$repo" rev-parse --path-format=absolute --git-common-dir)
export GANG_LAUNCH_ARGS
GANG_LAUNCH_ARGS=$(python3 -c 'import json,sys; print(json.dumps({"codex":["--sandbox","danger-full-access","--add-dir",sys.argv[1],"--add-dir",sys.argv[2]]}))' "$DEMO_STATE" "$metadata")
mkdir -p "$GANG_CONFIG_DIR/roles"
for tool in gang tmux claude codex vhs ffmpeg; do command -v "$tool"; done
[ ! -e "$repo/greet.py" ] || { echo 'Preserve and remove the previous greet.py before recording.' >&2; exit 1; }
cat > "$GANG_CONFIG_DIR/roles/demo.md" <<'ROLE'
You are in a public terminal demonstration. Keep responses short. Do only the assigned task; no commits or extra checks. Messages use gang send NAME with stdin, never --from. When idle, say Ready. A guide delegates to builder and ends its turn. When builder replies, guide replies exactly: Demo complete: Hello, team! The builder writes greet.py, runs it, and reports the observed output to guide.
ROLE
cleanup() {
  local rc=$?
  trap - EXIT
  if [ -f "$GANG_STATE_ROOT/teams/$GANG_SESSION/log.jsonl" ]; then
    if ! cp "$GANG_STATE_ROOT/teams/$GANG_SESSION/log.jsonl" "$DEMO_STATE/events.jsonl"; then
      echo 'Failed to preserve the event log; ending the private demo team.' >&2
      rc=1
    fi
  fi
  # Preserve diagnostics, but never let a failed capture prevent teardown.
  if [ -d "$GANG_STATE_ROOT/teams/$GANG_SESSION" ]; then
    if gang roster; then
      for name in guide builder; do
        if [ -e "$GANG_STATE_ROOT/teams/$GANG_SESSION/names/$name" ]; then
          if ! gang capture "$name" 2000 > "$DEMO_STATE/$name.txt"; then
            echo "Failed to capture $name; ending the private demo team." >&2
            rc=1
          fi
        fi
      done
    else
      echo 'Demo roster failed; ending the private demo team.' >&2
      rc=1
    fi
    if ! gang down "$GANG_SESSION"; then
      echo 'Demo teardown failed; inspect the retained event log.' >&2
      rc=1
    fi
  fi
  # gang down leaves the recorder shell; end only this private session.
  if ! tmux -S "$GANG_TMUX_SOCKET" kill-session -t "$GANG_SESSION"; then
    echo 'Failed to close the private recorder session.' >&2
    rc=1
  fi
  exit "$rc"
}
# A new explicit private socket; never inherit the live team's server.
tmux -S "$GANG_TMUX_SOCKET" -f /dev/null new-session -d -s "$GANG_SESSION" -n recorder -x 38 -y 48 -c "$repo"
trap cleanup EXIT
tmux -S "$GANG_TMUX_SOCKET" set-option -g default-size 38x48
tmux -S "$GANG_TMUX_SOCKET" set-option -g status off
[ "$(tmux -S "$GANG_TMUX_SOCKET" list-sessions -F '#S')" = "$GANG_SESSION" ]
tmux -S "$GANG_TMUX_SOCKET" list-sessions
gang hitch builder -c codex -r demo -d "$repo" -t 'Wait for guide. Say Ready and end this turn.'
gang wait builder --timeout 120s
gang hitch guide -c claude-code -r demo -d "$repo" -t 'Wait for a keyboard request. Say Ready and end this turn.'
gang wait guide --timeout 120s
# Joining existing panes preserves their registered sender identities.
guide=$(tmux -S "$GANG_TMUX_SOCKET" list-panes -a -F '#{pane_id} #{window_name}' | awk '$2 ~ /guide/ {print $1}')
builder=$(tmux -S "$GANG_TMUX_SOCKET" list-panes -a -F '#{pane_id} #{window_name}' | awk '$2 ~ /builder/ {print $1}')
[ -n "$guide" ] && [ -n "$builder" ]
tmux -S "$GANG_TMUX_SOCKET" join-pane -v -s "$builder" -t "$guide"
tmux -S "$GANG_TMUX_SOCKET" select-layout -t "$guide" even-vertical
tmux -S "$GANG_TMUX_SOCKET" set-option -g pane-border-status top
tmux -S "$GANG_TMUX_SOCKET" set-option -g pane-border-format " #{?#{==:#{pane_id},$guide},Claude Code / guide,Codex / builder} "
tmux -S "$GANG_TMUX_SOCKET" set-option -g pane-border-style 'fg=#7f8da0'
tmux -S "$GANG_TMUX_SOCKET" set-option -g pane-active-border-style 'fg=#84aad6'
tmux -S "$GANG_TMUX_SOCKET" select-pane -t "$guide" -T 'Claude Code / guide'
tmux -S "$GANG_TMUX_SOCKET" select-pane -t "$builder" -T 'Codex / builder'
tmux -S "$GANG_TMUX_SOCKET" select-pane -t "$guide"
tmux -S "$GANG_TMUX_SOCKET" select-window -t "$guide"
cd "$repo"
vhs site/demo/demo.tape
python3 site/demo/verify.py "$GANG_STATE_ROOT/teams/$GANG_SESSION/log.jsonl" "$DEMO_STATE/transcript.txt"
cp "$DEMO_STATE/transcript.txt" site/demo.txt
mv site/demo/.demo-render.mp4 "$DEMO_STATE/demo.mp4"
mv site/demo/.demo-render.gif "$DEMO_STATE/demo.gif"
[ -s "$DEMO_STATE/demo.mp4" ] && [ -s "$DEMO_STATE/demo.gif" ]
# Keep the entire native recording. No slide replacement or fabricated text.
cp "$DEMO_STATE/demo.mp4" site/demo.mp4
cp "$DEMO_STATE/demo.gif" site/demo.gif
ffmpeg -v error -y -sseof -3 -i site/demo.mp4 -frames:v 1 site/demo-poster.jpg
# Native terminals retain the same appearance in either site theme.
cp site/demo.mp4 site/demo-light.mp4
cp site/demo.gif site/demo-light.gif
cp site/demo-poster.jpg site/demo-poster-light.jpg
printf 'Recorded native session: %s\n' "$DEMO_STATE"
