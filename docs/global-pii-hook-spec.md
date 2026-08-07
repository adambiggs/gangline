# Host-global pre-push PII gate — implementation spec

Every `git push` from every repository on the operator's host is PII-scanned
before anything leaves the machine. Today only this repository is gated, and
only because a contributor ran one opt-in command.

This is an implementation record, not a standing document. When the work lands,
its durable rule moves to `docs/DECISIONS.md`, its commands to
`docs/reference.md`, and its recovery path to `docs/operations.md`, and this
file is deleted.

## Verified git behaviour

Every claim here was executed against this host's git in a disposable fixture,
with `GIT_CONFIG_GLOBAL` pointed at a scratch file so the operator's real global
configuration was never read or written. Reproduce with the same technique.

**V1 — a global `core.hooksPath` replaces `$GIT_DIR/hooks` entirely.** A repo
with an executable `.git/hooks/pre-push` and a global `core.hooksPath` ran only
the hooks-path hook. git-config(1), `core.hooksPath`:

> "By default Git will look for your hooks in the `$GIT_DIR/hooks` directory.
> Set this to different path, e.g. `/etc/git/hooks`, and Git will try to find
> your hooks in that directory, e.g. `/etc/git/hooks/pre-receive` instead of in
> `$GIT_DIR/hooks/pre-receive`."

Nothing chains on git's side. Anything that must still run, the dispatcher must
run itself.

**V2 — a repository-local `core.hooksPath` beats the global one.** Ordinary
last-wins config precedence; git-config(1) FILES:

> "The files are read in the order given above, with last value found taking
> precedence over values read earlier."

with `$GIT_DIR/config` read after the user-global file. Confirmed by execution:
with both set, the local directory's hook ran and the global one did not.

**V3 — a relative `core.hooksPath` resolves against the worktree root, not
against the config file.** git-config(1):

> "The path can be either absolute or relative. A relative path is taken as
> relative to the directory where the hooks are run (see the "DESCRIPTION"
> section of linkgit:githooks[5])."

Confirmed: a global value of `relhooks` found `<worktree>/relhooks/pre-push`.
**The installed global value must therefore be absolute.** `~/`-prefixed values
*are* expanded (confirmed by execution, git treats it as a path-type variable),
but the installer writes the fully expanded path and does not rely on that.

**V4 — `git rev-parse --git-path hooks` is rewritten by `core.hooksPath` and
must not be used to find the chained hook.** Under a global hooks path it
returned the hooks-path directory, i.e. the dispatcher's own directory. In
git's source, `find_hook()` in `hook.c` builds the path with
`strbuf_git_path(&path, "hooks/%s", name)` — the same substitution
`--git-path` exposes. Use `git rev-parse --git-common-dir` instead, which is
not rewritten (confirmed by execution).

**V5 — `--git-common-dir`, not `--git-dir`, is required for linked
worktrees.** In a linked worktree `--git-dir` returned
`<main>/.git/worktrees/<name>`, which has no `hooks/`; `--git-common-dir`
returned the main `.git`. In the main worktree both returned `.git`, which
resolves correctly because hooks run from the worktree root.

**V6 — git skips a non-executable hook.** `find_hook()` gates on
`access(path.buf, X_OK)`. Confirmed by execution: a non-executable
`pre-push` that exits 1 did not block the push, and git printed the
`advice.ignoredHook` hint. The dispatcher mirrors this: chain only if
executable.

**V7 — stdin and argv.** githooks(5), `pre-push`:

> "The hook is called with two parameters which provide the name and location
> of the destination remote, if a named remote is not being used both values
> will be the same."
>
> "Information about what is to be pushed is provided on the hook's standard
> input with lines of the form: `<local-ref> SP <local-object-name> SP
> <remote-ref> SP <remote-object-name> LF`"
>
> "If the foreign ref does not yet exist the *<remote-object-name>* will be the
> all-zeroes object name. If a ref is to be deleted, the *<local-ref>* will be
> supplied as (`delete`) and the *<local-object-name>* will be the all-zeroes
> object name."

Confirmed by execution for all three shapes, including the literal `(delete)`
token.

**V8 — the hook runs even when there is nothing to push.** `git push` on an
up-to-date branch invoked the hook with *empty* stdin. The loop simply does not
execute; do not treat empty stdin as an error.

**V9 — a hooks directory missing the hook fails open, silently.** A global
`core.hooksPath` pointing at a directory with no `pre-push` let the push
through with no diagnostic. An install that half-succeeded is therefore
indistinguishable from no install at all by inspection — which is why the
installer's acceptance step is a push that must be *refused*, not a file check.

**V10 — `$0` under a symlinked hook is the symlink path.** A `pre-push`
symlink to a `dispatch` script reported `$0` ending in `/pre-push`. One
dispatcher can therefore learn its hook name from `basename "$0"`.

## Decisions

### 1. Mechanism: global `core.hooksPath`

Use a user-global `core.hooksPath` pointing at an absolute directory outside
every repository.

`init.templateDir` is rejected: it seeds `$GIT_DIR/hooks` at `git init` and
`git clone` time only. Repositories that already exist stay ungated, an update
to the dispatcher reaches nothing already cloned, and a repository that already
has hooks does not receive it. Coverage would be a snapshot, and a security gate
whose coverage silently ages is worse than one that is loudly absent. git's own
documentation names the alternative in the same entry:

> "This configuration variable is useful in cases where you'd like to centrally
> configure your Git hooks instead of configuring them on a per-repository
> basis, or as a more flexible and centralized alternative to having an
> `init.templateDir` where you've changed default hooks."

A per-repository install script is rejected for the same reason plus a worse
one: it makes coverage a thing the operator must remember per repository, which
is exactly the failure the request exists to remove.

### 2. Scanner: `tools/pii-scan`, referenced live, not copied

Reuse `tools/pii-scan`. Do not add gitleaks.

The two tools do not overlap on the thing being asked for. gitleaks detects
secrets; `home-path`, `email`, and `ip` are PII classes it does not carry, and
those are the classes the operator curated. On the credential side gitleaks is
genuinely broader than `pii-scan`'s five shapes — but adding it means a second
gate whose clean verdict is not proven. `pii-scan` re-proves every pattern
against its own fixture on every invocation and refuses to report a pass it did
not earn; a secret scanner with a missing or shadowed rule set reports clean.
Under `CONSTITUTION.md` law 8 that difference decides it.

One scanner also preserves the property `docs/DECISIONS.md` already claims under
"PII prevention is prospective": the global gate, this repository's gate, and CI
cannot drift into checking different things.

The credential gap is real and is closed by adding a class to `pii-scan` — one
`class_regex` arm, one `class_fixture` arm, one word in `CLASSES` — which is
cheaper than adopting a second engine and keeps the self-test property. Adopting
gitleaks is a separate decision with its own consumer.

**Reference, do not copy.** The dispatcher resolves the scanner through
`git config --global --get gangline.piiScanner`, an absolute path into the
Gangline tree the installer was run from. A copy taken at install time goes
stale silently: a scanner that has not learned a new class passes content the
current one would flag, and nothing announces it. A live reference fails in the
other direction — move or break the tree and every push on the host stops with
an explicit error naming the path. Both are failures; only one is visible.

Read the key with `--global`, not the repository chain, so a repository cannot
redirect the host's scanner by setting `gangline.piiScanner` in its own config.

### 3. Chaining, and what it must preserve

By V1 a global hooks path suppresses `$GIT_DIR/hooks` completely, so without
chaining this change would silently disable: this host's `pre-commit`-framework
push gates, the git-lfs `pre-push` stubs, and — through pre-commit's own
`_run_legacy()`, which requires `os.access(legacy_hook, os.X_OK)` and forwards
pre-push stdin via `subprocess.run(cmd, input=stdin)` — the `pre-push.legacy`
hook it moved aside.

Order, and the reasoning:

1. **Scan first.** The scan is fast and is the security verdict. Running a
   repository's test gates first would spend minutes before reporting a leak.
2. **On scan failure, exit without chaining.** The push is already refused. The
   only next action is to remove the content or rewrite the range, after which
   every chained gate runs again anyway. Burning a suite to add a second opinion
   to a push that cannot proceed buys nothing.
3. **Otherwise chain to `<common-dir>/hooks/<hook-name>`**, if and only if it is
   executable (V6), forwarding argv verbatim and replaying stdin (V7).
4. **Exit with the chained hook's status**, so nothing that used to block still
   blocks.

Find the chained hook with `git rev-parse --git-common-dir` (V4, V5). Using
`--git-path hooks` would return the dispatcher's own directory and recurse.

**This repository is unaffected, by V2.** Its local `core.hooksPath=.githooks`
outranks the global value, so the dispatcher never runs here and
`.githooks/pre-push` continues to be the whole gate. The generalisation is the
known limit in §9: any repository that sets a local `core.hooksPath` opts out of
the host gate. Detect with `git config --get core.hooksPath`.

### 4. Scope: one name-generic dispatcher, one hook installed

Install the dispatcher as a real file named `gangline-hook` and a `pre-push`
symlink beside it. The script reads `basename "$0"` (V10) and dispatches on it,
refusing any name it has no action for. A later hook costs one symlink and one
`case` arm. Ship `pre-push` and nothing else.

### 5. Denylists: global plus per-repository

- Global: `${XDG_CONFIG_HOME:-$HOME/.config}/gangline/pii-scan-denylist`,
  mode 0600. Operator-specific strings that no general class can express —
  personal names, hostnames, employer identifiers — usable from every
  repository.
- Per-repository: `.pii-scan-denylist`, unchanged.

Both are read by `pii-scan` itself, not by the dispatcher. That is what makes
the global denylist reach this repository's own gate and CI-adjacent `--stdin`
uses without any chaining.

Preserve the refusal-if-tracked property and extend it to both files: refuse if
either denylist path is tracked in the repository being scanned. A committed
denylist prints the strings it exists to protect into the public tree.

### 6. Failure semantics: fail closed, fail loud

The dispatcher blocks the push, naming the cause, whenever:

- `gangline.piiScanner` is unset in the global config,
- the configured scanner path is missing or unreadable,
- the scanner exits non-zero for any reason — findings, a failed self-test, a
  tracked denylist, a `grep` that cannot run, a usage error, or not being
  executable. The exit status is reported in the message so a `2` or a `127`
  is distinguishable from a finding at a glance,
- the outgoing range cannot be computed.

`pii-scan` uses status 1 for both findings and its own hard refusals, so the
dispatcher must not claim "PII found". It states that the scan did not pass and
lets the scanner's own stderr, which is already explicit, say why.

`--no-verify` skips the hook. That is git's design, it is the operator's escape
hatch, and it is named in the refusal message. Do not fight it.

### 7. New repositories

Covered with no action. A global `core.hooksPath` is consulted per invocation
against the repository's config chain; `git init` and `git clone` produce
repositories that have no local `core.hooksPath`, so the global value applies to
the first push and every push after. This is precisely the property
`init.templateDir` lacks.

### 8. Range computation

Per push line, with `zero` the all-zeroes object name:

| Case | Recognised by | Range |
|---|---|---|
| Ref deletion | `local_sha` = `zero` (V7) | skipped — nothing is being sent |
| Remote ref exists and its tip is present locally | `remote_sha` ≠ `zero` and `git cat-file -e "${remote_sha}^{commit}"` | `"${remote_sha}..${local_sha}"` |
| New ref, or a remote tip this clone does not have | otherwise | `"$local_sha" --not --remotes` |
| Nothing to push | empty stdin (V8) | loop body never runs |

`--not --remotes` bounds the scan by everything already published on any
tracking ref. When those refs are stale or absent it degrades to scanning more
history than strictly necessary — slow, never unsafe. The expressions are the
ones `.githooks/pre-push` already uses; keep them identical so the two gates
cannot answer differently about the same push.

### 9. Known limits, accepted and documented

- A repository with its own `core.hooksPath` is not covered (V2). Deliberate:
  it is an explicit operator act, and this repository relies on it.
- `--no-verify` bypasses the gate.
- Annotated tag messages are not scanned. `pii-scan --range` scans commit
  messages and added diff lines; a tag object's message is neither.
- `pii-scan` excludes `tools/pii-scan` from diff content, so a file at that
  path in any repository is unscanned. The carve-out exists because the file
  embeds every class's self-test fixture as a literal, which is equally true of
  a copy of it living at that path elsewhere.
- `KNOWN_AUTHOR_EMAILS` walks the history reachable from `HEAD` on every scan.
  Bounded and interactive, but it is a full walk in a large repository.

### 10. Author-email allowlist semantics in arbitrary repositories

`pii-scan` allowlists every address in `git log --format=%ae`. The charter asks
whether that generalises. It does: the rule is "an address that authored a
commit in this repository is already in this repository's history", and pushing
such a commit publishes that address in the commit object regardless of what the
scanner says about the diff. Flagging it would be a false positive on content
git is about to send anyway.

Cloning a large public project widens the allowlist to that project's
contributors — addresses already public in the history that was cloned. Not a
new exposure.

One defect does not generalise: on an unborn `HEAD` the command fails, and under
`set -euo pipefail` it aborts `pii-scan` before it scans anything, with a bare
`fatal:` and no explanation. Confirmed by execution. Fixed in S2 below. An empty
allowlist flags more, not less, so the guarded form fails in the safe direction.

## Artifacts

### In this repository

- `tools/gangline-git-hook` — the dispatcher. `#!/usr/bin/env bash`, SPDX
  header, `set -euo pipefail`.
- `tools/git-hooks-install` — the installer, `--uninstall` to reverse.
- `tools/pii-scan` — four changes, all backward compatible (S1–S4).
- `test/lint.sh` — widen the file list from `tools/pii-scan` to `tools/*`, for
  the reason the list already gives for globbing `.githooks/*`: a tool added
  later is a shell file this repository runs and should not need an edit here
  to be linted.
- `test/integration.sh` — the checks in §Test plan.

### On the host, written by the installer

- `${XDG_CONFIG_HOME:-$HOME/.config}/git/hooks/gangline-hook` — a copy, 0755.
- `${XDG_CONFIG_HOME:-$HOME/.config}/git/hooks/pre-push` — symlink to it.
- `git config --global core.hooksPath <that directory, absolute>`
- `git config --global gangline.piiScanner <gangline tree>/tools/pii-scan`

The dispatcher is copied and the scanner is referenced, and the asymmetry is
deliberate. A dispatcher living inside the Gangline checkout would disappear
whenever a branch predating this work was checked out — the hook file would
vanish and, by V9, every push on the host would silently stop being scanned. A
stale *dispatcher* copy still calls the current scanner; a stale *scanner* is
the failure that matters. Updating the dispatcher means re-running the
installer; say so in `docs/reference.md`.

## Dispatcher contract

```sh
hook="$(basename "$0")"
```

1. If `GANGLINE_GIT_HOOK` is already set, a repository hook points back at this
   dispatcher. Print that and exit 0 — do not scan or chain twice. Otherwise
   export it.
2. `tmp="$(mktemp -d)"`, `trap 'rm -rf "$tmp"' EXIT`, `cat > "$tmp/stdin"`.
   Read stdin exactly once, into a file, before anything else; both the scan and
   the chained hook read from that file.
3. Dispatch on `$hook`. `pre-push` runs the action below. Any other name is a
   hard error: the dispatcher was installed under a name it has no action for.
4. Chain, then exit.

`pre-push` action:

```sh
scanner="$(git config --global --get gangline.piiScanner || true)"
[ -n "$scanner" ] || die "gangline.piiScanner is not set in the global git config"
[ -r "$scanner" ] || die "scanner not found at $scanner"

zero=0000000000000000000000000000000000000000
while read -r _local_ref local_sha _remote_ref remote_sha; do
  [ -n "${local_sha:-}" ] || continue
  [ "$local_sha" != "$zero" ] || continue
  if [ "$remote_sha" != "$zero" ] \
     && git cat-file -e "${remote_sha}^{commit}" 2>/dev/null; then
    revs=("${remote_sha}..${local_sha}")
  else
    revs=("$local_sha" --not --remotes)
  fi
  rc=0
  bash "$scanner" --range "${revs[@]}" || rc=$?
  [ "$rc" -eq 0 ] || return 1
done < "$tmp/stdin"
```

Invoke through `bash "$scanner"`, matching `.githooks/pre-push`, so the gate
does not depend on the file's execute bit. The `while` loop reads from a file,
not a pipe, so it is not a subshell and `return` leaves the function.

On failure, print and exit 1:

```
gangline-hook(pre-push): refusing — the PII scan above did not pass (status N).
          Fix the content, or the scanner, and push again.
          Bypass only after reading the findings: git push --no-verify
```

Chaining:

```sh
common="$(git rev-parse --git-common-dir 2>/dev/null || true)"
[ -n "$common" ] || die "cannot locate the repository's common git dir"
next="$common/hooks/$hook"
if [ -x "$next" ]; then
  rc=0
  "$next" "$@" < "$tmp/stdin" || rc=$?
  [ "$rc" -eq 0 ] || exit "$rc"
fi
```

`--git-common-dir` returns `.git` in a main worktree and an absolute path in a
linked one (V5); both resolve from the worktree root git runs hooks in.

## Scanner changes

**S1 — run where there is no worktree.** Replace the unconditional
`cd "$(git rev-parse --show-toplevel)"` with a conditional one, so a bare
repository does not abort the scanner on line 23:

```sh
top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[ -z "$top" ] || cd "$top"
```

**S2 — survive an unborn `HEAD`.**

```sh
KNOWN_AUTHOR_EMAILS="$(git log --format=%ae 2>/dev/null | sort -u || true)"
```

**S3 — read the global denylist as well.** Add
`GLOBAL_DENYLIST="${XDG_CONFIG_HOME:-$HOME/.config}/gangline/pii-scan-denylist"`.
`scan_denylist` iterates the global file then the repository file, keeping the
existing per-line semantics: skip blank and `#` lines, treat each remaining line
as an ERE, always flag, never allow, never print the matched value. Extend
`refuse_if_denylist_tracked` to both paths; `git ls-files --error-unmatch` on a
path outside the worktree exits non-zero, which is the right answer.

**S4 — refuse a denylist pattern that will not compile.** Sourcing patterns
from a file outside the repository makes the current silent-skip a real hole:
`grep -Eq "$pat" 2>/dev/null` returns non-zero for an invalid ERE exactly as it
does for no match, so a typo removes a denylist entry with no signal. Validate
each pattern once before use — `printf '' | grep -Eq "$pat"` returns 1 for a
valid pattern that did not match and greater than 1 for an invalid one — and
refuse to scan, naming file and line number, when one does not compile. Same
law as the existing self-test.

No other change. Do not fork the scanner.

## Installer contract

`tools/git-hooks-install`, idempotent and re-runnable.

1. `ROOT="$(cd -P "$(dirname "$0")/.." && pwd)"`. The tree the installer was run
   from is the tree that gets referenced.
2. Refuse if `$ROOT/tools/pii-scan` is absent.
3. `HOOKS_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/git/hooks"`, created if
   needed.
4. Read `git config --global --get core.hooksPath`. Unset, or already equal to
   `HOOKS_DIR`, proceeds. Anything else is a hard refusal that prints the
   existing value and stops — never silently take over a hooks path something
   else installed.
5. Write `$HOOKS_DIR/gangline-hook` (0755) from `$ROOT/tools/gangline-git-hook`,
   then `ln -sfn gangline-hook "$HOOKS_DIR/pre-push"`.
6. `git config --global core.hooksPath "$HOOKS_DIR"` and
   `git config --global gangline.piiScanner "$ROOT/tools/pii-scan"`.
7. **Acceptance, by behaviour.** In a `mktemp -d` fixture: a bare remote, a work
   repository with `user.email` set to an address in the allowlisted
   `.invalid` space, and a `.git/hooks/pre-push` recorder. Then, under the
   configuration just written:
   - a commit whose content carries a fixture PII value must be **refused**;
   - a clean commit must **push**;
   - the recorder must show it ran on the clean push.

   Any of the three failing reverts steps 5 and 6 to their prior values and
   exits non-zero. By V9 a half-installed hooks path is silently inert, so the
   installer may not report success on the strength of files existing.
8. Print the hooks directory, the scanner path, and the bypass flag.

`--uninstall` unsets both global keys, removes the symlink and the copied
dispatcher, and verifies by behaviour that a fixture push carrying a fixture PII
value is no longer refused.

## Test plan

All checks go in `test/integration.sh`, in its existing `pass`/`fail`/`equal`/
`contains` style. They are git operations only: no sleeps, no polling, no
timeouts.

Isolation, non-negotiable — the suite must never read or write the operator's
real configuration:

```sh
export GIT_CONFIG_GLOBAL="$RUN_ROOT/gitconfig"
export GIT_CONFIG_SYSTEM=/dev/null
export XDG_CONFIG_HOME="$RUN_ROOT/xdg"
```

`git config --global` honours `GIT_CONFIG_GLOBAL`, so the checks drive the real
installer rather than a re-implementation of it.

**Fixture PII values must be split across a string concatenation** so that the
line in `test/integration.sh` does not itself match the class it is testing —
this repository's own gate scans the added lines of that file. Split inside the
span the pattern requires, e.g. `"AKIA" "ABCDEFGHIJKLMNOP"` assembled at
runtime, `"/ho" "me/alice/notes.txt"`, `"sk-ant-" "abcdefghijklmnop"`. Comment
why. Do not extend the `tools/pii-scan` carve-out to cover the test file.

Checks:

1. Each of the eight classes, as a fixture value in a commit, gets its push
   **refused**.
2. A clean commit **pushes**. Without this the suite would pass against a hook
   that refuses everything.
3. Chaining: a `.git/hooks/pre-push` recorder writes its argv and stdin to a
   file. After a clean push, the recorded stdin equals the exact four-field line
   built from `git rev-parse`, and argv equals the remote name and URL.
4. A chained hook exiting non-zero **blocks** the push.
5. Scan failure does not chain: pre-create the recorder's output file with a
   sentinel; after a refused push the sentinel is unchanged. Asserting the file
   is absent would pass for a recorder that never ran *and* for one that was
   never installed.
6. `gangline.piiScanner` pointing at a nonexistent path **blocks** a clean push,
   and stderr names the path.
7. A scanner that exits 2 **blocks** a clean push, and the message reports 2.
8. A repository-local `core.hooksPath` shadows the dispatcher: a hook in that
   directory runs and a push carrying a fixture PII value succeeds. This is V2
   and the §9 limit, guarded.
9. A pattern in the global denylist **blocks** a push in a repository that has
   no `.pii-scan-denylist`.
10. New-branch bounding: content already reachable from a tracking ref does not
    re-flag; a fixture value in a new commit does.
11. `git push --delete` of a branch whose tip carries a fixture value
    **succeeds** — the zero local object name is skipped (V7).
12. An up-to-date push **succeeds** with empty stdin (V8).
13. `--no-verify` bypasses. Guarded so removing the bypass is a deliberate act.
14. Running the installer twice exits 0 and checks 1 and 2 still hold.
15. The installer refuses a foreign global `core.hooksPath` and leaves it
    unchanged.
16. After `--uninstall`, a push carrying a fixture value succeeds and
    `.git/hooks/pre-push` runs directly.
17. An uncompilable global denylist pattern **blocks**, naming file and line
    (S4).

## Documentation at landing

- `docs/DECISIONS.md` — one terse entry: the push gate is host-global, one
  scanner serves every gate, the dispatcher chains rather than replaces, and a
  repository-local hooks path opts out.
- `docs/reference.md` — install and uninstall commands, the two global config
  keys, both denylist paths, the bypass flag, and re-running the installer to
  update the dispatcher.
- `docs/operations.md` — recovery when the referenced tree moves and every push
  on the host starts refusing.
- Delete this file.
