#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Check durable ADR structure without relying on founding-import evidence."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ADR = ROOT / "docs" / "adr"
PLACEHOLDER = "Founding-import placeholder:"
FOUNDING_MAX_ID = 181
FOUNDING_PLACEHOLDER_MAX = 177
errors: list[str] = []


def fail(message: str) -> None:
    errors.append(message)


records = sorted(ADR.glob("[0-9][0-9][0-9][0-9]-*.md"))
seen_ids: set[str] = set()
record_rows: list[tuple[str, str, str, str, str]] = []
placeholder_ids: list[str] = []

for path in records:
    text = path.read_text()
    ident = path.name[:4]
    if ident in seen_ids:
        fail(f"duplicate ADR id {ident}")
    seen_ids.add(ident)

    frontmatter = re.match(r"\A---\n(.*?)\n---\n", text, re.DOTALL)
    if not frontmatter:
        fail(f"{path.name}: missing frontmatter")
        continue
    fields: dict[str, str] = {}
    keys: list[str] = []
    for line in frontmatter.group(1).splitlines():
        key, separator, value = line.partition(": ")
        if not separator:
            fail(f"{path.name}: malformed frontmatter line {line!r}")
            continue
        keys.append(key)
        fields[key] = value
    expected_keys = ["id", "status", "date", "supersedes", "superseded-by", "tags"]
    if keys != expected_keys:
        fail(f"{path.name}: frontmatter keys are {keys}, expected {expected_keys}")
    if fields.get("id") != ident:
        fail(f"{path.name}: frontmatter id is {fields.get('id')!r}")

    heading = re.search(rf"^# ADR-{ident}: (.+)$", text, re.MULTILINE)
    if not heading:
        fail(f"{path.name}: missing matching H1")
        continue
    title = heading.group(1)
    slug = "-".join(re.findall(r"[a-z0-9]+", title.lower()))
    expected_name = f"{ident}-{slug}.md"
    if path.name != expected_name:
        fail(f"{path.name}: complete title slug requires {expected_name}")
    if len(path.name.encode()) > 255:
        fail(f"{path.name}: filename exceeds 255 bytes")

    sections = re.findall(r"^## .+$", text, re.MULTILINE)
    if fields.get("status") == "superseded":
        if sections:
            fail(f"{path.name}: superseded tombstone retains body sections")
    elif sections != ["## Context", "## Decision", "## Consequences"]:
        fail(f"{path.name}: section shape is {sections}")

    placeholder_count = text.count(PLACEHOLDER)
    if placeholder_count > 1:
        fail(f"{path.name}: carries {placeholder_count} founding placeholders")
    if placeholder_count:
        placeholder_ids.append(ident)
        if int(ident) > FOUNDING_MAX_ID:
            fail(f"{path.name}: a post-founding record carries the founding placeholder")

    record_rows.append((ident, path.name, title, fields.get("status", ""), fields.get("date", "")))

if len(placeholder_ids) > FOUNDING_PLACEHOLDER_MAX:
    fail(
        f"founding placeholder population grew to {len(placeholder_ids)}; "
        f"maximum is {FOUNDING_PLACEHOLDER_MAX}"
    )

index = (ADR / "index.md").read_text()
row_pattern = re.compile(
    r"^\| \[(\d{4})\]\(([^)]+)\) \| (.+) \| ([a-z-]+) \| (\d{4}-\d{2}-\d{2}) \|$",
    re.MULTILINE,
)
index_rows = row_pattern.findall(index)
if index_rows != record_rows:
    fail("ADR index rows do not match filename-order record metadata")
for _, target, _, _, _ in index_rows:
    if not (ADR / target).is_file():
        fail(f"ADR index target does not exist: {target}")

if errors:
    for error in errors:
        print(f"adr-corpus: {error}", file=sys.stderr)
    raise SystemExit(1)

print(
    f"adr-corpus: {len(records)} records structurally sound; "
    f"{len(placeholder_ids)} founding placeholders remain"
)
