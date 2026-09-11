#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Source this file in a same-name test PATH shim, then call
# path_shim_guard TARGET "$0" LABEL before the shim invokes TARGET.

path_shim_guard() {
  gl_path_shim_target="${1-}"
  gl_path_shim_self="${2-}"
  gl_path_shim_label="${3:-path-shim}"

  case "$gl_path_shim_target" in
    /*) ;;
    *)
      printf '%s: target is not absolute: %s\n' \
        "$gl_path_shim_label" "${gl_path_shim_target:-<empty>}" >&2
      return 97
      ;;
  esac
  if [ ! -x "$gl_path_shim_target" ]; then
    printf '%s: target is not executable: %s\n' \
      "$gl_path_shim_label" "$gl_path_shim_target" >&2
    return 97
  fi

  case "$gl_path_shim_self" in
    /*) ;;
    */*)
      gl_path_shim_dir="$(cd -P "$(dirname "$gl_path_shim_self")" 2>/dev/null && pwd)" \
        || gl_path_shim_dir=""
      [ -n "$gl_path_shim_dir" ] \
        && gl_path_shim_self="$gl_path_shim_dir/$(basename "$gl_path_shim_self")"
      ;;
    *)
      gl_path_shim_found="$(command -v "$gl_path_shim_self" 2>/dev/null)" \
        || gl_path_shim_found=""
      [ -n "$gl_path_shim_found" ] && gl_path_shim_self="$gl_path_shim_found"
      ;;
  esac
  if [ -z "$gl_path_shim_self" ] || {
    [ ! -e "$gl_path_shim_self" ] && [ ! -L "$gl_path_shim_self" ]
  }; then
    printf '%s: shim identity is unreadable: %s\n' \
      "$gl_path_shim_label" "${gl_path_shim_self:-<empty>}" >&2
    return 97
  fi
  # POSIX leaves -ef unspecified, but every sh that sources this guard
  # implements it (dash, bash, busybox ash), and the alternatives compare
  # paths or inodes alone and would miss a hard link or a cross-device match.
  # shellcheck disable=SC3013
  if [ "$gl_path_shim_target" -ef "$gl_path_shim_self" ]; then
    printf '%s: target resolves to the shim itself: %s\n' \
      "$gl_path_shim_label" "$gl_path_shim_target" >&2
    return 97
  fi

  gl_path_shim_depth="${GANG_TEST_PATH_SHIM_DEPTH:-0}"
  case "$gl_path_shim_depth" in
    ''|*[!0-9]*)
      printf '%s: GANG_TEST_PATH_SHIM_DEPTH is not a count: %s\n' \
        "$gl_path_shim_label" "$gl_path_shim_depth" >&2
      return 97
      ;;
  esac
  if [ "$gl_path_shim_depth" -ge 8 ]; then
    printf '%s: depth %s reached the ceiling; refusing to recurse\n' \
      "$gl_path_shim_label" "$gl_path_shim_depth" >&2
    return 97
  fi
  GANG_TEST_PATH_SHIM_DEPTH=$((gl_path_shim_depth + 1))
  export GANG_TEST_PATH_SHIM_DEPTH
}
