package schema
import "time"
#ID: string & !=""
#Time: string & time.Format(time.RFC3339)
#Activity: "unknown" | "idle" | "busy" | "compacting" | "interrupting" | "blocked" | "wedged"
#Envelope: close({
 id: #ID
 token?: string & =~"^[0-9a-f]{16}$"
 from: close({kind: "agent" | "self_declared" | "gangline", name: #ID, hitch_id?: #ID})
 to: #ID
 recipient: #ID
 message: close({text: #ID})
 startup?: close({contract: #ID, doctrine?: string, role?: string})
 paste_only?: close({witness_id: string})
 purpose?: "startup" | "assignment" | "resume"
 created_at: #Time
 measured_at?: #Time
 not_before?: #Time
 not_after?: #Time
 outcome?: "accepted" | "delivered" | "failed" | "unverified" | "cancelled"
 reason?: string
})
#Compaction: close({id: #ID, resume: close({text: string}), resume_from?: close({kind: "agent" | "self_declared" | "gangline", name: #ID, hitch_id?: #ID}), requester?: close({kind: "agent", name: #ID, hitch_id: #ID}), started_at: #Time, deadline: #Time, status: "queued" | "submitted" | "completed" | "failed" | "unverified", continuation?: bool, resume_token?: string, resume_admitted?: bool, completed_at?: #Time, refusal_before?: int & >=0, reason?: string})
#Reading: close({
 kind: string, source: string, native_event?: string, at?: #Time, status: string, reason?: string, model?: string
 used?: int & >=0, limit?: int & >0, percent?: number & >=0
 limits?: [...close({label: #ID, used_percent: number & >=0, reset_at: int & >0, window_minutes?: int & >0})]
})
#Basis: close({
 screen: "blocked" | "compacting" | "idle" | "busy" | "unsubmitted" | "unreadable" | "unread"
 rule: "screen" | "open-turn" | "interrupt-pending" | "compaction-record" | "turn-failure" | "wedge" | "probe-failure" | "permission-request"
 compaction?: "queued" | "submitted" | "completed" | "failed" | "unverified"
})
#Fields: close({
 type: string
 at: #Time
 source?: "hook" | "command" | "watchdog"
 hitch_id?: #ID, name?: #ID, pane?: #ID, id?: #ID, reason?: string, status?: string
 activity?: #Activity, deadline?: #Time, envelope?: #Envelope, compaction?: #Compaction
 readings?: [...#Reading], native_event?: string, fingerprint?: #ID, basis?: #Basis
})

// Indexed by type, so validating an event is one lookup rather than a trial
// of every type's branch.
#Types: {
 hitch_claimed: {}, hitch_spawned: {}, hitch_ready: {}, hitch_blocked: {}, hitch_failed: {}, boot_reopened: {hitch_id: #ID, deadline: #Time}, renamed: {}, send_cancelled: {}, delivery_accepted: {}
 delivery_succeeded: {}, delivery_failed: {}, delivery_unverified: {}, activity_observed: {}, observation: {}, native_hook: {}, compaction_waiting: {}, compaction_submitted: {}
 compaction_completed: {}, compaction_failed: {}, compaction_unverified: {}, interrupt_requested: {}, interrupt_completed: {}, deadline_checked: {}, drop_started: {}, drop_finished: {}
 curfew_set: {}, curfew_cleared: {}, capacity_detected: {}, capacity_submitted: {}, capacity_cleared: {}, watchdog_available: {}, process_verification_available: {}
 send_queued: {envelope: #Envelope}
 context_band_crossed: {id: #ID, hitch_id: #ID, status: #ID, envelope: #Envelope, readings: [#Reading]}
 snooze_scheduled: {id: #ID, hitch_id: #ID, reason: #ID}
 snooze_cleared: {id: #ID, hitch_id: #ID, reason: #ID}
 snooze_failed: {id: #ID, hitch_id: #ID, reason: #ID}
 snooze_completed: {id: #ID, hitch_id: #ID}
 notice_failed: {id: #ID, hitch_id: #ID, reason: #ID}
 snooze_cap_rejected: {id: #ID, hitch_id: #ID, reason: #ID}
 snooze_rearmed: {id: #ID, hitch_id: #ID, deadline: #Time, reason: #ID}
 input_started: {id: #ID, status: #ID, hitch_id: #ID}
 input_finished: {id: #ID, status: #ID, hitch_id: #ID}
 hook_failed: {reason: #ID}
 watchdog_unavailable: {reason: #ID}
 watchdog_failed: {reason: #ID}
 process_verification_unavailable: {reason: #ID}
 tick: {source: "hook" | "command" | "watchdog"}
 tick_failed: {source: "hook" | "command" | "watchdog", reason: #ID}
 compaction_requested: {compaction: #Compaction}
}
#Event: #Fields & {type: _, #Types[type]}
