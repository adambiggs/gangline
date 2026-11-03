package main

import "testing"

func TestClearRequiresSmallContinuationArgument(t *testing.T) {
	for _, args := range [][]string{nil, {"--note", " "}, {"worker", "--note", "next"}, {"--fresh"}} {
		if err := (command{}).clear(args); errorStatus(err) != exitUsage {
			t.Fatalf("args=%v error=%v", args, err)
		}
	}
}
