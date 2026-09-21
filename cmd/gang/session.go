package main

import (
	"context"
	"fmt"
)

func (cmd command) attach(arguments []string) error {
	if err := noArguments(arguments, "attach"); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	backend, err := cmd.tmux(settings)
	if err != nil {
		return err
	}
	exists, err := backend.SessionExists(context.Background())
	if err != nil {
		return err
	}
	if !exists {
		return refuseError("no team %q is running; start it with 'gang up'", settings.Session)
	}
	windows, err := backend.Windows(context.Background())
	if err != nil {
		return err
	}
	if len(windows) == 0 {
		return fmt.Errorf("team %q has no panes", settings.Session)
	}
	return backend.Attach(context.Background(), windows[0].Pane.ID)
}
