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
			"{\"statusLine\":{\"type\":\"command\",\"command\":{{statusline.command.json}}},\"hooks\":{\"UserPromptSubmit\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PostToolUse\":[{\"matcher\":\"*\",\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"StopFailure\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}},\"async\":true}]}],\"Stop\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}},\"async\":true}]}],\"PermissionRequest\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PreCompact\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}}}]}],\"PostCompact\":[{\"hooks\":[{\"type\":\"command\",\"command\":{{hook.command.json}},\"async\":true}]}]}}",
		]
		events: {
			userpromptsubmit: {event: "turn-started", payload: {session_id: "session_id", transcript_path: "transcript_path", prompt: "prompt"}}
			posttooluse: {event: "activity", payload: {session_id: "session_id", transcript_path: "transcript_path"}}
			stopfailure: {event: "turn-failed", payload: {session_id: "session_id", transcript_path: "transcript_path", error: "error", error_details: "error_details"}}
 stop: {event: "turn-finished", payload: {session_id: "session_id", transcript_path: "transcript_path"}}
			permissionrequest: {event: "permission-requested", payload: {session_id: "session_id"}}
			precompact: {event: "compaction-started", payload: {session_id: "session_id", trigger: "trigger", transcript_path: "transcript_path"}}
			postcompact: {event: "compaction-finished", payload: {session_id: "session_id", trigger: "trigger", transcript_path: "transcript_path"}}
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
 telemetry: {name: "claude-status-line"}
		mid_turn: true
		startup: [{name: "claude-trust-prompt"}, {name: "claude-composer"}]
		composer: {name: "claude-composer"}
		submit: {name: "enter-submit", params: {paste: "bracketed", settle: "400ms"}}
		submit_witness: {name: "claude-pasted-content"}
		turn_boundary: {name: "hook-boundary"}
		blocked: {name: "screen-blocked", params: {
			prompt: "Do you want to proceed\\?|Allow .*\\?|needs your permission|Permission required"
			choice: "(?m)^[[:space:]❯>]*1\\. Yes|Yes, and don't ask again|Allow"
		}}
		context: {name: "claude-screen-context"}
		provider_limits: {name: "claude-screen-limits"}
		wedge: {name: "stable-busy-screen", params: {busy: "esc to interrupt|Retrying in [0-9]+s|API Error: 529 Overloaded\\.|▰|▱|(?m)^· (Doing|Crunching)…", after: "5m"}}
	}
	actions: {
		interrupt: {keys: ["Escape"]}
		compact: {text: "/compact {{instructions}}", submit: true}
		compact_recover: [{keys: ["Escape"]}, {keys: ["Enter"]}]
	}
	context_bands: {
		"*": [
			{name: "early", at: 0.10},
			{name: "late", at: 0.20},
		]
		"*haiku*": [
			{name: "yellow", at: 0.45},
			{name: "red", at: 0.65},
		]
	}
}
