package tmux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
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

// PaneClosed reports whether the pane is closed for good: the server that
// registered it still runs and no longer has it, or that server's process is
// witnessed gone. tmux does not reuse a pane id within a server lifetime. An
// unreachable or different server proves nothing while the registering one may
// still run, and neither does a server with no session, whose pane listing
// names no generation, or a pane outside the configured session.
func (b *Backend) PaneClosed(ctx context.Context, id PaneIdentity, server Identity) (bool, error) {
	if err := validPaneIdentity(id); err != nil {
		return false, err
	}
	out, err := b.listPanes(ctx, "#{"+generationOption+"}\t#{pane_id}")
	if err != nil {
		if (absentTmuxServer(out) || emptyTmuxServer(out)) && ctx.Err() == nil {
			return serverExited(server)
		}
		return false, tmuxError("read pane registration", err, out)
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			return false, fmt.Errorf("invalid pane registration record %q", line)
		}
		if fields[0] != id.Generation {
			return serverExited(server)
		}
		if fields[1] == id.Pane {
			return false, nil
		}
	}
	return true, nil
}

// ServerIdentity witnesses the process of the tmux server that registered the
// pane, which the server generation names. It is zero when this caller cannot
// read that server's process.
func (b *Backend) ServerIdentity(ctx context.Context, id PaneIdentity) (Identity, error) {
	if err := validPaneIdentity(id); err != nil {
		return Identity{}, err
	}
	read := func() (string, int, error) {
		out, err := b.run(ctx, "display-message", "-p", "#{"+generationOption+"}\t#{socket_path}\t#{pid}")
		if err != nil {
			return "", 0, tmuxError("read tmux server process", err, out)
		}
		fields := strings.Split(strings.TrimSuffix(out, "\n"), "\t")
		if len(fields) != 3 || fields[0] != id.Generation || fields[1] == "" {
			return "", 0, fmt.Errorf("read tmux server process: %w: the server is not the one that registered pane %s", ErrPaneReplaced, id.Pane)
		}
		pid, err := strconv.Atoi(fields[2])
		if err != nil || pid <= 0 {
			return "", 0, fmt.Errorf("invalid tmux server pid %q", fields[2])
		}
		return fields[1], pid, nil
	}
	socket, pid, err := read()
	if err != nil {
		return Identity{}, err
	}
	visible, err := serverProcessVisible(ctx, socket, pid)
	if err != nil || !visible {
		return Identity{}, err
	}
	r, err := readServerProcess(pid, read, readCurrentProcess)
	if err != nil {
		return Identity{}, err
	}
	boot, err := bootIdentity()
	if err != nil {
		return Identity{}, err
	}
	namespace, err := nativeProcessNamespace()
	if err != nil {
		return Identity{}, err
	}
	return Identity{PID: r.PID, Started: r.started, Version: r.version, UniqueID: r.uniqueID, BootID: boot, Namespace: namespace}, nil
}

// readServerProcess reads the server's record between two of its answers. The
// same server answering again held the PID throughout, so the record is its
// own. A record that vanished is a server that exited, and the second answer
// reports that exit.
func readServerProcess(pid int, answer func() (string, int, error), read func(int) (processRecord, error)) (processRecord, error) {
	r, readErr := read(pid)
	if readErr != nil && !processGone(readErr) {
		return processRecord{}, readErr
	}
	if _, again, err := answer(); err != nil {
		return processRecord{}, err
	} else if again != pid {
		return processRecord{}, fmt.Errorf("read tmux server process: %w: the server changed during the read", ErrPaneReplaced)
	}
	return r, readErr
}

// serverExited reports whether the witnessed server process is gone. Without a
// witness this caller can read, or with one from another boot, it is unknown
// and reads as not exited.
func serverExited(server Identity) (bool, error) {
	if !CanReadIdentity(server) {
		return false, nil
	}
	boot, err := bootIdentity()
	if err != nil {
		return false, err
	}
	if server.BootID != boot {
		return false, nil
	}
	r, err := readCurrentProcess(server.PID)
	if processGone(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !sameIdentity(server, r), nil
}

// listPanes reads every pane of the server. A client whose server exits while
// it is connected reports only that it lost the server, which says nothing of
// the panes, so that read is made once more: it finds the server absent, or
// finds its successor.
func (b *Backend) listPanes(ctx context.Context, format string) (string, error) {
	out, err := b.run(ctx, "list-panes", "-a", "-F", format)
	if err != nil && lostTmuxServer(out) && ctx.Err() == nil {
		out, err = b.run(ctx, "list-panes", "-a", "-F", format)
	}
	return out, err
}

func (b *Backend) registeredPane(ctx context.Context, pane string) (PaneIdentity, bool, error) {
	// One listing reads the pane and its sessions together: a session that
	// ends between two tmux commands would leave the second to misread it.
	out, err := b.listPanes(ctx, "#{"+generationOption+"}\t#{session_id}\t#{pane_id}\t#{session_name}")
	if err != nil {
		if (absentTmuxServer(out) || emptyTmuxServer(out)) && ctx.Err() == nil {
			return PaneIdentity{}, false, nil
		}
		return PaneIdentity{}, false, tmuxError("read pane registration", err, out)
	}
	found, configured := false, false
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return PaneIdentity{}, false, fmt.Errorf("invalid pane registration record %q", line)
		}
		inSession := fields[3] == b.config.Session
		configured = configured || inSession
		if fields[2] != pane {
			continue
		}
		found = true
		if inSession {
			return PaneIdentity{Generation: fields[0], Session: fields[1], Pane: fields[2]}, true, nil
		}
	}
	if !found {
		return PaneIdentity{}, false, nil
	}
	if !configured {
		return PaneIdentity{}, false, fmt.Errorf("%w: configured session %q is absent", ErrPaneReplaced, b.config.Session)
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
// A process recorded under an earlier boot ended with that boot, and its PID
// now names whatever holds it on this one, so it is never read. The pane went
// with its server: the generation guard finds it absent and removes nothing.
func (b *Backend) RemoveRegisteredNativePane(ctx context.Context, id PaneIdentity, expected Identity) error {
	if !CanReadIdentity(expected) {
		return fmt.Errorf("registered process namespace is not visible")
	}
	boot, err := bootIdentity()
	if err != nil {
		return err
	}
	if expected.BootID == boot {
		r, err := readCurrentProcess(expected.PID)
		if err == nil && !sameIdentity(expected, r) {
			return fmt.Errorf("refuse removal: pane process identity changed")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return b.mutateRegisteredPane(ctx, id, "kill-pane -t "+id.Pane, "", expected.PID, true)
}

// EarlierBoot reports whether the identity was recorded under an earlier boot
// of this host, which ended every process it records.
func EarlierBoot(expected Identity) (bool, error) {
	if expected.BootID == "" {
		return false, nil
	}
	boot, err := bootIdentity()
	return err == nil && expected.BootID != boot, err
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
	// Capture reads the application screen under both scrollback and command
	// output viewers. Cancel either viewer before typing into the application.
	cancelCopy := tmuxCommand("if-shell", "-F", "-t", id.Pane, viewingMode, tmuxCommand("send-keys", "-t", id.Pane, "-X", "cancel"))
	commands = append([]string{cancelCopy}, commands...)
	return b.mutateRegisteredPane(ctx, id, strings.Join(commands, " ; "), command, 0, false)
}

const viewingMode = "#{||:#{==:#{pane_mode},copy-mode},#{==:#{pane_mode},view-mode}}"

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

// registeredCondition holds for the registered pane while it belongs to its
// registered session. A window can belong to several sessions, and a pane
// target resolves to whichever of them tmux prefers, so membership is read by
// walking the registered session's panes rather than from the target's session.
func registeredCondition(id PaneIdentity) string {
	member := fmt.Sprintf("#{S:#{?#{==:#{session_id},%s},#{W:#{P:#{?#{==:#{pane_id},%s},1,}}},}}", id.Session, id.Pane)
	return fmt.Sprintf("#{&&:#{==:#{%s},%s},#{&&:%s,#{==:#{pane_id},%s}}}", generationOption, id.Generation, member, id.Pane)
}

// ReleaseRegisteredExit is ReleaseExit for a registered pane. The identity
// check runs in the same command queue as the release, so a pane that reuses
// the id on another server keeps its own remain-on-exit setting.
func (b *Backend) ReleaseRegisteredExit(ctx context.Context, id PaneIdentity) error {
	if err := validPaneIdentity(id); err != nil {
		return err
	}
	release := strings.Join([]string{
		tmuxCommand("display-message", "-p", "-t", id.Pane, "#{pane_dead},#{pane_dead_status}"),
		tmuxCommand("capture-pane", "-p", "-J", "-S", "-", "-t", id.Pane),
		tmuxCommand("set-option", "-p", "-u", "-t", id.Pane, "remain-on-exit"),
	}, " ; ")
	// The reap comes first so an exit already made reports its status.
	out, err := b.reaped(ctx, "if-shell", "-F", "-t", id.Pane, registeredCondition(id), release, "display-message -p '"+ErrPaneReplaced.Error()+"'")
	if err != nil {
		return tmuxError("release exited pane", err, out)
	}
	if strings.TrimSpace(out) == ErrPaneReplaced.Error() {
		return fmt.Errorf("release exited pane: %w", ErrPaneReplaced)
	}
	return releasedExit(out)
}

// TitleRegisteredPane displays a registered agent's name and status on its
// own border. Native terminal titles and sibling borders remain independent.
func (b *Backend) TitleRegisteredPane(ctx context.Context, id PaneIdentity, name string) error {
	if err := validPaneTitle(name); err != nil {
		return err
	}
	exists, err := b.CheckPane(ctx, id)
	if errors.Is(err, ErrPaneReplaced) || err == nil && !exists {
		return nil
	}
	if err != nil {
		return err
	}
	command := tmuxCommand("set-option", "-p", "-t", id.Pane, "@gangline_title", name) + " ; " +
		tmuxCommand("set-option", "-w", "-t", id.Pane, "pane-border-status", "top") + " ; " +
		tmuxCommand("set-option", "-w", "-t", id.Pane, "pane-border-format", "#{?@gangline_title,#{@gangline_title},#{pane_title}}")
	for _, option := range []string{"window-status-format", "window-status-current-format"} {
		format, err := b.run(ctx, "show-options", "-A", "-w", "-v", "-t", id.Pane, option)
		if err != nil {
			return tmuxError("read window status format", err, format)
		}
		format = strings.TrimSuffix(format, "\n")
		if updated := paneStatusFormat(format); updated != format {
			command += " ; " + tmuxCommand("set-option", "-w", "-t", id.Pane, option, updated)
		}
	}
	err = b.mutateRegisteredPane(ctx, id, command, "", 0, true)
	if errors.Is(err, ErrPaneReplaced) {
		return nil
	}
	return err
}

// Window status formats expand in the active pane's context. Keep the
// placement label and operator styling while displaying that pane's status.
func paneStatusFormat(format string) string {
	const title = "#{?@gangline_title,#{@gangline_title},#{window_name}}"
	var out strings.Builder
	for len(format) > 0 {
		switch {
		case strings.HasPrefix(format, "#"+title):
			out.WriteString("#" + title)
			format = format[len(title)+1:]
		case strings.HasPrefix(format, "##"):
			out.WriteString("##")
			format = format[2:]
		case strings.HasPrefix(format, title):
			out.WriteString(title)
			format = format[len(title):]
		case strings.HasPrefix(format, "#W"):
			out.WriteString(title)
			format = format[2:]
		case strings.HasPrefix(format, "#{window_name}"):
			out.WriteString(title)
			format = format[len("#{window_name}"):]
		default:
			out.WriteByte(format[0])
			format = format[1:]
		}
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
	condition := registeredCondition(id)
	if nativePID != 0 {
		condition = fmt.Sprintf("#{&&:%s,#{==:#{pane_pid},%d}}", condition, nativePID)
	}
	if foreground != "" {
		condition = fmt.Sprintf("#{&&:%s,#{==:#{pane_current_command},%s}}", condition, foreground)
		condition = fmt.Sprintf("#{&&:%s,#{||:#{==:#{pane_in_mode},0},#{&&:#{==:#{pane_in_mode},1},%s}}}", condition, viewingMode)
	}
	fallback := "display-message -p -t " + id.Pane + " 'registered pane identity changed or input mode is unsupported (pane=#{pane_id}, session=#{session_id}, foreground=#{pane_current_command}, modes=#{pane_in_mode}, mode=#{pane_mode})'"
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

// lostTmuxServer matches a client that connected to a server which then exited
// without answering.
func lostTmuxServer(output string) bool {
	return strings.HasPrefix(output, "server exited unexpectedly")
}

// emptyTmuxServer matches a read of every pane answered by a server with no
// session, which has no pane. A server ends its last session before it exits.
func emptyTmuxServer(output string) bool {
	return strings.HasPrefix(output, "no current target")
}

func absentTmuxServer(output string) bool {
	return strings.HasPrefix(output, "no server running on ") ||
		(strings.HasPrefix(output, "error connecting to ") &&
			(strings.Contains(output, "No such file or directory") || strings.Contains(output, "Connection refused")))
}
