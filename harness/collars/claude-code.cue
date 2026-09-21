package collars

collar: {
	name: "claude-code"
	launch: {
		command: "claude"
		args: []
	}
	hooks: {
		stop: {
			template: "hooks.Stop"
			event: "turn-boundary"
			payload: {
				session_id: "session_id"
			}
		}
	}
	primitives: {
		startup: [{name: "trust-prompt"}]
		composer: {name: "composer-read", params: {ghost_text: "true"}}
		submit: {name: "submit"}
		turn_boundary: {name: "hook-boundary", params: {hook: "stop"}}
		model_id: {name: "hook-model-id"}
		wedge: {name: "screen-wedge"}
	}
}
