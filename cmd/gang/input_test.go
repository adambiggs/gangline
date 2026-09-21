package main

import (
	"strings"
	"testing"
)

func TestReadBodyPreservesContentAndDropsOneTerminalNewline(t *testing.T) {
	got, err := readBody(strings.NewReader("first\nsecond\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "first\nsecond" {
		t.Fatalf("body = %q", got)
	}
}

func TestReadBodyRejectsEmptyInput(t *testing.T) {
	if _, err := readBody(strings.NewReader("")); err == nil {
		t.Fatal("empty input passed")
	}
}
