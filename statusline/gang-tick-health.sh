#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# tmux status-right segment: stay silent while the cooperative tick is healthy.
set -euo pipefail

health="${1:-}"
[ -n "$health" ] && [ -r "$health" ] || exit 0
IFS=$'\t' read -r state _ _ < "$health" || exit 0
[ "$state" = failed ] || exit 0
printf '#[fg=red,bold]tick!#[default]'
