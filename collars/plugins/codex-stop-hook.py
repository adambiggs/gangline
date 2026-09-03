#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Enforce verified peer-reply obligations at a native Stop boundary.

The helper is shared by the Codex and Claude Code collars. Gangline owns the
message provenance and returns a small TSV query; this adapter either refuses
Stop while a debt or ambiguity remains, or delegates a non-blocking boundary
to the ordinary ``gang hook`` bookkeeping path. Operator-authored prompts never
enter the query, so they neither create nor erase peer debt.

ONE REFUSAL PER TURN. Both harnesses mark every Stop that follows a Stop-hook
block with ``stop_hook_active`` and document the hook's duty as returning
success while it is set; Claude Code additionally overrides a hook after a
finite run of consecutive blocks and ends the turn without telling it. A
refusal is therefore a single chance to answer, not a hold: on the re-Stop the
query runs again, a clear record proceeds as usual, and a record still
outstanding releases the turn LOUDLY through ``gang reply-released`` — the
obligation stays recorded and visible, the notify target is told, and the next
delivery raises it again — instead of spending model turns until the harness
overrides the block behind Gangline's back.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass
from typing import Optional


# THE BUDGET IS THE COLLARS' NATIVE FUSE, SPENT IN NAMED PARTS. Both collars
# wire this helper with a 15-second native timeout, after which the harness
# ends the hook and reads no verdict at all. The query is retried only on a
# timeout — a lock held for a moment, not an answer — and only inside its own
# deadline, so a contended Gangline cannot hold Stop for longer than the
# deadline says; the release report and the ordinary boundary then take what is
# left. A fixture may scale the two query numbers through the environment; the
# production values are the defaults.
QUERY_ATTEMPT_SEC = float(os.environ.get("GANG_STOP_QUERY_ATTEMPT_SEC", "5"))
QUERY_DEADLINE_SEC = float(os.environ.get("GANG_STOP_QUERY_DEADLINE_SEC", "9"))
# The floor scales with the attempt so a scaled fixture keeps the same shape.
QUERY_MIN_ATTEMPT_SEC = QUERY_ATTEMPT_SEC / 5
RELEASE_TIMEOUT_SEC = 2
SETTLE_TIMEOUT_SEC = 3
QUERY_TIMEOUT = "query-timeout"
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


class QueryTimeout(Exception):
    """Every query attempt inside the deadline timed out; provenance is unread."""


def gang_run(
    gang: str,
    args: list[str],
    payload: Optional[str] = None,
    timeout: float = SETTLE_TIMEOUT_SEC,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [gang, *args],
        input=payload,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=timeout,
    )


def query_run(gang: str) -> subprocess.CompletedProcess[str]:
    started = time.monotonic()
    attempts = 0
    while True:
        remaining = QUERY_DEADLINE_SEC - (time.monotonic() - started)
        if attempts and remaining < QUERY_MIN_ATTEMPT_SEC:
            raise QueryTimeout(
                "Gangline query timed out %d time(s) inside its %gs deadline"
                % (attempts, QUERY_DEADLINE_SEC)
            )
        budget = max(min(QUERY_ATTEMPT_SEC, remaining), QUERY_MIN_ATTEMPT_SEC)
        attempts += 1
        try:
            return gang_run(gang, ["reply-obligations"], timeout=budget)
        except subprocess.TimeoutExpired:
            stderr("query attempt %d timed out after %gs" % (attempts, budget))


def query(gang: str) -> list[Verdict]:
    result = query_run(gang)
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


def block_reason(verdicts: list[Verdict]) -> str:
    stuck = [v for v in verdicts if v.status != "owed"]
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
    result = gang_run(gang, ["hook"], payload, timeout=SETTLE_TIMEOUT_SEC)
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        raise ValueError("ordinary Stop bookkeeping failed: " + detail)


def report_release(gang: str, why: str) -> None:
    # THE RELEASE IS REPORTED BEFORE THE BOUNDARY CLOSES, so the record and the
    # alert exist by the time the harness reads the allow. A failure here is
    # said on stderr and does not turn back into a block: the harness has
    # already been told once, and holding the turn again would only spend the
    # model turns this release exists to save.
    args = ["reply-released"] + ([why] if why else [])
    result = gang_run(gang, args, timeout=RELEASE_TIMEOUT_SEC)
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no diagnostic"
        raise ValueError("Gangline could not record the release: " + detail)


def read_payload(raw: str) -> bool:
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError("stdin is not readable JSON") from exc
    if not isinstance(value, dict) or value.get("hook_event_name") != "Stop":
        raise ValueError("stdin is not a native Stop payload")
    # A missing marker reads as a first Stop: a harness that never sets it is
    # refused on every Stop while debt stands, bounded by nothing but itself.
    return value.get("stop_hook_active") is True


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
    active = False
    try:
        raw = sys.stdin.read()
        active = read_payload(raw)
    except (OSError, ValueError) as exc:
        refuse(
            exc,
            "the native Stop payload could not be read",
            "preserve the current state and repair the collar's hook wiring",
        )
        return 0
    # THE TIMEOUT IS ITS OWN STATE, NOT AN UNREADABLE QUERY. A query that
    # answered nothing inside its deadline is refused once like any other
    # unread provenance, and released on the re-Stop under the timeout's own
    # name, so a wedged Gangline costs one turn rather than every turn until
    # the harness overrides.
    release_why = ""
    try:
        verdicts = query(gang)
    except QueryTimeout as exc:
        if not active:
            refuse(
                exc,
                "Gangline could not read verified peer-reply provenance in time",
                "preserve the current state and stop again",
            )
            return 0
        stderr(str(exc))
        verdicts = []
        release_why = QUERY_TIMEOUT
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
    if blocking and not active:
        block(block_reason(blocking))
        return 0
    if blocking or release_why:
        try:
            report_release(gang, release_why)
        except (OSError, ValueError, subprocess.SubprocessError) as exc:
            stderr("released with the obligation unrecorded: %s" % exc)
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
