#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Refuse a Codex launch that would stop on the native hooks-review menu.

Codex gates hook execution on a per-hook content hash recorded in
``[hooks.state]``. A hook whose hash it has not seen makes the TUI open

    Hooks need review
    N hooks are new or changed.
    > 1. Review hooks
      2. Trust all and continue
      3. Continue without trusting (hooks won't run)

before it draws a composer. The Gangline collar installs six hooks whose
command carries the install root, so any new install, upgrade or worktree
presents six unseen hashes. `hitch` then behaves exactly as designed — the
screen is occupied, delivery parks, and it waits for a person — and an
unattended team stalls there until a human attaches. This runs before the
harness so the hitch fails at once instead, saying what is wrong and how to
fix it.

CODEX ANSWERS THIS ABOUT ITSELF. The app-server's `hooks/list` returns every
configured hook with the `trustStatus` the TUI acts on, under the same `-c`
overrides the launch will use, so nothing here reproduces Codex's hash
algorithm or reads `[hooks.state]` behind its back. Observed on codex-cli
0.149.1.

NOTHING HERE GRANTS TRUST. Trust is a decision about what may run outside the
sandbox, so it stays the operator's: this reads the state and refuses, and the
remediation it prints is the operator answering the native menu once.

AN UNREADABLE ANSWER IS A REFUSAL, not a pass. A renamed method or a changed
response shape means Gangline can no longer tell a trusted hook set from an
untrusted one, and launching anyway would restore the silent stall this exists
to remove.
"""

import json
import os
import shlex
import subprocess
import sys
import threading
from typing import NoReturn

PROBE_TIMEOUT_SEC = 25
EXIT_REFUSED = 78  # EX_CONFIG
TRUSTED = ("trusted", "managed")


class Unreadable(Exception):
    """Codex's hook-trust state could not be read at all."""


def hold_corpse():
    """Keep the dead pane so `gang hitch` can quote the last line below.

    tmux destroys a window whose pane exits, and `launch_death_note` reads the
    pane only for a held corpse. Without this the refusal is printed to a pane
    that is gone before anything reads it, and the operator is left with a
    launch line and no reason attached to it.
    """
    pane = os.environ.get("TMUX_PANE")
    if not pane:
        return
    try:
        subprocess.run(
            ["tmux", "set-option", "-w", "-t", pane, "remain-on-exit", "on"],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10,
        )
    except (OSError, subprocess.SubprocessError):
        pass


def refuse(detail, last_line) -> NoReturn:
    # THE REFUSAL IS A DIAGNOSTIC, so it goes to stderr. The pane shows both, so
    # `hitch` still quotes the last line and `gang capture` still shows the
    # whole of it — but a caller reading this launch line's stdout, as
    # Gangline's own suite does when it inspects what a collar composes, gets
    # the harness's output rather than this text mixed into it.
    for line in detail:
        print(line, file=sys.stderr)
    print(last_line, file=sys.stderr)
    sys.stderr.flush()
    hold_corpse()
    sys.exit(EXIT_REFUSED)


def config_overrides(command):
    """The `-c key=value` pairs of the launch, and nothing else.

    `hooks/list` has to be asked under the same configuration the launch will
    run under, and the hook definitions themselves arrive as `-c` overrides.
    Every other word is a codex flag the app-server does not take.
    """
    out = []
    i = 1
    while i < len(command):
        if command[i] in ("-c", "--config") and i + 1 < len(command):
            out += [command[i], command[i + 1]]
            i += 2
        else:
            i += 1
    return out


def read_entries(message):
    if "error" in message:
        raise Unreadable("codex refused 'hooks/list' (%s)"
                         % json.dumps(message["error"])[:200])
    data = (message.get("result") or {}).get("data")
    if not isinstance(data, list):
        raise Unreadable("the 'hooks/list' result carries no data array")
    hooks = []
    for entry in data:
        if not isinstance(entry, dict) or not isinstance(entry.get("hooks"), list):
            raise Unreadable("a 'hooks/list' entry carries no hooks array")
        for hook in entry["hooks"]:
            if not isinstance(hook, dict):
                raise Unreadable("a 'hooks/list' hook is not an object")
            if not isinstance(hook.get("trustStatus"), str) \
                    or not isinstance(hook.get("eventName"), str):
                raise Unreadable(
                    "a 'hooks/list' hook carries no readable trustStatus and eventName")
            hooks.append(hook)
    return hooks


def hooks_list(command, cwd):
    """Every configured hook codex reports for `cwd`, with its trust status."""
    argv = [command[0], "app-server"] + config_overrides(command)
    try:
        probe = subprocess.Popen(
            argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL, text=True, cwd=cwd,
        )
    except OSError as exc:
        raise Unreadable("codex app-server could not be started (%s)" % exc)
    if probe.stdin is None or probe.stdout is None:
        probe.kill()
        raise Unreadable("codex app-server gave the probe no pipes to speak on")
    requests = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
            "clientInfo": {"name": "gangline", "title": "Gangline", "version": "1"},
            "capabilities": {"experimentalApi": True}}},
        {"jsonrpc": "2.0", "method": "initialized", "params": {}},
        {"jsonrpc": "2.0", "id": 2, "method": "hooks/list", "params": {"cwds": [cwd]}},
    ]
    # A HARNESS THAT NEVER ANSWERS MUST NOT BECOME THE STALL THIS PREVENTS, so
    # the reader runs under a wall-clock bound enforced by ending the probe
    # rather than by trusting it to exit.
    expired = threading.Event()

    def stop_probe():
        expired.set()
        probe.kill()

    deadline = threading.Timer(PROBE_TIMEOUT_SEC, stop_probe)
    deadline.start()
    try:
        try:
            for request in requests:
                probe.stdin.write(json.dumps(request) + "\n")
            probe.stdin.flush()
        except OSError as exc:
            raise Unreadable("the hook-state probe could not be written (%s)" % exc)
        for line in probe.stdout:
            try:
                message = json.loads(line)
            except ValueError:
                continue
            if message.get("id") == 2:
                return read_entries(message)
    finally:
        deadline.cancel()
        probe.kill()
        probe.wait()
    if expired.is_set():
        raise Unreadable("codex app-server did not answer 'hooks/list' within %ds"
                         % PROBE_TIMEOUT_SEC)
    raise Unreadable("codex app-server ended without answering 'hooks/list'")


def main():
    command = sys.argv[1:]
    if not command:
        print("codex-hooks-preflight: no codex command to launch", file=sys.stderr)
        sys.exit(2)
    cwd = os.getcwd()
    try:
        hooks = hooks_list(command, cwd)
    except Unreadable as exc:
        refuse(
            ["gang: codex's own hook-trust state could not be read, so whether this",
             "launch would stop on the native hooks-review menu is unknown rather",
             "than settled: %s." % exc,
             "Gangline installs six hooks here and will not launch blind into a",
             "prompt only a person can answer.",
             "",
             "Re-verify collars/plugins/codex-hooks-preflight.py against the",
             "installed codex: `codex app-server` and its 'hooks/list' method are",
             "what it reads."],
            "gang: codex hook-trust UNREADABLE (%s) — re-verify the collar preflight "
            "against this codex build, or start codex here by hand to see what it "
            "asks." % exc)
    pending = [h for h in hooks if h["trustStatus"] not in TRUSTED]
    if not pending:
        os.execvp(command[0], command)
    detail = [
        "gang: codex will not run these hooks until you trust them, and it asks on",
        "a startup menu no agent can answer:",
        "",
    ]
    for hook in pending:
        detail.append("  %-18s %-9s %s" % (
            hook["eventName"], hook["trustStatus"],
            (hook.get("command") or hook.get("key") or "")[:90]))
    detail += [
        "",
        "'modified' means codex has trusted a hook under this key before but not",
        "with this content — a Gangline hook command carries its install root, so a",
        "new install, an upgrade or a worktree presents a hook codex has not seen.",
        "",
        "Trusting is yours to grant, not Gangline's. Start codex once in",
        "%s with the same launch and answer" % cwd,
        "'Trust all and continue', then re-hitch:",
        "",
        "  " + " ".join(shlex.quote(word) for word in command),
        "",
    ]
    refuse(
        detail,
        "gang: %d codex hook(s) are untrusted here — run the codex line above in %s "
        "once, answer 'Trust all and continue', then re-hitch (this held window "
        "carries the full list: gang capture <name> 40)." % (len(pending), cwd))


if __name__ == "__main__":
    main()
