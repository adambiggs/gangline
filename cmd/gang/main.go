package main

import (
	"fmt"
	"io"
	"os"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "gang %s\n", version)
		return 0
	}

	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help")) {
		fmt.Fprintln(stdout, "usage: gang <command>")
		return 0
	}

	fmt.Fprintf(stderr, "gang: command %q is not implemented\n", args[0])
	return 2
}
