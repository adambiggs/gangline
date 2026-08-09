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

# This is the oldest tmux release on which Gangline's complete call set has been
# executed. Lower it only after running that call set on the lower release.
ver="$(tmux -V | tr -cd '0-9.')"
major="${ver%%.*}"
minor="${ver#*.}"; minor="${minor%%.*}"
[ "$major" -gt 2 ] || { [ "$major" -eq 2 ] && [ "$minor" -ge 6 ]; } \
  || die "tmux >= 2.6 required: the oldest version gang's whole call set has been run on, found $(tmux -V)"

python3 -c 'import json; assert json.loads("{\"ok\": true}")["ok"]' >/dev/null 2>&1 \
  || die "working python3 with JSON support required — native hook payloads and optional context lights use it"

# The whole tree is the tool: bin/gang reads collars/ relative to itself.
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
# `ln -sf` FOLLOWS an existing symlink to a directory: it writes gang INSIDE the
# referenced directory and leaves the link standing, so a re-run mutates a
# directory nobody named and only fails afterwards, when it executes the
# still-directory destination. -f does not prevent that. Remove the exact
# destination first when it is gang's own link or file, and refuse anything else
# rather than reaching through it.
if [ -L "$BIN_DIR/gang" ] || [ -f "$BIN_DIR/gang" ]; then
  rm -f "$BIN_DIR/gang" || die "could not remove the existing $BIN_DIR/gang"
elif [ -e "$BIN_DIR/gang" ]; then
  die "$BIN_DIR/gang exists and is not a file or a symlink — move it aside"
fi
ln -s "$HOME_DIR/bin/gang" "$BIN_DIR/gang" \
  || die "could not link $BIN_DIR/gang -> $HOME_DIR/bin/gang"

# Execute the installed tree before reporting success.
"$BIN_DIR/gang" collars >/dev/null || die "installed, but 'gang collars' failed"

echo
echo "gang installed -> $BIN_DIR/gang"
echo "  harnesses: $("$BIN_DIR/gang" collars | tr '\n' ' ')"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo
     echo "  $BIN_DIR is not on your PATH. Add it:"
     echo "      export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
echo
# `gang up` attaches to the lead.
echo "Start a team:  cd ~/your/repo && gang up"
echo "  that attaches you to the lead; detach with Ctrl-b then d, return with 'gang attach'"
