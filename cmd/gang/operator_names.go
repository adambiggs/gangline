package main

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

var tmuxHandle = regexp.MustCompile(`[%@$][0-9]+`)

// operatorNames retains names before an operation removes registrations. The
// stored records and backend errors keep their original internal handles.
type operatorNames struct {
	mu    sync.Mutex
	cmd   command
	names map[string]string
}

func newOperatorNames(cmd command) *operatorNames {
	n := &operatorNames{cmd: cmd, names: make(map[string]string)}
	if s, err := cmd.settings(); err == nil {
		if team, err := (store.Paths{Root: s.StateRoot}).Team(s.Session); err == nil {
			if agents, err := team.ListAgents(); err == nil {
				for _, a := range agents {
					if a.Pane != "" {
						n.names[a.Pane] = string(a.Name)
					}
				}
			}
		}
	}
	return n
}

func (n *operatorNames) text(text string) string {
	if !tmuxHandle.MatchString(text) {
		return text
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return tmuxHandle.ReplaceAllStringFunc(text, func(id string) string {
		if name := n.names[id]; name != "" {
			return name
		}
		kind := "pane"
		if id[0] == '@' {
			kind = "window"
		} else if id[0] == '$' {
			kind = "team session"
		}
		location := "unregistered " + kind + " (location unavailable)"
		if id[0] == '%' {
			if s, err := n.cmd.settings(); err == nil {
				if b, err := n.cmd.tmux(s); err == nil {
					ctx, cancel := n.cmd.timeout(operationTimeout)
					position, err := b.DescribePane(ctx, substrate.PaneID(id))
					cancel()
					if err == nil {
						location = position
					}
				}
			}
		}
		n.names[id] = location
		return location
	})
}

func (cmd command) operatorText(text string) string {
	if !tmuxHandle.MatchString(text) {
		return text
	}
	n := cmd.presentation
	if n == nil {
		n = newOperatorNames(cmd)
	}
	return n.text(text)
}

type operatorOutput struct {
	writer io.Writer
	names  *operatorNames
}

func (o operatorOutput) Write(p []byte) (int, error) {
	text := string(p)
	if json.Valid(p) {
		// Diagnostic JSON must remain valid when a pane position contains
		// quotes. Only handles are replaced; numeric values stay exact.
		text = tmuxHandle.ReplaceAllStringFunc(text, func(id string) string {
			encoded, _ := json.Marshal(o.names.text(id))
			return strings.TrimSuffix(strings.TrimPrefix(string(encoded), `"`), `"`)
		})
	} else {
		text = o.names.text(text)
	}
	if _, err := io.WriteString(o.writer, text); err != nil {
		return 0, err
	}
	return len(p), nil
}

type operatorError struct {
	error
	names *operatorNames
}

func (e operatorError) Error() string { return e.names.text(e.error.Error()) }
func (e operatorError) Unwrap() error { return e.error }
