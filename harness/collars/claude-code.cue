package collars

collar: {
	name: "claude-code"
	launch: {
		command: "claude"
		args: []
		resume_args: ["--resume", "{{session_id}}"]
		probe_args: ["--model", "fable"]
	}
	hooks: {
		install_args: [
			"--settings",
			"{\"hooks\":{\"UserPromptSubmit\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PostToolUse\":[{\"matcher\":\"*\",\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"Stop\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PermissionRequest\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PreCompact\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PostCompact\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}]}}",
		]
		events: {
			userpromptsubmit: {event: "turn-started", payload: {session_id: "session_id", transcript_path: "transcript_path", prompt: "prompt"}}
			posttooluse: {event: "activity", payload: {session_id: "session_id", transcript_path: "transcript_path"}}
			stop: {event: "turn-finished", payload: {session_id: "session_id", transcript_path: "transcript_path"}}
			permissionrequest: {event: "permission-requested", payload: {session_id: "session_id"}}
			precompact: {event: "compaction-started", payload: {session_id: "session_id", trigger: "trigger"}}
			postcompact: {event: "compaction-finished", payload: {session_id: "session_id", trigger: "trigger"}}
		}
	}
	models: {
		catalog: {name: "claude-help-models", params: {command: "claude", args: "--help"}}
		selected: {name: "claude-screen-model"}
		option: {args: ["--model", "{{value}}"]}
	}
	options: {
		effort: {args: ["--effort={{value}}"]}
		role_prompt: {args: ["--append-system-prompt", "{{value}}"]}
	}
	primitives: {
		startup: [{name: "claude-trust-prompt"}, {name: "claude-composer"}]
		composer: {name: "claude-composer"}
		submit: {name: "enter-submit", params: {paste: "bracketed", settle: "400ms"}}
		submit_witness: {name: "claude-pasted-content"}
		turn_boundary: {name: "hook-boundary"}
		context: {name: "claude-screen-context"}
		provider_limits: {name: "claude-screen-limits"}
		wedge: {name: "stable-busy-screen", params: {busy: "esc to interrupt|Retrying in [0-9]+s|API Error: 529 Overloaded\\.|▰|▱", after: "5m"}}
	}
	actions: {
		interrupt: {keys: ["Escape"]}
		compact: {text: "/compact {{instructions}}", submit: true}
		compact_recover: [{keys: ["Escape"]}, {keys: ["Enter"]}]
	}
	context_bands: {
		"*": [
			{name: "yellow", at: 0.20},
			{name: "red", at: 0.40},
		]
		"*haiku*": [
			{name: "yellow", at: 0.45},
			{name: "red", at: 0.65},
		]
	}
}
