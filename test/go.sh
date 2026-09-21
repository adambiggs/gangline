#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

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

go test -count=1 -timeout=90s ./...
echo "go: tests passed"
