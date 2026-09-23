package schema
import "time"
#ID: string & !=""
#Time: string & time.Format(time.RFC3339)
#Activity: "unknown" | "idle" | "busy" | "compacting" | "interrupting" | "blocked" | "wedged"
#Envelope: close({
 id: #ID
 from: close({kind: "agent" | "self_declared" | "gangline", name: #ID, hitch_id?: #ID})
 to: #ID
 recipient: #ID
 message: close({text: #ID})
 purpose?: "startup" | "assignment"
 created_at: #Time
 not_before?: #Time
 outcome?: "accepted" | "delivered" | "failed" | "unverified" | "cancelled"
 reason?: string
})
#Compaction: close({id: #ID, resume: close({text: string}), started_at: #Time, deadline: #Time, status: "queued" | "submitted" | "completed" | "failed" | "unverified", continuation?: bool, completed_at?: #Time, refusal_before?: int & >=0, reason?: string})
#Reading: close({
 kind: string, source: string, native_event?: string, at?: #Time, status: string, reason?: string, model?: string
 used?: int & >=0, limit?: int & >0, percent?: number & >=0
 limits?: [...close({label: #ID, used_percent: number & >=0, reset_at: int & >0})]
})
#Fields: close({
 type: "hitch_claimed" | "hitch_spawned" | "hitch_ready" | "hitch_blocked" | "hitch_failed" |
  "adopted" | "renamed" | "send_queued" | "send_cancelled" |
  "input_started" | "input_finished" | "delivery_accepted" | "delivery_succeeded" | "delivery_failed" | "delivery_unverified" |
  "activity_observed" | "observation" | "native_hook" | "hook_failed" | "context_band_crossed" |
  "compaction_requested" | "compaction_submitted" | "compaction_completed" | "compaction_failed" | "compaction_unverified" |
  "interrupt_requested" | "interrupt_completed" | "deadline_checked" |
  "drop_started" | "drop_finished" | "curfew_set" | "curfew_cleared" |
  "capacity_detected" | "capacity_submitted" | "capacity_cleared"
 at: #Time
 hitch_id?: #ID, name?: #ID, pane?: #ID, id?: #ID, reason?: string, status?: string
 activity?: #Activity, deadline?: #Time, envelope?: #Envelope, compaction?: #Compaction
 readings?: [...#Reading], native_event?: string, fingerprint?: #ID
})

#Event: #Fields & (
 {type: "hitch_claimed" | "hitch_spawned" | "hitch_ready" | "hitch_blocked" | "hitch_failed" | "adopted" | "renamed" | "send_cancelled" | "delivery_accepted" | "delivery_succeeded" | "delivery_failed" | "delivery_unverified" | "activity_observed" | "observation" | "native_hook" | "compaction_submitted" | "compaction_completed" | "compaction_failed" | "compaction_unverified" | "interrupt_requested" | "interrupt_completed" | "deadline_checked" | "drop_started" | "drop_finished" | "curfew_set" | "curfew_cleared" | "capacity_detected" | "capacity_submitted" | "capacity_cleared"} |
 {type: "send_queued", envelope: #Envelope} |
 {type: "context_band_crossed", id: #ID, hitch_id: #ID, status: #ID, envelope: #Envelope, readings: [#Reading]} |
 {type: "input_started" | "input_finished", id: #ID, status: #ID, hitch_id: #ID} |
 {type: "hook_failed", reason: #ID} |
 {type: "compaction_requested", compaction: #Compaction})
