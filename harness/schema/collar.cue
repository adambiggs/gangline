package harness

#Name: string & =~"^[a-z][a-z0-9-]*$"
#PrimitiveName: #Name

#Invocation: close({
	name: #PrimitiveName
	params?: [string]: string
})

#Hook: close({
	event: "turn-started" | "turn-finished" | "permission-requested" | "compaction-started" | "compaction-finished" | "activity"
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
	message: string & !=""
})

#Action: close({
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
		startup:       [...#Invocation] & [_, ...]
		composer:      #Invocation
		submit:        #Invocation
		submit_witness: #Invocation
		turn_boundary: #Invocation
		context:       #Invocation
		provider_limits: #Invocation
		wedge:         #Invocation
	})
	actions: close({
		interrupt: #Action
		compact: #Action
		compact_recover: [...#Action]
		queue_recall?: #Action
	})
	context_bands: [string & !=""]: [...#ContextBand]
})
