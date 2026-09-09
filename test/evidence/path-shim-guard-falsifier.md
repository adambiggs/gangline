# PATH-shim guard falsifier

Mutation: replace `path_shim_guard` in `test/path-shim-guard.sh` with a function
that returns success without inspecting its arguments.

The ordinary `test/gate.sh` verdict is red in `test/path-shim-guard-test.sh`,
which is invoked by the mandatory integration suite. It reports failures for:

- `a PATH-resolved target is refused`;
- `the ninth inherited delegation is refused`;
- `an absolute self-target is refused`; and
- `an absolute alias of the shim is refused`.

The last two cases reach `Cannot fork` inside their transient services instead
of consuming the host's ambient process table. Each launcher records
`pids.max`, refuses unless it is exactly 12, and only then starts the recursive
fake `git`, so that outcome is a falsifier for the local guards and a pass for
the independent safety boundary.
