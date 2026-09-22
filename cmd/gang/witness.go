package main

import (
	"context"
	"errors"
	"os"

	"github.com/adambiggs/gangline/store"
)

type changeWait interface {
	Wait(context.Context) error
	Close() error
}

func (cmd command) watch(path string) (changeWait, error) {
	if cmd.newWatch != nil {
		return cmd.newWatch(path)
	}
	return store.Watch(path)
}
func (cmd command) awaitWitness(ctx context.Context, p store.AgentPaths, before string) (store.Witness, error) {
	for {
		watch, err := cmd.watch(p.Witness)
		if err != nil {
			return store.Witness{}, err
		}
		witness, err := p.ReadWitness()
		if err == nil && witness.ID != before {
			_ = watch.Close()
			return witness, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = watch.Close()
			return witness, err
		}
		err = watch.Wait(ctx)
		closeErr := watch.Close()
		if err != nil {
			return witness, err
		}
		if closeErr != nil {
			return witness, closeErr
		}
	}
}
