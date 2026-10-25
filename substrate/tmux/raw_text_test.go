package tmux

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
)

func TestRawTextArgumentsPreserveBytes(t *testing.T) {
	for _, text := range []string{"", "plain text", "\x1b[200~first\n\n\té猫\x1b[201~", "\x00\r\x01\x7f"} {
		arguments := rawTextArguments("%1", text)
		if !reflect.DeepEqual(arguments[:4], []string{"-t", "%1", "-H", "--"}) {
			t.Fatalf("raw byte mode: %q", arguments[:4])
		}
		decoded, err := hex.DecodeString(strings.Join(arguments[4:], ""))
		if err != nil || string(decoded) != text {
			t.Fatalf("raw text = %q, want %q: %v", decoded, text, err)
		}
	}
}

func TestRawTextCommandsFitTheInlineBudget(t *testing.T) {
	commands := rawTextCommands("%1", strings.Repeat("x", maxInlineRegisteredCommandBytes))
	if len(commands) < 2 {
		t.Fatal("large raw text was not split into commands")
	}
	for _, command := range commands {
		if len(tmuxCommand("if-shell", command)) > maxInlineRegisteredCommandBytes {
			t.Fatal("quoted raw text command exceeds the inline budget")
		}
	}
}
