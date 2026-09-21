package prose

import (
	"bytes"
	"testing"
)

func TestEmbeddedProseComesFromCanonicalFiles(t *testing.T) {
	contract, err := Contract()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contract, []byte("# Gangline delivery contract")) {
		t.Fatalf("unexpected contract: %q", contract)
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
