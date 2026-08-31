#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Apply Gangline's report-before-idle rule at a Codex Stop boundary.

Codex passes the native Stop payload on stdin.  The only blocking reply this
program emits is the first proved ``unreported`` answer.  Codex marks the
second invocation with ``stop_hook_active``; that pass always ends the turn and
asks Gangline to notify the hitcher.  An unreadable Gangline answer is also an
immediate allow: a broken query must not wedge every Codex window.

The Gangline query is deliberately the authority for ancestry, current-turn
identity, and verified sends.  This helper only parses its small wire contract,
then preserves the existing generic Stop hook on every allowed turn end.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from dataclasses import dataclass
from typing import Optional


BLOCK_REASON = (
    "you may not go idle: continue working, or message the agent that hitched "
    "you with your status or blocker"
)


@dataclass(frozen=True)
class Verdict:
    status: str
    destination: str
    cause: str


STATUSES = frozenset(("reported", "unreported", "exempt", "unknown", "parent-gone"))

# Three bounded children can run on the cap path: query, attributed notice, and
# ordinary Stop bookkeeping.  Their total remains below the native 15-second
# fuse installed by the collar.  A timeout is an unknown, never a block.
GANG_TIMEOUT_SEC = 4


def stderr(message: str) -> None:
    print("codex-stop-hook: " + message, file=sys.stderr, flush=True)


def allow() -> None:
    print("{}", flush=True)


def block() -> None:
    print(json.dumps({"decision": "block", "reason": BLOCK_REASON}), flush=True)


def gang_run(gang: str, args: list[str], payload: Optional[str] = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [gang, *args],
        input=payload,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=GANG_TIMEOUT_SEC,
    )


def query(gang: str, turn_id: str) -> Verdict:
    result = gang_run(gang, ["reported-to-hitcher", turn_id])
    if result.returncode:
        raise ValueError("Gangline query exited %d%s" % (
            result.returncode,
            ": " + result.stderr.strip() if result.stderr.strip() else "",
        ))
    line = result.stdout
    if not line.endswith("\n") or line.count("\n") != 1:
        raise ValueError("Gangline query did not return one newline-terminated record")
    fields = line[:-1].split("\t")
    if len(fields) != 3 or fields[0] not in STATUSES:
        raise ValueError("Gangline query returned no recognized verdict")
    status, destination, cause = fields
    if not status or not destination or not cause:
        raise ValueError("Gangline query returned an empty status, destination, or cause")
    if status in ("reported", "unreported") and (destination == "-" or cause != "-"):
        raise ValueError("Gangline query attached a cause to a settled verdict")
    if status == "exempt" and (destination != "-" or cause != "-"):
        raise ValueError("Gangline query gave an exemption a delivery target or cause")
    if status in ("unknown", "parent-gone") and (destination == "-" or cause == "-"):
        raise ValueError("Gangline query gave an actionable failure no destination or cause")
    return Verdict(status, destination, cause)


def alert_body(verdict: Verdict, turn_id: str, last_message: Optional[str]) -> str:
    message = last_message if isinstance(last_message, str) else "<none>"
    return (
        "Gangline Stop-hook notice: this agent went idle without a verified report to "
        "its hitcher.\n"
        "verdict: %s\n"
        "cause: %s\n"
        "turn id: %s\n"
        "last assistant message:\n%s"
        % (verdict.status, verdict.cause, turn_id, message)
    )


def one_line(value: str) -> str:
    """Keep a tmux option record as one exact TSV row."""
    return "".join(
        char if char >= " " and char != "\x7f" else " "
        for char in value.replace("\t", " ")
    )


def tmux_run(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["tmux", *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=GANG_TIMEOUT_SEC,
    )


def idle_notice_records(value: str) -> list[list[str]]:
    """Read the append-only failure list without ever repairing malformed data."""
    if not value:
        return []
    rows: list[list[str]] = []
    for row in value.split("\n"):
        fields = row.split("\t", 2)
        if (
            len(fields) != 3
            or not fields[0]
            or not fields[1]
            or not fields[2]
            or any(
                ord(char) < 32 or ord(char) == 127
                for field in fields
                for char in field
            )
        ):
            raise ValueError("stored idle notice failure is malformed")
        rows.append(fields)
    return rows


def idle_notice_read(pane: str) -> list[list[str]]:
    result = tmux_run(["show-options", "-wqv", "-t", pane, "@gl_idle_notice_failed"])
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        raise ValueError("could not read idle notice failure: " + detail)
    return idle_notice_records(result.stdout.rstrip("\n"))


def idle_notice_failed(turn_id: str, destination: str, failure: str) -> None:
    """Append only a nonaccepted attributed notice on its calling child pane."""
    pane = os.environ.get("TMUX_PANE", "")
    if not pane:
        stderr("idle notice was not accepted and its child pane is unavailable")
        return
    try:
        records = idle_notice_read(pane)
    except ValueError as exc:
        stderr(str(exc) + "; refusing to overwrite it")
        return
    record = [turn_id, destination, one_line(failure)]
    value = "\n".join("\t".join(fields) for fields in [*records, record])
    result = tmux_run(["set-option", "-w", "-t", pane, "@gl_idle_notice_failed", value])
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        stderr("could not persist idle notice failure: " + detail)


def idle_notice_repaired(destination: str) -> None:
    """Retire every well-formed notice for a destination an accepted report repaired."""
    pane = os.environ.get("TMUX_PANE", "")
    if not pane:
        return
    try:
        records = idle_notice_read(pane)
    except ValueError as exc:
        stderr(str(exc) + "; refusing to retire it")
        return
    retained = [fields for fields in records if fields[1] != destination]
    if len(retained) == len(records):
        return
    if retained:
        result = tmux_run([
            "set-option", "-w", "-t", pane, "@gl_idle_notice_failed",
            "\n".join("\t".join(fields) for fields in retained),
        ])
    else:
        result = tmux_run(["set-option", "-uw", "-t", pane, "@gl_idle_notice_failed"])
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        stderr("could not retire repaired idle notice failure: " + detail)


def notify(gang: str, verdict: Verdict, turn_id: str, last_message: Optional[str]) -> None:
    result = gang_run(
        gang,
        ["send", "--to", verdict.destination, "--stdin"],
        alert_body(verdict, turn_id, last_message),
    )
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        stderr("idle notice for %s was not accepted: %s" % (verdict.destination, detail))
        idle_notice_failed(
            turn_id,
            verdict.destination,
            "ordinary attributed send was not accepted: " + detail,
        )


def settle_stop(gang: str, payload: str) -> None:
    """Preserve Gangline's ordinary Stop bookkeeping on a turn we allow."""
    result = gang_run(gang, ["hook"], payload)
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        stderr("ordinary Stop bookkeeping failed: " + detail)


def read_payload(raw: str) -> tuple[str, bool, Optional[str]]:
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError("stdin is not readable JSON") from exc
    if not isinstance(value, dict) or value.get("hook_event_name") != "Stop":
        raise ValueError("stdin is not a Codex Stop payload")
    turn_id = value.get("turn_id")
    active = value.get("stop_hook_active")
    last_message = value.get("last_assistant_message")
    if not isinstance(turn_id, str):
        turn_id = ""
    if not isinstance(active, bool):
        raise ValueError("Stop payload carries no boolean stop_hook_active")
    if last_message is not None and not isinstance(last_message, str):
        raise ValueError("Stop payload carries a non-string last_assistant_message")
    return turn_id, active, last_message


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        stderr("expected exactly one Gangline executable argument")
        allow()
        return 0
    gang = argv[1]
    raw = ""
    try:
        raw = sys.stdin.read()
        turn_id, active, last_message = read_payload(raw)
        verdict = query(gang, turn_id)
        if verdict.status == "unreported" and not active:
            block()
            return 0
        if verdict.status == "reported":
            idle_notice_repaired(verdict.destination)
        if verdict.status in ("unreported", "unknown", "parent-gone"):
            notify(gang, verdict, turn_id, last_message)
        settle_stop(gang, raw)
    except (OSError, ValueError, json.JSONDecodeError, subprocess.SubprocessError) as exc:
        # The native hook must fail open.  A self-error has no trustworthy
        # antecedent to block on, and the next Stop cannot rely on this process
        # having run at all.
        stderr(str(exc))
        try:
            settle_stop(gang, raw)
        except (OSError, subprocess.SubprocessError) as settle_exc:
            stderr("could not record failed Stop bookkeeping: " + str(settle_exc))
    allow()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
