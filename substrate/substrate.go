package substrate

import (
	"context"
	"strings"
)

// Substrate is the process and terminal surface required to drive a harness.
// Implementations translate their native terminal representation into Screen.
type Substrate interface {
	Spawn(context.Context, SpawnSpec) (Pane, error)
	ForegroundProcesses(context.Context, PaneID) ([]Process, error)
	SendKeys(context.Context, PaneID, Keys) error
	Capture(context.Context, PaneID) (Screen, error)
	Kill(context.Context, PaneID) error
	Attach(context.Context, PaneID) error
}

type PaneID string

type Pane struct {
	ID PaneID
}

// Process is one member of a pane's foreground process group.
type Process struct {
	PID       int
	ParentPID int
	GroupID   int
	Command   string
}

type SpawnSpec struct {
	Name      string
	Directory string
	Command   string
	Args      []string
	Env       map[string]string
	// KeepExited holds the pane open after its process exits, so the exit
	// status and final output stay readable until the pane is released.
	KeepExited bool
	// HoldLog names a file that receives what the hold printed and its exit
	// status when the pane could not hold itself. Such a pane closes without
	// starting its process.
	HoldLog string
}

// ExitedError reports a pane whose process has exited. Status is empty when
// the substrate has not collected one: the process ended on a signal, or the
// pane closed before its exit was reaped. Output holds the pane's last lines.
type ExitedError struct {
	Status string
	Output string
}

func (e *ExitedError) Error() string {
	text := "native process exited"
	if e.Status != "" {
		text += " with status " + e.Status
	}
	if e.Output != "" {
		text += ": " + strings.ReplaceAll(e.Output, "\n", " | ")
	}
	return text
}

type Keys struct {
	Text   string
	Names  []string
	Submit bool
}
