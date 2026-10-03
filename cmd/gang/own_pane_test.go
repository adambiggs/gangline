package main

import (
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestDropEndsOwnPaneByIdentityOrPane(t *testing.T) {
	a := core.Agent{ID: "hitch-lead", Pane: "%3"}
	for _, c := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"its identity", map[string]string{"GANG_AGENT_ID": "hitch-lead"}, true},
		{"its pane with the identity unset", map[string]string{"TMUX_PANE": "%3"}, true},
		{"another agent's pane", map[string]string{"GANG_AGENT_ID": "hitch-other", "TMUX_PANE": "%4"}, false},
		{"outside tmux", map[string]string{}, false},
	} {
		cmd := command{lookupEnv: func(key string) (string, bool) { value, ok := c.env[key]; return value, ok }}
		if got := cmd.dropEndsOwnPane(a); got != c.want {
			t.Errorf("%s: dropEndsOwnPane = %v, want %v", c.name, got, c.want)
		}
	}
	if (command{lookupEnv: func(string) (string, bool) { return "", false }}).dropEndsOwnPane(core.Agent{ID: "hitch-lead"}) {
		t.Error("an agent with no pane matched a caller outside tmux")
	}
}
