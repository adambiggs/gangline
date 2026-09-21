package harness

import (
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema/collar.cue
var collarSchema []byte

//go:embed collars/*.cue
var embeddedCollars embed.FS

type Collar struct {
	Name       string          `json:"name"`
	Launch     Launch          `json:"launch"`
	Hooks      map[string]Hook `json:"hooks,omitempty"`
	Primitives Primitives      `json:"primitives"`
}

type Launch struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Hook struct {
	Template string            `json:"template"`
	Event    string            `json:"event"`
	Payload  map[string]string `json:"payload,omitempty"`
}

type Invocation struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params,omitempty"`
}

type Primitives struct {
	Startup      []Invocation `json:"startup"`
	Composer     Invocation   `json:"composer"`
	Submit       Invocation   `json:"submit"`
	TurnBoundary Invocation   `json:"turn_boundary"`
	ModelID      *Invocation  `json:"model_id,omitempty"`
	Wedge        *Invocation  `json:"wedge,omitempty"`
}

func LoadCollar(filename string, data []byte) (Collar, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(collarSchema, cue.Filename("collar.cue")).LookupPath(cue.MakePath(cue.Def("Collar")))
	if err := schema.Err(); err != nil {
		return Collar{}, fmt.Errorf("compile collar schema: %w", err)
	}

	value := ctx.CompileBytes(data, cue.Filename(filename)).LookupPath(cue.MakePath(cue.Str("collar")))
	if err := value.Err(); err != nil {
		return Collar{}, fmt.Errorf("load collar %q: %w", filename, err)
	}
	validated := schema.Unify(value)
	if err := validated.Validate(cue.Concrete(true)); err != nil {
		return Collar{}, fmt.Errorf("validate collar %q: %w", filename, err)
	}

	var collar Collar
	if err := validated.Decode(&collar); err != nil {
		return Collar{}, fmt.Errorf("decode collar %q: %w", filename, err)
	}
	return collar, nil
}

func EmbeddedCollar(name string) (Collar, error) {
	data, err := embeddedCollars.ReadFile(filepath.Join("collars", name+".cue"))
	if err != nil {
		return Collar{}, fmt.Errorf("read embedded collar %q: %w", name, err)
	}
	return LoadCollar(name+".cue", data)
}

func EmbeddedCollarNames() ([]string, error) {
	entries, err := embeddedCollars.ReadDir("collars")
	if err != nil {
		return nil, fmt.Errorf("list embedded collars: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cue" {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".cue"))
	}
	sort.Strings(names)
	return names, nil
}
