package harness

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestCodexComposerIgnoresGhostText(t *testing.T) {
	screen := testScreen(
		testRow("reply", false),
		append(testCells("› hello", false), testCells(" placeholder", true)...),
	)
	composer, err := ReadComposer(Invocation{Name: "codex-composer"}, screen)
	if err != nil {
		t.Fatal(err)
	}
	if composer.Text != "hello" {
		t.Fatalf("composer = %q, want hello", composer.Text)
	}
}

func TestCodexMenuIsNotComposer(t *testing.T) {
	screen := testScreen(testRow("› 1. Review hooks", false))
	_, err := ReadComposer(Invocation{Name: "codex-composer"}, screen)
	if !errors.Is(err, ErrComposerOccupied) {
		t.Fatalf("error = %v, want occupied", err)
	}
}

func TestClaudeComposerReadsFramedMultilineBody(t *testing.T) {
	screen := testScreen(
		testRow("────────", false),
		append(testCells("❯ first", false), testCells(" ghost", true)...),
		testRow("second", false),
		testRow("────────", false),
		testRow("auto mode on", false),
	)
	composer, err := ReadComposer(Invocation{Name: "claude-composer"}, screen)
	if err != nil {
		t.Fatal(err)
	}
	if composer.Text != "first\nsecond" {
		t.Fatalf("composer = %q", composer.Text)
	}
}

func TestClaudeOverlayOwnsInput(t *testing.T) {
	screen := testScreen(
		testRow("▔▔▔▔", false),
		testRow("Choose a mode", false),
		testRow("────────", false),
		testRow("❯ draft", false),
		testRow("────────", false),
		testRow("Esc to cancel", false),
	)
	_, err := ReadComposer(Invocation{Name: "claude-composer"}, screen)
	if !errors.Is(err, ErrComposerOccupied) {
		t.Fatalf("error = %v, want occupied", err)
	}
}

func TestInspectStartupFindsTrustPrompt(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	screen := testScreen(
		testRow("Hooks need review", false),
		testRow("› 1. Review hooks", false),
	)
	startup, err := InspectStartup(collar, screen)
	if err != nil {
		t.Fatal(err)
	}
	if startup.State != StartupTrustRequired {
		t.Fatalf("startup state = %q", startup.State)
	}
}

func TestInspectStartupFindsClaudeExternalImportTrust(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	screen := testScreen(
		testRow("Only use Claude Code with files you trust.", false),
		testRow("❯ 1. Yes, allow external imports", false),
	)
	startup, err := InspectStartup(collar, screen)
	if err != nil {
		t.Fatal(err)
	}
	if startup.State != StartupTrustRequired {
		t.Fatalf("startup state = %q", startup.State)
	}
}

func TestReadContext(t *testing.T) {
	reading, err := ReadContext(
		Invocation{Name: "claude-screen-context"},
		testScreen(testRow("ctx 42k/200k 21%", false)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if reading.Used != 42_000 || reading.Limit != 200_000 || reading.Percent != 0.21 {
		t.Fatalf("reading = %+v", reading)
	}
}

func TestClaudeComposerAgainstLegacyFixtures(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
		err  error
	}{
		{name: "named parent", file: "claude-named-composer-parent.txt", want: ""},
		{name: "selected child", file: "claude-selected-subagent.txt", err: ErrForeignComposer},
		{name: "child cursor on main", file: "claude-subagent-cursor-on-main.txt", err: ErrForeignComposer},
		{name: "background sessions", file: "claude-background-sessions.txt", err: ErrBackgroundComposer},
		{name: "overlay", file: "claude-auto-mode-environment.txt", err: ErrComposerOccupied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			composer, err := ReadComposer(Invocation{Name: "claude-composer"}, fixtureScreen(t, test.file))
			if test.err != nil {
				if !errors.Is(err, test.err) {
					t.Fatalf("error = %v, want %v", err, test.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if composer.Text != test.want {
				t.Fatalf("text = %q, want %q", composer.Text, test.want)
			}
		})
	}
}

func fixtureScreen(t *testing.T, name string) substrate.Screen {
	t.Helper()
	data, err := os.ReadFile("../test/fixtures/" + name)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	rows := make([][]substrate.Cell, len(lines))
	for index, line := range lines {
		rows[index] = testRow(line, false)
	}
	return testScreen(rows...)
}

func testScreen(rows ...[]substrate.Cell) substrate.Screen {
	return substrate.Screen{Rows: rows}
}

func testRow(text string, dim bool) []substrate.Cell {
	return testCells(text, dim)
}

func testCells(text string, dim bool) []substrate.Cell {
	cells := make([]substrate.Cell, 0, len([]rune(text)))
	for _, char := range text {
		cells = append(cells, substrate.Cell{Text: string(char), Attributes: substrate.Attributes{Dim: dim}})
	}
	return cells
}
