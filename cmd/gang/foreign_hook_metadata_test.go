//go:build acceptance

package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/store"
)

func TestForeignHookRejectedBeforeMutation(t *testing.T) {
	for _, native := range []string{"UserPromptSubmit", "PostToolUse", "PermissionRequest", "PreCompact", "PostCompact"} {
		t.Run(native, func(t *testing.T) {
			fixture := nativeContextFixture(t, "rollout-test-parent.jsonl", 0, parentContextRecords)
			fixture.env["GANGLINE_HITCH_ID"] = "a"
			paths, err := fixture.run.team.Agent("a")
			if err != nil {
				t.Fatal(err)
			}
			if err := paths.WritePermissionWitness(store.PermissionWitness{At: fixture.cmd.now(), SessionID: "parent"}); err != nil {
				t.Fatal(err)
			}
			if err := paths.WriteBackground(store.Background{At: fixture.cmd.now(), Tasks: 2}); err != nil {
				t.Fatal(err)
			}
			before := contextStateSnapshot(t, paths.Directory)
			fixture.cmd.stdin = strings.NewReader(`{"hook_event_name":"` + native + `","session_id":"parent","agent_id":"child","transcript_path":"/sessions/child.jsonl","turn_id":"child-turn"}`)
			if err := fixture.cmd.handleHook(nil); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(contextStateSnapshot(t, paths.Directory), before) {
				t.Error("child hook changed parent state or witnesses")
			}
			retained, err := filepath.Glob(filepath.Join(fixture.run.team.Directory, "foreign-submits", "*.json"))
			if err != nil || (native == "UserPromptSubmit" && len(retained) != 1) || (native != "UserPromptSubmit" && len(retained) != 0) {
				t.Errorf("foreign payload retention: %v, %v", retained, err)
			}
		})
	}
}
