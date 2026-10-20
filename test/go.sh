#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ "$#" -ne 0 ]; then
  echo 'usage: test/go.sh' >&2
  exit 2
fi

# Each part reports its own wall time, and the last line the total.
part_start=$SECONDS
passed() {
  echo "go: $1 ($((SECONDS - part_start))s)"
  part_start=$SECONDS
}

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
passed "gofmt passed"

go vet ./...
passed "vet passed"

go tool go-check-sumtype -default-signifies-exhaustive=false ./...
passed "sum types are exhaustive"

module="$(go list -m)"
for package in substrate harness; do
  imports="$(go list -deps -f '{{.ImportPath}}' "./$package")"
  if printf '%s\n' "$imports" | grep -Fx "$module/core" >/dev/null; then
    echo "go: $package must not import core" >&2
    exit 1
  fi
done
passed "substrate and harness do not import core"

test/distribution.sh
passed "distribution passed"
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
  go test -count=1 -timeout=90s -skip '^Test(CommandLifecycleOnPrivateTmux|LiveClaudeCode|LiveCodex)$' ./...
  passed "tests passed"
else
  go test -count=1 -timeout=90s -skip '^TestLive(ClaudeCode|Codex)$' ./...
  passed "tests passed"
  GANGLINE_LIVE_PROVIDERS=1 go test -v -count=1 -timeout=175s -run '^TestLive(ClaudeCode|Codex)$' ./acceptance
  passed "live provider checks passed"
fi
echo "go: all checks passed in ${SECONDS}s"
