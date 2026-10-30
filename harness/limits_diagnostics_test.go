package harness

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLimitsQueryFailureDiagnostics(t *testing.T) {
	const diagnostic = "synthetic native config parse failure: invalid table at line 3"
	var requests bytes.Buffer
	_, protocolErr := queryCodexLimits(strings.NewReader(""), &requests)
	if !errors.Is(protocolErr, io.EOF) {
		t.Fatalf("protocol fixture error = %v", protocolErr)
	}
	for _, tc := range []struct {
		name        string
		contextErr  error
		protocolErr error
		exitErr     *exec.ExitError
		stderr      []byte
	}{
		{name: "codex", protocolErr: protocolErr, exitErr: &exec.ExitError{}, stderr: []byte(diagnostic + "\n")},
		{name: "claude", exitErr: &exec.ExitError{}, stderr: []byte(diagnostic + "\n")},
		{name: "cancelled codex", contextErr: context.Canceled, protocolErr: protocolErr, exitErr: &exec.ExitError{}, stderr: []byte(diagnostic)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := limitsQueryError(tc.name, tc.contextErr, tc.protocolErr, tc.exitErr, tc.stderr)
			if err == nil || !strings.Contains(err.Error(), diagnostic) || !strings.Contains(err.Error(), "limits query exited") {
				t.Fatalf("missing process diagnostic: %v", err)
			}
			var gotExit *exec.ExitError
			if !errors.As(err, &gotExit) || gotExit != tc.exitErr {
				t.Fatalf("process error lost: %v", err)
			}
			if tc.protocolErr != nil && !errors.Is(err, io.EOF) {
				t.Fatalf("protocol error lost: %v", err)
			}
			if tc.contextErr != nil && !errors.Is(err, tc.contextErr) {
				t.Fatalf("context error lost: %v", err)
			}
		})
	}
}

func TestLimitsStderrSnapshotPreservesWriterOffset(t *testing.T) {
	file, err := newLimitsStderr()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := os.Stat(file.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("capture path remains: %v", err)
	}
	for _, text := range []string{"native failure", "; descendant detail"} {
		if _, err := file.WriteString(text); err != nil {
			t.Fatal(err)
		}
		data, err := readLimitsStderr(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(data), text) {
			t.Fatalf("diagnostic snapshot = %q", data)
		}
	}
	if _, err := file.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := readLimitsStderr(file)
	if err != nil || string(data) != "native failure; descendant detail" {
		t.Fatalf("diagnostic = %q, error = %v", data, err)
	}
	if offset, err := file.Seek(0, io.SeekCurrent); err != nil || offset != 3 {
		t.Fatalf("writer offset = %d, error = %v", offset, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readLimitsStderr(file); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed capture error = %v", err)
	}
}

func TestLimitsQuerySuccessIgnoresDiagnosticOutput(t *testing.T) {
	input := `{"id":1,"result":{}}
{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":12,"resetsAt":1800000000}}}}`
	var requests bytes.Buffer
	limits, protocolErr := queryCodexLimits(strings.NewReader(input), &requests)
	if err := limitsQueryError("fixture", nil, protocolErr, nil, []byte("native warning\n")); err != nil {
		t.Fatal(err)
	}
	if len(limits) != 1 || limits[0].UsedPercent != 12 {
		t.Fatalf("limits = %+v", limits)
	}
}
