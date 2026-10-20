package harness

import (
	"strings"
	"testing"
)

func TestCodexComputerUseMenu(t *testing.T) {
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	// Screen text captured in issue #197.
	menu := fixtureLines(t, "codex-computer-use-permission.txt")
	cancelSelected := append([]string(nil), menu...)
	cancelSelected[5] = strings.Replace(cancelSelected[5], "›", " ", 1)
	cancelSelected[10] = strings.Replace(cancelSelected[10], "  4.", "› 4.", 1)
	for _, tc := range []struct {
		name  string
		lines []string
		want  bool
	}{
		{"captured menu", menu, true},
		{"cancel selected", cancelSelected, true},
		{"title only", menu[:4], false},
		{"choices only", menu[5:], false},
		{"unnumbered choice", []string{menu[1], "Allow"}, false},
		{"reply above a picker", append(append([]string{"• The permission menu reads:"}, menu...), "", "", "› 1. Model selection"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocked, found, err := InputBlocked(c, linesScreen(tc.lines))
			if err != nil || found != tc.want {
				t.Fatalf("blocked=%+v found=%v err=%v", blocked, found, err)
			}
			if found && !blockedEvidenceShape.MatchString(blocked.Evidence) {
				t.Fatalf("evidence=%q", blocked.Evidence)
			}
		})
	}

	lines := append(append([]string(nil), menu...), "", "› ")
	if blocked, found, err := InputBlocked(c, codexComposerScreen(lines)); err != nil || found {
		t.Fatalf("menu quoted above composer: blocked=%+v found=%v err=%v", blocked, found, err)
	}
}
