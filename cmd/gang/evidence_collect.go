package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/adambiggs/gangline/internal/evidence"
)

func (cmd command) collectEvidence(args []string, assignment string) error {
	want := 3
	if args[0] == "record" {
		want = 4
	}
	if len(args) != want || assignment == "" {
		return usageError("evidence: expected collect MANIFEST BUNDLE or record MANIFEST RECORDS BUNDLE, with --assignment ID")
	}
	input, err := readEvidenceFile(args[1])
	if err != nil {
		return errors.New("evidence: manifest unreadable")
	}
	m, err := evidence.Decode(input)
	if err != nil || m.Scope.Assignment != assignment {
		return errors.New("evidence: invalid manifest or assignment")
	}
	var records []byte
	if args[0] == "record" {
		records, err = readEvidenceFile(args[2])
		if err != nil {
			return errors.New("evidence: records unreadable")
		}
		m, err = evidence.Import(m, assignment, records)
		if err != nil {
			return errors.New("evidence: invalid or conflicting records")
		}
	}
	source, err := os.OpenRoot(filepath.Dir(args[1]))
	if err != nil {
		return errors.New("evidence: input directory unavailable")
	}
	defer source.Close()
	destination := args[len(args)-1]
	// Exclusive directory creation makes retry and overwrite behavior explicit.
	// Failed destinations are retained; no source or partial result is deleted.
	if os.Mkdir(destination, 0700) != nil {
		return errors.New("evidence: destination must be a new directory")
	}
	incomplete := errors.New("evidence: collection incomplete; destination retained")
	dest, err := os.OpenRoot(destination)
	if err != nil {
		return incomplete
	}
	defer dest.Close()
	if dest.Mkdir("artifacts", 0700) != nil {
		return incomplete
	}
	put := func(path string, data []byte) error {
		f, err := dest.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(data)
		syncErr := f.Sync()
		return errors.Join(writeErr, syncErr, f.Close())
	}
	collected, stats, err := evidence.Collect(m, assignment, evidenceReader(source), put)
	if err != nil {
		// Collect returns fixed reasons, never input paths or source contents.
		return errors.New("evidence: " + err.Error() + "; destination retained incomplete")
	}
	data, err := json.MarshalIndent(collected, "", "  ")
	if err != nil {
		return incomplete
	}
	data = append(data, '\n')
	if len(data) > 4<<20 {
		return errors.New("evidence: collected manifest exceeds reader limit; destination retained incomplete")
	}
	if put("source-manifest.json", input) != nil || (records != nil && put("imported-records.json", records) != nil) || put("manifest-content.json", data) != nil {
		return incomplete
	}
	if syncEvidenceDirectory(dest, "artifacts") != nil || syncEvidenceDirectory(dest, ".") != nil {
		return incomplete
	}
	// A no-replace hard link publishes the complete manifest. Keep its content
	// file too: success and failure perform no automatic deletion. A later sync
	// failure reports unknown durability even if the complete link is visible.
	if dest.Link("manifest-content.json", "manifest.json") != nil || syncEvidenceDirectory(dest, ".") != nil {
		return incomplete
	}
	parent, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return incomplete
	}
	err = errors.Join(parent.Sync(), parent.Close())
	if err != nil {
		return incomplete
	}
	sum := sha256.Sum256(data)
	inSum := sha256.Sum256(input)
	return json.NewEncoder(cmd.stdout).Encode(struct {
		evidence.Collection
		Manifest string `json:"manifest_sha256"`
		Input    string `json:"input_manifest_sha256"`
	}{stats, hex.EncodeToString(sum[:]), hex.EncodeToString(inSum[:])})
}

func syncEvidenceDirectory(root *os.Root, name string) error {
	d, err := root.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}
