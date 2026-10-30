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

go test -count=1 -timeout=0 ./...
passed "deterministic tests passed"
echo "go: all checks passed in ${SECONDS}s"
