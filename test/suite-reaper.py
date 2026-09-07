#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Every tmux server a suite run starts, ended with that run.

A tmux server is a daemon: it reparents to init and outlives whatever forked
it. A suite that starts one and tears it down from a shell trap therefore has
no teardown at all on the paths where a trap does not run — SIGKILL, a pane
destroyed under the run, an OOM kill — and no teardown for any server whose
socket the trap does not name, because a trap can only address the sockets its
author listed. Both leaks leave a server holding a pty and its fixture shell
for as long as the host is up.

A RUN CLAIMS ITS OWN DIRECTORY IN WRITING. Nothing here selects a server
because of where its socket happens to sit: a run first writes a marker holding
an unguessable token into the directory it owns, and only a directory carrying
such a marker is ever swept. Within one, the servers reached are those whose
sockets are bound inside it, and the tmux binary is recognised so that another
daemon under the same directory is left alone. The team's own tmux directory,
and every other directory on the host, carries no marker and is therefore not a
candidate however it is spelled.

THE TOKEN IS ALSO THE GENERATION. A directory can be removed and remade at the
same path by a later run, so a watcher armed for one run carries the token it
wrote and compares it against the marker it finds. A later run wrote a
different token, so the earlier watcher declines rather than reaping a
successor that merely inherited the path.

A sweep is exact and synchronous, so the run's own teardown can call it. The
watch is the half a trap cannot provide: a detached process holds a pidfd for
the run and sweeps when the kernel reports that process gone, which no signal
sent to the run can prevent. That half rests on Linux pidfds and on
/proc/net/unix; where either is missing there is no watch to be had, and this
program says so and fails rather than reporting a protection it is not giving.
"""

import errno
import os
import secrets
import select
import shutil
import signal
import subprocess
import sys

MARKER = ".suite-reaper"
EXTRA = ".suite-reaper-extra"
SERVER_COMM = "tmux: server"
# The kernel's table of bound unix sockets. A fixture points this elsewhere to
# drive the refusal a host without it would get.
UNIX_SOCKETS = os.environ.get("SUITE_REAPER_UNIX_SOCKETS", "/proc/net/unix")
# A tmux server exits on SIGTERM once it has closed its panes. Past this it is
# not shutting down, and the run that owned it is already gone.
TERM_GRACE_SECONDS = 5.0
# The marker, the per-run socket list, and the kernel's own table are all
# line-oriented, so a newline in a path has no representation anywhere in this
# program and is refused where it enters.
FORBIDDEN_IN_PATH = "\n"

GONE = "gone"
UNSUPPORTED = "unsupported"


def bound_paths():
    """Map socket inode -> bound filesystem path for every unix socket.

    The path is the last field and may itself contain spaces, so the line is
    split a fixed number of times and the remainder taken whole. A path holding
    a newline would split one kernel record across two lines and cannot be read
    here at all, which is why such a path is refused before it is created.
    """
    paths = {}
    try:
        with open(UNIX_SOCKETS, encoding="utf-8", errors="replace") as stream:
            next(stream, None)
            for line in stream:
                fields = line.split(None, 7)
                if len(fields) >= 8:
                    paths[fields[6]] = fields[7].rstrip("\n")
    except OSError:
        return {}
    return paths


def listening_socket(pid, paths, wanted):
    """The path this process listens on, when `wanted` accepts it."""
    fd_dir = "/proc/%d/fd" % pid
    try:
        entries = os.listdir(fd_dir)
    except OSError:
        return ""
    for entry in entries:
        try:
            target = os.readlink(os.path.join(fd_dir, entry))
        except OSError:
            continue
        if not target.startswith("socket:[") or not target.endswith("]"):
            continue
        candidate = paths.get(target[len("socket:["):-1], "")
        if candidate and wanted(candidate):
            return candidate
    return ""


def servers(wanted):
    """(pid, socket) for every tmux server whose socket `wanted` accepts.

    The bound path is read from the kernel rather than from the filesystem, so
    a server whose socket file was already unlinked is still identified. What
    makes a process a candidate is the socket; reading its name as well only
    keeps a different daemon under the same directory out of the answer.
    """
    paths = bound_paths()
    found = []
    for entry in os.listdir("/proc"):
        if not entry.isdigit():
            continue
        pid = int(entry)
        try:
            # A NAME ON THIS HOST IS NOT THIS PROGRAM'S TO ENCODE. Every
            # process on the box is read here, and a process name is whatever
            # bytes its owner gave it. A strict decode makes one such
            # neighbour raise out of this walk; claim() reads it too, and the
            # suites exit on a claim that fails, so an unrelated process would
            # decide whether this run may start at all. The socket table and
            # /proc/PID/stat are already read the tolerant way, and the name
            # is only ever compared against a literal.
            with open(
                "/proc/%d/comm" % pid, encoding="utf-8", errors="replace"
            ) as stream:
                comm = stream.read().strip()
        except OSError:
            continue
        if comm != SERVER_COMM:
            continue
        socket_path = listening_socket(pid, paths, wanted)
        if socket_path:
            found.append((pid, socket_path))
    return found


def representable(path):
    """A path this program can read back out of the formats it depends on."""
    return bool(path) and not any(ch in path for ch in FORBIDDEN_IN_PATH)


def usable_root(root):
    """A path that could name a directory a run owns.

    Only the canonical spelling is accepted. `/tmp/.`, `/tmp//`, `/tmp/x/..`
    and a symlink all name the same directory as some other path, and a check
    that reads the spelling rather than the directory can be walked straight
    past by choosing another one. The kernel reports a bound socket under one
    spelling too, so a root with two of them is also a root whose own servers
    can go unfound. `mktemp -d` under a canonical TMPDIR produces the canonical
    form already, so what this refuses is a path nobody had to write.
    """
    if not representable(root) or not root.startswith("/"):
        return False
    if os.path.normpath(root) != root or root == "/":
        return False
    return os.path.realpath(root) == root


def marker_token(root):
    """The token the run that owns this directory wrote, or "" if none."""
    if not usable_root(root):
        return ""
    try:
        # A marker holding bytes no run wrote is not this run's marker, which
        # is an answer this function already has. Raising instead would end
        # the caller over a file it is reading precisely because it does not
        # trust what is in it.
        with open(
            os.path.join(root, MARKER), encoding="utf-8", errors="replace"
        ) as stream:
            return stream.read().strip()
    except OSError:
        return ""


def host_tmux_directory():
    """Where tmux puts a socket for this user when no run has redirected it.

    A fixture points this at a directory of its own so that the refusal below
    can be driven without touching the one the team's server is bound in.
    """
    return os.environ.get("SUITE_REAPER_HOST_TMUX", "/tmp/tmux-%d" % os.getuid())


def claimable(root):
    """Why this directory cannot be a run's own, or "".

    A claim is a promise that the directory holds nothing but this run's work,
    because a sweep ends what is bound inside it and then removes it. Two things
    say otherwise without any guesswork: a directory that already holds someone
    else's tmux server, and a directory that contains the one tmux puts its
    sockets in by default, which is where the team's own server lives.
    """
    host = host_tmux_directory()
    # The root is canonical by the time it gets here; the protected directory
    # need not be, so it is compared under both of its spellings.
    for against in (host, os.path.realpath(host)):
        if root == against or against.startswith(os.path.join(root, "")):
            return "%s holds this host's own tmux sockets" % host
    standing = servers(wanted_for(root))
    if standing:
        return "a tmux server is already bound under it (pid %s)" % standing[0][0]
    return ""


def claim(root):
    """Mark a directory as owned by this run and return its fresh token."""
    refusal = claimable(root)
    if refusal:
        sys.stderr.write(
            "suite-reaper: %s cannot be claimed: %s\n" % (root, refusal)
        )
        return ""
    token = secrets.token_hex(16)
    os.makedirs(root, exist_ok=True)
    with open(os.path.join(root, MARKER), "w", encoding="utf-8") as stream:
        stream.write(token + "\n")
    return token


def extra_sockets(root):
    """Socket paths outside the root that the run registered as its own.

    Only the separator is removed. A socket path may legally end in a space,
    and trimming it here would record one path and look for another.
    """
    try:
        # These are socket paths, written as bytes by a shell. The kernel's
        # own table is read with the same substitution, so reading this file
        # any other way would spell one path here and another there and match
        # neither.
        with open(
            os.path.join(root, EXTRA), encoding="utf-8", errors="replace"
        ) as stream:
            return {line.rstrip("\n") for line in stream if line.rstrip("\n")}
    except OSError:
        return set()


def wanted_for(root):
    prefix = os.path.join(root, "")
    extra = extra_sockets(root)

    def wanted(path):
        return path.startswith(prefix) or path in extra

    return wanted


def gone(pid, seconds):
    """Wait for a process to exit, up to `seconds`.

    A pidfd that cannot be opened is a process that is already gone.
    """
    try:
        handle = os.pidfd_open(pid)
    except (AttributeError, OSError):
        return
    try:
        select.select([handle], [], [], seconds)
    finally:
        os.close(handle)


def sweep(root, token=""):
    """End every server this run owns, then remove the run's own directory.

    The marker is the authority. A directory carrying none was never claimed by
    a run, so nothing in it is selected; a caller holding a token must match the
    marker exactly, so a watcher armed for one run declines a later run that
    remade the same path.
    """
    found = marker_token(root)
    if not found or (token and found != token):
        return []
    wanted = wanted_for(root)
    reaped = []
    for pid, socket_path in servers(wanted):
        try:
            os.kill(pid, signal.SIGTERM)
        except OSError:
            continue
        reaped.append((pid, socket_path))
    for pid, _socket_path in reaped:
        gone(pid, TERM_GRACE_SECONDS)
    for pid, socket_path in servers(wanted):
        try:
            os.kill(pid, signal.SIGKILL)
        except OSError:
            continue
        if (pid, socket_path) not in reaped:
            reaped.append((pid, socket_path))
    # A killed server never unlinks its own socket. The ones inside the root go
    # with the root; a registered socket outside it has to be named to go.
    for path in extra_sockets(root):
        if not [pid for pid, bound in servers(wanted) if bound == path]:
            try:
                os.unlink(path)
            except OSError:
                pass
    shutil.rmtree(root, ignore_errors=True)
    return reaped


def start_time(pid):
    """Field 22 of /proc/PID/stat, which distinguishes a reused pid."""
    with open("/proc/%d/stat" % pid, encoding="utf-8", errors="replace") as stream:
        data = stream.read()
    return data[data.rindex(")") + 1:].split()[19]


def latch(pid, expected):
    """A pidfd for exactly the process that asked to be watched.

    Returns the open handle, GONE when that process has already exited, or
    UNSUPPORTED when this host cannot hold a pidfd at all. The three are kept
    apart because only the first two say anything about the run: reading a
    kernel with no pidfds as a run that has ended would sweep a suite that is
    still working.
    """
    try:
        handle = os.pidfd_open(pid)
    except AttributeError:
        return UNSUPPORTED
    except OSError as error:
        return GONE if error.errno == errno.ESRCH else UNSUPPORTED
    # The pidfd already refers to one process for good, so reading the start
    # time after taking it settles whether that process is the caller or a
    # reuse of its number that happened first.
    try:
        if start_time(pid) != expected:
            os.close(handle)
            return GONE
    except (OSError, ValueError, IndexError):
        os.close(handle)
        return GONE
    return handle


def watchable():
    """Why this host cannot hold a pidfd or read its bound sockets, or ""."""
    try:
        handle = os.pidfd_open(os.getpid())
    except AttributeError:
        return "this build of Python has no os.pidfd_open"
    except OSError as error:
        return "os.pidfd_open is unavailable here: %s" % error
    os.close(handle)
    if not os.access(UNIX_SOCKETS, os.R_OK):
        return "%s cannot be read, so a bound socket cannot be resolved" % UNIX_SOCKETS
    return ""


def announce(root, reaped):
    """Hand a run's barrier the fact that its sweep finished."""
    socket_path = os.environ.get("SUITE_REAPER_DONE_SOCKET", "")
    channel = os.environ.get("SUITE_REAPER_DONE_CHANNEL", "")
    tmux = os.environ.get("SUITE_REAPER_TMUX", "")
    log = os.environ.get("SUITE_REAPER_LOG", "")
    if log:
        try:
            with open(log, "a", encoding="utf-8") as stream:
                for pid, path in reaped:
                    stream.write("%s\t%s\t%s\n" % (root, pid, path))
                stream.write("%s\tdone\n" % root)
        except OSError:
            pass
    if socket_path and channel and tmux:
        try:
            subprocess.run(
                [tmux, "-S", socket_path, "wait-for", "-S", channel],
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
        except OSError:
            pass


def watch(root, pid, expected, token):
    handle = latch(pid, expected)
    if handle == UNSUPPORTED:
        sys.stderr.write(
            "suite-reaper: no pidfd for %s, so %s is not watched\n" % (pid, root)
        )
        return 3
    if handle != GONE:
        try:
            select.select([handle], [], [], None)
        finally:
            os.close(handle)
    announce(root, sweep(root, token))
    return 0


def main(argv):
    if len(argv) >= 3 and argv[1] in ("--sweep", "--watch", "--claim"):
        if not usable_root(argv[2]):
            sys.stderr.write(
                "suite-reaper: %r does not name a run root, so no server was"
                " selected; a run root is the canonical path of a real"
                " directory\n" % (argv[2],)
            )
            return 2
    if len(argv) >= 3 and argv[1] == "--claim":
        token = claim(argv[2])
        if not token:
            return 2
        sys.stdout.write(token)
        return 0
    if len(argv) >= 3 and argv[1] == "--sweep":
        sweep(argv[2], argv[3] if len(argv) >= 4 else "")
        return 0
    if len(argv) >= 3 and argv[1] == "--start-time":
        sys.stdout.write(start_time(int(argv[2])))
        return 0
    if len(argv) >= 2 and argv[1] == "--watchable":
        reason = watchable()
        if reason:
            sys.stderr.write("suite-reaper: %s\n" % reason)
            return 3
        return 0
    if len(argv) >= 6 and argv[1] == "--watch":
        return watch(argv[2], int(argv[3]), argv[4], argv[5])
    sys.stderr.write(
        "suite-reaper: --claim ROOT | --sweep ROOT [TOKEN]"
        " | --start-time PID | --watchable | --watch ROOT PID START TOKEN\n"
    )
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
