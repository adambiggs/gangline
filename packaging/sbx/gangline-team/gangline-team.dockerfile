# syntax=docker/dockerfile:1
# SPDX-License-Identifier: Apache-2.0
# The shell template the Docker agent mixins build against, plus tmux, gang,
# and an sshd that carries Gangline peer delivery.
FROM docker/sandbox-templates:shell-docker
USER root
# Host keys are generated per sandbox at create, so no two sandboxes from this
# image share one.
RUN apt-get update \
 && apt-get install -y --no-install-recommends tmux openssh-server \
 && rm -rf /var/lib/apt/lists/* /etc/ssh/ssh_host_*
# sshd keeps the first value it reads, so this drop-in sorts first.
COPY sshd.conf /etc/ssh/sshd_config.d/00-gangline.conf
# The Claude mixin writes a permission mode into ~/.claude/settings.json on
# every create; a mode in managed settings outranks it.
COPY managed-settings.json /etc/claude-code/managed-settings.json
COPY --chmod=0755 gang /usr/local/bin/gang
# A credential failure fails git instead of opening a prompt that hangs an
# agent's pane.
ENV GIT_TERMINAL_PROMPT=0
# The base image sets no locale, and in the POSIX locale tmux draws its
# borders with ACS fallback characters and the agents drop every non-ASCII
# glyph; this reaches sbx exec sessions too, which start from the image
# environment.
ENV LANG=C.UTF-8
USER agent
