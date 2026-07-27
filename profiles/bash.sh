# shellcheck shell=bash
# shellcheck disable=SC2034  # consumed by bin/gang load_profile via source
# SPDX-License-Identifier: Apache-2.0
# Plain bash — for testing gangline's own mechanics. Never busy. No scraped
# TUI markers, so no version pin ("any" = version-independent to gang doctor).
GANG_LAUNCH="bash"
GANG_BUSY_REGEX=""
GANG_VERIFIED_VERSIONS="any"
