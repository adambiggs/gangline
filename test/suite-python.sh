# shellcheck shell=bash
# SPDX-License-Identifier: Apache-2.0
#
# THE INTERPRETER, NOT THE NAME ON PATH. `python3` on a developer box is
# routinely a version-manager shim — asdf, pyenv, mise all install one — and a
# shim decides which interpreter to exec by reading a version file it finds by
# walking up from the working directory and then from $HOME. Nothing about that
# resolution survives a run that gives itself a private $HOME, which is exactly
# what test/integration.sh does: from that line on, every such shim exits 126
# with a message about a version that is not set.
#
# WHAT THAT COST. The failure surfaces as whichever python program the suite
# reached first, so on this shape of host the run ended at
# "libexec/gang-clock is absent or returned an unreadable value" — a file that
# is present, a value that was never produced, and no mention of python at all.
# Prepending an interpreter directory to PATH clears it, and a repair that
# lives outside the suite has to be found again on every host of this shape.
#
# THE RULE. Ask the shim, once, in the environment that still resolves — the
# caller's, before any fixture owns $HOME — which interpreter it would choose,
# and use that absolute path afterwards. `sys.executable` is a question a shim
# can answer, and a plain /usr/bin/python3 answers it with itself, so one rule
# covers a managed host and an unmanaged one without either being named here.
#
# AND RESOLVING IS NOT RUNNING. `command -v python3` succeeds against a shim
# with nothing behind it; test/lint.sh already learned that about shellcheck.
# So every branch below asks something to execute, and a failure names the
# interpreter or the shim it came from rather than leaving a later assertion to
# report the absence as its own defect.
suite_python3() { # stdout = the absolute python3 interpreter for this run
  local named interpreter diag
  named="$(command -v python3 2>/dev/null)" || named=""
  if [ -z "$named" ]; then
    printf 'suite: no python3 on PATH, so no interpreter can be resolved for this run\n' >&2
    return 1
  fi
  # The diagnostic is bought only on the failing branch: the shim's own stderr
  # is the one line that says which version file it wanted, and folding it into
  # the successful read would make an interpreter path unparseable.
  if ! interpreter="$("$named" -c 'import sys; sys.stdout.write(sys.executable or "")' 2>/dev/null)" \
     || [ -z "$interpreter" ]; then
    diag="$("$named" -c 'import sys; sys.stdout.write(sys.executable or "")' 2>&1)" || true
    printf 'suite: python3 on PATH (%s) named no interpreter here: %s\n' \
      "$named" "${diag:-it printed nothing}" >&2
    return 1
  fi
  if [ ! -x "$interpreter" ] || ! "$interpreter" -c '' 2>/dev/null; then
    printf 'suite: python3 on PATH (%s) names %s, which cannot run here\n' \
      "$named" "$interpreter" >&2
    return 1
  fi
  printf '%s' "$interpreter"
}

# ONE PLACE TO PIN IT, BECAUSE A VARIABLE CANNOT REACH A SHEBANG. Half of the
# python this suite runs is reached as `#!/usr/bin/env python3` — the shared
# clock, the tick deadline, process identity, usage, the Codex stop-hook plugin
# — so an exported path would fix the call sites a test wrote and leave every
# program the product launches on the broken shim. A directory the run owns,
# ahead of the host's on PATH, is the one rule both halves obey.
#
# AND IT FORWARDS RATHER THAN LINKS. A virtual environment is a directory whose
# bin/python3 is a symlink to a base interpreter with a pyvenv.cfg beside it,
# and CPython looks for that file next to the executable path it was HANDED,
# not next to the one it resolves to. A second symlink into a directory with no
# pyvenv.cfg therefore drops the run out of the caller's environment and into
# the base installation without failing: sys.prefix becomes the installation
# rather than the environment, which is a different interpreter version and a
# different set of installed packages than the caller is testing with. Exec'ing
# the resolved path leaves the executable location the caller's own.
suite_python3_pin() { # $1 = a directory this run owns and later removes
  local dir="$1" interpreter quoted guard guard_quoted pinned
  interpreter="$(suite_python3)" || return 1
  guard="${GANG_TEST_PATH_SHIM_GUARD:?the shared PATH-shim guard is not configured}"
  # POSIX single-quoting, because the wrapper is /bin/sh and an interpreter
  # path may hold a space or a quote of its own.
  quoted="'${interpreter//\'/\'\\\'\'}'"
  guard_quoted="'${guard//\'/\'\\\'\'}'"
  mkdir -p "$dir" && printf \
    '#!/bin/sh\n. %s\npath_shim_guard %s "$0" python3 || exit $?\nexec %s "$@"\n' \
    "$guard_quoted" "$quoted" "$quoted" > "$dir/python3" \
    && chmod +x "$dir/python3" || {
      printf 'suite: could not write the python3 wrapper in %s\n' "$dir" >&2
      return 1
    }
  # A PIN IS READ BACK, NOT ASSUMED. Every branch above can fail on a full or
  # read-only filesystem, and a pin that quietly did not take fails later as
  # the same unreadable python error this rule exists to replace.
  "$dir/python3" -c '' 2>/dev/null || {
    printf 'suite: the python3 wrapper in %s does not run\n' "$dir" >&2
    return 1
  }
  PATH="$dir:$PATH"
  export PATH
  hash -r 2>/dev/null || true
  pinned="$(command -v python3 2>/dev/null)" || pinned=""
  [ "$pinned" = "$dir/python3" ] || {
    printf 'suite: python3 on PATH is %s, not the pin in %s\n' \
      "${pinned:-nothing}" "$dir" >&2
    return 1
  }
}
