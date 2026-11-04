package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"

	"github.com/adambiggs/gangline/core"
)

// modelMetadata is optional display metadata, separate from the strict lifecycle
// schema so older writers can keep operating on the same agent. The launch nonce
// distinguishes rehitches even when the hitch ID and tmux server stay the same.
type modelMetadata struct {
	Version          int          `json:"version"`
	HitchID          core.HitchID `json:"hitch_id"`
	Generation       string       `json:"generation"`
	Session          string       `json:"session"`
	LaunchTokenHash  string       `json:"launch_token_hash"`
	ConfiguredModel  string       `json:"configured_model,omitempty"`
	ConfiguredEffort string       `json:"configured_effort,omitempty"`
	StateSHA256      string       `json:"state_sha256"`
	ObservedEffort   string       `json:"observed_effort,omitempty"`
}

func modelStateDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (p AgentPaths) readModelMetadata(state []byte, a *core.Agent) {
	var m modelMetadata
	path := filepath.Join(p.Directory, "model-metadata.json")
	if err := readJSON(path, &m); err != nil {
		// Optional display metadata must not block inventory or lifecycle
		// operations, including when a newer writer uses an unknown schema.
		return
	}
	if m.Version != 1 {
		return
	}
	r := a.Registration
	if r.Generation == "" || r.Session == "" || r.TokenHash == "" ||
		m.HitchID != a.ID || m.Generation != r.Generation || m.Session != r.Session || m.LaunchTokenHash != r.TokenHash {
		return
	}
	a.ConfiguredModel, a.ConfiguredEffort = m.ConfiguredModel, m.ConfiguredEffort
	if m.StateSHA256 == modelStateDigest(state) {
		a.Native.Effort = m.ObservedEffort
	}
}

// saveModelState runs under the agent lock. Write the side record first, naming
// the exact state bytes about to be committed. If the second write fails, readers
// see unknown effort unless the state bytes were already identical. Configuration
// is independently bound to the launch nonce, so legacy activity saves retain it.
func (p AgentPaths) saveModelState(a core.Agent, write func(string, any) error) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	data = append(data, '\n') // atomicJSON uses json.Encoder.Encode.
	r := a.Registration
	m := modelMetadata{Version: 1, HitchID: a.ID, Generation: r.Generation, Session: r.Session,
		LaunchTokenHash: r.TokenHash, ConfiguredModel: a.ConfiguredModel, ConfiguredEffort: a.ConfiguredEffort,
		StateSHA256: modelStateDigest(data), ObservedEffort: a.Native.Effort}
	if err := write(filepath.Join(p.Directory, "model-metadata.json"), m); err != nil {
		return err
	}
	return write(p.State, a)
}
