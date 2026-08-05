#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Record the public demo against a separate, disposable Gangline team.
set -euo pipefail

repo=$(cd "$(dirname "$0")/../.." && pwd)
demo_root=/tmp/gangline-demo-run
demo_session=gangline-demo
vhs_tmp=$(mktemp -d /tmp/gangline-vhs.XXXXXX)

[ "$demo_root" = /tmp/gangline-demo-run ] || {
  echo "refusing unexpected demo root: $demo_root" >&2
  exit 1
}
[ "$demo_session" = gangline-demo ] || {
  echo "refusing unexpected demo session: $demo_session" >&2
  exit 1
}

for tool in vhs ffmpeg ttyd chromium tmux codex git; do
  command -v "$tool" >/dev/null || {
    echo "missing demo dependency: $tool" >&2
    exit 1
  }
done

cleanup() {
  tmux has-session -t "$demo_session" 2>/dev/null &&
    tmux kill-session -t "$demo_session" || true
  rm -rf -- "$demo_root"
  rm -rf -- "$vhs_tmp"
}
trap cleanup EXIT

tmux has-session -t "$demo_session" 2>/dev/null &&
  tmux kill-session -t "$demo_session"
rm -rf -- "$demo_root"
mkdir -p "$demo_root"
git -C "$demo_root" init -q
printf '%s\n' \
  'Read this file, then write answer.txt containing exactly the single word substrate.' \
  > "$demo_root/TASK.md"

cd "$repo"
TMPDIR="$vhs_tmp" vhs site/demo/demo.tape
ffmpeg -v error -y -ss 00:00:56 -i site/demo.mp4 -frames:v 1 \
  site/demo-poster.jpg

echo "recorded site/demo.gif, site/demo.mp4, and site/demo-poster.jpg"
