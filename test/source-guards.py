#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Reject unclassified positive guards over combined observation surfaces.

Method:
  1. discover functions that produce tmux pane captures;
  2. follow their bytes through assignments, aliases, shell transforms, and
     wrapper functions;
  3. find positive helper, case, grep, and Bash-pattern guards consuming that
     evidence;
  4. require a statement-bound source classification or a reviewed migration
     fingerprint.

The migration ledger is a multiset, not a wildcard: changing, copying, moving
between files, or deleting a reviewed assertion forces another decision. Inline
classifications carry a digest of their statement, so changing the assertion
also invalidates the classification immediately.
"""

from __future__ import annotations

import argparse
import collections
import hashlib
import pathlib
import re
import shlex
import sys
from dataclasses import dataclass


REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
SOURCE_NAME = re.compile(r"(?:^|_)(?:transcript|log)(?:_|$)")
ASSIGNMENT_WORD = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)=(.*)$", re.S)
VARIABLE = re.compile(r"\$\{?([A-Za-z_][A-Za-z0-9_]*)")
FUNCTION = re.compile(r"^[ \t]*([A-Za-z_][A-Za-z0-9_]*)[ \t]*\(\)[ \t]*\{")
CLASSIFICATION = re.compile(
    r"^[ \t]*#[ \t]*source-guard:[ \t]*"
    r"(whole-surface|producer)@([0-9a-f]{12}):[ \t]*(\S.{15,})$"
)
SOURCE_GUARD_COMMENT = re.compile(r"^[ \t]*#[ \t]*source-guard:")
LEDGER_NOTE = re.compile(r"\S.{15,}", re.S)
LEDGER_PLACEHOLDER = "REVIEW_REQUIRED_REPLACE_ME"
COMMAND_PREFIX = r"(?:^[ \t]*|[;&|(){}]\s*|\b(?:if|elif|while|until|then|do)\s+)"
HEREDOC = re.compile(r"<<(-?)[ \t]*(['\"]?)([A-Za-z_][A-Za-z0-9_]*)\2")


@dataclass(frozen=True)
class Annotation:
    kind: str
    token: str
    reason: str


@dataclass(frozen=True)
class Statement:
    start: int
    text: str
    annotation: Annotation | None


@dataclass(frozen=True)
class Finding:
    path: pathlib.Path
    line: int
    digest: str
    statement_token: str
    text: str
    sources: tuple[str, ...]
    annotation: Annotation | None
    annotation_mismatch: bool


def command_substitution_open(text: str) -> bool:
    return text.count("$(") > text.count(")")


def logical_statements(text: str) -> list[Statement]:
    lines = text.splitlines()
    result: list[Statement] = []
    pending_annotation: Annotation | None = None
    i = 0
    while i < len(lines):
        line = lines[i]
        match = CLASSIFICATION.match(line)
        if match:
            pending_annotation = Annotation(*match.groups())
            i += 1
            continue
        if SOURCE_GUARD_COMMENT.match(line):
            pending_annotation = None
            i += 1
            continue
        if not line.strip() or line.lstrip().startswith("#"):
            pending_annotation = None
            i += 1
            continue

        start = i + 1
        parts = [line]
        heredocs = [(m.group(3), bool(m.group(1))) for m in HEREDOC.finditer(line)]
        case_depth = len(re.findall(rf"{COMMAND_PREFIX}case\s+", line))
        case_depth -= len(re.findall(r"\besac\b", line))
        while True:
            while heredocs and i + 1 < len(lines):
                delimiter, strips_tabs = heredocs.pop(0)
                while i + 1 < len(lines):
                    i += 1
                    body_line = lines[i]
                    parts.append(body_line)
                    compared = body_line.lstrip("\t") if strips_tabs else body_line
                    if compared == delimiter:
                        break
            joined = "\n".join(parts)
            if not (
                parts[-1].rstrip().endswith("\\")
                or command_substitution_open(joined)
                or case_depth > 0
            ):
                break
            if i + 1 >= len(lines):
                break
            i += 1
            next_line = lines[i]
            parts.append(next_line)
            case_depth += len(re.findall(rf"{COMMAND_PREFIX}case\s+", next_line))
            case_depth -= len(re.findall(r"\besac\b", next_line))
            heredocs.extend(
                (m.group(3), bool(m.group(1))) for m in HEREDOC.finditer(next_line)
            )

        result.append(Statement(start, "\n".join(parts), pending_annotation))
        pending_annotation = None
        i += 1
    return result


def normalize(statement: str) -> str:
    return " ".join(statement.replace("\\\n", " ").split())


def canonical_path(path: pathlib.Path) -> pathlib.Path:
    resolved = path.resolve()
    try:
        return resolved.relative_to(REPO_ROOT)
    except ValueError:
        return resolved


def statement_token(statement: str) -> str:
    return hashlib.sha256(normalize(statement).encode()).hexdigest()[:12]


def function_bodies(statements: list[Statement]) -> dict[str, str]:
    bodies: dict[str, list[str]] = {}
    current: str | None = None
    depth = 0
    for statement in statements:
        header = FUNCTION.match(statement.text)
        if header:
            current = header.group(1)
            bodies.setdefault(current, []).append(statement.text)
            depth = statement.text.count("{") - statement.text.count("}")
            if depth <= 0:
                current = None
            continue
        if current is not None:
            bodies[current].append(statement.text)
            depth += statement.text.count("{") - statement.text.count("}")
            if depth <= 0:
                current = None
    return {name: "\n".join(parts) for name, parts in bodies.items()}


def has_command(text: str, names: set[str]) -> bool:
    return any(
        re.search(
            rf"{COMMAND_PREFIX}{re.escape(name)}(?:\s|$)", text, re.MULTILINE
        )
        for name in names
    )


def capture_functions(bodies: dict[str, str]) -> set[str]:
    producers = {"pane", "pane_all"}
    for name, body in bodies.items():
        if "tmux capture-pane" in body or (
            " capture" in body and ("$GANG" in body or " gang " in f" {body} ")
        ):
            producers.add(name)
    changed = True
    while changed:
        changed = False
        for name, body in bodies.items():
            if name not in producers and has_command(body, producers):
                producers.add(name)
                changed = True
    return producers


def assertion_functions(bodies: dict[str, str]) -> set[str]:
    """Find shell helpers whose result is itself a positive assertion.

    This is deliberately separate from capture-producer discovery: a test
    function may contain both a capture and many assertions without making
    every value returned by that function captured evidence.
    """
    assertions = {"contains", "equal"}
    changed = True
    while changed:
        changed = False
        for name, body in bodies.items():
            if name not in assertions and (
                has_command(body, assertions)
                or positive_case(body, unknown_is_positive=False)
            ):
                assertions.add(name)
                changed = True
    return assertions


def calls_producer(text: str, producers: set[str]) -> list[str]:
    sources: list[str] = []
    if "tmux capture-pane" in text:
        sources.append("tmux capture-pane")
    if " capture" in text and ("$GANG" in text or re.search(r"\bgang\s+capture\b", text)):
        sources.append("gang capture")
    for producer in sorted(producers):
        escaped = re.escape(producer)
        if has_command(text, {producer}) and not FUNCTION.match(text):
            sources.append(producer)
        if re.search(rf"\$\([\s\n]*{escaped}(?:[\s\n)]|$)", text):
            sources.append(producer)
        if re.search(rf"`[\s\n]*{escaped}(?:[\s\n`]|$)", text):
            sources.append(producer)
    return sources


def assignment_pairs(statement: str) -> list[tuple[str, str]]:
    try:
        tokens = shlex.split(normalize(statement), comments=False, posix=True)
    except ValueError:
        tokens = []
    pairs: list[tuple[str, str]] = []
    for token in tokens:
        match = ASSIGNMENT_WORD.match(token)
        if match:
            pairs.append((match.group(1), match.group(2)))
    if pairs:
        return pairs
    fallback = re.match(
        r"^(?:(?:local|export|declare|readonly|typeset)\s+)?"
        r"([A-Za-z_][A-Za-z0-9_]*)=(.*)$",
        statement.strip(),
        re.S,
    )
    return [fallback.groups()] if fallback else []


def bare_declarations(statement: str) -> set[str]:
    """Return names reset by a shell local/export-style declaration."""
    try:
        tokens = shlex.split(normalize(statement), comments=False, posix=True)
    except ValueError:
        return set()
    if not tokens or tokens[0] not in {
        "local",
        "export",
        "declare",
        "readonly",
        "typeset",
    }:
        return set()
    return {
        token
        for token in tokens[1:]
        if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", token)
    }


def evidence_roots(
    text: str,
    producers: set[str],
    tainted: dict[str, tuple[str, ...]],
    tainted_files: dict[str, tuple[str, ...]] | None = None,
) -> list[str]:
    roots = calls_producer(text, producers)
    for referenced in VARIABLE.findall(text):
        roots.extend(tainted.get(referenced, ()))
        if SOURCE_NAME.search(referenced):
            roots.append(f"name:{referenced}")
    for path, sources in (tainted_files or {}).items():
        escaped_path = re.escape(path)
        if (
            re.search(
                rf"\bcat\s+(?:--\s+)?['\"]?{escaped_path}(?:['\"]|[\s)]|$)",
                text,
            )
            or re.search(
                rf"\$\(\s*<\s*['\"]?{escaped_path}(?:['\"]|[\s)]|$)", text
            )
            or re.search(
                rf"\bgrep\b[^\n]*['\"]?{escaped_path}(?:['\"]|\s|$)", text
            )
        ):
            roots.extend(sources)
    return roots


def positive_case(text: str, *, unknown_is_positive: bool = True) -> bool:
    """Recognize fail-on-absence case guards without flagging exclusions."""
    if not re.search(rf"{COMMAND_PREFIX}case\s+", text, re.MULTILINE):
        return False
    positive = [
        match.start()
        for match in re.finditer(
            rf"{COMMAND_PREFIX}pass(?:\s|$)", text, re.MULTILINE
        )
    ]
    positive.extend(
        match.start()
        for match in re.finditer(r"\bprintf\s+['\"](?:ok|PASS)(?:\s|['\"])", text)
    )
    negative = [
        match.start()
        for match in re.finditer(
            rf"{COMMAND_PREFIX}fail(?:\s|$)", text, re.MULTILINE
        )
    ]
    negative.extend(
        match.start()
        for match in re.finditer(r"\bprintf\s+['\"]FAIL(?:\s|['\"])", text)
    )
    if positive and negative:
        return min(positive) < min(negative)
    # An unclassified pattern match can still be a positive shell guard. Make
    # the author classify it rather than silently treating unknown as negative.
    return unknown_is_positive


def positive_grep(text: str) -> bool:
    """Recognize quiet grep used as a positive condition, but not `! grep`."""
    return bool(
        re.search(
            rf"{COMMAND_PREFIX}grep\s+(?:-[A-Za-z]*q[A-Za-z]*\s+|--quiet(?:\s|=))",
            text,
        )
    )


def positive_bracket(text: str) -> bool:
    """Recognize positive Bash pattern/regex tests over evidence."""
    return bool(re.search(r"\[\[.*(?:==|=~).*\]\]", text, re.S))


def transfer_targets(statement: str) -> set[str]:
    """Return variables populated by byte-carrying shell builtins."""
    try:
        tokens = shlex.split(normalize(statement), comments=False, posix=True)
    except ValueError:
        return set()
    targets: set[str] = set()
    for command in ("read", "mapfile", "readarray"):
        if command not in tokens:
            continue
        index = tokens.index(command) + 1
        candidates: list[str] = []
        while index < len(tokens) and not tokens[index].startswith("<<<"):
            token = tokens[index]
            if token.startswith("-"):
                index += 1
                continue
            if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", token):
                candidates.append(token)
            index += 1
        if command == "read":
            targets.update(candidates)
        elif candidates:
            targets.add(candidates[-1])
    for index, token in enumerate(tokens[:-1]):
        if token == "-v" and index > 0 and tokens[index - 1] == "printf":
            target = tokens[index + 1]
            if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", target):
                targets.add(target)
    return targets


def redirection_targets(text: str) -> set[str]:
    """Return literal files overwritten by a statement."""
    return {
        match.group(2)
        for match in re.finditer(
            r"(?<![0-9<>])>(?!>)[ \t]*(['\"]?)([^\s;|]+)\1", text
        )
        if match.group(2) != "/dev/null"
    }


def is_positive_guard(
    statement: Statement,
    assertions: set[str],
) -> bool:
    if FUNCTION.match(statement.text) or re.match(
        r"^[ \t]*cat\b[^\n]*<<", statement.text
    ):
        return False
    if has_command(statement.text, assertions):
        return True
    return (
        positive_case(statement.text)
        or positive_grep(statement.text)
        or positive_bracket(statement.text)
    )


def scan(path: pathlib.Path) -> list[Finding]:
    statements = logical_statements(path.read_text(encoding="utf-8"))
    bodies = function_bodies(statements)
    producers = capture_functions(bodies)
    assertions = assertion_functions(bodies)
    tainted: dict[str, tuple[str, ...]] = {}
    tainted_files: dict[str, tuple[str, ...]] = {}
    findings: list[Finding] = []
    display_path = canonical_path(path)

    for statement in statements:
        for name in bare_declarations(statement.text):
            tainted.pop(name, None)
        for name, value in assignment_pairs(statement.text):
            roots = evidence_roots(value, producers, tainted, tainted_files)
            if roots:
                tainted[name] = tuple(sorted(set(roots)))
            else:
                tainted.pop(name, None)

        statement_roots = evidence_roots(
            statement.text, producers, tainted, tainted_files
        )
        for name in transfer_targets(statement.text):
            if statement_roots:
                tainted[name] = tuple(sorted(set(statement_roots)))
            else:
                tainted.pop(name, None)
        for target in redirection_targets(statement.text):
            if statement_roots:
                tainted_files[target] = tuple(sorted(set(statement_roots)))
            else:
                tainted_files.pop(target, None)

        if not is_positive_guard(statement, assertions):
            continue
        roots = evidence_roots(statement.text, producers, tainted, tainted_files)
        if not roots:
            continue

        normal = normalize(statement.text)
        digest = hashlib.sha256(
            f"{display_path.as_posix()}\0{normal}".encode()
        ).hexdigest()[:20]
        token = statement_token(statement.text)
        annotation = statement.annotation
        mismatch = annotation is not None and annotation.token != token
        if mismatch:
            annotation = None
        findings.append(
            Finding(
                path=display_path,
                line=statement.start,
                digest=digest,
                statement_token=token,
                text=normal,
                sources=tuple(sorted(set(roots))),
                annotation=annotation,
                annotation_mismatch=mismatch,
            )
        )
    return findings


def read_ledger(path: pathlib.Path) -> collections.Counter[str]:
    allowed: collections.Counter[str] = collections.Counter()
    if not path.exists():
        return allowed
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split("\t", 2)
        if (
            len(fields) != 3
            or not re.fullmatch(r"[0-9a-f]{20}", fields[0])
            or not fields[1].isdigit()
            or int(fields[1]) <= 0
            or not LEDGER_NOTE.fullmatch(fields[2])
            or fields[2] == LEDGER_PLACEHOLDER
        ):
            raise ValueError(
                f"{path}:{number}: expected DIGEST<TAB>POSITIVE-COUNT<TAB>"
                "REVIEW-NOTE (at least 16 characters, not the generated placeholder)"
            )
        allowed[fields[0]] += int(fields[1])
    return allowed


def print_finding(finding: Finding) -> None:
    print(
        f"{finding.path}:{finding.line}: unclassified positive guard reads a "
        f"combined surface ({', '.join(finding.sources)})",
        file=sys.stderr,
    )
    if finding.annotation_mismatch:
        print("  its source-guard fingerprint no longer matches the statement", file=sys.stderr)
    print(f"  {finding.text}", file=sys.stderr)
    print(
        "  bind the claim to its producer, or classify why the whole surface is "
        "the evidence by adding immediately above it:",
        file=sys.stderr,
    )
    print(
        f"  # source-guard: producer@{finding.statement_token}: "
        "<independent witness and why it binds>",
        file=sys.stderr,
    )
    print(
        f"  # source-guard: whole-surface@{finding.statement_token}: "
        "<why any visible producer is valid>",
        file=sys.stderr,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--ledger",
        type=pathlib.Path,
        default=pathlib.Path("test/source-guards.allow"),
    )
    parser.add_argument(
        "--discover",
        type=pathlib.Path,
        metavar="TEST-DIR",
        help="scan every *.sh test except source-guards-fixtures.sh",
    )
    parser.add_argument("--emit-ledger", action="store_true")
    parser.add_argument("files", nargs="*", type=pathlib.Path)
    args = parser.parse_args()

    if args.discover is not None and args.files:
        parser.error("--discover and explicit files are mutually exclusive")
    if args.discover is not None:
        if not args.discover.is_dir():
            parser.error(f"discovery directory does not exist: {args.discover}")
        files = sorted(
            path
            for path in args.discover.glob("*.sh")
            if path.name != "source-guards-fixtures.sh"
        )
        if not files:
            parser.error(f"discovery found no shell tests in: {args.discover}")
    elif args.files:
        files = args.files
    else:
        parser.error("provide --discover TEST-DIR or at least one file")

    findings = [finding for path in files for finding in scan(path)]
    unclassified = [finding for finding in findings if finding.annotation is None]
    counts = collections.Counter(finding.digest for finding in unclassified)

    if args.emit_ledger:
        for digest in sorted(counts):
            print(f"{digest}\t{counts[digest]}\t{LEDGER_PLACEHOLDER}")
        return 0

    try:
        allowed = read_ledger(args.ledger)
    except (OSError, UnicodeError, ValueError) as error:
        print(f"source-guards: {error}", file=sys.stderr)
        return 2

    failed = False
    seen: collections.Counter[str] = collections.Counter()
    for finding in unclassified:
        seen[finding.digest] += 1
        if seen[finding.digest] <= allowed[finding.digest]:
            continue
        failed = True
        print_finding(finding)

    stale = allowed - counts
    if stale:
        failed = True
        for digest, count in sorted(stale.items()):
            print(
                f"{args.ledger}: stale reviewed fingerprint {digest} ({count} unused); "
                "remove it or classify the changed guard",
                file=sys.stderr,
            )

    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
