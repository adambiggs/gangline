package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestNamedPaneLayouts(t *testing.T) {
	for _, split := range []bool{true, false} {
		t.Run(fmt.Sprintf("split=%v", split), func(t *testing.T) {
			root, err := os.MkdirTemp(os.TempDir(), "layout-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(root) })
			repo, err := filepath.Abs("..")
			if err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(root, "gang")
			build := exec.Command("go", "build", "-o", binary, "./cmd/gang")
			build.Dir = repo
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			collars := filepath.Join(root, "collars")
			if err := os.Mkdir(collars, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(collars, "acceptance.cue"), []byte(commandAcceptanceCollar(exe)), 0600); err != nil {
				t.Fatal(err)
			}
			const session = "named-pane-layout"
			socket := filepath.Join(root, "tmux.sock")
			state := filepath.Join(root, "state")
			env := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
				"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_STATE_ROOT="+state, "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance", "GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANGLINE_ACCEPTANCE_SCREEN_LABEL=1", "GANGLINE_ACCEPTANCE_TMUX=tmux", "GANGLINE_ACCEPTANCE_TMUX_SOCKET="+socket, "GANGLINE_ACCEPTANCE_LEDGER="+filepath.Join(root, "received"), "GANGLINE_ACCEPTANCE_ARGV_LEDGER="+filepath.Join(root, "argv"))
			runner := tmuxRunner{binary: "tmux", socket: socket, env: env}
			gang := func(args ...string) string {
				t.Helper()
				cmd := exec.Command(binary, args...)
				cmd.Dir, cmd.Env = repo, env
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("gang %v: %v\n%s", args, err, out)
				}
				return string(out)
			}
			if got := gang("hitch", "lead", "--role", "lead"); got != "lead\n" {
				t.Fatalf("hitch output = %q", got)
			}
			t.Cleanup(func() { gang("roster"); gang("down", "--yes") })
			if out, err := runner.run("list-sessions", "-F", "#{session_name}"); err != nil || strings.TrimSpace(out) != session {
				t.Fatalf("private server contains other sessions: %q %v", out, err)
			}
			// The fixture draws a native composer once; keep it wide enough after
			// splits, as on a large monitor, without a resize redraw.
			if out, err := runner.run("resize-window", "-t", session, "-x", "240", "-y", "60"); err != nil {
				t.Fatalf("viewport: %v %s", err, out)
			}
			for i, name := range []string{"worker", "peer"} {
				args := []string{"hitch", name}
				if split {
					target := "lead"
					if i == 1 {
						target = "worker"
					}
					args = append(args, "--split", target)
					if i == 1 {
						args = append(args, "--vertical")
					}
				}
				if got := gang(args...); got != name+"\n" {
					t.Fatalf("hitch output = %q", got)
				}
			}
			out, err := runner.run("list-windows", "-t", session, "-F", "#{window_id}")
			if err != nil || len(strings.Fields(out)) != map[bool]int{true: 1, false: 3}[split] {
				t.Fatalf("window layout = %q %v", out, err)
			}
			team, _ := (store.Paths{Root: state}).Team(session)
			agents, err := team.ListAgents()
			if err != nil || len(agents) != 3 {
				t.Fatalf("agents = %+v %v", agents, err)
			}
			byName := map[string]core.Agent{}
			for _, a := range agents {
				byName[string(a.Name)] = a
				if got := gang("send", string(a.Name), "--from", "operator", "target "+string(a.Name)); !strings.Contains(got, "\tdelivered\n") {
					t.Fatalf("delivery was not confirmed: %q", got)
				}
				if out, err := runner.run("wait-for", fmt.Sprintf("received-%s-2", a.ID)); err != nil {
					t.Fatalf("delivery barrier: %v %s", err, out)
				}
				captured := gang("capture", string(a.Name))
				id := string(a.ID)
				if !strings.Contains(captured, "READY "+id[len(id)-8:]+" literal %1 $100 @2") {
					t.Fatalf("capture %s reached another pane: %q", a.Name, captured)
				}
				if out, err := runner.run("display-message", "-p", "-t", a.Pane, "#{@gangline_title}"); err != nil || !strings.Contains(out, string(a.Name)) {
					t.Fatalf("independent title %s = %q %v", a.Name, out, err)
				}
				p, _ := team.Agent(a.ID)
				current, err := p.Read()
				if err != nil || current.LastDelivered == "" {
					t.Fatalf("delivery %s has no receipt: %+v %v", a.Name, current, err)
				}
				e, err := p.ReadEnvelope("cur", current.LastDelivered)
				if err != nil || e.Outcome != "delivered" || e.Message.Text != "target "+string(a.Name) {
					t.Fatalf("delivery %s = %+v %v", a.Name, e, err)
				}
			}
			if split {
				positions := map[string][]string{}
				for name, a := range byName {
					out, err := runner.run("display-message", "-p", "-t", a.Pane, "#{pane_left},#{pane_top}")
					if err != nil {
						t.Fatal(err)
					}
					positions[name] = strings.Split(strings.TrimSpace(out), ",")
				}
				if positions["lead"][0] == positions["worker"][0] || positions["worker"][0] != positions["peer"][0] || positions["worker"][1] == positions["peer"][1] {
					t.Fatalf("split orientations = %+v", positions)
				}
			}
			gang("rename", "worker", "renamed")
			if captured := gang("capture", "renamed"); !strings.Contains(captured, "READY") {
				t.Fatal("renamed agent lost its pane")
			}
			gang("drop", "renamed")
			for _, name := range []string{"lead", "peer"} {
				if captured := gang("capture", name); !strings.Contains(captured, "READY") {
					t.Fatalf("dropping a sibling lost %s", name)
				}
				if out, err := runner.run("display-message", "-p", "-t", byName[name].Pane, "#{@gangline_title}"); err != nil || !strings.Contains(out, name) {
					t.Fatalf("sibling title %s = %q %v", name, out, err)
				}
			}
		})
	}
}
