#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

[ "$#" -eq 2 ] || { echo 'usage: build-release.sh TAG OUTPUT_DIR' >&2; exit 2; }
tag=$1
output_dir=$2
case "$tag" in
  gangline-v*) version=${tag#gangline-v} ;;
  *) echo "invalid release tag: $tag" >&2; exit 2 ;;
esac
case "$version" in
  ''|*[!0-9.]*|.*|*..*|*.) echo "invalid release tag: $tag" >&2; exit 2 ;;
esac
[ "$(printf '%s' "$version" | tr -cd '.' | wc -c)" -eq 2 ] \
  || { echo "invalid release tag: $tag" >&2; exit 2; }

mkdir -p "$output_dir"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os=${target%/*}
  arch=${target#*/}
  asset="$tag-$os-$arch"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=$version" \
    -o "$output_dir/$asset" ./cmd/gang
done
(
  cd "$output_dir"
  sha256sum "$tag"-* > SHA256SUMS
)
