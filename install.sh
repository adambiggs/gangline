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
#   GANGLINE_RELEASE_BASE_URL  release assets root (default: GitHub releases)
#   GANGLINE_HOME  where the tree lives   (default: ~/.local/share/gangline)
#   GANGLINE_BIN   where `gang` is linked (default: ~/.local/bin)
set -eu

REPO="${GANGLINE_REPO:-https://github.com/adambiggs/gangline.git}"
HOME_DIR="${GANGLINE_HOME:-$HOME/.local/share/gangline}"
BIN_DIR="${GANGLINE_BIN:-$HOME/.local/bin}"

die() { echo "gangline: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required but not installed"; }
valid_number() {
  case "$1" in
    ''|*[!0123456789]*|0?*) return 1 ;;
    *) return 0 ;;
  esac
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
decimal_relation() {
  left=$1 right=$2
  left_length=${#left} right_length=${#right}
  if [ "$left_length" -gt "$right_length" ]; then
    printf '1\n'
    return
  elif [ "$left_length" -lt "$right_length" ]; then
    printf '%s\n' '-1'
    return
  fi
  while [ -n "$left" ]; do
    left_digit=${left%"${left#?}"}
    right_digit=${right%"${right#?}"}
    if [ "$left_digit" -gt "$right_digit" ]; then
      printf '1\n'
      return
    elif [ "$left_digit" -lt "$right_digit" ]; then
      printf '%s\n' '-1'
      return
    fi
    left=${left#?}
    right=${right#?}
  done
  printf '0\n'
}
semver_relation() {
  parse_semver "$1" || return 1
  left_major=$SEMVER_MAJOR left_minor=$SEMVER_MINOR left_patch=$SEMVER_PATCH
  parse_semver "$2" || return 1
  for pair in \
    "$left_major:$SEMVER_MAJOR" \
    "$left_minor:$SEMVER_MINOR" \
    "$left_patch:$SEMVER_PATCH"; do
    left_number=${pair%%:*}
    right_number=${pair#*:}
    component_relation="$(decimal_relation "$left_number" "$right_number")"
    if [ "$component_relation" -ne 0 ]; then
      printf '%s\n' "$component_relation"
      return
    fi
  done
  printf '0\n'
}

need git
latest_release_tag() {
  refs="$(git ls-remote --refs --tags "$REPO" 'refs/tags/gangline-v*')" \
    || die "could not read release tags from $REPO"
  best_tag=""
  best_version=""
  while read -r _ ref; do
    case "$ref" in refs/tags/gangline-v*) ;; *) continue ;; esac
    release="${ref#refs/tags/gangline-v}"
    parse_semver "$release" || continue
    if [ -z "$best_tag" ] \
      || [ "$(semver_relation "$release" "$best_version")" -gt 0 ]; then
      best_tag="gangline-v$release"
      best_version=$release
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
  relation="$(semver_relation "$1" "$2")" || return 1
  distance=current
  if [ "$current_major" != "$latest_major" ]; then
    distance=major
  elif [ "$current_minor" != "$latest_minor" ]; then
    distance=minor
  elif [ "$current_patch" != "$latest_patch" ]; then
    distance='patch'
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

# Keep the tagged source so upgrades remain inspectable and reproducible.
if [ -L "$HOME_DIR" ] || { [ -e "$HOME_DIR" ] && [ ! -d "$HOME_DIR/.git" ]; }; then
  die "$HOME_DIR exists and is not an installer-managed release"
fi
if [ -d "$HOME_DIR/.git" ]; then
  state="$(git -C "$HOME_DIR" status --porcelain)" \
    || die "could not inspect the existing install at $HOME_DIR"
  [ -z "$state" ] || die "$HOME_DIR has local changes; move them aside before upgrading"
  echo "installing $tag over $HOME_DIR"
else
  echo "installing $tag into $HOME_DIR"
fi

home_parent="$(dirname "$HOME_DIR")"
mkdir -p "$home_parent" "$BIN_DIR"
stage_root="$(mktemp -d "$home_parent/.gangline-install.XXXXXX")" \
  || die "could not create an installation stage beside $HOME_DIR"
cleanup_stage() {
  [ -z "$stage_root" ] || rm -rf "$stage_root"
}
trap cleanup_stage EXIT
trap 'cleanup_stage; exit 1' HUP INT TERM

# Clone and validate the selected tag away from the active checkout. A failed
# build or smoke test therefore leaves the installed command
# and retained source untouched.
candidate="$stage_root/release"
git -c advice.detachedHead=false clone --branch "$tag" --depth 1 --quiet \
  "$REPO" "$candidate" \
  || die "could not clone release $tag from $REPO"

mkdir -p "$BIN_DIR"
if [ -L "$BIN_DIR/gang" ] && [ -d "$BIN_DIR/gang" ]; then
  die "$BIN_DIR/gang is a symlink to a directory — move it aside"
fi
if [ -e "$BIN_DIR/gang" ] && [ ! -f "$BIN_DIR/gang" ] && [ ! -L "$BIN_DIR/gang" ]; then
  die "$BIN_DIR/gang exists and is not a file or a symlink — move it aside"
fi
host_os="$(uname -s)"
host_arch="$(uname -m)"
case "$host_os" in Linux) asset_os=linux ;; Darwin) asset_os=darwin ;; *) asset_os="" ;; esac
case "$host_arch" in
  x86_64|amd64) asset_arch=amd64 ;;
  aarch64|arm64) asset_arch=arm64 ;;
  *) asset_arch="" ;;
esac
release_base="${GANGLINE_RELEASE_BASE_URL:-}"
if [ -z "$release_base" ]; then
  case "$REPO" in
    https://github.com/*) release_base="${REPO%.git}/releases/download" ;;
  esac
fi

candidate_command="$stage_root/gang"
asset_available=0
if [ -n "$asset_os" ] && [ -n "$asset_arch" ] && [ -n "$release_base" ]; then
  need curl
  asset="$tag-$asset_os-$asset_arch"
  asset_url="${release_base%/}/$tag/$asset"
  http_status="$(curl -L -sS -o "$candidate_command" -w '%{http_code}' "$asset_url")" \
    || die "could not download $asset_url"
  case "$http_status" in
    200)
      sums="$stage_root/SHA256SUMS"
      curl -fLsS -o "$sums" "${release_base%/}/$tag/SHA256SUMS" \
        || die "could not download checksums for $tag"
      expected="$(awk -v name="$asset" '$2 == name { print $1 }' "$sums")"
      case "$expected" in
        *[!0123456789abcdef]*|'') die "invalid checksum for $asset" ;;
      esac
      [ "${#expected}" -eq 64 ] || die "invalid checksum for $asset"
      if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$candidate_command" | awk '{ print $1 }')"
      elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "$candidate_command" | awk '{ print $1 }')"
      else
        die "sha256sum or shasum is required to verify $asset"
      fi
      [ "$actual" = "$expected" ] || die "checksum mismatch for $asset"
      chmod +x "$candidate_command"
      asset_available=1
      echo "verified prebuilt $asset"
      ;;
    404) rm -f "$candidate_command" ;;
    *) die "could not download $asset_url (HTTP $http_status)" ;;
  esac
fi

if [ "$asset_available" -eq 1 ]; then
  release_kind=compiled
elif [ -f "$candidate/go.mod" ] && [ -d "$candidate/cmd/gang" ]; then
  release_kind=compiled
  echo "no prebuilt asset for $host_os/$host_arch; building $tag from source (Go required)"
  need go
  CGO_ENABLED=0 go -C "$candidate" build -trimpath \
    -ldflags "-s -w -X main.version=$latest" -o "$candidate_command" ./cmd/gang \
    || die "could not build gang $tag"
elif [ -x "$candidate/bin/gang" ] && [ -r "$candidate/version.txt" ]; then
  release_kind=retained-tree
  candidate_command="$candidate/bin/gang"
  need python3
  python3 -c 'import json; assert json.loads("{\"ok\": true}")["ok"]' >/dev/null 2>&1 \
    || die "working python3 with JSON support required by $tag"
  legacy_version="$(cat "$candidate/version.txt")" \
    || die "could not read the release version at $candidate/version.txt"
  [ "$legacy_version" = "$latest" ] \
    || die "$tag has mismatched version.txt value '$legacy_version'"
else
  die "$tag has an unsupported release layout"
fi

candidate_version="$("$candidate_command" --version)" \
  || die "$tag failed its staged 'gang --version' check"
[ "$candidate_version" = "gangline $latest" ] \
  || die "$tag version mismatch: expected 'gangline $latest', got '$candidate_version'"
"$candidate_command" collars >/dev/null \
  || die "$tag failed its staged 'gang collars' check"

previous="$stage_root/previous"
had_previous=0
if [ -d "$HOME_DIR/.git" ]; then
  mv "$HOME_DIR" "$previous" \
    || die "could not preserve the installed release at $HOME_DIR"
  had_previous=1
fi
if ! mv "$candidate" "$HOME_DIR"; then
  if [ "$had_previous" -eq 1 ]; then
    mv "$previous" "$HOME_DIR" \
      || die "could not restore the installed release at $HOME_DIR"
  fi
  die "could not activate $tag at $HOME_DIR"
fi

mkdir -p "$BIN_DIR"
activation_ok=1
case "$release_kind" in
  compiled)
    mv -f "$candidate_command" "$BIN_DIR/gang" || activation_ok=0
    ;;
  retained-tree)
    if [ "$BIN_DIR/gang" != "$HOME_DIR/bin/gang" ]; then
      new_link="$BIN_DIR/.gang.new.$$"
      ln -s "$HOME_DIR/bin/gang" "$new_link" \
        && mv -f "$new_link" "$BIN_DIR/gang" \
        || activation_ok=0
      [ "$activation_ok" -eq 1 ] || rm -f "$new_link"
    fi
    ;;
esac
if [ "$activation_ok" -ne 1 ]; then
  failed_candidate="$stage_root/failed-release"
  if [ "$had_previous" -eq 1 ] \
    && mv "$HOME_DIR" "$failed_candidate" \
    && mv "$previous" "$HOME_DIR"; then
    die "could not install $BIN_DIR/gang; the previous release was preserved"
  fi
  die "could not install $BIN_DIR/gang or restore the previous release"
fi

# Execute through the installed path before reporting success.
installed_version="$("$BIN_DIR/gang" --version)" \
  || die "installed, but '$BIN_DIR/gang --version' failed"
[ "$installed_version" = "gangline $latest" ] \
  || die "installed version mismatch: expected 'gangline $latest', got '$installed_version'"
if [ "$release_kind" = compiled ]; then
  "$BIN_DIR/gang" statusline --install \
    || die "installed, but status-line settings installation failed"
fi
"$BIN_DIR/gang" collars >/dev/null \
  || die "installed, but 'gang collars' failed"

cleanup_stage
stage_root=""

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
