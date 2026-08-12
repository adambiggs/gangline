# User configuration, operator doctrine, and hitch discipline — implementation spec

> Status: Landed at `132bfd6` on 2026-08-07; retained as a dated
> implementation record. Superseded in part: §2.3 and §2.6 argue the 8192-byte
> ceiling as a category-error guard over the doctrine slot. The bound was
> removed because byte count predicted neither pane delivery nor system-prompt
> acceptance. Sections 1.4 and 1.6 also promoted environment-only implementation
> seams into persistent configuration; their tables below reflect the narrower
> landed allowlist. The body is left as it was written; see
> `docs/DECISIONS.md`, "The contract rides the system prompt where a collar has
> one" and "Persistent config exposes operator choices, not implementation
> seams".

Four coupled changes: a user configuration file behind the `GANG_*` environment
surface, an operator doctrine slot appended to the startup contract, a
hitch-discipline line in that contract, and deferred self-compaction for the
claude-code profile.

Every claim about current behaviour below was read out of the tree at
`da6541a` and is cited by owning symbol with the line quoted. Claims about
external tools and about the delivery path were measured, and the measurements
are dated where they stand as evidence.

## 1. User configuration file

### 1.1 The question the operator asked

*"There must be a lib for this right?"* — surveyed below, and the honest answer
is that the library saves less than it costs here.

**(a) `git config -f`, reusing git's INI parser.** Measured against
`git version 2.34.1`:

| probe | result |
|---|---|
| `--list` outside any repository | works, prints `section.key=value` |
| key case | folded to lower case on output; values keep their case |
| key `boot_timeout` | `fatal: bad config line 2` — **underscore is not a legal key character** |
| duplicated key, `--get` | returns the **last** value, exit 0, no warning |
| value `/tmp/a b # c` | returns `/tmp/a b` — **`#` silently truncates** an unquoted value |
| valueless key | `--list` prints the key with no `=`; `--get` prints empty, exit 0 |
| missing file | `fatal: unable to read config file`, exit 128 |
| malformed file | `fatal: bad config line 1 in file bad.ini`, exit 128 |
| `[include] path = …` under `-f` | not expanded; only `include.path` was listed |
| user's global gitconfig | does not leak into a `-f` read |

Three findings decide it:

1. **`git` is not a runtime dependency of `bin/gang` today.** `grep -n '\bgit\b'
   bin/gang` returns nothing; the only git in the tree is in `install.sh`.
   Adopting git's parser makes every `gang status`, every `gang send`, and every
   `gang hook` — which runs on every `UserPromptSubmit` and `PostToolUse` of
   every agent — depend on a binary Gangline does not otherwise need. `bin/gang`
   does hard-require `python3`, but at the point of use inside `cmd_hitch`. A
   config read has no point of use; it is on every path.
2. **Key names cannot mirror the variable names.** `_` is refused, so
   `GANG_BOOT_TIMEOUT` becomes `gang.boot-timeout` and the implementation carries
   a hand-maintained mapping table — the code the free parser was meant to save.
3. **The parse still has to be policed.** Silent last-wins on a duplicate and
   silent truncation at an unquoted `#` are the silent fallbacks law 8 forbids,
   so the implementation must enumerate `--list`, count duplicates, and reject
   unknown keys itself — the same allowlist loop option (b) needs. git hands back
   a parse that must then be audited, not a decision.

The `[include]` question is closed independently of git's version: an unknown key
is fatal under §1.5, and `include.path` is an unknown key.

**(b) A strictly parsed `KEY=VALUE` file — parse, never source.** No new
dependency. The file uses the **exact variable names** the operator already
types, so `GANG_BOOT_TIMEOUT=45 gang hitch …` and the config line
`GANG_BOOT_TIMEOUT=45` are the same string; nothing to learn, nothing to map.
The parser is small because the format is deliberately trivial — no sections, no
types, no arrays, no includes, no escapes.

**(c) A parser dependency (`yq`, a TOML reader).** A new runtime binary on every
invocation, absent from base systems, for a dozen flat scalar keys. Fails law 5
and the no-new-runtime-deps rule. Rejected.

**Recommendation: (b).** Sourcing is not among the options: a sourced file
executes arbitrary code on every `gang` invocation, including `gang hook` inside
every agent's turn, from a file an operator may have synced from another machine.

### 1.2 Location

```
GANG_CONFIG_DIR   default ${XDG_CONFIG_HOME:-$HOME/.config}/gangline
  config          the settings file (§1.5)
  DOCTRINE.md     the operator doctrine slot (§2)
```

`GANG_CONFIG_DIR` is an environment variable only — it names the file the config
layer is read from, so it cannot be set inside that file. It exists for two
reasons that `XDG_CONFIG_HOME` cannot serve:

- **Pinning the config root into hitched agents (§1.7).** Setting
  `XDG_CONFIG_HOME` in a launch line would reconfigure the *harness* for any
  harness that honours XDG, which is not Gangline's business.
- **Suite isolation (§5.1)** without perturbing fixture harnesses the same way.

**The resolved root must be an absolute path.** Whichever of the three sources
supplies it — `GANG_CONFIG_DIR`, `XDG_CONFIG_HOME`, or `$HOME` — a root that does
not begin with `/` is fatal, in one check on the resolved value:

```sh
case "$CONFIG_DIR" in /*) ;; *) die "… must be an absolute path, got '…'" ;; esac
```

A relative root does not name one directory; it names a different directory per
working directory, and `cmd_hitch` launches its child with `-d` in a working
directory that is frequently not the caller's. Pinning a relative string into the
launch line would therefore hand the child a *different* config file and doctrine
— the exact failure §1.7 exists to prevent. Refused rather than normalised,
because normalising is only meaningful once the directory exists, and this
directory legitimately may not: resolving `.` and `..` by hand for a path that
cannot be `cd`'d into is code for a case that should not exist.

Absent config file means no config layer, silently — absence is the default
state, not a failure. An absent *directory* is the same thing; only a malformed
root is fatal.

### 1.3 Precedence

Environment variable > config file > built-in default. Environment stays
authoritative so CI, one-off invocations, and the `env` prefix Gangline itself
writes into launch lines keep working unchanged.

"Set in the environment" means **set**, not non-empty: `GANG_PROFILES=` from the
environment is an operator deliberately clearing it, and outranks the file.

### 1.4 Settable keys

Exactly these, with the defaults they have today:

| key | default | where the default lives now |
|---|---|---|
| `GANG_PROFILE` | `claude-code` | `cmd_hitch`: `prof="${GANG_PROFILE:-claude-code}"` |
| `GANG_SESSION` | `gangline` | `SESSION="${GANG_SESSION:-$DEFAULT_SESSION}"` |
| `GANG_PROFILES` | unset | `profile_file`, `all_profiles` |
| `GANG_LOCK_DIR` | `/tmp/gangline-$(id -u)` | `lock_pane`, `spool_root_of` |
| `GANG_CONTEXT_LIGHTS` | `off` | `GANG_CONTEXT_LIGHTS="${GANG_CONTEXT_LIGHTS:-off}"` |
| `GANG_BOOT_TIMEOUT` | `30` | three `boot="${GANG_BOOT_TIMEOUT:-30}"` sites |
| `GANG_CHURN_WAIT` | `0.5` | top-of-file default block |
| `GANG_ACTIVITY_WINDOW` | `5` | top-of-file default block |
| `GANG_TURN_LIMIT` | `300` | top-of-file default block |

**Refused with their own message, not as unknown keys:**

- **Profile-contract declarations** — `GANG_LAUNCH`, `GANG_RESUME_LAUNCH`,
  `GANG_MODEL_OPT`, `GANG_EFFORT_OPT`, `GANG_EFFORT_CMD`, `GANG_BUSY_REGEX`,
  `GANG_OCCUPIED_REGEX`, `GANG_QUEUED_REGEX`, `GANG_QUEUE_RECALL_KEY`,
  `GANG_INTERRUPT_KEY`, `GANG_STOP_HOOK`, `GANG_QUIET_AT_REST`,
  `GANG_MIDTURN_INPUT`, `GANG_COMPACT_CMD`, `GANG_SELF_COMPACT`,
  `GANG_SESSION_KEY`. `load_profile` clears each to `""` before sourcing the
  profile, so a config value would be discarded without a trace. The refusal
  names where the value belongs: *"…is a profile declaration, not operator
  configuration — `load_profile` clears it before sourcing the profile, so a
  value here would be discarded. Put it in a custom profile and point
  `GANG_PROFILES` at its directory."*
- **`GANG_TEST_PROFILES`** — a per-invocation switch for Gangline's own suite.
  Persisting it would make the `bash` stand-in permanently hitchable, which
  `cmd_hitch` refuses on purpose.
- **`GANG_CONFIG_DIR`** — names the file being read; setting it inside that file
  states nothing. Refused as bootstrap.
- **`GANG_MODEL`, `GANG_PROFILE_FILE`** — internal. `GANG_MODEL` is set for the
  lifetime of one `sh -c` in `effort_levels`; `GANG_PROFILE_FILE` is stamped by
  `load_profile`.

Anything else is an unknown key.

### 1.5 File grammar and failure semantics

**Byte guard, before any line parsing.** The file must be free of NUL bytes and
of control characters other than tab and newline. Both checks are needed and
neither is optional:

- `while IFS= read -r` **silently drops NUL**: measured 2026-08-07, a file
  containing `GANG_SESSION=alpha\0omega` yields the value `alphaomega` with no
  refusal. Detect by comparing `wc -c < "$file"` against
  `LC_ALL=C tr -d '\000' < "$file" | wc -c`.
- **CR survives trailing-blank stripping**: measured 2026-08-07, a CRLF line
  `GANG_BOOT_TIMEOUT=30\r\n` yields a three-byte value `30\r`, because `[:blank:]`
  is space and tab only. The invisible byte then reaches validation and use.

Use the idiom `bin/gang` already applies to `$ROOT` in
`profiles/claude-code.sh` (`case "$ROOT" in *[[:cntrl:]]*`), over the whole file
with tab and newline deleted first:

```sh
probe="$(printf '%s' "$raw" | tr -d '\11\12')"
case "$probe" in *[[:cntrl:]]*) die "… contains control characters …" ;; esac
```

Verified 2026-08-07 to refuse the CRLF case above and to accept UTF-8 prose under
the UTF-8 locale `utf8_locale` establishes at startup.

**Encoding is not validated on this path, deliberately.** Validating UTF-8 needs
`python3`, which `bin/gang` requires only inside `cmd_hitch`; requiring it for
`gang status` would be a new hard dependency on every command. Config values are
short scalars consumed by tmux and the filesystem, not prose rendered into a
terminal — a malformed byte in a path surfaces as a filesystem error naming the
path. Doctrine, which *is* prose pasted into a TUI, is validated (§2.3).

**Line grammar.** Never sourced, never `eval`'d as text, no `$` expansion, no
quote removal, no escape processing.

- A line that is empty or all blanks is skipped.
- A line whose first non-blank character is `#` is a comment. **`#` starts a
  comment only at the start of a line**; inside a value it is literal.
- Otherwise the line must be `NAME=VALUE`:
  - leading blanks on the line are stripped;
  - `NAME` is everything up to the **first** `=` and must contain no blanks —
    `GANG_SESSION = x` is refused, because this file's syntax is the shell
    assignment syntax the operator already uses and a second dialect is how a
    format grows;
  - `VALUE` is everything after the first `=`, taken literally, with trailing
    blanks stripped. A value may itself contain `=`. No Gangline value has a
    meaningful trailing space, and an invisible one is the misconfiguration
    fail-loud cannot catch: it would surface as *"unknown profile 'codex '"*,
    blaming the wrong thing.
- An **empty value** is refused: *"a key with no value states nothing — delete
  the line to take the default."*
- A **duplicated key** is refused, naming both line numbers. Last-wins is a
  silent fallback.
- An **unknown key** is refused, naming the file, the line number, the key, and
  the full settable set.
- A file that exists but is not a regular file, or cannot be read, is fatal.
- A missing final newline is not an error:
  `while IFS= read -r line || [ -n "$line" ]`.

**Unknown keys are fatal, everywhere, including `gang hook`.** The blast radius
is real and is documented rather than hidden: see §1.10.

### 1.6 Implementation

`config_load` is called after `fail`/`die` are defined and before the first use
of any settable variable. `DEFAULT_SESSION` / `SESSION=` and the top-of-file
default block both move below that call. No read site changes: every consumer
already reads through `${X:-default}` or through the default block.

```sh
GANG_CONFIG_KEYS='GANG_PROFILE GANG_SESSION GANG_PROFILES GANG_LOCK_DIR
GANG_CONTEXT_LIGHTS GANG_BOOT_TIMEOUT GANG_CHURN_WAIT GANG_ACTIVITY_WINDOW
GANG_TURN_LIMIT'
```

`CONFIG_ORIGINS` accumulates one tab-separated record per key, newline-joined:
`NAME<TAB>env`, `NAME<TAB>env<TAB><lineno>` (environment set and the file also
named it), or `NAME<TAB>config<TAB><lineno>`. A key with no record came from its
built-in default. bash-3.2 has no associative arrays; this string plus a
`config_origin NAME` reader is the substitute, for a dozen entries.

Two idioms are load-bearing, and both avoid indirect expansion so they hold on
bash 3.2 (macOS `/bin/bash`) without relying on `${!name+word}`:

```sh
eval "envset=\${$k+set}"    # was this key set in the environment at all
eval "$k=\$value"           # assign; the VALUE is a quoted parameter reference,
                            # never interpolated text, so no content can inject
```

`$k` comes only from `GANG_CONFIG_KEYS`, a literal in this script. **The
assignment must never be written `eval "$k='$value'"`** — that form is injectable
by a value containing a quote, and it is the line a reviewer should check first.

Values are assigned as ordinary shell variables and **not exported**. Exporting
would change the environment of every command `gang` runs — the harness launch
line, the profile's `GANG_EFFORT_CMD` producer — giving the config layer reach
the environment layer only has when an operator asked for it, and making
"environment beats config" untestable from inside a child process.

### 1.7 What a hitched agent inherits

§3 tells every agent to hitch teammates, so every agent is a live caller of
`cmd_hitch`, which consumes `GANG_PROFILE`, `GANG_CONTEXT_LIGHTS`, and
`GANG_BOOT_TIMEOUT`. `tmux new-window` runs its command in the tmux **server's**
environment, so a child's `HOME` and `XDG_CONFIG_HOME` are not guaranteed to be
the caller's, and a child left to resolve for itself could read a different
config file — or none.

**The config root is pinned; the environment layer is not inherited.**
`cmd_hitch` adds one entry to the launch line it already builds:

```sh
launch_env="GANG_SESSION=$(shell_quote "$SESSION")"
launch_env="$launch_env GANG_CONFIG_DIR=$(shell_quote "$CONFIG_DIR")"
[ -z "${GANG_PROFILES:-}" ] || launch_env="$launch_env GANG_PROFILES=…"
[ -z "${GANG_LOCK_DIR:-}" ] || launch_env="$launch_env GANG_LOCK_DIR=…"
```

`CONFIG_DIR` is the **absolute** resolved directory of §1.2, whether it came from
`GANG_CONFIG_DIR` or from the XDG default. Absoluteness is what makes the pin a
pin: the child runs in the `-d` working directory, not the caller's, so a
relative root would name a different file on the other side of the launch. With
it, the child reads the same file as its parent and resolves the same config
layer — so a lead whose `GANG_PROFILE=codex` came from the file hitches teammates
on codex, and configured lights and boot bounds reach nested hitches.

The three team-identity keys stay pinned as today, because `docs/reference.md`
requires that *"every process addressing one team must agree on `GANG_SESSION`,
`GANG_PROFILES`, and `GANG_LOCK_DIR`"*.

**Environment-layer values other than those are per-invocation and do not
propagate.** `GANG_BOOT_TIMEOUT=90 gang up lead` gives that one hitch a 90-second
bound; the lead's own hitches use the configured or default bound. This is
today's behaviour, and it is the right one: propagating the environment layer
wholesale would make a one-off override sticky for an agent's entire life. It is
documented rather than changed, and §5.2 tests the config layer's coherence
across a nested hitch.

### 1.8 Blaming the right layer

With three layers, *"GANG_BOOT_TIMEOUT must be a whole number of seconds, got
'abc'"* no longer says where to go and fix it. A helper appends the origin:

```sh
config_origin_note GANG_BOOT_TIMEOUT
#  -> " (from ~/.config/gangline/config line 4)"
#  -> " (from the environment)"
#  -> ""   when the value is the built-in default
```

Applied at exactly these sites and no others:

- `require_whole` and `require_interval` — both already take *"the name to
  blame"* as `$1`, so the note is one append in each and covers every call site.
- The two `context_lights_parse` refusals.
- `cmd_hitch`'s unknown-profile path, **only when `-p` was not given** — with
  `-p` the value came from the command line and the note would lie.
  `load_profile` is also called with adopted-profile names and roster rows, so
  the note belongs in `cmd_hitch` beside `prof="${GANG_PROFILE:-…}"`.

Paths print with `$HOME` collapsed to `~`.

### 1.9 `gang config`

Three layers with no introspection means an operator seeing Gangline address the
wrong session cannot tell whether it came from the environment, the file, or a
default — and the commonest failure of a new config file is *"my config file
isn't working"*, whose answer is almost always an environment variable outranking
it. It is also the only place the doctrine slot's state is visible before a hitch
delivers it.

`gang config` takes no arguments and needs no tmux server:

```
GANG_PROFILE=codex                      environment (overriding config line 2)
GANG_SESSION=arc-cfg                    ~/.config/gangline/config line 3
GANG_PROFILES=                          default
…
doctrine  ~/.config/gangline/DOCTRINE.md  present
```

**Values are sanitised on output.** The config layer cannot carry control bytes
(§1.5), but the environment layer can: `GANG_SESSION=$'\e]0;x\a'` would otherwise
let a value drive the operator's terminal from a report they ran to *inspect*
that value. Every C0 control, DEL, and CR is replaced with a visible `?` before
printing; tab prints as a space. Sanitise regardless of layer — the guarantee
belongs to the command, not to one of its inputs.

Wire it into `usage()` and the dispatcher (`config) cmd_config "$@" ;;`), and
replace the `Environment:` block in `usage()` with a pointer to `gang config`
and `docs/reference.md` — that block already omits half the variables.

### 1.10 Blast radius of a fatal parse, and its recovery

`config_load` runs before dispatch, so a malformed config refuses **every**
command, `gang hook` included. Every hook-enabled window then loses turn
brackets, Stop-driven spool drain, deferred compaction, occupancy raising, and
lights until the file is repaired, and the harness surfaces a hook error on every
turn.

That is the chosen direction, and the alternative is worse: an operator who typed
`GANG_BOOT_TIMOUT=600` and got a warning believes a bound is set that is not, and
finds out when a hitch times out at 30 seconds with nothing on screen connecting
the two. A hook quietly running on defaults while `gang status` refused would be
exactly the split-brain law 8 exists to forbid. The precedent in `bin/gang` is
unambiguous: an unknown `GANG_SELF_COMPACT` value dies, an unknown
`GANG_STOP_HOOK` value dies, a key name outside the tmux key alphabet dies, an
unknown `hitch` argument dies.

What the blast radius buys is an obligation to make recovery trivial and
documented:

- Every refusal names the file, the line number, the offending key, and the
  settable set. The fix is one edit.
- The refusal reaches stderr and a non-zero exit from `gang hook` too, so it is
  visible in the harness's hook error rather than swallowed (§5.2.14).
- `docs/operations.md` gains a recovery entry: the symptom (every agent's hooks
  failing at once, every `gang` command refusing), the cause, and the two
  commands that resolve it — read the error, or move the file aside and run
  `gang config` to confirm the layer is gone.

## 2. Operator doctrine injection

### 2.1 The slot

`$GANG_CONFIG_DIR/DOCTRINE.md`, default `~/.config/gangline/DOCTRINE.md`.

Core ships no doctrine and no example doctrine. Nothing in the repository creates
this file. Its deletion path (law 6) is that the operator deletes the file;
Gangline never writes it, and a hitched agent's copy dies with the window.

### 2.2 Who gets it: every hitch, because Gangline cannot see an operator

Doctrine binds delegation behaviour, and the operator's intent is that it bind
the lead. **Gangline cannot observe that distinction, and will not infer it.**

`bin/gang` has no concept of a lead: `cmd_up` merely defaults the name to `lead`,
and `hitch lead` is a name like any other. The nearest available signal is
`self_window`, which returns non-zero unless `$TMUX_PANE` is set *and* that pane
is listed in `$SESSION` — that is membership in the **target session**, not
authorship, and it is wrong in both directions:

- an agent in team A running `GANG_SESSION=B gang hitch …` is absent from B and
  would be read as the operator, so B receives host doctrine;
- an agent invoking through any shell without `$TMUX_PANE` would be read as the
  operator likewise;
- an operator typing `gang up` from a shell pane inside the target session would
  be read as an agent, and the lead would be starved of the doctrine written for
  it.

So the rule is the one the state can prove:

> **Every hitch's startup contract carries the doctrine.** `gang adopt` carries
> none, because *"adoption does not inject startup text"* at all.

The operator scopes doctrine in the doctrine's own text — the existing file
already opens *"These rules bind every hitch you make"*, which is true for
whoever reads it. Prose in an agent's prompt is what law 9 names as the answer
when the alternative is machinery.

**Rejected: a `--doctrine` / `--no-doctrine` flag.** It would be an honest
assertion rather than a false inference, but it adds public surface for a
distinction the operator can state in one sentence of their own file, and law 5
finds no consumer for it on the day it merges. If teammate doctrine later proves
costly in practice, that flag is the change to make, and this is the paragraph it
supersedes.

### 2.3 Guards, all before the window opens

`cmd_hitch` requires `python3` today *after* launching the window
(`command -v python3 … || die "python3 is required for native hook payloads and
context lights"`). Move that check above the launch: the doctrine guard needs it,
and failing before opening a window is better than failing after regardless.

1. **Not a regular file, or unreadable** → fatal, naming the path. Absence is
   silent; presence Gangline cannot read is not.
2. **NUL bytes** → fatal. Detected by comparing `wc -c < "$file"` against
   `LC_ALL=C tr -d '\000' < "$file" | wc -c`.
3. **Control characters other than tab and newline** → fatal, via the
   `*[[:cntrl:]]*` idiom of §1.5. Carriage returns are refused: a CRLF doctrine
   would paste stray CRs into a composer.
4. **Invalid UTF-8** → fatal:
   ```sh
   python3 -c 'import sys; sys.stdin.buffer.read().decode("utf-8")' < "$file"
   ```
   Verified 2026-08-07: accepts prose containing an em dash, exits 1 on a lone
   `0xff`. This check is required because the `[[:cntrl:]]` guard **does not**
   catch invalid UTF-8 — measured the same day, a doctrine containing a bare
   `0xff` passes both the NUL and control-character guards. Doctrine is prose
   pasted into a terminal UI; a malformed byte there is a rendering the profile's
   readers must then interpret.
5. **Size ceiling: 8192 bytes.** See §2.6 for what this guard does and does not
   claim.

### 2.4 Reading it: byte-exact, and no new escaping hazard

**`doctrine="$(cat "$file")"` is not verbatim.** Command substitution strips
every trailing newline: measured 2026-08-07, an 11-byte file ending in two blank
lines yields 8 bytes. Use the sentinel idiom `bin/gang` already applies in
`envelope` and `stdin_body`, which preserves all 11:

```sh
marked="$(cat "$file"; printf '\034')"
case "$marked" in
  *$'\034') doctrine="${marked%$'\034'}" ;;
  *) die "internal: the doctrine sentinel was lost while reading $file" ;;
esac
```

The delivery path itself is already binary-safe and already carries multi-line
bodies:

- **No shell interpolation anywhere.** `inject` writes the body with
  `printf '%s' "$2" | tmux load-buffer -b "$buf" -` and pastes with
  `tmux paste-buffer -p -d -b "$buf" -t "$1"`. Nothing is eval'd; nothing reaches
  a command line.
- **Tag-shaped text is already neutralised.** `envelope` rewrites bracketed
  `gang:` openers over the whole body before wrapping it, so a doctrine
  containing `[gang: …]` cannot forge an envelope. The doctrine must therefore be
  concatenated into the brief **before** `envelope` is called — assembling first
  is what puts it under that guard.
- **Multi-line is a proven path.** `test/integration.sh` sends a two-line body
  through the same `inject` and asserts the recorded composer contains the tail
  line (`"MARK_MULTI_TAIL"`). The startup contract has been single-line by
  accident of content, not by any property of the delivery.

### 2.5 The assembled contract

`startup_brief` gains the doctrine and the §3 line, and keeps `End this turn.`
**last** — it is the terminator, and anything after it reads as further
instruction:

```
You are <name> in Gangline. Peer messages name their sender in the gang
envelope. To message a peer: gang send --to NAME --stdin. At natural
checkpoints compact with: gang compact <name>. <hitch-discipline line, §3>
<marathon rule, §3>

Operator doctrine (~/.config/gangline/DOCTRINE.md):

<doctrine, byte-exact>

End this turn.
```

The origin line lets the agent tell operator doctrine from Gangline mechanism and
name its source. With no doctrine file the contract is today's text plus the §3
line, with no separator and no blank lines.

`startup_brief` takes the agent name today; it gains a second argument for the
doctrine body, empty when there is none, so the caller does the reading and the
string builder stays a string builder.

Existing assertions survive unedited: `contains … "You are alpha in Gangline"`
and `contains … "End this turn."` both still hold.

### 2.6 What the ceiling claims, and what bounds delivery instead

**The ceiling is a category-error guard. It is not a deliverability bound, and
must not be described as one.**

Delivery is bounded by what the target's composer renders and by pane geometry,
which Gangline cannot compute. Measured 2026-08-07 against the Bash fixture on an
80×24 pane, through `gang send` on a private socket:

| body | result |
|---|---|
| 600 bytes over 10 short lines | delivered |
| 2048 bytes on one wrapped line | failed: *"cannot read the input box … after pasting"* |
| 2000 bytes over 20 lines of 99 chars | failed |
| 8192 bytes over 10 long lines | failed |
| 32768 bytes, any shape | failed |

Byte count does not predict it and line count does not predict it: what predicts
it is whether the paste's **rendered rows** push the fixture's `❯` prompt off the
pane, so `profile_input` can no longer find the composer to read back. That bound
belongs to the profile's rendering and the pane's height. A real TUI composer is
a framed box that scrolls internally, so its bound is its own and is not this
one; Gangline knows neither.

Therefore:

- A doctrine the target cannot accept fails **at delivery**, loudly, through the
  verified path that already exists, and `cmd_hitch` already prints the recovery
  (*"Inspect it with 'gang attach' … run 'gang drop $name' and hitch again"*).
  That refusal is the safety property, not the ceiling.
- The ceiling's only job is the category error — the slot pointed at a document
  tree, a log, or a binary. 8192 bytes is high enough that prose written to bind
  an agent's behaviour is never refused by it, and low enough that a mis-pointed
  slot is caught at once. Wrong in the small direction it refuses a real doctrine
  before any window opens, costing one edit; wrong in the large direction it
  merely defers to the delivery refusal above, which is loud and already
  documented.
- **A consequence to document, not to hide:** the assembled contract is now
  potentially far larger than today's one-liner, so a hitch into a short pane can
  fail where it used to succeed. `docs/operations.md` gets this alongside the
  §1.10 entry, and the recovery is the existing one.

## 3. Hitch discipline in the startup contract

One line, in every hitch's contract:

> When you hitch a teammate, choose its model and reasoning effort deliberately
> and pass them as `-m` and `-e`; where the profile declares no such option gang
> refuses the flag, so that choice is the profile's rather than a default's.
> Never let an unexamined default stand in for a decision.

It enforces that a choice is made, never which choice — no model names, no effort
levels, no tiering policy. That belongs in DOCTRINE.md.

Operator-authorized post-approval scope delta: every lead/adopt startup contract
also carries this sentence alongside the hitch-discipline line:

> Marathon rule: never halt the session to wait on the operator — resolve forks
> by doctrine and report decisions past-tense; when a fork genuinely needs the
> operator (irreversible, outside doctrine), state it in your report, park that
> one lane, and keep every other lane moving.

This bans the turn-blocking mechanism in an operator-absent marathon while
preserving a reporting path for the narrow class of forks doctrine cannot settle.

This belongs in core because it is the prose form of a refusal core already
implements. `cmd_hitch` refuses an effort it cannot validate, and says why —
*"An unchecked effort is not refused by the harness either — it runs at a level
nobody chose — so it is refused here"* — with the reasoning recorded above it:
*"the operator gets a live agent at an effort nobody chose, or one that dies as
soon as it is briefed."* Gangline already spends code to stop an unchosen effort
reaching a window. Telling the agent that does the hitching to choose is the same
decision, one layer up, for free.

## 4. Deferred self-compaction for claude-code

### 4.1 The mechanism, read out of the source

`cmd_compact` dispatches on two facts — whether the caller is the target, and
whether the profile defers:

```sh
if [ "$mine" -eq 1 ] && [ "$GANG_SELF_COMPACT" = deferred ]; then
  self_compact_request "$AGENT_ID" >/dev/null
  echo "self-compaction scheduled for the end of this turn"
  return 0
fi
if [ "$mine" -ne 1 ]; then
  busy "$AGENT_ID" || busy_rc=$?
  …
fi
inject "$AGENT_ID" "$GANG_COMPACT_CMD" head
```

`profiles/codex.sh` declares `GANG_SELF_COMPACT=deferred`;
`profiles/claude-code.sh` declares nothing, and `load_profile` clears the variable
before sourcing. So on claude-code an agent compacting itself takes `mine=1` with
no deferral: the first branch is skipped, **the busy check is skipped too** — it
is guarded by `[ "$mine" -ne 1 ]` — and `inject` types `/compact` into the
agent's own composer while its own turn is running, which is the only time an
agent can call this.

claude-code parks input arriving mid-turn; the profile records the rendering
(*"renders a queued body in the transcript styled exactly like a submitted prompt
and empties the composer"*). `inject`'s post-Enter verification catches the park
and fails loudly, but the `/compact` is now in the harness's queue, and `inject`'s
**preflight** then refuses every subsequent delivery to that window
(*"nothing gang sends can enter the session until the queue is recovered"*). That
is the trap: a lead that self-compacts becomes unreachable, and the recorded
recovery on 2026-08-07 was a drop and re-hitch.

### 4.2 The fix: both declarations move inside the branch that installs the hook

`profiles/claude-code.sh` composes its native hooks into `--settings` only inside

```sh
if [ -n "${ROOT:-}" ] && [ -x "$ROOT/bin/gang" ]; then
  case "$ROOT" in
    *[\'\"\\]*|*[[:cntrl:]]*) ;;      # no hooks on this path
    *) … "Stop":[{"hooks":[…gang hook]}] … ;;
```

but declares `GANG_STOP_HOOK=1` unconditionally, outside it. On the unhooked path
the declaration is already a claim the launch line does not honour, which today
misleads `--spool`. Adding `GANG_SELF_COMPACT=deferred` unconditionally would turn
that into a worse failure: `cmd_compact` would record the request and type
nothing, no Stop event could ever dispatch it, and `self_compact_request` refuses
a second request while one is outstanding (*"self-compaction is already waiting
for this turn to end"*) — so the agent could never compact again, silently.
codex is not equivalent here: its hook flags are composed unconditionally.

**Move both `GANG_STOP_HOOK=1` and `GANG_SELF_COMPACT=deferred` inside the `*)`
arm that actually composes the hook JSON.** `load_profile` clears both before
sourcing, so on the unhooked path the profile then truthfully declares neither:
`--spool` refuses (*"a profile that declares no `GANG_STOP_HOOK` refuses
`--spool`"*), and `gang compact <self>` falls through to the non-deferred path —
the pre-existing parked-queue failure, loud, on a path where Gangline cannot
install hooks at all.

This widens the change beyond the one line originally proposed, and it is not
optional: the deferred declaration cannot be made truthful without it.

Rejected: refusing to launch at all on the unhooked path. It would break bare
`claude` hitches that work today for send, status, and capture, which is a
behaviour change no finding asks for.

With hooks installed, every precondition holds: `GANG_COMPACT_CMD="/compact"`;
the Stop hook is composed independently of `GANG_CONTEXT_LIGHTS`, which only
appends `statusLine`; `cmd_hook` routes `Stop) … self_compact_dispatch`;
`load_profile` accepts `''|deferred`. `status` and `roster` already report pending
and failed self-compaction, and ordering against the spool drain is unchanged from
codex — `self_compact_dispatch` is backgrounded before `spool_stop`, and both
serialise on the pane lock inside `inject`.

## 5. Test plan

All behavioural. No test asserts that a file exists or greps a config value;
each asserts the artifact the behaviour produced — the value a command resolved,
the text on a pane, the option a command stamped, the message a refusal printed.
Two declaration checks are called out as such where behaviour cannot reach.

### 5.1 Suite isolation — mandatory, and a prerequisite for everything below

`test/integration.sh` must export a private config root beside its existing
`GANG_SESSION` / `GANG_LOCK_DIR` exports:

```sh
export GANG_CONFIG_DIR="$RUN_ROOT/config"
```

Without it every fixture hitch reads the developer's real config and injects
their real `DOCTRINE.md`, and `.githooks/pre-push` — which runs the suite from a
`git archive` tree but not in a clean environment — inherits the same leak. This
is the first change the implementer should make. `GANG_CONFIG_DIR` rather than
`XDG_CONFIG_HOME` so fixture harnesses are not perturbed too.

### 5.2 Configuration file

1. **The file supplies a value.** With `env -u GANG_SESSION` and a config naming
   a session that exists on the private tmux server, `gang roster` lists that
   session's window.
2. **Environment outranks the file.** Environment and file name different
   existing sessions; `gang roster` lists the environment one's window.
3. **The file is never executed.** The config contains
   `GANG_SESSION=$(touch "$RUN_ROOT/executed")`. Assert the marker does not exist
   **and** that `gang config` reports that literal text as the value. The positive
   half keeps the negative half from passing on a fixture that did nothing.
4. **An unknown key refuses**, naming file, line number, and key.
5. **A profile declaration refuses with its own message**, pointing at
   `GANG_PROFILES` — asserted by content, distinct from the unknown-key message.
6. **`GANG_TEST_PROFILES` refuses with its own message.**
7. **`GANG_CONFIG_DIR` inside the config refuses as bootstrap.**
8. **A duplicated key refuses**, naming both line numbers.
9. **An empty value refuses.**
10. **A NUL byte refuses**, and the refusal is not the value silently losing
    bytes — assert the message, not just the exit status.
11. **A CRLF line refuses**, and **a bare ESC in a value refuses**.
12. **A value keeps `#`, spaces, quotes, backticks, and an embedded `=`
    literally.** `GANG_LOCK_DIR` set to a path containing a space and a `#`;
    assert `gang send` creates its lock under exactly that directory. A separate
    case asserts a value containing `` ` `` and `$(…)` reaches `gang config`
    unexecuted and uninterpreted.
13. **A trailing blank is stripped**, and **a missing final newline still
    parses** — the last line of a newline-free file supplies its value.
14. **A broken config makes `gang hook` fail visibly.** Feed valid event JSON on
    stdin with a malformed config; assert non-zero exit and that the message
    naming the file reaches stderr. This is the §1.10 obligation: the outage must
    be observable, not swallowed.
15. **A bad value blames the file.** `GANG_BOOT_TIMEOUT=abc` in the config; the
    refusal names the config file and its line. Then the same value in the
    environment; the refusal says the environment. Two assertions, because a note
    that is always the same string is not attribution.
16. **`gang config` attributes each layer**: one key from the environment marked
    as overriding the file's line, one from the file, one from neither marked
    `default`.
17. **`gang config` sanitises a control byte** in an environment-layer value: the
    raw ESC does not reach stdout.
18. **A nested hitch resolves the same config layer, from a different working
    directory.** Hitch the `bash` fixture with `GANG_PROFILE` and `GANG_SESSION`
    supplied by the file only and with `-d` pointing somewhere other than the
    suite's own working directory; then from that agent's pane hitch a second
    one, again with a different `-d`, and assert the second window's profile is
    the file's profile and it joined the file's session. This is the §1.7
    coherence claim. **The differing `-d` is load-bearing:** with parent and child
    sharing a working directory the assertion passes even when the pinned root is
    relative, which is precisely the hole this guard exists to cover.
19. **A relative config root refuses**, in all three sources: `GANG_CONFIG_DIR`
    set to a relative path; `XDG_CONFIG_HOME` relative with `GANG_CONFIG_DIR`
    unset; `HOME` relative with both unset. Each names the offending variable.

### 5.3 Doctrine

1. **Present and injected.** A doctrine fixture with a marker sentence; the pane
   carries both `You are <name> in Gangline` and the marker. Both halves asserted,
   so a pane that rendered nothing cannot pass.
2. **Absent.** The pane carries the base contract and not the `Operator doctrine`
   origin line.
3. **A hitch made from inside the team carries it too.** Run `gang hitch` with
   `TMUX_PANE` set to an existing agent's pane — the idiom the suite already uses
   for in-team calls — and assert the marker is present. This is the §2.2 rule,
   and it is the assertion that would go red if a membership predicate were
   reintroduced.
4. **A cross-session hitch carries it too.** `GANG_SESSION` pointing at a second
   disposable session, invoked from a pane in the first; assert the marker. The
   counterexample that killed the membership predicate, kept as a guard.
5. **Multi-line doctrine lands whole**, with distinct markers on the first and
   last lines and **both** asserted.
6. **Trailing newlines are preserved, counted.** Delivery plus an unchanged final
   terminator does **not** prove this: a `doctrine="$(cat "$file")"`
   implementation still delivers and still ends with `End this turn.`, so an
   assertion on those two facts is green on the very defect it was written to
   catch. The guard must count.

   Give the doctrine a `TAIL_MARK` final line followed by two blank lines, and
   assert the **number of blank rows between `TAIL_MARK` and the terminator** in
   the body Gangline recorded. Measured 2026-08-07 against the assembly of §2.5:
   the sentinel read yields 4, command substitution yields 1.

   Read that count off `@gl_parked` rather than the pane, by hitching a fixture
   profile that parks its input — the recording the suite already trusts for
   exactly this question, where it asserts that a parked multi-line body records
   *"every line of it … not just the first"*. The hitch fails, which is expected
   and asserted; the window option is stamped regardless.

   **This test is not accepted until it has been watched going red against a
   `$(cat)` implementation** — the repo's own standard, and the reason the
   original version of this assertion was worthless.
7. **Tag-shaped doctrine is neutralised**: the pane shows `(gang:` and not a
   second `[gang:` opener.
8. **Invalid UTF-8 refuses** — a lone `0xff` — **before the window opens**.
9. **A NUL byte and a bare CR each refuse**, before the window opens.
10. **Over-ceiling doctrine refuses before the window opens.** Non-zero exit, the
    message names the file and the ceiling, and **no window with that name
    exists** — the artifact separating "refused" from "refused after launching".
11. **Unreadable doctrine refuses; absent doctrine does not.**
12. **A doctrine too large for the pane fails loudly at delivery and leaves the
    window for inspection.** Under the ceiling but beyond what an 80×24 fixture
    pane can render: assert non-zero exit, a message naming the undelivered
    contract, and that the window still exists. This is §2.6's real safety
    property, and it is the guard the ceiling does not provide.
13. **`gang config` reports the doctrine present and absent.**

No test asserts that a ceiling-sized doctrine delivers. §2.6 measured why that
would be a claim about pane geometry rather than about Gangline, and a suite
assertion whose truth depends on render timing is the wall-clock evidence
`CONTRIBUTING.md` forbids.

### 5.4 Hitch discipline

The hitched pane's contract contains the hitch-discipline line and the exact
operator-authorized marathon-rule sentence, and `End this turn.` is still
present — the existing assertion at that site is not edited.

### 5.5 Deferred self-compaction

The claude-code declaration cannot be proven end to end in the mandatory suite:
that needs a real `claude` turn, and `CONTRIBUTING.md` puts real harness turns in
operator smoke tests. Stating that plainly rather than dressing a source read as
behaviour:

1. **Mechanism guard (mandatory, behavioural).** Extend the existing deferred
   self-compaction fixture. Today it proves the deferred path dispatches at Stop.
   Add the half the defect lives in: with the fixture's pane **painting a busy
   marker**, `gang compact <self>` leaves the composer empty and stamps
   `@gl_self_compact_requested`; on a fixture profile with no
   `GANG_SELF_COMPACT`, the same call types into the composer mid-turn. Two
   directions, so the guard is not green both ways.
2. **Both claude-code branches (mandatory, declaration).** The suite already
   sources `profiles/codex.sh` in a subshell and asserts `GANG_SELF_COMPACT`.
   Extend the same pattern to claude-code in both directions: sourced with a
   clean `ROOT`, both `GANG_STOP_HOOK` and `GANG_SELF_COMPACT` are declared;
   sourced with a `ROOT` containing a quote — the path-character guard's false
   branch — **both are empty**. That second case is the §4.2 defect, and without
   it the fix is unproven on the branch that motivated it.
3. **Operator smoke test (not mandatory).** In a separately named disposable
   session, hitch a real claude-code agent, have it run `gang compact <self>`
   mid-turn, and observe that its composer stays empty, that `gang status`
   reports the pending self-compaction, and that `/compact` is submitted after
   the turn ends. Never against the live `gangline` session or the development
   agent.

### 5.6 Lint

`test/lint.sh` needs no new rule: it globs `bin/gang`, `collars/*.sh`, and
`test/*.sh` already. New shell files carry an SPDX identifier.

## 6. Documentation at landing

- `docs/reference.md` — the `Environment` section becomes `Configuration`: the
  precedence rule, `GANG_CONFIG_DIR` and the file path, the grammar and byte
  rules, the settable set, what is refused and why, and `gang config` under
  *Discovery and hooks*. The `gang hitch` description gains the doctrine slot and
  what a hitched agent inherits (§1.7).
- `docs/operations.md` — two recovery entries: a malformed config refusing every
  command including hooks (§1.10), and a startup contract too large for the
  target pane failing at delivery (§2.6).
- `docs/DECISIONS.md` — three terse entries in the house voice:
  - *Configuration is parsed, never sourced* — the file mirrors the environment
    names, the environment stays authoritative, an unknown key is fatal.
  - *Doctrine is the operator's, and every hitch carries it* — core ships the
    slot and no content; Gangline does not infer an operator it cannot see.
  - *A hitch states its model and its effort* — the contract requires the choice,
    never the choice's content.
- `README.md` — unchanged. This adds no product claim.
- `CHANGELOG.md` — untouched; Release Please owns it.

Commits are Conventional and atomic — one logical change each: suite isolation,
the config loader, `gang config`, the doctrine slot, the discipline line, the
claude-code profile branch, docs.
