package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// sessionEndingTmux returns a tmux binary that ends session around the nth
// command whose arguments match pattern once armed: before that command, or
// after it when after is set. ended reports whether the session was ended.
func sessionEndingTmux(t *testing.T, binary, socket, session, pattern string, nth int, after bool) (wrapper string, arm func(), ended func() bool) {
	t.Helper()
	dir := t.TempDir()
	wrapper = filepath.Join(dir, "tmux")
	when := "before"
	if after {
		when = "after"
	}
	script := fmt.Sprintf(`#!/bin/sh
dir='%s'
end() { "$0.real" -S '%s' kill-session -t '=%s' >"$dir/kill.out" 2>&1 && : >"$dir/ended"; }
hit=
case "$*" in *%s*)
	if [ -e "$dir/armed" ]; then
		n=$(( $(cat "$dir/count") + 1 )); echo "$n" >"$dir/count"
		[ "$n" = %d ] && hit=1
	fi;;
esac
[ -n "$hit" ] && [ %s = before ] && end
"$0.real" "$@"; status=$?
[ -n "$hit" ] && [ %s = after ] && end
exit $status
`, dir, socket, session, pattern, nth, when, when)
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(binary, wrapper+".real"); err != nil {
		t.Fatal(err)
	}
	arm = func() {
		if err := os.WriteFile(filepath.Join(dir, "count"), []byte("0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "armed"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ended = func() bool {
		_, err := os.Stat(filepath.Join(dir, "ended"))
		return err == nil
	}
	return wrapper, arm, ended
}

// sessionEndCase starts the configured session and, when keep is set, a
// second session that keeps the server running after the first ends.
func sessionEndCase(t *testing.T, binary string, keep bool) (socket, session string) {
	t.Helper()
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	socket = filepath.Join(privateTmuxRoot(t), "tmux.sock")
	session = "ending"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	if keep {
		runTmux(t, binary, socket, "new-session", "-d", "-s", session+"-keep")
	}
	t.Cleanup(func() {
		for _, name := range []string{session, session + "-keep"} {
			_, _ = runTmuxResult(binary, socket, "kill-session", "-t", "="+name)
		}
	})
	return socket, session
}

func serverModes() []struct {
	name string
	keep bool
} {
	return []struct {
		name string
		keep bool
	}{{"server-exits", false}, {"server-stays", true}}
}

// A registered pane is read from one pane listing. A session that ends after
// that listing leaves no second tmux command to fail or to misread as a pane
// outside its session.
func TestRegisteredPaneSurvivesSessionEndAfterListing(t *testing.T) {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	for _, mode := range serverModes() {
		t.Run(mode.name, func(t *testing.T) {
			socket, session := sessionEndCase(t, binary, mode.keep)
			real, err := New(Config{Binary: binary, Socket: socket, Session: session})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			pane, err := real.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: t.TempDir(), Command: "cat"})
			if err != nil {
				t.Fatal(err)
			}
			id, err := real.RegisterPane(ctx, pane.ID)
			if err != nil {
				t.Fatal(err)
			}
			wrapper, arm, ended := sessionEndingTmux(t, binary, socket, session, `" list-panes -a -F "`, 1, true)
			b, err := New(Config{Binary: wrapper, Socket: socket, Session: session})
			if err != nil {
				t.Fatal(err)
			}
			arm()
			present, err := b.CheckPane(ctx, id)
			if !ended() {
				t.Fatal("the session did not end after the pane listing")
			}
			if err != nil || !present {
				t.Fatalf("CheckPane = %v, %v; want the pane the listing showed", present, err)
			}
		})
	}
}

// A pane listed only outside the configured session is refused, and the
// refusal says whether that session is absent or the pane left it.
func TestRegisteredPaneNamesWhyItIsOutsideItsSession(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "tmux")
	b, err := New(Config{Binary: binary, Session: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ listing, want string }{
		{`g\t$1\t%%1\tother\n`, `configured session "test" is absent`},
		{`g\t$1\t%%1\tother\ng\t$2\t%%2\ttest\n`, `pane %1 is outside configured session "test"`},
	} {
		script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nprintf '" + tc.listing + "'\n"
		if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		_, exists, err := b.registeredPane(context.Background(), "%1")
		if exists || !errors.Is(err, ErrPaneReplaced) || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("listing %q: exists=%v err=%v, want %q", tc.listing, exists, err, tc.want)
		}
	}
}
