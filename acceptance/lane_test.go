package acceptance

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The mandatory lane must be decidable without observing a live process.
func TestMandatoryLaneIsolation(t *testing.T) {
	for _, file := range []string{"gate.sh", "go.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "test", file))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "go test ") && (!strings.Contains(line, "-timeout=0") || strings.Contains(line, "-tags")) {
				t.Fatalf("%s: mandatory tests require disabled package deadlines and default build tags: %s", file, line)
			}
			if strings.HasPrefix(line, "timeout ") || strings.Contains(line, "test/distribution.sh") || strings.Contains(line, "GANGLINE_LIVE_PROVIDERS=1") {
				t.Fatalf("%s: real-service check in the mandatory lane: %s", file, line)
			}
		}
	}
	for _, file := range []string{"acceptance/tmux_test.go", "cmd/gang/state_test.go", "substrate/tmux/tmux_test.go", "store/store_test.go"} {
		data, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Fatal(err)
		}
		line, _, _ := strings.Cut(string(data), "\n")
		expr, err := constraint.Parse(line)
		if err != nil {
			t.Fatalf("%s: service fixtures require a build constraint: %v", file, err)
		}
		if expr.Eval(func(string) bool { return false }) || !expr.Eval(func(tag string) bool { return tag == "acceptance" }) {
			t.Fatalf("%s: service fixtures must run only in acceptance", file)
		}
	}
}
