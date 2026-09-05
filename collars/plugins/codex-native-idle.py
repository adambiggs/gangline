#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Answer whether a Codex rollout has recorded the end of a turn.

Usage: codex-native-idle.py ROLLOUT [TURN_ID]

Codex runs its Stop hooks inside the task that owns the turn. The composer
paints idle for the whole of that hook, and an Enter typed there is dropped
without a trace: the rollout records neither a submission nor a refusal. Only
after every Stop hook has returned does the task emit its terminal event, and
the rollout recorder appends that event to the session file before anything
else can happen. So the positive post-Stop witness Gangline can observe is the
rollout's ``task_complete`` (or ``turn_aborted``) record for the turn the Stop
payload names. Observed on codex-cli 0.151.0: the record reached the file three
milliseconds after the hook exited, and a slash command entered after that
point ran natively.

The verdict is about the newest lifecycle record that speaks for the turn,
read from the end of the file backwards so an appended record is seen before
anything older:

  0  the turn has ended (prints which record proved it)
  1  the turn is still open, or a later turn has started since
  2  the rollout gives no answer (missing, unreadable, or no record for the
     turn); the caller must not treat this as idle

A ``turn_aborted`` record may carry no turn id. It aborts whichever turn was
active when it was written, so it is attributed to the nearest older
``task_started`` while walking backwards. No other record is read past its
name: a ``task_complete`` that names no turn cannot answer for a named one,
and a turn id that is not a string is malformed and gives no answer at all.
"""

import json
import os
import sys

CHUNK = 65536
LIFECYCLE = ("task_started", "task_complete", "turn_aborted")
# A turn id that is present but not a string. It correlates with nothing, and
# the record carrying it is reported rather than read past.
MALFORMED = object()


def records_newest_first(path):
    """Yield parsed JSON objects from the end of the file backwards.

    Lines that do not parse are skipped: a partial trailing line while the
    recorder is mid-append is the ordinary case, and one broken line must not
    hide the complete records before it.
    """
    with open(path, "rb") as handle:
        handle.seek(0, os.SEEK_END)
        pos = handle.tell()
        carry = b""
        while pos > 0:
            size = min(CHUNK, pos)
            pos -= size
            handle.seek(pos)
            parts = (handle.read(size) + carry).split(b"\n")
            carry = parts[0]
            for raw in reversed(parts[1:]):
                yield parse(raw)
        yield parse(carry)


def parse(raw):
    raw = raw.strip()
    if not raw:
        return None
    try:
        record = json.loads(raw)
    except ValueError:
        return None
    return record if isinstance(record, dict) else None


def lifecycle(record):
    """Return (kind, turn_id) for a lifecycle event record, else None."""
    if record is None or record.get("type") != "event_msg":
        return None
    payload = record.get("payload")
    if not isinstance(payload, dict):
        return None
    kind = payload.get("type")
    if kind not in LIFECYCLE:
        return None
    turn = payload.get("turn_id")
    if turn is not None and not isinstance(turn, str):
        turn = MALFORMED
    return kind, turn


def verdict(path, wanted):
    """Return (status, message) for the rollout at path and the wanted turn."""
    if not path:
        return 2, "no rollout path was given"
    try:
        records = records_newest_first(path)
        pending_abort = False
        later = None
        for record in records:
            event = lifecycle(record)
            if event is None:
                continue
            kind, turn = event
            if turn is MALFORMED:
                return 2, "the newest %s record carries a turn id that is not a string" % kind
            if wanted and turn is not None and turn != wanted:
                # Another turn's records say nothing about the wanted one on
                # their own. A start among them is remembered: once a record
                # for the wanted turn is found below it, that start proves a
                # later turn has opened since; with no such record it proves
                # nothing about a turn the rollout never saw.
                if kind == "task_started" and later is None:
                    later = turn
                continue
            if kind == "turn_aborted" and turn is None:
                pending_abort = True
                continue
            if kind == "task_complete" and turn is None and wanted:
                # Only an abort may go unnamed: a completion that names no
                # turn cannot be correlated with the one asked about.
                return 2, "the newest task_complete record names no turn, so it cannot speak for turn %s" % wanted
            if later is not None:
                return 1, "a later turn (%s) has started since turn %s" % (later, wanted)
            if kind == "task_started":
                if pending_abort:
                    return 0, "turn_aborted for turn %s" % (turn or "(unnamed)")
                return 1, "turn %s is still open: task_started is its newest record" % (
                    turn or "(unnamed)"
                )
            return 0, "%s for turn %s" % (kind, turn or "(unnamed)")
    except OSError as exc:
        return 2, "the rollout could not be read: %s" % exc
    if wanted:
        return 2, "the rollout holds no lifecycle record for turn %s" % wanted
    return 2, "the rollout holds no turn lifecycle record"


def main(argv):
    if len(argv) < 2 or len(argv) > 3:
        sys.stderr.write("usage: codex-native-idle.py ROLLOUT [TURN_ID]\n")
        return 2
    status, message = verdict(argv[1], argv[2] if len(argv) == 3 else "")
    print(message)
    return status


if __name__ == "__main__":
    sys.exit(main(sys.argv))
