package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/internal/prose"
)

// The contract is the only command reference an agent is given at hitch.
func TestContractNamesRealAgentCommands(t *testing.T) {
	contract, err := prose.Contract()
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, m := range regexp.MustCompile("`(?:[^`|]*\\| )?gang ([a-z]+)").FindAllStringSubmatch(string(contract), -1) {
		named[m[1]] = true
	}
	for command := range named {
		if _, ok := commandUsage[command]; !ok && command != "help" {
			t.Errorf("contract names unknown command gang %s", command)
		}
	}
	for _, want := range []string{"send", "compact", "context", "whoami", "roster", "hitch", "drop"} {
		if !named[want] {
			t.Errorf("contract omits gang %s", want)
		}
	}
	if !strings.Contains(string(contract), "gang compact --resume") {
		t.Error("contract omits the resume note for self-compaction")
	}
}
