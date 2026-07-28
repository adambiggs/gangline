#!/usr/bin/env bash
# Record site/demo/demo.tape against a disposable demo HOME at /home/demo.
#
# The demo runs three real harnesses (Claude Code lead, Codex worker, opencode
# reviewer) on the invoking user's subscription auth, inside an isolated HOME
# so no real path, hostname, or config can leak on screen. This script stages
# that HOME idempotently, resets per-take state, runs vhs, and moves the
# rendered videos into site/. Auth files are copied byte-for-byte and never
# printed; wipe them with --wipe-auth when you are done recording.
#
# Usage:
#   site/demo/record.sh              stage + record
#   site/demo/record.sh --wipe-auth  remove copied credentials from /home/demo
set -euo pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
DEMO=/home/demo
REAL_HOME="$HOME"

wipe_auth() {
  rm -f "$DEMO/.claude/.credentials.json" \
        "$DEMO/.claude.json" \
        "$DEMO/.codex/auth.json" \
        "$DEMO/.local/share/opencode/auth.json"
  echo "wiped copied credentials from $DEMO (re-staged on next run)"
}

if [ "${1:-}" = "--wipe-auth" ]; then
  wipe_auth
  exit 0
fi

for tool in vhs ffmpeg tmux git claude codex opencode node python3; do
  command -v "$tool" >/dev/null || { echo "missing: $tool" >&2; exit 1; }
done
if [ ! -d "$DEMO" ]; then
  echo "create the demo HOME first (owned by you):" >&2
  echo "  sudo mkdir -m 755 $DEMO && sudo chown $(id -u):$(id -g) $DEMO" >&2
  exit 1
fi

# ── Layout ───────────────────────────────────────────────────────────────────
mkdir -p "$DEMO/bin" "$DEMO/.tmux" "$DEMO/.claude" "$DEMO/.codex" \
         "$DEMO/.config/opencode" "$DEMO/.local/share/opencode" "$DEMO/hello"

# Frozen copy of gangline itself, so repo churn never changes a take mid-series.
rsync -a --delete "$REPO/bin" "$REPO/profiles" "$REPO/roles" "$REPO/statusline" \
  "$DEMO/gangline/"

# The tape's PATH is only $DEMO/bin + $DEMO/gangline/bin + system dirs; this
# symlink farm is how version-managed tools stay reachable without their
# real (user-specific) install paths ever appearing in the recorded env.
for tool in claude codex opencode node python3 git tmux; do
  ln -sf "$(command -v "$tool")" "$DEMO/bin/$tool"
done

cat > "$DEMO/.tmux.conf" <<'EOF'
# Demo cosmetics: no hostname/clock in the status bar, quiet colors.
set -g status-right ""
set -g status-style "bg=colour235,fg=colour245"
set -g window-status-current-style "fg=colour231,bold"
set -g status-left-length 20
EOF

cat > "$DEMO/.gitconfig" <<'EOF'
[user]
	name = demo
	email = demo@localhost
[init]
	defaultBranch = main
EOF

# A .git makes the harnesses treat ~/hello as a project root; it stays
# commitless — the demo shows the work, not the checkpoint.
[ -d "$DEMO/hello/.git" ] || git -C "$DEMO/hello" init -q

cat > "$DEMO/.claude/settings.json" <<'EOF'
{
  "permissions": {
    "allow": [
      "Bash",
      "Read",
      "Glob",
      "Grep",
      "Write",
      "Edit"
    ]
  },
  "statusLine": {
    "type": "command",
    "command": "/home/demo/gangline/statusline/claude-code-context.sh"
  },
  "theme": "dark"
}
EOF

cat > "$DEMO/.codex/config.toml" <<'EOF'
model = "gpt-5.6-sol"
model_reasoning_effort = "medium"
approval_policy = "never"
# gang talks to tmux over its unix socket; codex's workspace-write sandbox
# blocks socket connect() via seccomp regardless of landlock writable roots
# (verified live: touch in the socket dir succeeds, connect() gets EPERM).
# The demo env is a disposable throwaway HOME, so the sandbox comes off.
sandbox_mode = "danger-full-access"

[projects."/home/demo/hello"]
trust_level = "trusted"

[notice]
hide_full_access_warning = true
EOF

cat > "$DEMO/.config/opencode/opencode.json" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "model": "github-copilot/gemini-3.1-pro-preview",
  "permission": {
    "external_directory": {
      "/home/demo/gangline/**": "allow"
    }
  }
}
EOF

# ── Auth (copied, never printed) ─────────────────────────────────────────────
copy_auth() {
  [ -f "$1" ] || { echo "missing auth source: $1" >&2; exit 1; }
  install -m 600 "$1" "$2"
}
copy_auth "$REAL_HOME/.claude/.credentials.json" "$DEMO/.claude/.credentials.json"
copy_auth "$REAL_HOME/.codex/auth.json"          "$DEMO/.codex/auth.json"
copy_auth "$REAL_HOME/.local/share/opencode/auth.json" \
          "$DEMO/.local/share/opencode/auth.json"

# Claude Code keeps its account identity and folder trust in ~/.claude.json;
# seed just those so the demo boots straight past login and the trust dialog.
if [ ! -f "$DEMO/.claude.json" ]; then
  python3 - "$REAL_HOME/.claude.json" "$DEMO/.claude.json" <<'EOF'
import json, sys
real = json.load(open(sys.argv[1]))
seed = {
    "oauthAccount": real["oauthAccount"],
    "hasCompletedOnboarding": True,
    "projects": {"/home/demo/hello": {"hasTrustDialogAccepted": True}},
}
with open(sys.argv[2], "w") as f:
    json.dump(seed, f, indent=1)
EOF
  chmod 600 "$DEMO/.claude.json"
fi

# ── Per-take reset ───────────────────────────────────────────────────────────
# `env -u TMUX` is load-bearing: a tmux client inside a tmux pane takes its
# socket from $TMUX and ignores TMUX_TMPDIR, so without this the kill-server
# lands on the invoking user's own server.
env -u TMUX TMUX_TMPDIR="$DEMO/.tmux" tmux kill-server 2>/dev/null || true
rm -f "$DEMO/hello/hello.py" "$DEMO/demo.webm" "$DEMO/demo.mp4"

# ── Record ───────────────────────────────────────────────────────────────────
# The tape's Env lines build the clean recorded environment; vhs otherwise runs
# in the invoking env so it can find its browser — minus $TMUX, so a gang inside
# the tape can only ever reach the demo socket. Expect several minutes of hidden
# wall time — the cuts wait on real agents finishing real turns.
cd "$DEMO"
env -u TMUX vhs "$REPO/site/demo/demo.tape"

mv "$DEMO/demo.webm" "$REPO/site/demo.webm"
mv "$DEMO/demo.mp4"  "$REPO/site/demo.mp4"
ls -l "$REPO/site/demo.webm" "$REPO/site/demo.mp4"
echo
echo "before committing: frame-audit the video (ffmpeg -i site/demo.mp4 -vf fps=1 f%03d.png)"
echo "and check every frame for anything that should not be on screen."
echo "done recording? wipe credentials: site/demo/record.sh --wipe-auth"
