//go:build !claude_native

package main

import (
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func (run *runtime) tryClaudeNative(_ *store.LockedAgent, _ *core.Agent, _ core.Envelope, _ harness.Collar) (string, bool, error) {
	return "", false, nil
}
