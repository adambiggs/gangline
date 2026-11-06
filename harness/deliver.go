package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var deliverPrimitives = []string{"codex-queue"}

// NativeDelivery is one call that hands a message to the harness's own input
// queue instead of typing it. Only its exit status decides the outcome.
type NativeDelivery struct {
	Argv []string
	Env  []string
}

// NativeDeliveryFor builds the deliver primitive's call for wire in the
// native session bound to transcript. It returns false when the collar has no
// deliver primitive or the session is not idle after a completed turn, and an
// error when the bound session cannot be addressed.
func NativeDeliveryFor(collar Collar, session, transcript, wire string, environ []string) (NativeDelivery, bool, error) {
	primitive := collar.Primitives.Deliver
	if primitive == nil || session == "" || transcript == "" {
		return NativeDelivery{}, false, nil
	}
	if primitive.Name != "codex-queue" {
		return NativeDelivery{}, false, fmt.Errorf("unknown deliver primitive %q", primitive.Name)
	}
	if strings.HasPrefix(session, "-") || strings.ContainsAny(session, `/\`) {
		return NativeDelivery{}, false, fmt.Errorf("native session %q is not a thread ID", session)
	}
	home, ok := codexHomeOf(transcript, session)
	if !ok {
		return NativeDelivery{}, false, fmt.Errorf("bound transcript %s is not a Codex rollout of session %s", transcript, session)
	}
	// The thread has no rollout until its first turn, and codex queue
	// refuses it until then. Codex starts queued input when it is idle after
	// a completed turn, or when its running turn completes. After an
	// interrupt the input waits until a later turn completes, and a running
	// turn may yet be interrupted, so the queue is used only when the last
	// turn completed.
	completed, err := codexTurnCompleted(transcript)
	if errors.Is(err, os.ErrNotExist) || err == nil && !completed {
		return NativeDelivery{}, false, nil
	} else if err != nil {
		return NativeDelivery{}, false, err
	}
	env := LaunchEnv(collar, environ)
	// The rollout's own home is where its thread lives, whatever the
	// recipient's environment said when it started.
	env = append(env, "CODEX_HOME="+home)
	return NativeDelivery{
		Argv: []string{collar.Launch.Command, "queue", "--thread", session, "--message", wire},
		Env:  env,
	}, true, nil
}

// codexHomeOf returns the Codex home that holds a rollout at
// <home>/sessions/YYYY/MM/DD/rollout-*-<session>.jsonl.
func codexHomeOf(transcript, session string) (string, bool) {
	if !filepath.IsAbs(transcript) {
		return "", false
	}
	transcript = filepath.Clean(transcript)
	name := filepath.Base(transcript)
	if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, "-"+session+".jsonl") {
		return "", false
	}
	dir := filepath.Dir(transcript)
	for _, width := range []int{2, 2, 4} {
		part := filepath.Base(dir)
		if len(part) != width || strings.Trim(part, "0123456789") != "" {
			return "", false
		}
		dir = filepath.Dir(dir)
	}
	if filepath.Base(dir) != "sessions" {
		return "", false
	}
	home := filepath.Dir(dir)
	return home, home != dir
}

// codexTurnCompleted reports whether the last turn record in the rollout's
// bounded tail is a completion without an error. A window with no turn
// record reads as not completed.
func codexTurnCompleted(path string) (bool, error) {
	file, err := OpenTranscript(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return false, err
	}
	start, err := boundedTranscriptStart(file, 0, size)
	if err != nil {
		return false, err
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return false, err
	}
	completed := false
	reader := bufio.NewReader(io.LimitReader(file, size-start))
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			// A record still being written may start the next turn.
			return completed && len(line) == 0, nil
		}
		if err != nil {
			return false, err
		}
		if !bytes.Contains(line, []byte(`"event_msg"`)) {
			continue
		}
		var r struct {
			Type    string `json:"type"`
			Payload struct {
				Type  string          `json:"type"`
				Error json.RawMessage `json:"error"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &r) != nil || r.Type != "event_msg" {
			continue
		}
		switch r.Payload.Type {
		case "task_started", "turn_aborted":
			completed = false
		case "task_complete":
			completed = len(r.Payload.Error) == 0 || string(r.Payload.Error) == "null"
		}
	}
}
