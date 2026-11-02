package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adambiggs/gangline/internal/evidence"
)

func (cmd command) evidence(arguments []string) error {
	var assignment string
	stage := "ci"
	flags := boundFlagSet("evidence", map[string]any{"assignment": &assignment, "require": &stage})
	args, err := parseOptions(flags, arguments)
	if err != nil {
		return usageError("evidence: invalid options (run 'gang help evidence')")
	}
	if len(args) > 0 && (args[0] == "collect" || args[0] == "record") {
		if flagWasSet(flags, "require") {
			return usageError("evidence: collect and record preserve inputs; use verify to require a stage")
		}
		return cmd.collectEvidence(args, assignment)
	}
	if len(args) != 2 || (args[0] != "verify" && args[0] != "show") || assignment == "" {
		return usageError("evidence: expected verify|show, --assignment ID and MANIFEST.json")
	}
	// Do not echo input filenames, JSON decoder errors or payloads. They can
	// contain private task text even when the manifest is malformed.
	data, err := readEvidenceFile(args[1])
	if err != nil {
		return errors.New("evidence: manifest unreadable")
	}
	manifest, err := evidence.Decode(data)
	if err != nil {
		return errors.New("evidence: invalid or unsupported manifest")
	}
	root, err := os.OpenRoot(filepath.Dir(args[1]))
	if err != nil {
		return errors.New("evidence: input directory unavailable")
	}
	defer root.Close()
	read := evidenceReader(root)
	result := evidence.Verify(manifest, assignment, stage, read)
	enc := json.NewEncoder(cmd.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return errors.New("evidence: output write failed")
	}
	if args[0] == "show" {
		return nil
	}
	switch result.Status {
	case "pass":
		return nil
	case "fail":
		return commandError{status: exitRefused, text: "evidence: verification failed"}
	default:
		return commandError{status: exitUnknown, text: fmt.Sprintf("evidence: verification %s", result.Status)}
	}
}

func evidenceReader(root *os.Root) func(string, int64) ([]byte, error) {
	return func(path string, size int64) ([]byte, error) {
		file, err := root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("not a regular file")
		}
		return io.ReadAll(io.LimitReader(file, size+1))
	}
}
func readEvidenceFile(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	const manifestLimit = 4 << 20
	b, err := io.ReadAll(io.LimitReader(f, manifestLimit+1))
	if len(b) > manifestLimit {
		return nil, errors.New("manifest too large")
	}
	return b, err
}
