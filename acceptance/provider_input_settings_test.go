package acceptance

import (
	"reflect"
	"testing"
)

func TestLiveClaudeInputSettingsUseSupportedOptions(t *testing.T) {
	base := [][]string{{"set-option", "-s", "extended-keys", "on"}, {"set-option", "-g", "default-terminal", "tmux-256color"}}
	for _, tc := range []struct {
		name, options string
		format        bool
	}{
		{name: "without format option", options: "extended-keys off\nescape-time 500\n"},
		{name: "with format option", options: "extended-keys off\nextended-keys-format csi-u\n", format: true},
		{name: "similar option name", options: "extended-keys-format-other xterm\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := [][]string{base[0]}
			if tc.format {
				want = append(want, []string{"set-option", "-s", "extended-keys-format", "xterm"})
			}
			want = append(want, base[1])
			if got := liveClaudeInputSettings(tc.options); !reflect.DeepEqual(got, want) {
				t.Fatalf("settings=%q, want %q", got, want)
			}
		})
	}
}
