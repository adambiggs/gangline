#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/gangline-distribution.XXXXXX")"
cleanup() {
  if [ -n "${asset_server_pid:-}" ]; then
    kill "$asset_server_pid"
    wait "$asset_server_pid" || :
  fi
  case "$scratch" in
    "${TMPDIR:-/tmp}"/gangline-distribution.*) rm -rf -- "$scratch" ;;
    *) printf 'distribution: refusing to remove unexpected scratch path %s\n' "$scratch" >&2 ;;
  esac
}
trap cleanup EXIT HUP INT TERM

export GIT_CONFIG_GLOBAL="$scratch/gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export GOPROXY=off
export GOSUMDB=off

mkdir -p "$scratch/shims"
cat >"$scratch/shims/tmux" <<'EOF'
#!/bin/sh
[ "$#" -eq 1 ] && [ "$1" = -V ] || exit 2
printf 'tmux 3.2\n'
EOF
chmod +x "$scratch/shims/tmux"

init_repo() {
  local repo=$1
  git init -q "$repo"
  git -C "$repo" config user.name 'Gangline Test'
  git -C "$repo" config user.email 'test@example.invalid'
}

commit_release() {
  local repo=$1 version=$2
  git -C "$repo" add .
  git -C "$repo" commit -qm "test: fixture release $version"
  git -C "$repo" tag "gangline-v$version"
}

run_install() {
  local repo=$1 install_root=$2 output=$3
  mkdir -p "$install_root"
    GANGLINE_REPO="file://$repo" \
    GANGLINE_HOME="$install_root/.local/share/gangline" \
    GANGLINE_BIN="$install_root/.local/bin" \
    PATH="$scratch/shims:$PATH" \
    /bin/sh "$ROOT/install.sh" >"$output"
}

legacy_repo="$scratch/legacy-repo"
init_repo "$legacy_repo"
mkdir -p "$legacy_repo/bin"
cat >"$legacy_repo/bin/gang" <<'EOF'
#!/bin/sh
case "${1:-}" in
  --version) printf 'gangline 0.9.0\n' ;;
  collars) printf 'codex\n' ;;
  *) exit 2 ;;
esac
EOF
chmod +x "$legacy_repo/bin/gang"
printf '0.9.0\n' >"$legacy_repo/version.txt"
commit_release "$legacy_repo" 0.9.0
git -C "$legacy_repo" tag gangline-v01.0.0
git -C "$legacy_repo" tag gangline-v0.9.0-rc1
git -C "$legacy_repo" tag gangline-v10x.20.30

legacy_home="$scratch/legacy-home"
run_install "$legacy_repo" "$legacy_home" "$scratch/legacy-install.out"
legacy_version="$("$legacy_home/.local/bin/gang" --version)"
[ "$legacy_version" = 'gangline 0.9.0' ] || {
  printf 'distribution: legacy install returned %s\n' "$legacy_version" >&2
  exit 1
}
printf 'distribution: legacy release install passed (gangline 0.9.0)\n'

git -C "$legacy_repo" tag gangline-v2.0.0
if run_install "$legacy_repo" "$legacy_home" "$scratch/legacy-unknown.out" \
  2>"$scratch/legacy-unknown.err"; then
  echo 'distribution: unknown upgrade layout replaced the legacy install' >&2
  exit 1
fi
grep -F "mismatched version.txt value '0.9.0'" "$scratch/legacy-unknown.err" >/dev/null
[ "$("$legacy_home/.local/bin/gang" --version)" = 'gangline 0.9.0' ] || {
  echo 'distribution: unknown upgrade layout damaged the legacy command' >&2
  exit 1
}
git -C "$legacy_repo" tag -d gangline-v2.0.0 >/dev/null

rm -f "$legacy_repo/bin/gang" "$legacy_repo/version.txt"
mkdir -p "$legacy_repo/cmd/gang"
cat >"$legacy_repo/go.mod" <<'EOF'
module example.invalid/gangline-transition-fixture

go 1.23
EOF
printf 'package main\nfunc main( {\n' >"$legacy_repo/cmd/gang/main.go"
git -C "$legacy_repo" add -A
git -C "$legacy_repo" commit -qm 'test: broken compiled release'
git -C "$legacy_repo" tag gangline-v1.0.0
if run_install "$legacy_repo" "$legacy_home" "$scratch/legacy-build-failure.out" \
  2>"$scratch/legacy-build-failure.err"; then
  echo 'distribution: failed compiled build replaced the legacy install' >&2
  exit 1
fi
grep -F 'could not build gang gangline-v1.0.0' "$scratch/legacy-build-failure.err" >/dev/null
[ "$("$legacy_home/.local/bin/gang" --version)" = 'gangline 0.9.0' ] || {
  echo 'distribution: failed compiled build damaged the legacy command' >&2
  exit 1
}

cat >"$legacy_repo/cmd/gang/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}
	switch os.Args[1] {
	case "--version":
		if len(os.Args) != 2 {
			os.Exit(2)
		}
		fmt.Printf("gangline %s\n", version)
	case "collars":
		if len(os.Args) != 2 {
			os.Exit(2)
		}
		fmt.Println("codex")
	case "statusline":
		if len(os.Args) != 3 || os.Args[2] != "--install" {
			os.Exit(2)
		}
	default:
		os.Exit(2)
	}
}
EOF
git -C "$legacy_repo" add cmd/gang/main.go
git -C "$legacy_repo" commit -qm 'test: compiled transition release'
git -C "$legacy_repo" tag -d gangline-v1.0.0 >/dev/null
git -C "$legacy_repo" tag gangline-v1.0.0
run_install "$legacy_repo" "$legacy_home" "$scratch/legacy-transition.out"
[ "$("$legacy_home/.local/bin/gang" --version)" = 'gangline 1.0.0' ] || {
  echo 'distribution: current bootstrap did not migrate the legacy install' >&2
  exit 1
}
[ ! -L "$legacy_home/.local/bin/gang" ] || {
  echo 'distribution: legacy migration left a retained-tree symlink' >&2
  exit 1
}
printf 'distribution: failed upgrades preserve legacy; bootstrap migration passed\n'

go_repo="$scratch/go-repo"
init_repo "$go_repo"
mkdir -p "$go_repo/cmd/gang"
cat >"$go_repo/go.mod" <<'EOF'
module example.invalid/gangline-fixture

go 1.23
EOF
cat >"$go_repo/cmd/gang/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}
	switch os.Args[1] {
	case "--version":
		if len(os.Args) != 2 {
			os.Exit(2)
		}
		fmt.Printf("gangline %s\n", version)
	case "collars":
		if len(os.Args) != 2 {
			os.Exit(2)
		}
		fmt.Println("codex")
	case "statusline":
		if len(os.Args) != 3 || os.Args[2] != "--install" {
			os.Exit(2)
		}
	default:
		os.Exit(2)
	}
}
EOF
commit_release "$go_repo" 1.0.0
git -C "$go_repo" tag gangline-v01.0.0
git -C "$go_repo" tag gangline-v1.0.0-rc1
git -C "$go_repo" tag gangline-v10x.20.30

directory_link_home="$scratch/directory-link-home"
mkdir -p "$directory_link_home/.local/bin" "$scratch/directory-link-target"
ln -s "$scratch/directory-link-target" "$directory_link_home/.local/bin/gang"
if run_install "$go_repo" "$directory_link_home" "$scratch/directory-link.out" \
  2>"$scratch/directory-link.err"; then
  echo 'distribution: installer accepted a gang symlink to a directory' >&2
  exit 1
fi
grep -F 'gang is a symlink to a directory' "$scratch/directory-link.err" >/dev/null
[ -z "$(find "$scratch/directory-link-target" -mindepth 1 -print -quit)" ] || {
  echo 'distribution: installer wrote through a gang symlink to a directory' >&2
  exit 1
}
printf 'distribution: destination directory symlink refusal passed\n'

go_home="$scratch/go-home"
run_install "$go_repo" "$go_home" "$scratch/go-install.out"
go_version="$("$go_home/.local/bin/gang" --version)"
[ "$go_version" = 'gangline 1.0.0' ] || {
  printf 'distribution: Go install returned %s\n' "$go_version" >&2
  exit 1
}
[ ! -L "$go_home/.local/bin/gang" ] || {
  echo 'distribution: Go install left a symlink instead of a binary' >&2
  exit 1
}
printf 'distribution: Go release install passed (gangline 1.0.0)\n'

large_version=99999999999999999999.0.0
git -C "$go_repo" tag "gangline-v$large_version"
  GANGLINE_REPO="file://$go_repo" \
  GANGLINE_HOME="$go_home/.local/share/gangline" \
  GANGLINE_BIN="$go_home/.local/bin" \
  PATH="$scratch/shims:$PATH" \
  /bin/sh "$ROOT/install.sh" --check >"$scratch/large-check.out"
grep -F "upgrade available: v1.0.0 -> v$large_version (major update)" \
  "$scratch/large-check.out" >/dev/null
large_home="$scratch/large-home"
run_install "$go_repo" "$large_home" "$scratch/large-install.out"
large_installed="$("$large_home/.local/bin/gang" --version)"
[ "$large_installed" = "gangline $large_version" ] || {
  printf 'distribution: large SemVer install returned %s\n' "$large_installed" >&2
  exit 1
}
printf 'distribution: large SemVer comparison passed (%s)\n' "$large_version"
git -C "$go_repo" tag -d "gangline-v$large_version" >/dev/null

unknown_repo="$scratch/unknown-repo"
init_repo "$unknown_repo"
printf 'unsupported\n' >"$unknown_repo/README"
commit_release "$unknown_repo" 2.0.0
if run_install "$unknown_repo" "$scratch/unknown-home" "$scratch/unknown-install.out" \
  2>"$scratch/unknown-install.err"; then
  echo 'distribution: installer accepted an unknown release layout' >&2
  exit 1
fi
grep -F 'unsupported release layout' "$scratch/unknown-install.err" >/dev/null
printf 'distribution: unknown release layout refusal passed\n'

case "$(uname -s)" in Linux) asset_os=linux ;; Darwin) asset_os=darwin ;; esac
case "$(uname -m)" in x86_64|amd64) asset_arch=amd64 ;; aarch64|arm64) asset_arch=arm64 ;; esac
asset_name="gangline-v1.0.0-$asset_os-$asset_arch"
asset_root="$scratch/assets"
asset_dir="$asset_root/gangline-v1.0.0"
mkdir -p "$asset_dir"
CGO_ENABLED=0 go -C "$go_repo" build -trimpath \
  -ldflags '-s -w -X main.version=1.0.0' -o "$asset_dir/$asset_name" ./cmd/gang
(
  cd "$asset_dir"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$asset_name" > SHA256SUMS
  else
    shasum -a 256 "$asset_name" > SHA256SUMS
  fi
)

port_fifo="$scratch/asset-port"
mkfifo "$port_fifo"
python3 - "$asset_root" "$port_fifo" <<'PY' &
import http.server
import os
import socketserver
import sys

os.chdir(sys.argv[1])
with socketserver.TCPServer(("127.0.0.1", 0), http.server.SimpleHTTPRequestHandler) as server:
    with open(sys.argv[2], "w", encoding="utf-8") as port_file:
        port_file.write(f"{server.server_address[1]}\n")
    server.serve_forever()
PY
asset_server_pid=$!
read -r asset_port <"$port_fifo"
asset_url="http://127.0.0.1:$asset_port"

http_fallback_home="$scratch/http-fallback-home"
GANGLINE_RELEASE_BASE_URL="$asset_url/missing" \
  run_install "$go_repo" "$http_fallback_home" "$scratch/http-fallback.out"
grep -F 'building gangline-v1.0.0 from source (Go required)' \
  "$scratch/http-fallback.out" >/dev/null

printf '%064d  %s\n' 0 "$asset_name" >"$asset_dir/SHA256SUMS"
if GANGLINE_RELEASE_BASE_URL="$asset_url" \
  run_install "$go_repo" "$scratch/http-corrupt-home" "$scratch/http-corrupt.out" \
  2>"$scratch/http-corrupt.err"; then
  echo 'distribution: installer accepted a mismatched release checksum' >&2
  exit 1
fi
grep -F "checksum mismatch for $asset_name" "$scratch/http-corrupt.err" >/dev/null
(
  cd "$asset_dir"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$asset_name" > SHA256SUMS
  else
    shasum -a 256 "$asset_name" > SHA256SUMS
  fi
)

cat >"$scratch/shims/go" <<'EOF'
#!/bin/sh
echo 'distribution: Go was called on the binary install path' >&2
exit 99
EOF
chmod +x "$scratch/shims/go"
http_binary_home="$scratch/http-binary-home"
GANGLINE_RELEASE_BASE_URL="$asset_url" \
  run_install "$go_repo" "$http_binary_home" "$scratch/http-binary.out"
grep -F "verified prebuilt $asset_name" "$scratch/http-binary.out" >/dev/null
[ "$("$http_binary_home/.local/bin/gang" --version)" = 'gangline 1.0.0' ]
printf 'distribution: HTTP release asset install passed without Go\n'

title="$scratch/title"
printf 'feat!: replace the public command\n' >"$title"
"$ROOT/.githooks/commit-msg" --subject-only "$title"
printf 'chore(main): release gangline 1.0.0\n' >"$title"
"$ROOT/.githooks/commit-msg" --subject-only "$title"
for exempt in \
  'Merge branch main' \
  'fixup! fix(installer): build the release' \
  'squash! fix(installer): build the release' \
  'amend! fix(installer): build the release'; do
  printf '%s\n' "$exempt" >"$title"
  if "$ROOT/.githooks/commit-msg" --subject-only "$title" >"$scratch/title.out" 2>"$scratch/title.err"; then
    printf 'distribution: PR title validator accepted %s\n' "$exempt" >&2
    exit 1
  fi
done

printf 'feat!: replace the public command\n' >"$title"
if "$ROOT/.githooks/commit-msg" "$title" >"$scratch/message.out" 2>"$scratch/message.err"; then
  echo 'distribution: full commit validator accepted a missing breaking footer' >&2
  exit 1
fi
grep -F 'carries no BREAKING CHANGE: footer' "$scratch/message.err" >/dev/null
cat >"$title" <<'EOF'
feat!: replace the public command

BREAKING CHANGE: callers must use gang instead.
EOF
"$ROOT/.githooks/commit-msg" "$title"
printf 'distribution: commit and PR title validation passed\n'
