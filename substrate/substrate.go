package substrate

import "context"

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
}

type Keys struct {
	Text   string
	Names  []string
	Submit bool
}
