package harness

#Name: string & =~"^[a-z][a-z0-9-]*$"
#PrimitiveName: #Name | string & =~"^exec:.+"

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
	flag: string & !=""
	joined?: bool
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

#Collar: close({
	name: #Name
	launch: close({
		command: string & !=""
		args?: [...string]
		resume_args?: [...string]
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
		turn_boundary: #Invocation
		context:       #Invocation
		wedge:         #Invocation
	})
	context_bands: [...#ContextBand]
})
