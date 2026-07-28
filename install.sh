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

# Not a hard requirement: gang hitches, sends and sweeps without python3. It reads
# the Claude Code hook payload and the statusline beacon, which is the whole of
# context measurement for that harness. macOS ships no python3 by default, so say
# what breaks rather than let the beacon fail later with no explanation.
command -v python3 >/dev/null 2>&1 || echo \
  "gangline: no python3 — 'gang context', the roster context column and self-compaction warnings will not work for Claude Code agents" >&2

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
