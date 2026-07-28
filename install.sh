#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Gangline installer. POSIX sh, so it runs under whatever /bin/sh is:
#
#   curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
#
# Re-running updates an existing install. Override any of:
#   GANGLINE_REPO  source to clone from   (default: the GitHub repo)
#   GANGLINE_HOME  where the tree lives   (default: ~/.local/share/gangline)
#   GANGLINE_BIN   where `gang` is linked (default: ~/.local/bin)
set -eu

REPO="${GANGLINE_REPO:-https://github.com/adambiggs/gangline.git}"
HOME_DIR="${GANGLINE_HOME:-$HOME/.local/share/gangline}"
BIN_DIR="${GANGLINE_BIN:-$HOME/.local/bin}"

die() { echo "gangline: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required but not installed"; }

need git
need tmux

# paste-buffer -p (bracketed paste) is how every send reaches an agent.
ver="$(tmux -V | tr -cd '0-9.')"
major="${ver%%.*}"
minor="${ver#*.}"; minor="${minor%%.*}"
[ "$major" -gt 3 ] || { [ "$major" -eq 3 ] && [ "$minor" -ge 2 ]; } \
  || die "tmux >= 3.2 required for bracketed paste, found $(tmux -V)"

# python3 is a requirement, not a nicety. Three of the four shipped profiles run
# it: claude-code through the statusline beacon and the hook payload, codex to
# bind an agent to its rollout and parse token_count, opencode to join the model
# catalogue. It also builds the context-hook reply. Without it, context reading,
# the roster's context column, patrol's band warnings and `gang vet`'s format
# gates all fail — most of what makes a team self-managing. Naming that at
# install time beats four different failures later, one per profile.
command -v python3 >/dev/null 2>&1 \
  || die "python3 required — context readouts, band warnings and vet's format gates all run it (macOS ships none by default: brew install python)"

# The whole tree is the tool: bin/gang reads profiles/ and roles/ relative to
# itself, so this clones rather than dropping a single file on the PATH.
if [ -d "$HOME_DIR/.git" ]; then
  echo "updating $HOME_DIR"
  git -C "$HOME_DIR" pull --ff-only --quiet \
    || die "could not fast-forward $HOME_DIR — it has local commits; resolve them or move it aside"
else
  echo "cloning into $HOME_DIR"
  mkdir -p "$(dirname "$HOME_DIR")"
  git clone --depth 1 --quiet "$REPO" "$HOME_DIR" \
    || die "could not clone $REPO"
fi

mkdir -p "$BIN_DIR"
ln -sf "$HOME_DIR/bin/gang" "$BIN_DIR/gang"

# Prove it runs rather than announcing success and hoping. Both commands read the
# tree that was just installed, so either one failing means the install is broken
# even though every step above reported success -- swallowing one hides exactly
# the failure this check exists to catch.
"$BIN_DIR/gang" profiles >/dev/null || die "installed, but 'gang profiles' failed"
"$BIN_DIR/gang" roles    >/dev/null || die "installed, but 'gang roles' failed"

echo
echo "gang installed -> $BIN_DIR/gang"
echo "  harnesses: $("$BIN_DIR/gang" profiles | tr '\n' ' ')"
echo "  roles:     $("$BIN_DIR/gang" roles | tr '\n' ' ')"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo
     echo "  $BIN_DIR is not on your PATH. Add it:"
     echo "      export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
echo
echo "Start a team:  cd ~/your/repo && gang up"
echo "Claude Code also wants the context beacon wired into settings.json;"
echo "see $HOME_DIR/README.md (Self-compaction)."
