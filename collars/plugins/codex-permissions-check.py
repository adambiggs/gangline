#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Refuse a Codex permission profile that cannot initialize CODEX_HOME."""

from __future__ import annotations

import os
import sys
from pathlib import Path


def refuse(message: str, status: int) -> int:
    print(message)
    return status


def beneath(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
    except ValueError:
        return False
    return True


def resolved(value: str, home: Path) -> Path:
    if value == "~":
        return home
    if value.startswith("~/"):
        return (home / value[2:]).resolve(strict=False)
    path = Path(value)
    if not path.is_absolute():
        raise ValueError(f"permission path {value!r} is not absolute or home-relative")
    return path.resolve(strict=False)


def main() -> int:
    if len(sys.argv) != 2:
        return refuse("Codex permission preflight requires the hitch directory", 4)

    user_home = Path.home().resolve(strict=False)
    codex_home = Path(
        os.environ.get("CODEX_HOME") or user_home / ".codex"
    ).expanduser().resolve(strict=False)
    hitch_root = Path(sys.argv[1]).expanduser().resolve(strict=False)
    config_path = codex_home / "config.toml"
    if not config_path.is_file():
        return refuse(
            f"Codex config {config_path} is absent, so Gangline cannot identify "
            "the permission profile this hitch would use",
            1,
        )

    try:
        import tomllib

        with config_path.open("rb") as stream:
            config = tomllib.load(stream)
    except (ImportError, OSError, UnicodeError, ValueError) as error:
        return refuse(
            f"Codex config {config_path} is not readable TOML "
            f"({type(error).__name__}), so Gangline cannot validate its permission profile",
            1,
        )

    if "sandbox_mode" in config or "sandbox_workspace_write" in config:
        return refuse(
            f"Codex config {config_path} selects legacy sandbox settings, so its "
            "default_permissions profile is not the policy this hitch would use",
            2,
        )

    selected = config.get("default_permissions")
    if not isinstance(selected, str) or not selected:
        return refuse(
            f"Codex config {config_path} does not name default_permissions, so "
            "Gangline cannot identify the permission profile this hitch would use",
            2,
        )

    permissions = config.get("permissions", {})
    if not isinstance(permissions, dict):
        return refuse(
            f"Codex config {config_path} has a non-table permissions value, so "
            f"profile {selected!r} cannot be resolved",
            2,
        )

    direct_rules: dict[Path, str] = {}
    workspace_rules: dict[str, str] = {}
    workspace_roots: dict[Path, bool] = {hitch_root: True}
    unresolved: list[str] = []
    full_access = False

    def add_path_rule(path: Path, access: object) -> None:
        if access not in ("read", "write", "deny"):
            raise ValueError(f"filesystem rule for {path} has invalid access {access!r}")
        access = str(access)
        previous = direct_rules.get(path)
        rank = {"read": 1, "write": 2, "deny": 3}
        if previous is None or rank[access] > rank[previous]:
            direct_rules[path] = access

    def add_scoped_rule(base: Path, name: str, access: object) -> None:
        if access not in ("read", "write", "deny"):
            raise ValueError(f"filesystem rule for {base}/{name} has invalid access {access!r}")
        if any(char in name for char in "*?["):
            unresolved.append(str(base / name))
            return
        relative = Path(name)
        if relative.is_absolute() or ".." in relative.parts:
            raise ValueError(f"scoped permission path {name!r} leaves its root")
        add_path_rule(base if name == "." else (base / relative).resolve(strict=False), access)

    def apply_builtin(name: str) -> None:
        nonlocal full_access
        if name == ":danger-full-access":
            full_access = True
            return
        add_path_rule(Path("/"), "read")
        if name == ":read-only":
            return
        if name != ":workspace":
            raise KeyError(name)
        tmpdir = Path(os.environ.get("TMPDIR") or "/tmp").resolve(strict=False)
        add_path_rule(tmpdir, "write")
        add_path_rule(Path("/tmp"), "write")
        workspace_rules["."] = "write"
        # Codex protects this workspace-owned configuration path even under the
        # built-in writable profile unless a child profile explicitly reopens it.
        workspace_rules[".codex"] = "read"

    def apply_profile(name: str, visiting: tuple[str, ...] = ()) -> None:
        if name.startswith(":"):
            apply_builtin(name)
            return
        if name in visiting:
            raise ValueError(f"permission profile inheritance cycle at {name!r}")
        profile = permissions.get(name)
        if not isinstance(profile, dict):
            raise KeyError(name)
        parent = profile.get("extends")
        if parent is not None:
            if not isinstance(parent, str) or not parent:
                raise ValueError(f"permission profile {name!r} has an invalid parent")
            if parent == ":danger-full-access":
                raise ValueError(f"permission profile {name!r} extends forbidden parent {parent!r}")
            apply_profile(parent, visiting + (name,))

        roots = profile.get("workspace_roots", {})
        if not isinstance(roots, dict):
            raise ValueError(f"permission profile {name!r} has non-table workspace_roots")
        for raw_root, enabled in roots.items():
            if not isinstance(raw_root, str) or not isinstance(enabled, bool):
                raise ValueError(f"permission profile {name!r} has an invalid workspace root")
            workspace_roots[resolved(raw_root, user_home)] = enabled

        filesystem = profile.get("filesystem", {})
        if not isinstance(filesystem, dict):
            raise ValueError(f"permission profile {name!r} has a non-table filesystem")
        for raw_base, access in filesystem.items():
            if raw_base == "glob_scan_max_depth":
                continue
            if raw_base == ":workspace_roots":
                if not isinstance(access, dict):
                    raise ValueError(f"permission profile {name!r} has invalid :workspace_roots rules")
                for subpath, scoped_access in access.items():
                    if not isinstance(subpath, str):
                        raise ValueError(f"permission profile {name!r} has a non-string scoped path")
                    if scoped_access not in ("read", "write", "deny"):
                        raise ValueError(f"permission profile {name!r} has invalid scoped access")
                    workspace_rules[subpath] = str(scoped_access)
                continue
            if raw_base == ":root":
                base = Path("/")
            elif raw_base == ":tmpdir":
                base = Path(os.environ.get("TMPDIR") or "/tmp").resolve(strict=False)
            elif raw_base == ":slash_tmp":
                base = Path("/tmp")
            elif raw_base == ":minimal":
                # The documented minimal runtime set is intentionally not a
                # Codex state-home grant.
                continue
            elif raw_base.startswith(":"):
                raise ValueError(f"permission profile {name!r} uses unknown token {raw_base!r}")
            elif any(char in raw_base for char in "*?["):
                unresolved.append(raw_base)
                continue
            else:
                base = resolved(raw_base, user_home)
            if isinstance(access, dict):
                for subpath, scoped_access in access.items():
                    if not isinstance(subpath, str):
                        raise ValueError(f"permission profile {name!r} has a non-string scoped path")
                    add_scoped_rule(base, subpath, scoped_access)
            else:
                add_path_rule(base, access)

    try:
        apply_profile(selected)
    except KeyError as error:
        missing = error.args[0]
        return refuse(
            f"Codex config {config_path} selects permission profile {selected!r}, "
            f"but profile {missing!r} is not defined",
            2,
        )
    except ValueError as error:
        return refuse(
            f"Codex permission profile {selected!r} in {config_path} cannot be "
            f"validated: {error}",
            2,
        )

    if full_access:
        return 0

    for root, enabled in workspace_roots.items():
        if not enabled:
            continue
        for subpath, access in workspace_rules.items():
            if any(char in subpath for char in "*?["):
                unresolved.append(str(root / subpath))
                continue
            add_scoped_rule(root, subpath, access)

    def glob_may_touch_home(pattern: str) -> bool:
        wildcard = min(
            (pattern.find(char) for char in "*?[" if char in pattern),
            default=len(pattern),
        )
        prefix = pattern[:wildcard].rstrip("/") or "/"
        prefix_path = Path(prefix)
        if not prefix_path.is_absolute():
            return True
        prefix_path = prefix_path.resolve(strict=False)
        return beneath(codex_home, prefix_path) or beneath(prefix_path, codex_home)

    if any(glob_may_touch_home(pattern) for pattern in unresolved):
        return refuse(
            f"Codex permission profile {selected!r} in {config_path} uses glob "
            "filesystem rules that may cover CODEX_HOME, so Gangline cannot prove it is wholly writable",
            2,
        )

    covering = [
        (len(path.parts), access)
        for path, access in direct_rules.items()
        if beneath(codex_home, path)
    ]
    access = max(covering, default=(-1, "deny"))[1]
    carved_out = any(
        beneath(path, codex_home) and path != codex_home and rule != "write"
        for path, rule in direct_rules.items()
    )
    if access != "write" or carved_out:
        return refuse(
            f"Codex permission profile {selected!r} in {config_path} does not "
            f"grant write access to CODEX_HOME {codex_home}; nested codex cannot initialize there",
            3,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
