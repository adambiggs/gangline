package schema

import "time"

#ID: string & !=""
#Text: string & !=""
#Time: string & time.Format(time.RFC3339)

#Message: close({text: #Text})

#Sender: close({
	kind: "agent" | "self_declared"
	name: #ID
	hitch_id?: #ID
})

#Hitch: close({
	capacity?: #Capacity
	id: #ID
	name: #ID
	collar: #ID
	role?: string
	directory: #ID
	status?: "starting" | "booting" | "active" | "dropping" | "dropped" | "failed"
	activity?: "unknown" | "idle" | "busy" | "delivering" | "compacting" | "interrupting" | "blocked" | "wedged"
	pane?: string
	boot_deadline?: #Time
	drop_deadline?: #Time
	interrupt_deadline?: #Time
	interrupt_reason?: string
	pending_compact_id?: #ID
	blocked_evidence?: #Text
	blocked_from?: "unknown" | "idle" | "busy" | "delivering" | "compacting" | "interrupting" | "blocked" | "wedged"
	wedge_evidence?: #Text
	previous_activity?: "unknown" | "idle" | "busy" | "delivering" | "compacting" | "interrupting" | "blocked" | "wedged"
})

#Envelope: close({
	id: #ID
	from: #Sender
	to: #ID
	message: #Message
	created_at: #Time
})

#Compaction: close({
	submitted?: bool
	id: #ID
	hitch_id: #ID
	resume: #Message
	deadline: #Time
	status?: "queued" | "running" | "succeeded" | "failed" | "unverified" | "cancelled"
	reason?: #Text
})

#Event: #Observation | #CapacityDetected | #CapacityRetryRequested | #CapacityExpired | #CapacityCleared | #NativeHook | #HitchRequested | #AdoptRequested | #RenameRequested | #HitchSpawned | #HitchReady | #HitchLaunchFailed |
	#TurnStarted | #TurnBoundaryReached | #BlockedDetected | #BlockedCleared | #SendRequested | #TimedDeliveryReleased | #TimedDeliveriesCleared | #DeliverySucceeded |
	#DeliveryInputStarted | #DeliveryRetryRequested | #DeliveryDeferred | #DeliveryFailed | #DeliveryUnverified |
	#CompactionRequested | #CompactionCompleted | #CompactionSubmitted | #CompactionUnverified | #CompactionFailed | #InterruptRequested | #InterruptSucceeded | #InterruptFailed |
	#DropRequested | #DropSucceeded | #DropFailed | #PaneVanished | #WedgeDetected |
	#WedgeCleared | #OperationTimedOut | #CurfewSet | #CurfewCleared | #TransitionRejected

#HitchRequested: close({type: "hitch_requested", at: #Time, hitch: #Hitch, boot_deadline: #Time})
#Capacity: close({fingerprint?: #ID, evidence?: #Text, deadline?: #Time, next_at?: #Time, attempts?: int & >=0, envelope_id?: #ID, expired?: bool})
#CapacityDetected: close({type: "capacity_detected", at: #Time, hitch_id: #ID, fingerprint: #ID, evidence: #Text, deadline: #Time})
#CapacityRetryRequested: close({type: "capacity_retry_requested", at: #Time, hitch_id: #ID, fingerprint: #ID})
#CapacityExpired: close({type: "capacity_expired", at: #Time, hitch_id: #ID})
#CapacityCleared: close({type: "capacity_cleared", at: #Time, hitch_id: #ID})
#AdoptRequested: close({type: "adopt_requested", at: #Time, hitch: #Hitch, pane: #ID})
#RenameRequested: close({type: "rename_requested", at: #Time, hitch_id: #ID, name: #ID})
#HitchSpawned: close({type: "hitch_spawned", at: #Time, hitch_id: #ID, pane: #ID})
#HitchReady: close({type: "hitch_ready", at: #Time, hitch_id: #ID})
#HitchLaunchFailed: close({type: "hitch_launch_failed", at: #Time, hitch_id: #ID, reason: #Text})
#TurnStarted: close({type: "turn_started", at: #Time, hitch_id: #ID})
#TurnBoundaryReached: close({type: "turn_boundary_reached", at: #Time, hitch_id: #ID})
#BlockedDetected: close({type: "blocked_detected", at: #Time, hitch_id: #ID, evidence: #Text})
#BlockedCleared: close({type: "blocked_cleared", at: #Time, hitch_id: #ID})
#SendRequested: close({mid_turn?: bool, type: "send_requested", at: #Time, envelope: #Envelope, deadline: #Time, not_before?: #Time})
#TimedDeliveryReleased: close({type: "timed_delivery_released", at: #Time, envelope_id: #ID})
#TimedDeliveriesCleared: close({type: "timed_deliveries_cleared", at: #Time, recipient: #ID})
#DeliverySucceeded: close({type: "delivery_succeeded", at: #Time, envelope_id: #ID})
#DeliveryRetryRequested: close({type: "delivery_retry_requested", at: #Time, envelope_id: #ID})
#DeliveryDeferred: close({type: "delivery_deferred", at: #Time, envelope_id: #ID, reason: #Text})
#DeliveryFailed: close({type: "delivery_failed", at: #Time, envelope_id: #ID, reason: #Text})
#DeliveryInputStarted: close({type: "delivery_input_started", at: #Time, envelope_id: #ID})
#DeliveryUnverified: close({blocked_evidence?: #Text,type: "delivery_unverified", at: #Time, envelope_id: #ID, evidence: #Text})
#CompactionRequested: close({type: "compaction_requested", at: #Time, compaction: #Compaction})
#CompactionCompleted: close({type: "compaction_completed", at: #Time, compaction_id: #ID, continuation?: #Envelope})
#CompactionSubmitted: close({type: "compaction_submitted", at: #Time, compaction_id: #ID})
#CompactionUnverified: close({type: "compaction_unverified", at: #Time, compaction_id: #ID, evidence: #Text})
#CompactionFailed: close({type: "compaction_failed", at: #Time, compaction_id: #ID, reason: #Text})
#InterruptRequested: close({type: "interrupt_requested", at: #Time, hitch_id: #ID, reason?: string, deadline: #Time})
#InterruptSucceeded: close({type: "interrupt_succeeded", at: #Time, hitch_id: #ID})
#InterruptFailed: close({type: "interrupt_failed", at: #Time, hitch_id: #ID, reason: #Text})
#DropRequested: close({type: "drop_requested", at: #Time, hitch_id: #ID, deadline: #Time})
#DropSucceeded: close({type: "drop_succeeded", at: #Time, hitch_id: #ID})
#DropFailed: close({type: "drop_failed", at: #Time, hitch_id: #ID, reason: #Text})
#PaneVanished: close({type: "pane_vanished", at: #Time, hitch_id: #ID, evidence: #Text})
#WedgeDetected: close({type: "wedge_detected", at: #Time, hitch_id: #ID, evidence: #Text})
#WedgeCleared: close({type: "wedge_cleared", at: #Time, hitch_id: #ID})
#OperationTimedOut: close({
	type: "operation_timed_out"
	at: #Time
	operation: "boot" | "turn" | "delivery" | "compaction" | "interrupt" | "drop" | "wait"
	id: #ID
	deadline: #Time
	evidence: #Text
})
#CurfewSet: close({type: "curfew_set", at: #Time, deadline: #Time})
#CurfewCleared: close({type: "curfew_cleared", at: #Time})
#TransitionRejected: close({type: "transition_rejected", at: #Time, event: #ID, reason: #Text})

#NativeHook: close({
 type: "native_hook"
 at: #Time
 id: #ID
 hitch_id?: #ID
 native_event?: #Text
 status: "received" | "completed" | "ignored" | "failed"
 reason?: #Text
})

#Reading: close({
 kind: #ID
 source: "native-hook" | "session-log" | "status-line" | "screen"
 native_event?: #Text
 at?: #Time
 status: "observed" | "unknown" | "error"
 reason?: #Text
 model?: #Text
 used?: int & >=0
 limit?: int & >0
 percent?: number & >=0
 limits?: [...close({label: #Text, used_percent: number & >=0, reset_at: int & >0})]
})
#Observation: close({
 type: "observation"
 at: #Time
 hitch_id: #ID
 let ObservedAt = at
 let ObservedHitch = hitch_id
 observation: close({
  at: ObservedAt
  hitch_id: ObservedHitch
  collar: #ID
  session_id?: #Text
  transcript?: #Text
  offset?: int & >=0
  readings: [...#Reading]
 })
})
