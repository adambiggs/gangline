# The team kit for Docker Sandboxes

`gangline-team` is a [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/)
workload kit: a sandbox image holding tmux, `gang` and an sshd for peer
delivery, built for Docker's Claude and Codex mixins to layer the agents onto.
Each release publishes it as `docker.io/adambiggs/gangline:<version>` with
that release's `gang`; running it is described in
[docs/operations.md](../../docs/operations.md#a-team-inside-docker-sandboxes).

Network access follows the user's Docker Sandboxes preset.

- `gangline-team/gangline-team.yaml`: the kit manifest (Docker's kit schema
  3): the proxy-managed GitHub token, install and startup
  commands.
- `gangline-team/gangline-team.dockerfile`: the image, from Docker's shell
  template.
- `gangline-team/sshd.conf`: the sshd drop-in; forced commands carry peer
  delivery.
- `gangline-team/managed-settings.json`: Claude Code's managed settings in the
  image.
- `stage.sh`: copies the kit beside `gang` binaries named by architecture,
  the build context `sbx run` or `docker buildx build` takes. The release
  workflow (`release-kit` in `.github/workflows/release.yml`) stages the
  release's own binaries, checks them against the release's `SHA256SUMS`, and
  pushes the image for `linux/amd64` and `linux/arm64`.
- `systemd/gangline-sbx@.service`: a user unit that restores the sandbox at
  login.
