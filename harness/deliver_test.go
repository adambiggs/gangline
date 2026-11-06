package harness

import "testing"

func TestCodexHomeOfReadsOnlyARolloutLayout(t *testing.T) {
	const session = "019a0000-0000-7000-8000-000000000001"
	for path, want := range map[string]string{
		"/h/.codex/sessions/2026/11/06/rollout-2026-11-06T21-00-00-" + session + ".jsonl": "/h/.codex",
		"/h/.codex/sessions/2026/11/06/../06/rollout-x-" + session + ".jsonl":             "/h/.codex",
		"h/.codex/sessions/2026/11/06/rollout-x-" + session + ".jsonl":                    "",
		"/h/.codex/sessions/2026/11/rollout-x-" + session + ".jsonl":                      "",
		"/h/.codex/archive/2026/11/06/rollout-x-" + session + ".jsonl":                    "",
		"/h/.codex/sessions/2026/1/06/rollout-x-" + session + ".jsonl":                    "",
		"/h/.codex/sessions/2026/11/06/rollout-x-other.jsonl":                             "",
		"/h/.codex/sessions/2026/11/06/" + session + ".jsonl":                             "",
		"/sessions/2026/11/06/rollout-x-" + session + ".jsonl":                            "/",
	} {
		got, ok := codexHomeOf(path, session)
		if ok != (want != "") || got != want {
			t.Errorf("codexHomeOf(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
}
