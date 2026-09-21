package main

import (
	"context"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

func windowTitle(hitch core.Hitch) string {
	marker := byte(0)
	switch hitch.Status {
	case core.HitchStarting, core.HitchBooting, core.HitchDropping:
		marker = '?'
	case core.HitchFailed:
		marker = '!'
	case core.HitchActive:
		switch hitch.Activity {
		case core.ActivityIdle:
			marker = '~'
		case core.ActivityBusy, core.ActivityDelivering, core.ActivityCompacting, core.ActivityInterrupting:
			marker = '-'
		case core.ActivityBlocked, core.ActivityWedged:
			marker = '!'
		default:
			marker = '?'
		}
	default:
		return ""
	}
	return string(marker) + string(hitch.Name) + string(marker)
}

func (run *runtime) markChangedWindows(previous, next core.State) error {
	targets := make(map[string]string)
	for id, hitch := range next.Hitches {
		title := windowTitle(hitch)
		if hitch.Pane == "" || title == "" {
			continue
		}
		prior, found := previous.Hitches[id]
		if found && prior.Pane == hitch.Pane && windowTitle(prior) == title {
			continue
		}
		targets[hitch.Pane] = title
	}
	return run.applyWindowMarks(targets)
}

func (run *runtime) reconcileWindowMarks(state core.State) error {
	targets := make(map[string]string)
	for _, hitch := range state.Hitches {
		if title := windowTitle(hitch); hitch.Pane != "" && title != "" {
			targets[hitch.Pane] = title
		}
	}
	return run.applyWindowMarks(targets)
}

func (run *runtime) applyWindowMarks(targets map[string]string) error {
	if len(targets) == 0 {
		return nil
	}
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	exists, err := backend.SessionExists(context.Background())
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	windows, err := backend.Windows(context.Background())
	if err != nil {
		return err
	}
	for _, window := range windows {
		title, found := targets[string(window.Pane.ID)]
		if !found || window.Name == title {
			continue
		}
		if err := backend.Rename(context.Background(), substrate.PaneID(window.Pane.ID), title); err != nil {
			return err
		}
	}
	return nil
}
