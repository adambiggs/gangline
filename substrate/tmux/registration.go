package tmux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/adambiggs/gangline/substrate"
)

const generationOption = "@gangline_generation"

// tmux packs all command arguments into a 16 KiB IPC message. Leave room for
// the if-shell guard and stream longer commands through source-file.
const maxInlineRegisteredCommandBytes = 8 * 1024

// ErrPaneReplaced means a saved pane identity no longer owns the live target.
var ErrPaneReplaced = errors.New("registered pane was replaced")

// PaneIdentity binds a pane to one tmux server lifetime and session.
type PaneIdentity struct {
	Generation string
	Session    string
	Pane       string
}

func (b *Backend) RegisterPane(ctx context.Context, pane substrate.PaneID) (PaneIdentity, error) {
	if !numericTmuxID(string(pane), '%') {
		return PaneIdentity{}, fmt.Errorf("invalid pane id %q", pane)
	}
	if id, ok := b.bornPane(pane); ok {
		exists, err := b.CheckPane(ctx, id)
		if err != nil {
			return PaneIdentity{}, err
		}
		if !exists {
			return PaneIdentity{}, fmt.Errorf("created pane %s is absent", pane)
		}
		return id, nil
	}
	token, err := randomGeneration()
	if err != nil {
		return PaneIdentity{}, err
	}
	out, err := b.run(ctx, "set-option", "-soq", generationOption, token)
	if err != nil {
		return PaneIdentity{}, tmuxError("initialize server generation", err, out)
	}
	id, exists, err := b.registeredPane(ctx, string(pane))
	if err != nil {
		return PaneIdentity{}, err
	}
	if !exists {
		return PaneIdentity{}, fmt.Errorf("pane %s is absent", pane)
	}
	if err := validPaneIdentity(id); err != nil {
		return PaneIdentity{}, err
	}
	return id, nil
}

func randomGeneration() (string, error) {
	var token [32]byte
	_, err := rand.Read(token[:])
	return hex.EncodeToString(token[:]), err
}

func (b *Backend) bornPane(pane substrate.PaneID) (PaneIdentity, bool) {
	b.birthMu.Lock()
	defer b.birthMu.Unlock()
	id, ok := b.birth[pane]
	return id, ok
}

// CheckPane distinguishes an absent pane from a reused or moved pane.
func (b *Backend) CheckPane(ctx context.Context, expected PaneIdentity) (bool, error) {
	if err := validPaneIdentity(expected); err != nil {
		return false, err
	}
	actual, exists, err := b.registeredPane(ctx, expected.Pane)
	if err != nil || !exists {
		return false, err
	}
	if actual != expected {
		return false, fmt.Errorf("%w: pane %s differs from its registered server or session", ErrPaneReplaced, expected.Pane)
	}
	return true, nil
}

func (b *Backend) registeredPane(ctx context.Context, pane string) (PaneIdentity, bool, error) {
	out, err := b.run(ctx, "list-panes", "-a", "-F", "#{"+generationOption+"}\t#{session_id}\t#{pane_id}")
	if err != nil {
		if absentTmuxServer(out) && ctx.Err() == nil {
			return PaneIdentity{}, false, nil
		}
		return PaneIdentity{}, false, tmuxError("read pane registration", err, out)
	}
	var matches []PaneIdentity
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return PaneIdentity{}, false, fmt.Errorf("invalid pane registration record %q", line)
		}
		if fields[2] == pane {
			matches = append(matches, PaneIdentity{fields[0], fields[1], fields[2]})
		}
	}
	if len(matches) == 0 {
		return PaneIdentity{}, false, nil
	}
	// '=' prevents tmux's usual session-name prefix matching.
	out, err = b.run(ctx, "display-message", "-p", "-t", "="+b.config.Session+":", "#{session_id}")
	if err != nil {
		if strings.HasPrefix(out, "can't find session:") {
			return PaneIdentity{}, false, fmt.Errorf("%w: configured session %q is absent", ErrPaneReplaced, b.config.Session)
		}
		return PaneIdentity{}, false, tmuxError("resolve registered session", err, out)
	}
	for _, id := range matches {
		if id.Session == strings.TrimSpace(out) {
			return id, true, nil
		}
	}
	return PaneIdentity{}, false, fmt.Errorf("%w: pane %s is outside configured session %q", ErrPaneReplaced, pane, b.config.Session)
}

// RemoveRegisteredPane checks the identity again in the same tmux command queue
// that performs the removal. A replacement server cannot inherit the check.
func (b *Backend) RemoveRegisteredPane(ctx context.Context, id PaneIdentity) error {
	return b.mutateRegisteredPane(ctx, id, "kill-pane -t "+id.Pane, "", 0, true)
}

// RemoveRegisteredNativePane retains native replacement protection alongside
// the server generation guard when the saved process namespace is visible.
func (b *Backend) RemoveRegisteredNativePane(ctx context.Context, id PaneIdentity, expected Identity) error {
	if !CanReadIdentity(expected) {
		return fmt.Errorf("registered process namespace is not visible")
	}
	boot, err := bootIdentity()
	if err != nil {
		return err
	}
	if expected.BootID != boot {
		return fmt.Errorf("refuse removal: registered process boot changed")
	}
	r, err := readCurrentProcess(expected.PID)
	if err == nil && !sameIdentity(expected, r) {
		return fmt.Errorf("refuse removal: pane process identity changed")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return b.mutateRegisteredPane(ctx, id, "kill-pane -t "+id.Pane, "", expected.PID, true)
}

// SendRegisteredKeys keeps the final identity check and typing in one tmux
// command queue, including when a server restarts between observation and send.
func (b *Backend) SendRegisteredKeys(ctx context.Context, id PaneIdentity, command string, keys substrate.Keys) error {
	if command == "" || strings.ContainsFunc(command, func(c rune) bool {
		return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._+-", c))
	}) {
		return fmt.Errorf("invalid foreground command basename %q", command)
	}
	var commands []string
	if keys.Text != "" {
		commands = append(commands, tmuxCommand("send-keys", "-t", id.Pane, "-l", "--", keys.Text))
	}
	names := append([]string(nil), keys.Names...)
	if keys.Submit {
		names = append(names, "Enter")
	}
	if len(names) != 0 {
		commands = append(commands, tmuxCommand("send-keys", append([]string{"-t", id.Pane, "--"}, names...)...))
	}
	if len(commands) == 0 {
		_, err := b.CheckPane(ctx, id)
		return err
	}
	return b.mutateRegisteredPane(ctx, id, strings.Join(commands, " ; "), command, 0, false)
}

// tmux parses if-shell branches as command strings, not shell scripts. Literal
// newlines in shell-quoted arguments lose indentation and join escaped lines.
// Encode controls so the parser sees one line and reconstructs the exact bytes.
func tmuxCommand(command string, arguments ...string) string {
	var out strings.Builder
	for i, word := range append([]string{command}, arguments...) {
		if i != 0 {
			out.WriteByte(' ')
		}
		out.WriteByte('"')
		for j := 0; j < len(word); j++ {
			c := word[j]
			switch {
			case c < ' ' || c == 127:
				fmt.Fprintf(&out, "\\%03o", c)
			case c == '\\' || c == '"' || c == '$' || c == '~':
				out.WriteByte('\\')
				out.WriteByte(c)
			default:
				out.WriteByte(c)
			}
		}
		out.WriteByte('"')
	}
	return out.String()
}

func (b *Backend) mutateRegisteredPane(ctx context.Context, id PaneIdentity, command, foreground string, nativePID int, absentOK bool) error {
	exists, err := b.CheckPane(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		if absentOK {
			return nil
		}
		return fmt.Errorf("registered pane %s is absent", id.Pane)
	}
	condition := fmt.Sprintf("#{&&:#{==:#{%s},%s},#{&&:#{==:#{session_id},%s},#{==:#{pane_id},%s}}}", generationOption, id.Generation, id.Session, id.Pane)
	if nativePID != 0 {
		condition = fmt.Sprintf("#{&&:%s,#{==:#{pane_pid},%d}}", condition, nativePID)
	}
	if foreground != "" {
		condition = fmt.Sprintf("#{&&:%s,#{==:#{pane_current_command},%s}}", condition, foreground)
	}
	fallback := "display-message -p 'registered pane identity changed'"
	var out string
	if len(command) > maxInlineRegisteredCommandBytes {
		script := tmuxCommand("if-shell", "-F", "-t", id.Pane, condition, command, fallback) + "\n"
		out, err = b.runWithInput(ctx, strings.NewReader(script), "source-file", "-")
	} else {
		out, err = b.run(ctx, "if-shell", "-F", "-t", id.Pane, condition, command, fallback)
	}
	if err != nil {
		if exists, checkErr := b.CheckPane(ctx, id); absentOK && checkErr == nil && !exists {
			return nil
		}
		return tmuxError("mutate registered pane", err, out)
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("mutate registered pane: %s", strings.TrimSpace(out))
	}
	return nil
}

// KillUnregisteredPane cleans up a newly created pane after registration failed.
// Its witness comes from the command queue that created the pane; rediscovering
// a witness here could authorize removal of a replacement server's pane.
func (b *Backend) KillUnregisteredPane(ctx context.Context, pane substrate.PaneID) error {
	if !numericTmuxID(string(pane), '%') {
		return fmt.Errorf("invalid pane id %q", pane)
	}
	id, ok := b.bornPane(pane)
	if !ok {
		return fmt.Errorf("created pane %s has no saved birth identity; cleanup is unverified", pane)
	}
	return b.RemoveRegisteredPane(ctx, id)
}

func numericTmuxID(value string, prefix byte) bool {
	if len(value) < 2 || value[0] != prefix {
		return false
	}
	for _, c := range value[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func validPaneIdentity(id PaneIdentity) error {
	decoded, err := hex.DecodeString(id.Generation)
	if err != nil || len(decoded) != 32 || !numericTmuxID(id.Session, '$') || !numericTmuxID(id.Pane, '%') {
		return fmt.Errorf("invalid registered pane identity")
	}
	return nil
}

func absentTmuxServer(output string) bool {
	return strings.HasPrefix(output, "no server running on ") ||
		(strings.HasPrefix(output, "error connecting to ") &&
			(strings.Contains(output, "No such file or directory") || strings.Contains(output, "Connection refused")))
}
