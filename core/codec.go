package core

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema/events.cue
var eventSchema string

var schema = sync.OnceValues(func() (cue.Value, error) {
	v := cuecontext.New().CompileString(eventSchema)
	return v.LookupPath(cue.ParsePath("#Event")), v.Err()
})
var schemaMu sync.Mutex

func ValidateEventJSON(data []byte) error {
	schemaMu.Lock()
	defer schemaMu.Unlock()
	v, err := schema()
	if err != nil {
		return err
	}
	value := v.Context().CompileBytes(data)
	return v.Unify(value).Validate(cue.Concrete(true))
}

func EncodeEvent(event Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	if err := ValidateEventJSON(data); err != nil {
		return nil, fmt.Errorf("validate %s event: %w", event.Type, err)
	}
	return data, nil
}

func DecodeEvent(data []byte) (Event, error) {
	var event Event
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return event, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return event, fmt.Errorf("event has trailing data")
	}
	if err := ValidateEventJSON(data); err != nil {
		return event, err
	}
	return event, nil
}
