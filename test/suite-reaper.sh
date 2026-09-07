# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
#
# THE TMUX SERVERS A RUN STARTS, ENDED WITH THAT RUN. A tmux server daemonises
# and reparents to init, so nothing about the shell that started it reaches it
# again. A run that tears its servers down from an EXIT trap therefore leaves
# every one of them behind whenever that trap does not run — a SIGKILL, a pane
# destroyed under the run, the kernel's OOM killer — and leaves behind any
# server whose socket the trap does not name, because a trap can only address
# the sockets its author wrote down. Each survivor holds a pty and its fixture
# shell until the host reboots.
#
# A RUN CLAIMS ITS DIRECTORY IN WRITING. suite_reaper_start writes a marker
# holding an unguessable token into the run's own directory, and only a
# directory carrying such a marker is ever swept — so no other directory on
# this host, the team's tmux directory included, is a candidate however it is
# spelled. Within the claimed directory the servers reached are the ones whose
# sockets are bound inside it. The token is also the generation: a watcher
# carries the token it wrote, so it declines a later run that remade the same
# path.
#
# suite_reaper_sweep is exact and synchronous and belongs in the run's own
# teardown. suite_reaper_start adds the half a trap cannot provide: a detached
# watcher holding a pidfd for the run, which sweeps when the kernel reports
# that process gone. It runs as a transient user service, outside both the
# caller's execution cgroup and any child PID namespace the caller occupies.
#
# A server whose socket lives outside the root — a `-L` label taken with
# TMUX_TMPDIR unset resolves under the host's own tmux directory — is invisible
# to that rule, so the run registers it with suite_reaper_track.
#
# THE WATCH IS EITHER PROVIDED OR REFUSED, NEVER ASSUMED. It rests on Linux
# pidfds, on a readable /proc/net/unix, and on a systemd user manager that can
# launch outside the run's execution boundary. A host missing any of them
# cannot be watched, and suite_reaper_start fails there and says which piece is
# absent, because a suite that believes it is protected and is not leaks in
# exactly the way this file exists to stop.

suite_reaper_program() { # stdout = the reaper this file ships beside
  # Builtins only: this is reached on the path that refuses a host missing the
  # tools the watch needs, where PATH may name nothing at all.
  printf '%s' "$(cd -P "${BASH_SOURCE[0]%/*}" && pwd)/suite-reaper.py"
}

# THE INTERPRETER IS RESOLVED ONCE, WHILE IT STILL RESOLVES. The suites give
# themselves a private $HOME and a private PATH, and the watcher runs after the
# run and its pinned interpreter directory are both gone, so the absolute path
# is captured before any of that and reused afterwards. The capture has to
# happen in the run's own shell: called for its output alone this function runs
# in a subshell, where an assignment reaches nothing.
suite_reaper_resolve_python() { # sets SUITE_REAPER_PYTHON in the caller's shell
  [ -n "${SUITE_REAPER_PYTHON:-}" ] && return 0
  SUITE_REAPER_PYTHON="$(suite_python3)" || return 1
  export SUITE_REAPER_PYTHON
}

suite_reaper_python() { # stdout = the interpreter the reaper runs under
  suite_reaper_resolve_python || return 1
  printf '%s' "$SUITE_REAPER_PYTHON"
}

suite_reaper_representable() { # $1 = a path the reaper would have to read back
  # The marker, the adopted-socket list and the kernel's own socket table are
  # all one record per line, so a path holding a newline cannot be read back
  # out of any of them and is refused here rather than silently going missing.
  case "$1" in
    '' | */../* | */..) return 1 ;;
    *$'\n'*) return 1 ;;
    /*) return 0 ;;
  esac
  return 1
}

suite_reaper_track() { # $1 = a socket path outside the run root this run owns
  local root="${SUITE_REAPER_ROOT:-}"
  [ -n "$root" ] || return 0
  if ! suite_reaper_representable "$1"; then
    printf 'suite: %q cannot be recorded as this run'"'"'s socket\n' "$1" >&2
    return 1
  fi
  printf '%s\n' "$1" >> "$root/.suite-reaper-extra"
}

suite_reaper_sweep() { # $1 = a run root -> its tmux servers and the root are gone
  local root="$1" python program
  [ -n "$root" ] || return 0
  python="$(suite_reaper_python)" || return 0
  program="$(suite_reaper_program)"
  # A SWEEP THAT REFUSED AND A SWEEP THAT FOUND NOTHING LOOK THE SAME once its
  # stderr is thrown away, and the first is a run whose servers are all still
  # standing. The teardown carries on either way, but it says so.
  "$python" "$program" --sweep "$root" \
    || printf 'suite: reaper sweep of %s exited %s\n' "$root" "$?" >&2
}

suite_reaper_claim() { # $1 = a directory this shell owns, stdout = its token
  local root="$1" python program
  suite_reaper_representable "$root" || return 1
  python="$(suite_reaper_python)" || return 1
  program="$(suite_reaper_program)"
  "$python" "$program" --claim "$root"
}

suite_reaper_start() { # $1 = the run root this shell owns
  local root="$1" python program parent_fd systemd error name
  local -a service_env=()
  if ! suite_reaper_representable "$root"; then
    printf 'suite: %q cannot be a run root the reaper reads back\n' "$root" >&2
    return 1
  fi
  systemd="$(command -v systemd-run)" || {
    printf 'suite: no systemd-run, so a watcher cannot leave this run'"'"'s execution boundary\n' >&2
    return 1
  }
  suite_reaper_resolve_python || return 1
  python="$SUITE_REAPER_PYTHON"
  program="$(suite_reaper_program)"
  "$python" "$program" --watchable || return 1
  SUITE_REAPER_TOKEN="$(suite_reaper_claim "$root")" || return 1
  [ -n "$SUITE_REAPER_TOKEN" ] || return 1
  SUITE_REAPER_ROOT="$root"
  export SUITE_REAPER_ROOT SUITE_REAPER_TOKEN
  # A PID NUMBER INSIDE A CHILD NAMESPACE DOES NOT NAME THIS PROCESS TO THE
  # host user manager. Holding a fresh inode open gives the service one exact
  # parent it can find through the host's /proc without translating a number.
  # The systemd-run client has that descriptor closed, so readiness cannot
  # deadlock on finding both the caller and the process launching its watcher.
  # The outer transient service moves the inner systemd-run client through the
  # user manager before that client creates the watcher. That bridge keeps both
  # the launcher and watcher out of a caller's cgroup or PID namespace; the
  # outer --wait still makes readiness and any launch refusal synchronous.
  rm -f "$root/.suite-reaper-watch-error"
  : > "$root/.suite-reaper-parent"
  exec {parent_fd}< "$root/.suite-reaper-parent"
  for name in SUITE_REAPER_DONE_SOCKET SUITE_REAPER_DONE_CHANNEL \
    SUITE_REAPER_TMUX SUITE_REAPER_LOG SUITE_REAPER_UNIX_SOCKETS; do
    [ -z "${!name:-}" ] || service_env+=("--setenv=$name=${!name}")
  done
  if ! "$systemd" --user --quiet --wait --pipe --collect --service-type=exec \
      "$systemd" --user --quiet --collect --service-type=notify \
      "${service_env[@]}" "$python" "$program" --watch \
      "$root" "$SUITE_REAPER_TOKEN" {parent_fd}<&-; then
    error="$(sed -n '1p' "$root/.suite-reaper-watch-error" 2>/dev/null)"
    [ -n "$error" ] || error="suite: the detached watcher service could not be started"
    printf '%s\n' "$error" >&2
    exec {parent_fd}<&-
    rm -f "$root/.suite-reaper" "$root/.suite-reaper-parent" \
      "$root/.suite-reaper-watch-error"
    SUITE_REAPER_ROOT=""
    SUITE_REAPER_TOKEN=""
    export SUITE_REAPER_ROOT SUITE_REAPER_TOKEN
    return 1
  fi
}
