#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Enforce verified peer-reply obligations at a native Stop boundary.

The helper is shared by the Codex and Claude Code collars. Gangline owns the
message provenance and returns a small TSV query; this adapter either blocks
Stop while a debt or ambiguity remains, or delegates a non-blocking boundary
to the ordinary ``gang hook`` bookkeeping path. Operator-authored prompts never
enter the query, so they neither create nor erase peer debt.
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
from dataclasses import dataclass
from typing import Optional


GANG_TIMEOUT_SEC = 5
NONCE = re.compile(r"[a-f0-9]{16}\Z")
AGENT = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*\Z")


@dataclass(frozen=True)
class Verdict:
    status: str
    nonce: str
    peer: str
    detail: str


def stderr(message: str) -> None:
    print("peer-reply-stop-hook: " + message, file=sys.stderr, flush=True)


def allow() -> None:
    print("{}", flush=True)


def block(reason: str) -> None:
    print(json.dumps({"decision": "block", "reason": reason}), flush=True)


def gang_run(
    gang: str, args: list[str], payload: Optional[str] = None
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [gang, *args],
        input=payload,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=GANG_TIMEOUT_SEC,
    )


def query(gang: str) -> list[Verdict]:
    result = gang_run(gang, ["reply-obligations"])
    if result.returncode:
        raise ValueError(
            "Gangline query exited %d%s"
            % (
                result.returncode,
                ": " + result.stderr.strip() if result.stderr.strip() else "",
            )
        )
    if not result.stdout.endswith("\n") or not result.stdout.strip():
        raise ValueError("Gangline query returned no complete record")
    verdicts: list[Verdict] = []
    seen: set[str] = set()
    for line in result.stdout[:-1].split("\n"):
        fields = line.split("\t")
        if len(fields) != 4:
            raise ValueError("Gangline query returned a malformed record")
        verdict = Verdict(*fields)
        if verdict.status == "clear":
            if (verdict.nonce, verdict.peer, verdict.detail) != ("-", "-", "-"):
                raise ValueError("Gangline query returned a malformed clear record")
        elif verdict.status == "owed":
            if (
                not NONCE.fullmatch(verdict.nonce)
                or not AGENT.fullmatch(verdict.peer)
                or verdict.detail != "live"
            ):
                raise ValueError("Gangline query returned a malformed owed record")
        elif verdict.status == "retired":
            if (
                not NONCE.fullmatch(verdict.nonce)
                or not AGENT.fullmatch(verdict.peer)
                or verdict.detail != "sender-gone"
            ):
                raise ValueError("Gangline query returned a malformed retired record")
        elif verdict.status == "unknown":
            if (
                verdict.nonce != "-" and not NONCE.fullmatch(verdict.nonce)
            ) or (
                verdict.peer != "-" and not AGENT.fullmatch(verdict.peer)
            ) or not verdict.detail or verdict.detail == "-":
                raise ValueError("Gangline query returned a malformed unknown record")
        else:
            raise ValueError("Gangline query returned an unknown status")
        if verdict.nonce != "-":
            if verdict.nonce in seen:
                raise ValueError("Gangline query repeated a message nonce")
            seen.add(verdict.nonce)
        verdicts.append(verdict)
    if any(v.status == "clear" for v in verdicts) and len(verdicts) != 1:
        raise ValueError("Gangline query mixed clear with outstanding evidence")
    return verdicts


# An ambiguous record whose request half is complete enough for Gangline to
# correlate a reply against it is cleared by replying; every other ambiguity
# names evidence no message of the blocked agent can change.
ANSWERABLE = ("provenance-prompt-request", "provenance-verified-request")


def answerable(verdict: Verdict) -> bool:
    if verdict.status == "owed":
        return True
    return (
        verdict.status == "unknown"
        and verdict.detail in ANSWERABLE
        and bool(AGENT.fullmatch(verdict.peer))
    )


def block_reason(verdicts: list[Verdict]) -> str:
    stuck = [v for v in verdicts if not answerable(v)]
    if stuck:
        details = ", ".join(dict.fromkeys(v.detail for v in stuck))
        return (
            "you may not go idle: Gangline cannot prove peer-reply provenance "
            "clear (%s), and no message you can send changes that record. "
            "Preserve the evidence and raise it with the operator" % details
        )
    peers = list(dict.fromkeys(v.peer for v in verdicts))
    return (
        "you may not go idle: reply to %s with any concise genuine reply or "
        "acknowledgement (gang send --to NAME --stdin). It may say you are "
        "waiting on background work and will report later"
        % ", ".join(peers)
    )


def settle_stop(gang: str, payload: str) -> None:
    result = gang_run(gang, ["hook"], payload)
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        raise ValueError("ordinary Stop bookkeeping failed: " + detail)


def read_payload(raw: str) -> None:
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError("stdin is not readable JSON") from exc
    if not isinstance(value, dict) or value.get("hook_event_name") != "Stop":
        raise ValueError("stdin is not a native Stop payload")


def refuse(exc: BaseException, failure: str, remedy: str) -> None:
    stderr(str(exc))
    block("you may not go idle: %s (%s); %s" % (failure, exc, remedy))


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        stderr("expected exactly one Gangline executable argument")
        block("you may not go idle: the peer-reply query helper is misconfigured")
        return 0
    gang = argv[1]
    # EACH STAGE FAILS UNDER ITS OWN NAME. One handler over all three read the
    # same sentence out for an unreadable Stop payload, an unreadable query and
    # a failed boundary, so the agent was sent to repair a query path that had
    # already answered. Reading stdin belongs inside the payload stage: bytes
    # that are not UTF-8 raise before any handler otherwise, and the hook then
    # prints no verdict at all.
    raw = ""
    try:
        raw = sys.stdin.read()
        read_payload(raw)
    except (OSError, ValueError) as exc:
        refuse(
            exc,
            "the native Stop payload could not be read",
            "preserve the current state and repair the collar's hook wiring",
        )
        return 0
    try:
        verdicts = query(gang)
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        refuse(
            exc,
            "Gangline could not read verified peer-reply provenance",
            "preserve the current state and repair the query path",
        )
        return 0
    blocking = [
        verdict
        for verdict in verdicts
        if verdict.status not in ("clear", "retired")
    ]
    if blocking:
        block(block_reason(blocking))
        return 0
    try:
        settle_stop(gang, raw)
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        refuse(
            exc,
            "peer-reply provenance is clear, but the native Stop boundary "
            "could not be closed",
            "preserve the current state and repair the Gangline hook path",
        )
        return 0
    allow()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
