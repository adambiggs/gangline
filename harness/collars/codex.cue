package collars

collar: {
	name: "codex"
	launch: {
		command: "codex"
		args: ["-c", "check_for_update_on_startup=false"]
		resume_args: ["resume", "{{session_id}}", "-c", "check_for_update_on_startup=false", "-c", "tui.resume_cwd=\"current\""]
		probe_args: ["--dangerously-bypass-hook-trust"]
	}
	hooks: {
		install_args: [
			"-c", "hooks.UserPromptSubmit=[{ hooks = [{ type = \"command\", command = {{hook.command.json}} }] }]",
			"-c", "hooks.PostToolUse=[{ hooks = [{ type = \"command\", command = {{hook.command.json}} }] }]",
			"-c", "hooks.Stop=[{ hooks = [{ type = \"command\", command = {{hook.command.json}}, async = true, timeout = {{hook.timeout}} }] }]",
			"-c", "hooks.PermissionRequest=[{ hooks = [{ type = \"command\", command = {{hook.command.json}} }] }]",
			"-c", "hooks.PreCompact=[{ hooks = [{ type = \"command\", command = {{hook.command.json}} }] }]",
			"-c", "hooks.PostCompact=[{ hooks = [{ type = \"command\", command = {{hook.command.json}}, async = true, timeout = {{hook.timeout}} }] }]",
		]
		events: {
			userpromptsubmit: {event: "turn-started", payload: {session_id: "session_id", transcript_path: "transcript_path", prompt: "prompt", turn_id: "turn_id"}}
			posttooluse: {event: "activity", payload: {session_id: "session_id", transcript_path: "transcript_path", turn_id: "turn_id"}}
			stop: {event: "turn-finished", payload: {session_id: "session_id", transcript_path: "transcript_path", turn_id: "turn_id"}}
			permissionrequest: {event: "permission-requested", payload: {session_id: "session_id", turn_id: "turn_id"}}
			precompact: {event: "compaction-started", payload: {session_id: "session_id", trigger: "trigger"}}
			postcompact: {event: "compaction-finished", payload: {session_id: "session_id", trigger: "trigger"}}
		}
	}
	models: {
		catalog: {name: "codex-debug-models", params: {command: "codex", args: "debug models"}}
		selected: {name: "codex-screen-model"}
		option: {args: ["-m", "{{value}}"]}
	}
	options: {
		effort: {args: ["-c", "model_reasoning_effort={{value}}"]}
	}
	primitives: {
		mid_turn: true
		startup: [{name: "codex-trust-prompt"}, {name: "codex-composer"}]
		composer: {name: "codex-composer"}
		submit: {name: "enter-submit", params: {paste: "bracketed", settle: "400ms"}}
		submit_witness: {name: "exact-prompt"}
		turn_boundary: {name: "hook-boundary"}
		blocked: {name: "screen-blocked", params: {
			prompt: "Would you like to run|Do you want to allow|requires approval|approval required"
			choice: "Yes, proceed|Yes, and don't ask again|Press enter to confirm"
		}}
		context: {name: "codex-screen-context"}
		provider_limits: {name: "codex-screen-limits"}
		wedge: {name: "stable-busy-screen", params: {busy: "esc to interrupt", after: "5m"}}
	}
	actions: {
		interrupt: {keys: ["Escape"]}
		compact: {text: "/compact", submit: true}
		compact_recover: [{keys: ["Escape"]}, {keys: ["Enter"]}]
	}
	context_bands: {
		"*": [
			{name: "yellow", at: 0.75},
			{name: "red", at: 0.90},
		]
	}
}
