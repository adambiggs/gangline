package collars

collar: {
	name: "codex"
	launch: {
		command: "codex"
		args: []
	}
	hooks: {
		notify: {
			template: "notify"
			event: "turn-boundary"
			payload: {
				turn_id: "turn-id"
			}
		}
	}
	primitives: {
		startup: [{name: "trust-prompt"}]
		composer: {name: "composer-read", params: {ghost_text: "true"}}
		submit: {name: "submit"}
		turn_boundary: {name: "hook-boundary", params: {hook: "notify"}}
		model_id: {name: "screen-model-id"}
		wedge: {name: "screen-wedge"}
	}
}
