package harness

#Name: string & =~"^[a-z][a-z0-9-]*$"
#PrimitiveName: #Name

#Invocation: close({
	name: #PrimitiveName
	params?: [string]: string
})

#Hook: close({
	event: "turn-started" | "turn-finished" | "permission-requested" | "compaction-started" | "compaction-finished" | "activity" | "turn-failed"
	payload?: [string]: string
})

#Hooks: close({
	install_args: [...string] & [_, ...]
	events: [#Name]: #Hook
})

#Option: close({
	args: [...string] & [_, ...]
})

#Models: close({
	catalog: #Invocation
	selected?: #Invocation
	option: #Option
})

#ContextBand: close({
	name: #Name
	at: number & >=0 & <=1
})

#Action: close({
 refusal?: string
	text?: string
	keys?: [...string]
	submit?: bool
})

#Collar: close({
	name: #Name
	launch: close({
		command: string & !=""
		args?: [...string]
		resume_args?: [...string]
		probe_args?: [...string]
		env?: [string]: string
	})
	hooks?: #Hooks
	models: #Models
	options?: close({
		effort?: #Option
		role_prompt?: #Option
	})
	primitives: close({
 telemetry?: #Invocation
		capacity?: #Invocation
		mid_turn?: bool
		startup:       [...#Invocation] & [_, ...]
		composer:      #Invocation
		submit:        #Invocation
		submit_witness: #Invocation
		queue_witness?: #Invocation
		turn_boundary: #Invocation
		blocked:       #Invocation
		context:       #Invocation
		provider_limits: #Invocation
		limits_query?: #Invocation
		wedge:         #Invocation
	})
	actions: close({
		interrupt: #Action
		compact: #Action
		compact_recover: [...#Action]
	})
	context_bands: [string & !=""]: [...#ContextBand]
})
