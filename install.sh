#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Gangline installer. POSIX sh, so it runs under whatever /bin/sh is:
#
#   curl -fsSL https://raw.githubusercontent.com/adambiggs/gangline/main/install.sh | sh
#
# The script itself may come from main, but it installs the newest stable
# gangline-v* release tag rather than that branch. Re-running upgrades an
# existing install. Override any of:
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
need python3

latest_release_tag() {
  refs="$(git ls-remote --refs --tags "$REPO" 'refs/tags/gangline-v*')" \
    || die "could not read release tags from $REPO"
  tag="$(printf '%s\n' "$refs" | python3 -c '
import re
import sys

releases = []
for line in sys.stdin:
    fields = line.split()
    if len(fields) != 2:
        raise SystemExit(2)
    ref = fields[1]
    match = re.fullmatch(r"refs/tags/(gangline-v(\d+)\.(\d+)\.(\d+))", ref)
    if match:
        releases.append(((int(match[2]), int(match[3]), int(match[4])), match[1]))
if not releases:
    raise SystemExit(1)
print(max(releases)[1])
')" || die "could not determine a stable gangline-vMAJOR.MINOR.PATCH release tag from $REPO"
  printf '%s\n' "$tag"
}

case "${1:-}" in
  '') ;;
  --check)
    [ "$#" -eq 1 ] || die "--check takes no other arguments"
    tag="$(latest_release_tag)"
    latest="${tag#gangline-v}"
    [ -r "$HOME_DIR/version.txt" ] \
      || die "cannot read the installed version at $HOME_DIR/version.txt"
    current="$(sed -n '1p' "$HOME_DIR/version.txt")"
    [ -n "$current" ] || die "installed version is empty in $HOME_DIR/version.txt"
    relation="$(python3 -c '
import re
import sys

def version(value):
    match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)", value)
    if not match:
        raise SystemExit(1)
    return tuple(map(int, match.groups()))

current = version(sys.argv[1])
latest = version(sys.argv[2])
print((current > latest) - (current < latest))
' "$current" "$latest")" \
      || die "cannot compare installed version '$current' with release '$latest'"
    case "$relation" in
      0) echo "gangline $current is the latest release" ;;
      -1) echo "gangline upgrade available: $current -> $latest" ;;
      1) echo "gangline $current is newer than the latest release $latest" ;;
      *) die "could not compare installed version '$current' with release '$latest'" ;;
    esac
    exit 0
    ;;
  *) die "unknown argument '$1'" ;;
esac

tag="$(latest_release_tag)"

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
  state="$(git -C "$HOME_DIR" status --porcelain)" \
    || die "could not inspect the existing install at $HOME_DIR"
  [ -z "$state" ] || die "$HOME_DIR has local changes; move them aside before upgrading"
  echo "installing $tag over $HOME_DIR"
  git -C "$HOME_DIR" fetch --depth 1 --quiet "$REPO" "refs/tags/$tag" \
    || die "could not fetch $tag from $REPO"
  git -C "$HOME_DIR" checkout --detach --quiet FETCH_HEAD \
    || die "could not check out release $tag in $HOME_DIR"
else
  echo "installing $tag into $HOME_DIR"
  mkdir -p "$(dirname "$HOME_DIR")"
  git clone --branch "$tag" --depth 1 --quiet "$REPO" "$HOME_DIR" \
    || die "could not clone release $tag from $REPO"
fi

mkdir -p "$BIN_DIR"
# `ln -sf` FOLLOWS an existing symlink to a directory: it writes gang INSIDE the
# referenced directory and leaves the link standing, so a re-run mutates a
# directory nobody named and only fails afterwards, when it executes the
# still-directory destination. -f does not prevent that. Remove the exact
# destination first when it is gang's own link or file, and refuse anything else
# rather than reaching through it.
if [ "$BIN_DIR/gang" != "$HOME_DIR/bin/gang" ]; then
  if [ -L "$BIN_DIR/gang" ] || [ -f "$BIN_DIR/gang" ]; then
    rm -f "$BIN_DIR/gang" || die "could not remove the existing $BIN_DIR/gang"
  elif [ -e "$BIN_DIR/gang" ]; then
    die "$BIN_DIR/gang exists and is not a file or a symlink — move it aside"
  fi
  ln -s "$HOME_DIR/bin/gang" "$BIN_DIR/gang" \
    || die "could not link $BIN_DIR/gang -> $HOME_DIR/bin/gang"
fi

# Execute the installed tree before reporting success.
"$BIN_DIR/gang" collars >/dev/null || die "installed, but 'gang collars' failed"

echo
echo "gang $tag installed -> $BIN_DIR/gang"
echo "  harnesses: $("$BIN_DIR/gang" collars | tr '\n' ' ')"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo
     echo "  $BIN_DIR is not on your PATH. Add it:"
     echo "      export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
echo
echo "Start a team:  cd ~/your/repo && gang up"
echo "  that attaches you to the lead; detach with Ctrl-b then d, return with 'gang attach'"
