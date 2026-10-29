#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Stages the gangline-team kit with static gang binaries in its build context,
# ready for `sbx run` or `docker buildx build`. Each binary is given as
# ARCH=PATH for a target architecture the image is built for.
set -eu

[ "$#" -ge 2 ] || { echo 'usage: stage.sh DIR ARCH=GANG_BINARY...' >&2; exit 2; }
dir=$1
shift
[ ! -e "$dir" ] || { echo "already exists: $dir" >&2; exit 2; }
for pair in "$@"; do
  case "$pair" in
    amd64=*|arm64=*) [ -f "${pair#*=}" ] || { echo "no such gang binary: ${pair#*=}" >&2; exit 2; } ;;
    *) echo "expected amd64=PATH or arm64=PATH: $pair" >&2; exit 2 ;;
  esac
done

cp -R "$(dirname "$0")/gangline-team" "$dir"
for pair in "$@"; do
  install -m 0755 "${pair#*=}" "$dir/gang-${pair%%=*}"
done
