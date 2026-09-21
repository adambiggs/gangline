package substrate

import "context"

// Substrate is the process and terminal surface required to drive a harness.
// Implementations translate their native terminal representation into Screen.
type Substrate interface {
	Spawn(context.Context, SpawnSpec) (Pane, error)
	SendKeys(context.Context, PaneID, Keys) error
	Capture(context.Context, PaneID) (Screen, error)
	Kill(context.Context, PaneID) error
	Attach(context.Context, PaneID) error
}

type PaneID string

type Pane struct {
	ID PaneID
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
