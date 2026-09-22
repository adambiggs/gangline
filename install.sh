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
valid_number() {
  case "$1" in 0|[1-9][0-9]*) return 0 ;; *) return 1 ;; esac
}
parse_semver() {
  SEMVER_MAJOR="${1%%.*}"
  rest="${1#*.}"
  [ "$rest" != "$1" ] || return 1
  SEMVER_MINOR="${rest%%.*}"
  SEMVER_PATCH="${rest#*.}"
  [ "$SEMVER_PATCH" != "$rest" ] || return 1
  case "$SEMVER_PATCH" in *.*) return 1 ;; esac
  valid_number "$SEMVER_MAJOR" && valid_number "$SEMVER_MINOR" \
    && valid_number "$SEMVER_PATCH"
}

need git
need go

latest_release_tag() {
  refs="$(git ls-remote --refs --tags "$REPO" 'refs/tags/gangline-v*')" \
    || die "could not read release tags from $REPO"
  best_tag=""
  best_major=0 best_minor=0 best_patch=0
  while read -r _ ref; do
    case "$ref" in refs/tags/gangline-v*) ;; *) continue ;; esac
    release="${ref#refs/tags/gangline-v}"
    parse_semver "$release" || continue
    major=$SEMVER_MAJOR minor=$SEMVER_MINOR patch=$SEMVER_PATCH
    if [ -z "$best_tag" ] \
      || [ "$major" -gt "$best_major" ] \
      || { [ "$major" -eq "$best_major" ] && [ "$minor" -gt "$best_minor" ]; } \
      || { [ "$major" -eq "$best_major" ] && [ "$minor" -eq "$best_minor" ] && [ "$patch" -gt "$best_patch" ]; }; then
      best_tag="gangline-v$release"
      best_major=$major best_minor=$minor best_patch=$patch
    fi
  done <<EOF
$refs
EOF
  [ -n "$best_tag" ] \
    || die "could not determine a stable gangline-vMAJOR.MINOR.PATCH release tag from $REPO"
  printf '%s\n' "$best_tag"
}

installed_release_version() {
  [ -x "$BIN_DIR/gang" ] \
    || die "installed version is unknown: cannot execute $BIN_DIR/gang"
  installed="$("$BIN_DIR/gang" --version)" \
    || die "installed version is unknown: $BIN_DIR/gang --version failed"
  current="${installed#gangline }"
  [ "$installed" = "gangline $current" ] \
    || die "installed version is malformed: $BIN_DIR/gang --version returned '$installed'"
  printf '%s\n' "$current"
}

release_relation() { # current latest -> relation and the changed semver component
  parse_semver "$1" || return 1
  current_major=$SEMVER_MAJOR current_minor=$SEMVER_MINOR current_patch=$SEMVER_PATCH
  parse_semver "$2" || return 1
  latest_major=$SEMVER_MAJOR latest_minor=$SEMVER_MINOR latest_patch=$SEMVER_PATCH
  relation=0 distance=current
  if [ "$current_major" -ne "$latest_major" ]; then
    distance=major
  elif [ "$current_minor" -ne "$latest_minor" ]; then
    distance=minor
  elif [ "$current_patch" -ne "$latest_patch" ]; then
    distance='patch'
  fi
  if [ "$current_major" -gt "$latest_major" ] \
    || { [ "$current_major" -eq "$latest_major" ] && [ "$current_minor" -gt "$latest_minor" ]; } \
    || { [ "$current_major" -eq "$latest_major" ] && [ "$current_minor" -eq "$latest_minor" ] && [ "$current_patch" -gt "$latest_patch" ]; }; then
    relation=1
  elif [ "$current_major" -lt "$latest_major" ] \
    || { [ "$current_major" -eq "$latest_major" ] && [ "$current_minor" -lt "$latest_minor" ]; } \
    || { [ "$current_major" -eq "$latest_major" ] && [ "$current_minor" -eq "$latest_minor" ] && [ "$current_patch" -lt "$latest_patch" ]; }; then
    relation=-1
  fi
  printf '%s %s\n' "$relation" "$distance"
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

# Keep the tagged source so upgrades remain inspectable and reproducible. The
# installed command itself is one static binary and has no runtime tree.
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
if [ -e "$BIN_DIR/gang" ] && [ ! -f "$BIN_DIR/gang" ] && [ ! -L "$BIN_DIR/gang" ]; then
  die "$BIN_DIR/gang exists and is not a file or a symlink — move it aside"
fi
new_binary="$BIN_DIR/.gang.new.$$"
trap 'rm -f "$new_binary"' EXIT HUP INT TERM
CGO_ENABLED=0 go -C "$HOME_DIR" build -trimpath \
  -ldflags "-s -w -X main.version=$latest" -o "$new_binary" ./cmd/gang \
  || die "could not build gang $tag"
mv -f "$new_binary" "$BIN_DIR/gang" \
  || die "could not install $BIN_DIR/gang"
trap - EXIT HUP INT TERM

# Repair the retired checkout status-line path without replacing custom settings.
"$BIN_DIR/gang" statusline --install || die "installed, but status-line settings repair failed"

# Execute the installed binary before reporting success.
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
