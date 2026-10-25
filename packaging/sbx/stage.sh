#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Stages the gangline-team kit with a gang binary in its build context, ready
# for `sbx run`.
set -eu

[ "$#" -eq 2 ] || { echo 'usage: stage.sh GANG_BINARY DIR' >&2; exit 2; }
gang=$1
dir=$2
[ -x "$gang" ] || { echo "not an executable gang binary: $gang" >&2; exit 2; }
[ ! -e "$dir" ] || { echo "already exists: $dir" >&2; exit 2; }

cp -R "$(dirname "$0")/gangline-team" "$dir"
install -m 0755 "$gang" "$dir/gang"
