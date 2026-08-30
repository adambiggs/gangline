#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Record the public demo against a separate, disposable Gangline team.
set -euo pipefail

repo=$(cd "$(dirname "$0")/../.." && pwd)
demo_root=/tmp/gangline-demo-run
demo_session=gangline-demo
proof_session=gangline-demo-proof
vhs_tmp=$(mktemp -d /tmp/gangline-vhs.XXXXXX)
demo_tmux_root=$(mktemp -d /tmp/gangline-demo-tmux.XXXXXX)

# A recorder may itself run inside an agent window. Make every bare tmux and
# gang invocation resolve through the disposable server, and keep a second
# checkout's guard shim out of the PATH inherited by the recorded agents.
unset TMUX TMUX_PANE
export TMUX_TMPDIR="$demo_tmux_root"
export GANG_LOCK_DIR="$demo_tmux_root/locks"
export GANG_ARCHIVE_DIR="$demo_tmux_root/archive"
export XDG_STATE_HOME="$demo_tmux_root/state"
# A generated follow-up occupies Claude Code's composer after the lead ends its
# turn, so Gangline correctly parks the worker report instead of overwriting it.
export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false
# The recorder tty may have no usable systemd user bus. The exact private
# server and session teardown below own this short-lived team's containment.
export GANG_SCOPE=off
clean_path=""
old_ifs=$IFS
IFS=:
for path_entry in $PATH; do
  case "$path_entry" in
    */libexec/gang-tmux-guard) continue ;;
  esac
  clean_path="${clean_path:+$clean_path:}$path_entry"
done
IFS=$old_ifs
PATH="$clean_path"
export PATH

[ "$demo_root" = /tmp/gangline-demo-run ] || {
  echo "refusing unexpected demo root: $demo_root" >&2
  exit 1
}
[ "$demo_session" = gangline-demo ] || {
  echo "refusing unexpected demo session: $demo_session" >&2
  exit 1
}
[ "$proof_session" = gangline-demo-proof ] || {
  echo "refusing unexpected proof session: $proof_session" >&2
  exit 1
}

for tool in vhs ffmpeg ttyd chromium tmux gang claude codex git; do
  command -v "$tool" >/dev/null || {
    echo "missing demo dependency: $tool" >&2
    exit 1
  }
done

cleanup() {
  tmux -S "$demo_tmux_root/tmux-$(id -u)/default" \
    has-session -t "=$demo_session" 2>/dev/null &&
    tmux -S "$demo_tmux_root/tmux-$(id -u)/default" \
      kill-session -t "=$demo_session" || true
  rm -rf -- "$demo_root"
  rm -rf -- "$vhs_tmp"
  rm -rf -- "$demo_tmux_root"
}
trap cleanup EXIT

rm -rf -- "$demo_root"
mkdir -p "$demo_root"
git -C "$demo_root" init -q
printf '%s\n' \
  'Read this file, then write /tmp/gangline-demo-run/answer.txt containing exactly the single word substrate.' \
  > "$demo_root/TASK.md"

# Establish and prove the private socket before either native agent launches.
# Ending its only proof session lets tmux exit; hitch then creates the recorded
# team on the same isolated socket root without an unregistered window in it.
tmux new-session -d -s "$proof_session" -n socket-proof 'tail -f /dev/null'
[ "$(tmux list-sessions -F '#S')" = "$proof_session" ] || {
  echo "private demo server contains an unexpected session" >&2
  tmux list-sessions >&2
  exit 1
}
tmux kill-session -t "=$proof_session"

cd "$repo"
GANG_CONFIG_DIR="${GANG_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/gangline}" \
  XDG_CONFIG_HOME="$vhs_tmp/chromium-config" TMPDIR="$vhs_tmp" \
  vhs site/demo/demo.tape
ffmpeg -v error -y -sseof -3 -i site/demo.mp4 -frames:v 1 \
  site/demo-poster.jpg

echo "recorded site/demo.gif, site/demo.mp4, and site/demo-poster.jpg"
