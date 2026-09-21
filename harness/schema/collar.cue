package harness

#Name: string & =~"^[a-z][a-z0-9-]*$"

#Invocation: close({
	name: #Name
	params?: [string]: string
})

#Hook: close({
	template: string & !=""
	event:    #Name
	payload?: [string]: string
})

#Collar: close({
	name: #Name
	launch: close({
		command: string & !=""
		args?: [...string]
		env?: [string]: string
	})
	hooks?: [#Name]: #Hook
	primitives: close({
		startup:       [...#Invocation] & [_, ...]
		composer:      #Invocation
		submit:        #Invocation
		turn_boundary: #Invocation
		model_id?:     #Invocation
		wedge?:        #Invocation
	})
})
