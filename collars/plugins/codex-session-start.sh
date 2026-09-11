#!/bin/sh
# SPDX-License-Identifier: Apache-2.0

printf '%s\n\n%s\n' \
  "Codex long-command guidance: when an exec call yields a running session, keep its session id and do not re-poll it on a timer. Do independent work first, and continue the session only when its result is needed. A yield is not completion evidence." \
  "Do not detach a long command expecting this Gangline window to wake itself. Plain background children do not survive Codex's exec boundary here, the sandbox may not reach the user service manager, and Gangline refuses self-addressed send and at messages."
