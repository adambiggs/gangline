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
configured hook with the presence, enablement and `trustStatus` the TUI acts
on, under the same configuration layers the launch will use, so nothing here
reproduces Codex's hash algorithm or reads `[hooks.state]` behind its back.
Codex-cli 0.151.0 does not allow a profile-v2 `-p` layer on app-server at all,
so those launches are refused as unprobeable instead of being checked under a
different layer stack.

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
import re
import shlex
import subprocess
import sys
import threading
from typing import NoReturn

PROBE_TIMEOUT_SEC = 25
EXIT_REFUSED = 78  # EX_CONFIG
TRUSTED = ("trusted", "managed")
GANGLINE_EVENTS = {
    "userpromptsubmit", "posttooluse", "permissionrequest",
    "precompact", "postcompact", "stop",
}
VALUE_LAYER_FLAGS = ("-c", "--config", "--enable", "--disable")
LONG_VALUE_LAYER_FLAGS = ("--config=", "--enable=", "--disable=")
HOOK_VALUE = re.compile(
    r'^\[\{ hooks = \[\{ type = "command", command = '
    r'("(?:\\.|[^"\\])*")(?:, timeout = [0-9]+)? \}\] \}\]$'
)


class Unreadable(Exception):
    """Codex's hook-trust state could not be read at all."""


def profile_layer(command):
    """Name the first profile-v2 layer app-server cannot accept, if any."""
    i = 1
    while i < len(command):
        word = command[i]
        if word == "--":
            break
        if word in ("-p", "--profile"):
            if i + 1 >= len(command):
                return word + " <missing value>"
            return "%s %s" % (word, command[i + 1])
        if word.startswith("--profile=") or (word.startswith("-p") and len(word) > 2):
            return word
        i += 1
    return None


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


def config_layers(command):
    """Project the launch's configuration-affecting layers onto app-server."""
    out = []
    i = 1
    while i < len(command):
        word = command[i]
        if word == "--":
            break
        if word in VALUE_LAYER_FLAGS:
            if i + 1 >= len(command):
                raise Unreadable("%s has no value in the codex launch" % word)
            out += [command[i], command[i + 1]]
            i += 2
        elif word == "--strict-config" or word.startswith(LONG_VALUE_LAYER_FLAGS):
            out.append(word)
            i += 1
        else:
            i += 1
    return out


def config_values(command):
    """Yield only config override values from a launch, in precedence order."""
    layers = config_layers(command)
    i = 0
    while i < len(layers):
        if layers[i] in ("-c", "--config"):
            yield layers[i + 1]
            i += 2
        elif layers[i].startswith("--config="):
            yield layers[i].split("=", 1)[1]
            i += 1
        elif layers[i] in VALUE_LAYER_FLAGS:
            i += 2
        else:
            i += 1


def expected_hooks(command):
    """Read the six collar-owned event/command pairs from its inline layers."""
    candidates = {}
    for option in config_values(command):
        key, separator, value = option.partition("=")
        if not separator or not key.startswith("hooks."):
            continue
        event = key[6:].lower()
        if event not in GANGLINE_EVENTS:
            continue
        match = HOOK_VALUE.fullmatch(value)
        if match is None:
            raise Unreadable("the Gangline %s hook override has an unknown shape" % event)
        try:
            command_text = json.loads(match.group(1))
        except (TypeError, ValueError):
            raise Unreadable("the Gangline %s hook command is not readable" % event)
        candidates.setdefault(event, set()).add(command_text)
    missing = sorted(GANGLINE_EVENTS - set(candidates))
    ambiguous = sorted(event for event, commands in candidates.items()
                       if len(commands) != 1)
    if missing:
        raise Unreadable("the codex launch carries no Gangline hook override for %s"
                         % ", ".join(missing))
    if ambiguous:
        raise Unreadable("the codex launch carries conflicting Gangline hooks for %s"
                         % ", ".join(ambiguous))
    return {event: next(iter(commands)) for event, commands in candidates.items()}


def diagnostic_text(item):
    """Keep native config diagnostics visible without assuming their schema."""
    if isinstance(item, str):
        return item
    return json.dumps(item, sort_keys=True)[:500]


def read_entries(message):
    if "error" in message:
        raise Unreadable("codex refused 'hooks/list' (%s)"
                         % json.dumps(message["error"])[:200])
    data = (message.get("result") or {}).get("data")
    if not isinstance(data, list) or not data:
        raise Unreadable("the 'hooks/list' result carries no data array")
    hooks = []
    warnings = []
    for entry in data:
        if not isinstance(entry, dict) or not isinstance(entry.get("hooks"), list):
            raise Unreadable("a 'hooks/list' entry carries no hooks array")
        if not isinstance(entry.get("errors"), list):
            raise Unreadable("a 'hooks/list' entry carries no errors array")
        if not isinstance(entry.get("warnings"), list):
            raise Unreadable("a 'hooks/list' entry carries no warnings array")
        if entry["errors"]:
            raise Unreadable("codex reported hook configuration errors: %s"
                             % "; ".join(diagnostic_text(item)
                                        for item in entry["errors"]))
        warnings.extend(diagnostic_text(item) for item in entry["warnings"])
        for hook in entry["hooks"]:
            if not isinstance(hook, dict):
                raise Unreadable("a 'hooks/list' hook is not an object")
            if not isinstance(hook.get("trustStatus"), str) \
                    or not isinstance(hook.get("eventName"), str) \
                    or not isinstance(hook.get("command"), str) \
                    or not isinstance(hook.get("enabled"), bool):
                raise Unreadable(
                    "a 'hooks/list' hook carries no readable event, command, enabled, "
                    "and trust fields")
            hooks.append(hook)
    return hooks, warnings


def hooks_list(command, cwd):
    """Every configured hook codex reports for `cwd`, with its trust status."""
    argv = [command[0]] + config_layers(command) + ["app-server"]
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
    profile = profile_layer(command)
    if profile is not None:
        refuse(
            ["gang: this codex launch selects a -p/--profile configuration layer,",
             "but codex-cli 0.151.0 does not permit --profile on `codex app-server`.",
             "Without that same native layer, hooks/list cannot prove the hooks the",
             "TUI will load; flattening it into -c would change layer precedence and",
             "hook source identity, including the trust keys. Gangline will not",
             "launch blind.",
             "",
             "Remove -p/--profile from this collar, or start Codex here by hand."],
            "gang: codex hook-trust PROFILE UNPROBEABLE (%s) — codex-cli 0.151.0 "
            "app-server cannot inspect a -p/--profile launch; remove that layer or "
            "start codex here by hand." % profile)
    try:
        expected = expected_hooks(command)
        hooks, warnings = hooks_list(command, cwd)
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
    for warning in warnings:
        print("gang: codex hook warning: %s" % warning, file=sys.stderr)

    missing = []
    disabled = []
    mismatched = []
    for event, expected_command in sorted(expected.items()):
        same_event = [hook for hook in hooks
                      if hook["eventName"].lower() == event]
        exact = [hook for hook in same_event
                 if hook["command"] == expected_command]
        if not same_event:
            missing.append(event)
        elif not exact:
            mismatched.append(event)
        elif not any(hook["enabled"] for hook in exact):
            disabled.append(event)
    if missing or disabled or mismatched:
        detail = [
            "gang: codex did not report the six enabled Gangline hooks this launch",
            "requires, so its declared native turn boundaries are unavailable:",
            "",
        ]
        if missing:
            detail.append("  missing: %s" % ", ".join(missing))
        if disabled:
            detail.append("  disabled: %s" % ", ".join(disabled))
        if mismatched:
            detail.append("  command does not match: %s" % ", ".join(mismatched))
        detail += [
            "",
            "Do not disable hooks for a Gangline Codex launch. Remove a",
            "`--disable hooks` layer or set `features.hooks=true` in the layer that",
            "disabled them, then re-hitch.",
        ]
        refuse(
            detail,
            "gang: required codex hooks are missing, disabled, or changed — set "
            "features.hooks=true and restore the Gangline hook overrides before "
            "re-hitching.")
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
