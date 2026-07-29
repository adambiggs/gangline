#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Atomic context-compliance event log and its human report."""

from __future__ import print_function

import fcntl
import os
import re
import sys
from datetime import datetime
try:
    from urllib.parse import quote, unquote
except ImportError:  # pragma: no cover - Python 3 is required by install.sh
    from urllib import quote, unquote


MAX_ROW = 4096
VERSION = "v1"
UNKNOWN = "COULD-NOT-DETERMINE"
KEY_RE = re.compile(r"^[a-z][a-z0-9_]*$")
INT_RE = re.compile(r"^[0-9]+$")


def die(message):
    print("context-events: " + message, file=sys.stderr)
    raise SystemExit(1)


def parse_field_args(args):
    fields = []
    seen = set()
    for item in args:
        if "=" not in item:
            die("field has no '=': {!r}".format(item))
        key, value = item.split("=", 1)
        if not KEY_RE.match(key):
            die("invalid field name: {!r}".format(key))
        if key in seen:
            die("duplicate field: {}".format(key))
        seen.add(key)
        fields.append((key, value))
    return fields


def encode_row(fields):
    cells = [VERSION]
    for key, value in fields:
        cells.append(key + "=" + quote(str(value), safe="-._~,:?"))
    return ("\t".join(cells) + "\n").encode("utf-8")


def oversize_row(fields):
    values = dict(fields)
    compact = [
        ("ts", values.get("ts", UNKNOWN)),
        ("kind", "read_ctd"),
        ("intended_kind", values.get("kind", UNKNOWN)),
        ("session", values.get("session", UNKNOWN)[:128]),
        ("window_id", values.get("window_id", UNKNOWN)[:64]),
        ("agent", values.get("agent", UNKNOWN)[:128]),
        ("profile", values.get("profile", UNKNOWN)[:64]),
        ("leg", values.get("leg", UNKNOWN)[:32]),
        ("hook_event", UNKNOWN),
        ("seam", UNKNOWN),
        ("tokens", UNKNOWN),
        ("window", UNKNOWN),
        ("band", UNKNOWN),
        ("threshold", UNKNOWN),
        ("thresholds", UNKNOWN),
        ("reason", "intended-row-exceeded-4096-bytes"),
    ]
    payload = encode_row(compact)
    if len(payload) > MAX_ROW:
        die("even the bounded CTD replacement row exceeds 4096 bytes")
    return payload


def one_write(path, payload):
    if len(payload) > MAX_ROW:
        raise ValueError("row exceeds 4096 bytes")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
    try:
        written = os.write(fd, payload)
        if written != len(payload):
            raise OSError("short append: {} of {} bytes".format(written, len(payload)))
    finally:
        os.close(fd)


def lock_file(path, exclusive=True):
    lock_path = path + ".lock"
    fd = os.open(lock_path, os.O_RDWR | os.O_CREAT, 0o600)
    fcntl.flock(fd, fcntl.LOCK_EX if exclusive else fcntl.LOCK_SH)
    return fd


def unlock_file(fd):
    try:
        fcntl.flock(fd, fcntl.LOCK_UN)
    finally:
        os.close(fd)


def append_event(path, max_bytes, fields):
    if max_bytes < MAX_ROW * 2:
        die("log bound must be at least {} bytes".format(MAX_ROW * 2))
    parent = os.path.dirname(path) or "."
    os.makedirs(parent, mode=0o700, exist_ok=True)
    payload = encode_row(fields)
    if len(payload) > MAX_ROW:
        payload = oversize_row(fields)

    lock_fd = lock_file(path, exclusive=True)
    try:
        dropped_old = False
        try:
            size = os.path.getsize(path)
        except OSError:
            size = 0
        if size and size + len(payload) > max_bytes:
            rotated = path + ".1"
            dropped_old = os.path.exists(rotated)
            os.replace(path, rotated)
        if dropped_old:
            vals = dict(fields)
            marker = encode_row([
                ("ts", vals.get("ts", UNKNOWN)),
                ("kind", "retention_gap"),
                ("session", vals.get("session", UNKNOWN)),
                ("window_id", vals.get("window_id", UNKNOWN)),
                ("agent", vals.get("agent", UNKNOWN)),
                ("profile", vals.get("profile", UNKNOWN)),
                ("leg", vals.get("leg", UNKNOWN)),
                ("hook_event", UNKNOWN),
                ("seam", UNKNOWN),
                ("tokens", UNKNOWN),
                ("window", UNKNOWN),
                ("band", UNKNOWN),
                ("threshold", UNKNOWN),
                ("thresholds", UNKNOWN),
                ("reason", "older-rotation-deleted"),
            ])
            one_write(path, marker)
        one_write(path, payload)
    finally:
        unlock_file(lock_fd)


def parse_row(raw, source, line_number):
    try:
        text = raw.decode("utf-8").rstrip("\n")
    except UnicodeDecodeError as exc:
        return None, "{}:{} invalid UTF-8 ({})".format(source, line_number, exc)
    cells = text.split("\t")
    if not cells or cells[0] != VERSION:
        return None, "{}:{} missing {} record version".format(source, line_number, VERSION)
    row = {"_source": source, "_line": str(line_number)}
    for cell in cells[1:]:
        if "=" not in cell:
            return None, "{}:{} field without '='".format(source, line_number)
        key, value = cell.split("=", 1)
        if not KEY_RE.match(key) or key in row:
            return None, "{}:{} invalid or duplicate field {!r}".format(source, line_number, key)
        row[key] = unquote(value)
    return row, None


def read_records(path):
    records = []
    malformed = []
    lock_fd = lock_file(path, exclusive=False)
    try:
        for candidate in (path + ".1", path):
            try:
                handle = open(candidate, "rb")
            except OSError:
                continue
            with handle:
                for number, raw in enumerate(handle, 1):
                    row, error = parse_row(raw, candidate, number)
                    if error:
                        malformed.append(error)
                    else:
                        records.append(row)
    finally:
        unlock_file(lock_fd)
    return records, malformed


def as_int(value):
    if value is None or not INT_RE.match(value):
        return None
    return int(value)


def threshold_list(value):
    if not value or value == UNKNOWN:
        return None
    parts = value.split(",")
    if not parts or any(not INT_RE.match(part) for part in parts):
        return None
    return [int(part) for part in parts]


def stamp(value):
    number = as_int(value)
    if number is None:
        return UNKNOWN
    return datetime.utcfromtimestamp(number).strftime("%Y-%m-%dT%H:%M:%SZ")


def row_key(row):
    return (row.get("session", UNKNOWN), row.get("window_id", UNKNOWN), row.get("agent", UNKNOWN))


def validate_record(row):
    common = ("ts", "kind", "session", "window_id", "agent", "profile", "leg",
              "hook_event", "seam", "tokens", "window", "band", "threshold",
              "thresholds")
    missing = [key for key in common if key not in row]
    if missing:
        return "missing required field(s): " + ",".join(missing)
    if as_int(row.get("ts")) is None:
        return "timestamp is not an integer"
    if row.get("seam") not in ("yes", "no", UNKNOWN):
        return "seam is not yes, no, or CTD"

    kind = row.get("kind")
    required = {
        "liveness": ("first_success",),
        "note": ("note_count",),
        "compact_request": ("requester", "issue_tokens", "first_threshold"),
        "context_drop": ("previous_band", "notes_since_drop", "last_note_ts",
                         "pre_ts", "pre_tokens", "pre_window", "pre_band",
                         "pre_thresholds", "sample_staleness", "sample_quality",
                         "provenance", "provenance_candidates",
                         "evidence_availability", "evidence_result",
                         "request_quality", "issue_tokens", "first_threshold"),
        "read_ctd": ("reason",),
        "retention_gap": ("reason",),
    }
    if kind not in required:
        return "unknown record kind: {}".format(kind)
    missing = [key for key in required[kind] if key not in row]
    if missing:
        return "{} missing required field(s): {}".format(kind, ",".join(missing))

    if kind in ("liveness", "note", "compact_request", "context_drop"):
        for key in ("tokens", "window", "band", "threshold"):
            if as_int(row.get(key)) is None:
                return "{} has non-integer {}".format(kind, key)
        vector = threshold_list(row.get("thresholds"))
        band = as_int(row.get("band"))
        if vector is None or band is None or band > len(vector):
            return "{} has an uninterpretable resolved ladder".format(kind)
    return None


def seam_summary(notes, expected):
    if expected is None or expected != len(notes):
        return UNKNOWN, UNKNOWN
    if not notes:
        return "none", "none"
    last = notes[-1]
    leg = last.get("leg", UNKNOWN)
    event = last.get("hook_event", "-")
    delivered = leg if event in ("", "-") else leg + "/" + event
    seam_positions = [i + 1 for i, note in enumerate(notes) if note.get("seam") == "yes"]
    unknown_seam = any(note.get("seam") not in ("yes", "no") for note in notes)
    ordinal = len(notes)
    if unknown_seam:
        interpretation = UNKNOWN
    elif last.get("seam") == "no":
        interpretation = "NON-SEAM #{}".format(ordinal)
    elif seam_positions and seam_positions[0] == ordinal:
        interpretation = "FIRST-SEAM (#{} overall)".format(ordinal)
    else:
        seam_ordinal = len([pos for pos in seam_positions if pos <= ordinal])
        interpretation = "seam #{} (#{} overall)".format(seam_ordinal, ordinal)
    return delivered, interpretation


def final_exposure(notes, expected, drop):
    thresholds = threshold_list(drop.get("thresholds"))
    window = as_int(drop.get("window"))
    if thresholds is None or window is None or not thresholds:
        return UNKNOWN
    if thresholds[-1] > window:
        return UNKNOWN
    if expected is None or expected != len(notes):
        return UNKNOWN
    for note in notes:
        vector = threshold_list(note.get("thresholds"))
        band = as_int(note.get("band"))
        if vector and band == len(vector):
            return "yes"
    return "no"


def report(path, live_gaps):
    if not os.path.exists(path) and not os.path.exists(path + ".1"):
        print("Context compliance report")
        print("log: {}".format(path))
        if live_gaps:
            print("COULD-NOT-DETERMINE: logger liveness never established; write failure is active")
            for gap in live_gaps[:5]:
                print("  live logger gap: " + gap)
        else:
            print("COULD-NOT-DETERMINE: no successful logger liveness record exists")
            print("  an empty path is not evidence that the week was quiet")
        return 1

    records, malformed = read_records(path)
    valid = []
    for row in records:
        error = validate_record(row)
        if error:
            malformed.append("{}:{} {}".format(
                row.get("_source", path), row.get("_line", UNKNOWN), error))
        else:
            valid.append(row)
    records = valid
    pending = {}
    drops = []
    interval_bands = {}
    nonseam_notes = 0
    spent_nonseam = 0
    read_ctd = 0
    retention_gaps = 0
    recovered_gaps = 0
    liveness = set()
    measured_events = 0

    def close_bands(key):
        nonlocal spent_nonseam
        for flags in interval_bands.get(key, {}).values():
            if flags.get("nonseam") and not flags.get("seam"):
                spent_nonseam += 1
        interval_bands[key] = {}

    for row in records:
        kind = row.get("kind", UNKNOWN)
        key = row_key(row)
        if kind == "liveness":
            liveness.add(key)
            continue
        measured_events += 1
        if row.get("prior_log_gap"):
            recovered_gaps += 1
        if kind == "read_ctd":
            read_ctd += 1
            continue
        if kind == "retention_gap":
            retention_gaps += 1
            continue
        if kind == "note":
            pending.setdefault(key, []).append(row)
            band_key = row.get("band", UNKNOWN)
            flags = interval_bands.setdefault(key, {}).setdefault(band_key, {})
            if row.get("seam") == "yes":
                flags["seam"] = True
            elif row.get("seam") == "no":
                flags["nonseam"] = True
                nonseam_notes += 1
            continue
        if kind == "context_drop":
            notes = pending.get(key, [])
            expected = as_int(row.get("notes_since_drop"))
            last_delivery, seam_test = seam_summary(notes, expected)
            exposure = final_exposure(notes, expected, row)
            drops.append((row, expected, last_delivery, seam_test, exposure))
            pending[key] = []
            close_bands(key)

    for key in list(interval_bands):
        close_bands(key)

    outcomes = {"proven-compliant": 0, "proven-non-compliant": 0, "could-not-determine": 0}
    print("Context compliance report")
    print("log: {} (+ one rotation)".format(path))
    print("")
    print("context drops (pre-drop sample is the measured peak, not the true peak):")
    if not drops:
        print("  none retained")
    for row, expected, last_delivery, seam_test, exposure in drops:
        pre = row.get("pre_tokens", UNKNOWN)
        pre_at = stamp(row.get("pre_ts"))
        stale = row.get("sample_staleness", UNKNOWN)
        post = row.get("tokens", UNKNOWN)
        band = row.get("band", UNKNOWN)
        agent = row.get("agent", UNKNOWN)
        provenance = row.get("provenance", UNKNOWN)
        availability = row.get("evidence_availability", UNKNOWN)
        result = row.get("evidence_result", UNKNOWN)
        count = str(expected) if expected is not None else UNKNOWN
        print("  {} {}: pre={} at {} (staleness={}s) -> post={} band={}; nudges={}; last={}; {}; final-band={}; provenance={}; evidence={}/{}".format(
            stamp(row.get("ts")), agent, pre, pre_at, stale, post, band, count,
            last_delivery, seam_test, exposure, provenance, availability, result))
        if exposure == "yes":
            if provenance == "self-issued":
                outcomes["proven-compliant"] += 1
            elif provenance in ("peer-issued", "harness-auto"):
                outcomes["proven-non-compliant"] += 1
            else:
                outcomes["could-not-determine"] += 1
        elif exposure == UNKNOWN:
            outcomes["could-not-determine"] += 1

    # A live final-band interval has no outcome yet. It is CTD, never failure.
    for notes in pending.values():
        if not notes:
            continue
        last = notes[-1]
        vector = threshold_list(last.get("thresholds"))
        band = as_int(last.get("band"))
        if vector and band == len(vector):
            outcomes["could-not-determine"] += 1

    structural = len(malformed) + read_ctd + retention_gaps + recovered_gaps + len(live_gaps)
    print("")
    print("logger liveness:")
    print("  successful first-write records retained for {} agent window(s)".format(len(liveness)))
    if measured_events:
        print("  measured event rows retained: {}".format(measured_events))
    else:
        print("  measured event rows retained: none (logger worked; no measured event was recorded)")
    if not liveness and measured_events:
        print("  event rows prove writes occurred; no quiet-interval liveness row is retained")
    print("")
    print("final-band outcomes (three-way; no ambiguous row enters either proven count):")
    print("  proven-compliant: {}".format(outcomes["proven-compliant"]))
    print("  proven-non-compliant: {}".format(outcomes["proven-non-compliant"]))
    print("  could-not-determine: {} (+ {} structural CTD records/gaps)".format(
        outcomes["could-not-determine"], structural))
    determinate = outcomes["proven-compliant"] + outcomes["proven-non-compliant"]
    if structural or outcomes["could-not-determine"] > determinate or not (determinate + outcomes["could-not-determine"]):
        print("  verdict: THIS DATASET CANNOT ANSWER THE QUESTION")
    else:
        print("  verdict: determinate outcomes are the majority; CTD remains co-reported")

    suffix = ""
    if structural:
        suffix = " (LOWER BOUNDS; {} structural CTD records/gaps)".format(structural)
    print("")
    print("delivery-topology hypothesis test:")
    print("  notes delivered at a non-seam (PostToolUse): {}{}".format(nonseam_notes, suffix))
    print("  bands spent non-seam with no seam note ever following: {}{}".format(spent_nonseam, suffix))
    print("")
    print("CTD detail: readout/state={}, malformed={}, retention={}, recovered-log={}, live-log={}".format(
        read_ctd, len(malformed), retention_gaps, recovered_gaps, len(live_gaps)))
    for error in malformed[:5]:
        print("  malformed: " + error)
    for gap in live_gaps[:5]:
        print("  live logger gap: " + gap)
    return 0


def clear(path):
    parent = os.path.dirname(path) or "."
    if not os.path.isdir(parent):
        print("no context event log existed at {}".format(path))
        return
    lock_fd = lock_file(path, exclusive=True)
    removed = []
    try:
        for candidate in (path, path + ".1"):
            try:
                os.unlink(candidate)
                removed.append(candidate)
            except FileNotFoundError:
                pass
    finally:
        unlock_file(lock_fd)
    # This command is the explicit Law 6 deletion path. Collection should be
    # stopped before clearing; removing the coordination inode too is what makes
    # the path delete every artifact the measurement created.
    try:
        os.unlink(path + ".lock")
    except FileNotFoundError:
        pass
    if removed:
        print("deleted context event log: " + ", ".join(removed))
    else:
        print("no context event log existed at {}".format(path))


def main(argv):
    if len(argv) < 3:
        die("usage: context_events.py append|report|clear LOG ...")
    command, path = argv[1], os.path.abspath(os.path.expanduser(argv[2]))
    if command == "append":
        if len(argv) < 5:
            die("append needs MAX_BYTES and fields")
        try:
            bound = int(argv[3])
        except ValueError:
            die("MAX_BYTES is not an integer")
        append_event(path, bound, parse_field_args(argv[4:]))
        return 0
    if command == "report":
        return report(path, argv[3:])
    if command == "clear":
        clear(path)
        return 0
    die("unknown command: " + command)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
