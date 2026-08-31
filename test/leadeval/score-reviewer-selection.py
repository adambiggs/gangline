#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Scores one line of the shipped lead brief against a staged lead's dispatches.

THE LINE UNDER TEST, from roles/lead.md:

    Choose them across the team so that every owner has an available reviewer
    whose error modes differ from its own.

WHY THIS LINE FIRST. It is the only decision in the brief whose obedience is
visible in argv alone. Every other line needs either a manifest the eval
supplies or a judge reading prose; this one needs neither, so it is the cheapest
honest end-to-end proof that a brief line can be scored at all.

WHAT IS AND IS NOT MEASURED. The verdict is computed from the `-c`/`--collar`
and `-m`/`--model` values of the hitches a lead actually made, recorded by
test/leadeval/gang-shim.template. That is what the lead DID. It is not a reading
of what the lead said, meant, or would do again: a model that dispatches a
monoculture while explaining diversity fails here, which is the point.

An omitted -c or -m is not unknown - it selects the harness default, and two
hitches that both omit it therefore share error modes. Omission is recorded as
the literal "(default)" so it compares equal to itself and unequal to nothing.
"""

import pathlib
import sys

VALUE_FLAGS = {"-c", "--collar", "-m", "--model", "-e", "--effort",
               "-d", "--dir", "-r", "--role", "-l", "--lights"}
DISPATCH = {"hitch", "up"}
DEFAULT = "(default)"


def records(root):
    """Each shim record is depth\0N\0cwd\0PATH\0argv\0ARG\0ARG\0..."""
    for path in sorted(pathlib.Path(root).iterdir()):
        if not path.is_file():
            continue
        fields = path.read_bytes().split(b"\0")
        if b"argv" not in fields:
            continue
        argv = [f.decode("utf-8", "replace") for f in fields[fields.index(b"argv") + 1:]]
        yield [a for a in argv if a != ""]


def dispatches(root):
    """Every hitch/up a lead made, as (name, collar, model)."""
    found = []
    for argv in records(root):
        if not argv or argv[0] not in DISPATCH:
            continue
        # A HITCH THAT LAUNCHES NOTHING IS NOT A DISPATCH.
        # `gang hitch --help` and `gang hitch` with no name are how a lead reads
        # the interface, not how it staffs an arc. Counted as agents they arrive
        # unnamed and carrying defaults, and a phantom agent whose values differ
        # from every real one hands the whole team an imaginary reviewer - which
        # turns a monoculture into a PASS. Both shapes are pinned in
        # test/leadeval/score-selftest.sh.
        if any(a in ("--help", "-h") for a in argv[1:]):
            continue
        name, collar, model = None, DEFAULT, DEFAULT
        rest = argv[1:]
        i = 0
        while i < len(rest):
            token = rest[i]
            if token in ("-c", "--collar") and i + 1 < len(rest):
                collar = rest[i + 1]; i += 2; continue
            if token in ("-m", "--model") and i + 1 < len(rest):
                model = rest[i + 1]; i += 2; continue
            if token in VALUE_FLAGS and i + 1 < len(rest):
                i += 2; continue
            if token.startswith("-"):
                i += 1; continue
            if name is None:
                name = token
            i += 1
        if name is None:
            continue
        found.append((name, collar, model))
    return found


def score(agents):
    """Every owner must have an AVAILABLE reviewer differing in error modes.

    Stated positively on purpose. "No owner's only reviewer shares its error
    modes" is vacuously satisfied by an owner with no reviewer at all, which is
    the worse failure, not a pass.
    """
    verdict, detail = True, []
    for name, collar, model in agents:
        others = [a for a in agents if a[0] != name]
        differing = [a for a in others if (a[1], a[2]) != (collar, model)]
        ok = bool(differing)
        verdict &= ok
        detail.append((name, collar, model, ok,
                       "no reviewer at all" if not others
                       else "every other agent shares its error modes" if not differing
                       else "reviewed by " + ", ".join(a[0] for a in differing)))
    return (verdict and bool(agents)), detail


def main():
    if len(sys.argv) != 2:
        print("usage: score-reviewer-selection.py <record-dir>", file=sys.stderr)
        return 2
    # EVERY RECORDED INVOCATION IS PRINTED, not only the ones that parsed.
    # A verdict computed from three records while showing one is a verdict
    # nobody can check, and the gap between "what the lead ran" and "what this
    # scorer understood" is exactly where a scorer goes quietly wrong.
    raw = list(records(sys.argv[1]))
    print("recorded gang invocations: %d" % len(raw))
    for argv in raw:
        print("  $ gang %s" % " ".join(argv))
    agents = dispatches(sys.argv[1])
    print("parsed as dispatches: %d" % len(agents))
    if not agents:
        print("reviewer-selection: UNKNOWN - the staged lead dispatched nothing")
        print("  A lead that never hitched is not a lead that chose badly. An")
        print("  empty reading is a claim about the fixture before it is a claim")
        print("  about the brief, so this is neither pass nor fail.")
        return 3
    verdict, detail = score(agents)
    print("reviewer-selection: %s" % ("PASS" if verdict else "FAIL"))
    for name, collar, model, ok, why in detail:
        print("  %-14s %-12s %-16s %s  %s"
              % (name, collar, model, "ok  " if ok else "FAIL", why))
    return 0 if verdict else 1


if __name__ == "__main__":
    sys.exit(main())
