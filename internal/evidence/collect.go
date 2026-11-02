package evidence

import (
	"encoding/json"
	"errors"
)

// Collection describes preservation, not acceptance of the recorded assertions.
type Collection struct {
	Status      string `json:"status"`
	Artifacts   int    `json:"copied_artifacts"`
	Withheld    int    `json:"withheld_artifacts"`
	Unavailable int    `json:"unavailable_artifacts"`
}

// Import appends explicitly supplied records. Existing identities cannot be
// replaced; Decode rejects conflicts and coalesces identical repeated records.
func Import(m Manifest, assignment string, input []byte) (Manifest, error) {
	var records []Record
	if strict(input, &records) != nil || records == nil {
		return Manifest{}, errors.New("invalid evidence records")
	}
	m.Records = append(m.Records[:len(m.Records):len(m.Records)], records...)
	return collectionManifest(m, assignment)
}

func collectionManifest(m Manifest, assignment string) (Manifest, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return Manifest{}, errors.New("invalid evidence manifest")
	}
	copy, err := Decode(data)
	if err != nil {
		return Manifest{}, err
	}
	if copy.Scope.Assignment != assignment {
		return Manifest{}, errors.New("assignment mismatch")
	}
	return copy, nil
}

// Collect copies only artifacts explicitly present in m. read must enforce the
// source manifest's directory boundary. put must create a new immutable file;
// the caller publishes the returned manifest only after every write succeeds.
// Withheld/unavailable references stay in the manifest and are never opened.
func Collect(m Manifest, assignment string, read func(string, int64) ([]byte, error), put func(string, []byte) error) (Manifest, Collection, error) {
	stats := Collection{Status: "incomplete"}
	m, err := collectionManifest(m, assignment)
	if err != nil {
		return Manifest{}, stats, err
	}
	v := verifier{result: Result{Status: "pass"}, records: map[string]Record{}, read: read, data: map[string][]byte{}, checked: map[string]bool{}}
	for _, r := range m.Records {
		v.records[r.ID] = r
	}
	for _, r := range m.Records {
		if r.Artifact == nil {
			continue
		}
		switch r.Artifact.Status {
		case "withheld":
			stats.Withheld++
			continue
		case "unavailable":
			stats.Unavailable++
			continue
		}
		data, ok := v.artifact(r.ID)
		if !ok {
			return Manifest{}, stats, errors.New(v.result.Issues[len(v.result.Issues)-1].Reason)
		}
		path := "artifacts/" + r.ID
		if put(path, data) != nil {
			return Manifest{}, stats, errors.New("artifact preservation failed")
		}
		delete(v.data, r.ID)
		r.Artifact.Path = path
		stats.Artifacts++
	}
	stats.Status = "collected"
	return m, stats, nil
}
