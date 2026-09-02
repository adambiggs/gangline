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

installed_release_version() {
  [ -r "$HOME_DIR/version.txt" ] \
    || die "installed version is unknown: cannot read $HOME_DIR/version.txt"
  current="$(sed -n '1p' "$HOME_DIR/version.txt")"
  [ -n "$current" ] \
    || die "installed version is unknown: $HOME_DIR/version.txt is empty"
  printf '%s\n' "$current"
}

release_relation() { # current latest -> relation and the changed semver component
  python3 -c '
import re
import sys

def version(value):
    match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)", value)
    if not match:
        raise SystemExit(1)
    return tuple(map(int, match.groups()))

current = version(sys.argv[1])
latest = version(sys.argv[2])
relation = (current > latest) - (current < latest)
if relation == 0:
    distance = "current"
elif current[0] != latest[0]:
    distance = "major"
elif current[1] != latest[1]:
    distance = "minor"
else:
    distance = "patch"
print(relation, distance)
' "$1" "$2"
}

mode=""
case "${1:-}" in
  '') ;;
  --check)
    [ "$#" -eq 1 ] || die "--check takes no other arguments"
    mode=check
    ;;
  *) die "unknown argument '$1'" ;;
esac

case "${GANGLINE_UPGRADE:-0}" in 0|1) ;; *) die "GANGLINE_UPGRADE is internal and must be 0 or 1" ;; esac
if [ "${GANGLINE_UPGRADE:-0}" -eq 1 ]; then
  [ -e "$HOME_DIR/.git" ] \
    || die "gang upgrade requires an installer-managed release at $HOME_DIR"
  git -C "$HOME_DIR" rev-parse --verify HEAD >/dev/null 2>&1 \
    || die "gang upgrade cannot verify the installed release at $HOME_DIR"
  branch="$(git -C "$HOME_DIR" symbolic-ref --quiet --short HEAD 2>/dev/null)" || branch=""
  [ -z "$branch" ] \
    || die "gang upgrade refuses source checkout branch '$branch' at $HOME_DIR; update it with: git -C '$HOME_DIR' pull --ff-only"
  if [ "$mode" != check ]; then
    state="$(git -C "$HOME_DIR" status --porcelain)" \
      || die "could not inspect the existing install at $HOME_DIR"
    [ -z "$state" ] || die "$HOME_DIR has local changes; move them aside before upgrading"
  fi
fi

if [ "${GANGLINE_UPGRADE:-0}" -eq 1 ] || [ "$mode" = check ]; then
  current="$(installed_release_version)"
fi

tag="$(latest_release_tag)"
latest="${tag#gangline-v}"

if [ "${GANGLINE_UPGRADE:-0}" -eq 1 ] || [ "$mode" = check ]; then
  comparison="$(release_relation "$current" "$latest")" \
    || die "installed version '$current' is malformed; expected MAJOR.MINOR.PATCH"
  relation="${comparison%% *}"
  distance="${comparison#* }"
  case "$mode:$relation" in
    check:0)
      echo "gang is current: v$current -> v$latest (already at latest release)"
      exit 0
      ;;
    check:-1)
      echo "upgrade available: v$current -> v$latest ($distance update)"
      exit 0
      ;;
    check:1)
      echo "installed version is newer than latest: v$current -> v$latest; no changes made"
      exit 0
      ;;
    :0)
      echo "gang is current: v$current -> v$latest (already at latest release); no changes made"
      exit 0
      ;;
    :-1)
      echo "upgrading from v$current -> v$latest"
      ;;
    :1)
      die "refusing downgrade from installed v$current -> selected v$latest; no changes made"
      ;;
    *) die "could not compare installed version '$current' with release '$latest'" ;;
  esac
fi

need tmux

# `gang wait` uses indexed hook arrays and list-command filters, so Gangline's
# complete call set requires tmux 3.2 even though its other calls are older.
ver="$(tmux -V | tr -cd '0-9.')"
major="${ver%%.*}"
minor="${ver#*.}"; minor="${minor%%.*}"
[ "$major" -gt 3 ] || { [ "$major" -eq 3 ] && [ "$minor" -ge 2 ]; } \
  || die "tmux >= 3.2 required: gang wait needs indexed hooks and list-command filters, found $(tmux -V)"

python3 -c 'import json; assert json.loads("{\"ok\": true}")["ok"]' >/dev/null 2>&1 \
  || die "working python3 with JSON support required — native hook payloads and optional context lights use it"

# The whole tree is the tool: bin/gang reads collars/ relative to itself.
if [ -d "$HOME_DIR/.git" ]; then
  state="$(git -C "$HOME_DIR" status --porcelain)" \
    || die "could not inspect the existing install at $HOME_DIR"
  [ -z "$state" ] || die "$HOME_DIR has local changes; move them aside before upgrading"
  echo "installing $tag over $HOME_DIR"
  shallow="$(git -C "$HOME_DIR" rev-parse --is-shallow-repository)" \
    || die "could not determine whether $HOME_DIR is shallow"
  case "$shallow" in
    true)
      git -C "$HOME_DIR" fetch --depth 1 --quiet "$REPO" "refs/tags/$tag" \
        || die "could not fetch $tag from $REPO"
      ;;
    false)
      git -C "$HOME_DIR" fetch --quiet "$REPO" "refs/tags/$tag" \
        || die "could not fetch $tag from $REPO"
      ;;
    *) die "could not interpret the shallow-repository state '$shallow' for $HOME_DIR" ;;
  esac
  git -C "$HOME_DIR" checkout --detach --quiet FETCH_HEAD \
    || die "could not check out release $tag in $HOME_DIR"
else
  echo "installing $tag into $HOME_DIR"
  mkdir -p "$(dirname "$HOME_DIR")"
  # A release is a tag, so the clone lands on a detached HEAD and git explains
  # that at length. --quiet does not cover the advice; only turning it off does.
  git -c advice.detachedHead=false clone --branch "$tag" --depth 1 --quiet \
    "$REPO" "$HOME_DIR" \
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
