#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

mode=full
if [ "$#" -ne 0 ]; then
  if [ "$#" -ne 1 ] || [ "$1" != --push ]; then
    echo 'usage: test/go.sh [--push]' >&2
    exit 2
  fi
  mode=push
fi

command -v go >/dev/null 2>&1 || {
  echo "go: Go is required to verify this tree" >&2
  exit 1
}

unformatted=""
while IFS= read -r file; do
  needs_format="$(gofmt -l "$file")"
  [ -z "$needs_format" ] \
    || unformatted="${unformatted}${unformatted:+
}${needs_format}"
done < <(git ls-files --cached --others --exclude-standard -- '*.go')
if [ -n "$unformatted" ]; then
  printf '%s\n' "go: gofmt required:" "$unformatted" >&2
  exit 1
fi
echo "go: gofmt passed"

go vet ./...
echo "go: vet passed"

go tool go-check-sumtype -default-signifies-exhaustive=false ./...
echo "go: sum types are exhaustive"

module="$(go list -m)"
for package in substrate harness; do
  imports="$(go list -deps -f '{{.ImportPath}}' "./$package")"
  if printf '%s\n' "$imports" | grep -Fx "$module/core" >/dev/null; then
    echo "go: $package must not import core" >&2
    exit 1
  fi
done
echo "go: substrate and harness do not import core"

packages="$(go list ./...)"
unit_packages=()
while IFS= read -r package; do
  case "$package" in
    */acceptance) ;;
    *) unit_packages+=("$package") ;;
  esac
done <<< "$packages"

if [ "$mode" = push ]; then
  go test -count=1 -timeout=90s "${unit_packages[@]}"
else
  test/distribution.sh
  skip_reason=""
  if [ "$(uname -s)" = Linux ]; then
    if ! command -v systemctl >/dev/null 2>&1; then
      skip_reason='systemctl is unavailable'
    elif bus_output="$(systemctl --user show-environment 2>&1)"; then
      :
    else
      skip_reason="systemctl --user show-environment failed: $bus_output"
    fi
  fi
  if [ -n "$skip_reason" ]; then
    printf 'go: bus-dependent acceptance SKIP: no reachable systemd user bus (%s)\n' "$skip_reason"
    go test -count=1 -timeout=90s -skip '^Test(AdoptOwnsExistingPrivatePane|CommandLifecycleOnPrivateTmux|LiveClaudeCode|LiveCodex)$' ./...
  else
    go test -count=1 -timeout=90s -skip '^TestLive(ClaudeCode|Codex)$' ./...
    GANGLINE_LIVE_PROVIDERS=1 go test -v -count=1 -timeout=175s -run '^TestLive(ClaudeCode|Codex)$' ./acceptance
  fi
fi
echo "go: tests passed"
