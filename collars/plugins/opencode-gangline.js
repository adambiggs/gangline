// SPDX-License-Identifier: Apache-2.0
//
// OpenCode has no native hook command, so the identity every other collar gets
// from a harness hook is carried here instead: this plugin turns OpenCode's own
// session bus events into the one `gang hook` payload Gangline needs to stamp a
// resumable session id on the window.
//
// THE SHAPE IS THE SDK'S, NOT THIS FILE'S. `EventSessionCreated` and
// `EventSessionUpdated` both carry `properties.info` typed as `Session`, whose
// `id` is the value `opencode -s <id>` resumes and whose optional `parentID`
// marks a child session. Both are read defensively: a bus event this file
// cannot recognise stamps nothing rather than stamping a guess, because a wrong
// id is worse than none — `gang drop` would print a relaunch line that lands on
// somebody else's conversation.
//
// NOTHING ELSE IS CLAIMED. The payload is shaped as a Notification whose kind
// the collar declares no stall meaning for, so Gangline stamps the identity and
// acts on nothing: no turn bracket is opened that no event would ever close,
// and the collar keeps declaring no Stop hook, which is the truth about a
// harness whose turn boundaries Gangline still reads from the pane.
import { spawn } from "node:child_process"

const ID_SHAPE = /^[A-Za-z0-9._:-]+$/

export const GanglineIdentity = async ({ directory }) => {
  const gang = process.env.GANGLINE_HOOK
  const seen = new Set()

  // TMUX_PANE IS READ AT EVERY STAMP, not captured once: the plugin loads in
  // whatever process OpenCode starts it in, and a pane that is not there is the
  // reason to stamp nothing rather than something to work around.
  const stamp = (id, kind) => {
    const pane = process.env.TMUX_PANE
    if (!gang || !pane || seen.has(id)) return
    seen.add(id)
    const payload = JSON.stringify({
      hook_event_name: "Notification",
      notification_type: kind,
      session_id: id,
      cwd: directory,
    })
    try {
      const child = spawn(gang, ["hook"], {
        env: process.env,
        stdio: ["pipe", "ignore", "ignore"],
      })
      // A stamp that never reached gang must be retryable, so a spawn that
      // failed gives the id back rather than recording it as delivered.
      child.on("error", () => seen.delete(id))
      child.stdin.on("error", () => seen.delete(id))
      child.stdin.end(payload)
    } catch {
      seen.delete(id)
    }
  }

  return {
    event: async ({ event }) => {
      if (!event || (event.type !== "session.created" && event.type !== "session.updated")) return
      const info = event.properties && event.properties.info
      if (!info || typeof info.id !== "string" || !ID_SHAPE.test(info.id)) return
      // A CHILD SESSION IS NOT THE WINDOW'S IDENTITY. OpenCode creates one per
      // subagent; resuming onto it would drop the operator into a fragment of
      // the conversation they asked to come back to.
      if (info.parentID) return
      stamp(info.id, event.type)
    },
  }
}
