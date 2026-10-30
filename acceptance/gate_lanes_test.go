//go:build acceptance

package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateLanes(t *testing.T) {
	for _, lane := range []string{"gate", "acceptance"} {
		for _, mode := range []string{"pass", "refuse", "interrupt"} {
			fail := mode != "pass"
			name := lane + "/" + mode
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				if err := os.MkdirAll(filepath.Join(root, "test"), 0700); err != nil {
					t.Fatal(err)
				}
				write := func(name, text string) {
					t.Helper()
					if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0700); err != nil {
						t.Fatal(err)
					}
				}
				for _, name := range []string{"gate.sh", "go.sh", "acceptance.sh"} {
					data, err := os.ReadFile(filepath.Join("..", "test", name))
					if err != nil {
						t.Fatal(err)
					}
					write("test/"+name, string(data))
				}
				write("test/distribution.sh", "#!/bin/sh\nprintf 'distribution\\n' >> \"$CALLS\"\n")
				write("git", "#!/bin/sh\nexit 0\n")
				write("go", `#!/bin/sh
printf '%s\n' "$*" >> "$CALLS"
case "$*" in
  'list -m') printf 'example.invalid/fixture\n';;
  test*)
    if [ "$FAIL_TEST" = 2 ]; then kill -TERM "$LANE_PID"; exit 0; fi
    [ "$FAIL_TEST" != 1 ] || exit 42;;
esac
`)
				write("uname", "#!/bin/sh\nprintf Darwin\\n\n")
				// A gate trying to acquire a host lock fails rather than touching it.
				write("flock", "#!/bin/sh\nexit 91\n")
				failed := "0"
				if fail {
					failed = "1"
				}
				if mode == "interrupt" {
					failed = "2"
				}
				calls := filepath.Join(root, "calls")
				command := exec.Command("bash", "-c", `export LANE_PID=$$; exec bash "$1"`, "bash", filepath.Join(root, "test", lane+".sh"))
				command.Env = append(withoutEnvironment(os.Environ(), "PATH", "CALLS", "FAIL_TEST", "_GANGLINE_GATE_LOCKED", "_GANGLINE_ACCEPTANCE_LOCKED"),
					"PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"), "CALLS="+calls, "FAIL_TEST="+failed, "_GANGLINE_ACCEPTANCE_LOCKED=1")
				output, err := command.CombinedOutput()
				want := "PASS (status 0)"
				if fail {
					want = "REFUSED (status 42)"
				}
				if mode == "interrupt" {
					want = "UNKNOWN (status 143)"
				}
				if (err != nil) != fail || !strings.Contains(string(output), lane+": VERDICT "+want) {
					t.Fatalf("%v\n%s", err, output)
				}
				if !strings.Contains(string(output), "total wall time") {
					t.Fatalf("missing total timing:\n%s", output)
				}
				data, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				trace := string(data)
				if lane == "gate" {
					if !strings.Contains(trace, "test -count=1 -timeout=0 ./...") || strings.Contains(trace, "acceptance") || strings.Contains(trace, "distribution") {
						t.Fatalf("mandatory lane ran the wrong checks:\n%s", trace)
					}
				} else if !strings.Contains(trace, "distribution\n") || !strings.Contains(trace, "test -tags=acceptance -count=1 -timeout=0 -skip") || (!fail && !strings.Contains(trace, "-run ^TestLive(ClaudeCode|Codex)$ ./acceptance")) {
					t.Fatalf("acceptance lane omitted checks:\n%s", trace)
				}
			})
		}
	}
}

func TestReleaseRequiresAcceptanceBeforeAssets(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	_, job, ok := strings.Cut(string(data), "  release-binaries:\n")
	if !ok {
		t.Fatal("missing release-binaries job")
	}
	job, _, _ = strings.Cut(job, "\n  # The team kit")
	acceptance := strings.Index(job, "run: test/acceptance.sh")
	build := strings.Index(job, "run: packaging/build-release.sh")
	if acceptance < 0 || build < acceptance || strings.Contains(job, "continue-on-error:") {
		t.Fatal("asset build must follow a required acceptance step")
	}
}
