package schema

import "time"

#ID: string & !=""
#Time: string & time.Format(time.RFC3339)

#Hitch: close({
	id:        #ID
	name:      #ID
	collar:    string & !=""
	role?:     string
	directory: string & !=""
	status?:   "starting" | "active" | "dropping" | "dropped" | "failed"
	pane?:     string
})

#Message: close({
	text: string
})

#Envelope: close({
	id:         #ID
	from:       #ID
	to:         #ID
	message:    #Message
	created_at: #Time
})

#Event: #HitchRequested | #HitchReady | #HitchLaunchFailed | #SendRequested |
	#DeliverySucceeded | #DeliveryFailed | #DropRequested | #DropSucceeded |
	#DropFailed | #TransitionRejected

#HitchRequested: close({
	type: "hitch_requested"
	at:   #Time
	hitch: #Hitch
})

#HitchReady: close({
	type:     "hitch_ready"
	at:       #Time
	hitch_id: #ID
	pane:     string & !=""
})

#HitchLaunchFailed: close({
	type:     "hitch_launch_failed"
	at:       #Time
	hitch_id: #ID
	reason:   string & !=""
})

#SendRequested: close({
	type:     "send_requested"
	at:       #Time
	envelope: #Envelope
})

#DeliverySucceeded: close({
	type:        "delivery_succeeded"
	at:          #Time
	envelope_id: #ID
})

#DeliveryFailed: close({
	type:        "delivery_failed"
	at:          #Time
	envelope_id: #ID
	reason:      string & !=""
})

#DropRequested: close({
	type:     "drop_requested"
	at:       #Time
	hitch_id: #ID
})

#DropSucceeded: close({
	type:     "drop_succeeded"
	at:       #Time
	hitch_id: #ID
})

#DropFailed: close({
	type:     "drop_failed"
	at:       #Time
	hitch_id: #ID
	reason:   string & !=""
})

#TransitionRejected: close({
	type:   "transition_rejected"
	at:     #Time
	event:  string & !=""
	reason: string & !=""
})
