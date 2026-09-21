package main

import (
	"os"

	"github.com/adambiggs/gangline/substrate/tmux"
)

func (cmd command) tmux(settings settings) (*tmux.Backend, error) {
	config := tmux.Config{
		Binary:  valueOr(cmd.environment("GANGLINE_TMUX"), "tmux"),
		Socket:  settings.Socket,
		Session: settings.Session,
	}
	config.Stdin, _ = cmd.stdin.(*os.File)
	config.Stdout, _ = cmd.stdout.(*os.File)
	config.Stderr, _ = cmd.stderr.(*os.File)
	return tmux.New(config)
}
