//go:build acceptance

package acceptance

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exercise the real provider setup and teardown with synthetic CLIs. No native
// provider, authentication service, tmux server, or installed gang is invoked.
func TestProviderFixtureIsolation(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, state := range []string{"home", "xdg"} {
			for _, mode := range []string{"cleanup", "prompt", "cleanup-failure"} {
				t.Run(provider+"/"+state+"/"+mode, func(t *testing.T) {
					root := t.TempDir()
					home := filepath.Join(root, "home")
					inherited := filepath.Join(root, "inherited-state")
					temp := filepath.Join(root, "tmp")
					shims := filepath.Join(root, "shims")
					xdg := ""
					defaultState := filepath.Join(home, ".local", "state", "gangline")
					if state == "xdg" {
						xdg = filepath.Join(root, "xdg")
						defaultState = filepath.Join(xdg, "gangline")
					}
					for _, dir := range []string{filepath.Join(home, ".codex"), defaultState, inherited, temp, shims} {
						if err := os.MkdirAll(dir, 0700); err != nil {
							t.Fatal(err)
						}
					}
					write := func(path, content string) {
						t.Helper()
						if err := os.WriteFile(path, []byte(content), 0700); err != nil {
							t.Fatal(err)
						}
					}
					write(filepath.Join(defaultState, "live-sentinel"), "preserve default state")
					write(filepath.Join(inherited, "live-sentinel"), "preserve inherited state")
					write(filepath.Join(home, ".codex", "auth.json"), "synthetic auth")
					write(filepath.Join(home, ".codex", "config.toml"), "[projects.'/synthetic/checkout']\ntrust_level = 'trusted'\n")
					write(filepath.Join(shims, provider), "#!/bin/sh\nprintf '{\"loggedIn\":true}\\n'\n")
					write(filepath.Join(shims, "git"), "#!/bin/sh\nprintf '/synthetic/checkout/.git\\n'\n")
					write(filepath.Join(shims, "go"), `#!/bin/sh
[ "$1 $2" = 'build -o' ] || exit 91
printf '%s\n' "$3" > "$FIXTURE/binary"
cp "$FIXTURE/gang" "$3"
`)
					write(filepath.Join(shims, "tmux"), `#!/bin/sh
printf '%s\n' "$GANG_STATE_ROOT" > "$FIXTURE/state-root"
printf '%s\n' "$GANG_TMUX_SOCKET" > "$FIXTURE/socket"
printf '%s\n' "$CODEX_HOME" > "$FIXTURE/profile"
shift 5
case "$1" in
 list-sessions) printf '%s\n' "$GANG_SESSION";;
 show-options) printf 'extended-keys-format xterm\n';;
esac
`)
					write(filepath.Join(root, "gang"), `#!/bin/sh
mkdir -p "$GANG_STATE_ROOT/teams/$GANG_SESSION"
printf 'synthetic team\n' > "$GANG_STATE_ROOT/teams/$GANG_SESSION/fixture"
case "$1" in
 models)
  if [ "$FIXTURE_MODE" != prompt ]; then printf 'synthetic model failure\n' >&2; exit 42; fi
  printf 'sonnet\tlow\ngpt-5.6-luna\tlow\n';;
 hitch) printf 'synthetic trust prompt\n' >&2; exit 42;;
 status|capture) printf 'synthetic trust prompt\n';;
 down)
  if [ "$FIXTURE_MODE" = cleanup-failure ]; then printf 'synthetic cleanup failure\n' >&2; exit 42; fi;;
esac
`)
					beforeDefault := providerFixtureSnapshot(t, defaultState)
					beforeInherited := providerFixtureSnapshot(t, inherited)
					beforeHome := providerFixtureSnapshot(t, filepath.Join(home, ".codex"))
					exe, err := os.Executable()
					if err != nil {
						t.Fatal(err)
					}
					testName := "TestLiveClaudeCode"
					if provider == "codex" {
						testName = "TestLiveCodex"
					}
					command := exec.Command(exe, "-test.v", "-test.run=^"+testName+"$")
					command.Env = append(withoutEnvironment(os.Environ(), "PATH", "HOME", "XDG_STATE_HOME", "GANG_STATE_ROOT", "TMPDIR", "GANGLINE_LIVE_PROVIDERS", "FIXTURE", "FIXTURE_MODE"),
						"PATH="+shims+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+home, "XDG_STATE_HOME="+xdg, "GANG_STATE_ROOT="+inherited, "TMPDIR="+temp,
						"GANGLINE_LIVE_PROVIDERS=1", "FIXTURE="+root, "FIXTURE_MODE="+mode)
					output, err := command.CombinedOutput()
					wantFailure := "synthetic model failure"
					if mode == "prompt" {
						wantFailure = "native prompt needs operator"
					}
					if mode == "cleanup-failure" {
						wantFailure = "synthetic cleanup failure"
					}
					if err == nil || !strings.Contains(string(output), wantFailure) {
						t.Fatalf("provider did not reach synthetic %s: %v\n%s", mode, err, output)
					}
					for _, pair := range []struct {
						path   string
						before map[string]string
					}{
						{defaultState, beforeDefault}, {inherited, beforeInherited}, {filepath.Join(home, ".codex"), beforeHome},
					} {
						if after := providerFixtureSnapshot(t, pair.path); !reflect.DeepEqual(after, pair.before) {
							t.Errorf("provider changed ambient state %s: before=%v after=%v", pair.path, pair.before, after)
						}
					}
					readPath := func(name string) string {
						t.Helper()
						data, err := os.ReadFile(filepath.Join(root, name))
						if err != nil {
							t.Fatal(err)
						}
						return strings.TrimSpace(string(data))
					}
					selected := readPath("state-root")
					if !strings.HasPrefix(selected, temp+string(os.PathSeparator)) {
						t.Errorf("selected state %s is not owned temporary state", selected)
					}
					for _, name := range []string{"binary", "socket", "profile"} {
						if name == "profile" && provider != "codex" {
							continue
						}
						path := readPath(name)
						if !strings.HasPrefix(path, selected+string(os.PathSeparator)) {
							t.Errorf("%s %s is outside selected state %s", name, path, selected)
						}
						_, err := os.Stat(path)
						if name != "socket" && mode != "cleanup" && err != nil {
							t.Errorf("retained %s: %v", name, err)
						}
					}
					entries, err := os.ReadDir(temp)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "cleanup" && len(entries) != 0 {
						t.Errorf("successful teardown left fixtures: %v", entries)
					}
					if mode != "cleanup" {
						if _, err := os.Stat(selected); err != nil {
							t.Errorf("lost retained state: %v", err)
						}
						if !strings.Contains(string(output), readPath("socket")) {
							t.Errorf("retained socket not reported: %s", output)
						}
					}
				})
			}
		}
	}
}

func providerFixtureSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(" %x", sha256.Sum256(data))
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += " " + target
		}
		result[relative] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
