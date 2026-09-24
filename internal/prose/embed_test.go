package prose

import (
	"bytes"
	"os"
	"testing"
)

func TestEmbeddedProseComesFromCanonicalFiles(t *testing.T) {
	contract, err := Contract()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("CONTRACT.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contract, source) {
		t.Fatal("embedded contract differs from CONTRACT.md")
	}
	names, err := RoleNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"lead", "worker"} {
		found := false
		for _, name := range names {
			found = found || name == want
		}
		if !found {
			t.Fatalf("roles = %q, missing %q", names, want)
		}
	}
}
