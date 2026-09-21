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
	Name         string                   `json:"name"`
	Launch       Launch                   `json:"launch"`
	Hooks        *Hooks                   `json:"hooks,omitempty"`
	Models       Models                   `json:"models"`
	Options      Options                  `json:"options,omitempty"`
	Primitives   Primitives               `json:"primitives"`
	Actions      Actions                  `json:"actions"`
	ContextBands map[string][]ContextBand `json:"context_bands"`
}

type Launch struct {
	Command    string            `json:"command"`
	Args       []string          `json:"args,omitempty"`
	ResumeArgs []string          `json:"resume_args,omitempty"`
	ProbeArgs  []string          `json:"probe_args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

type Hooks struct {
	InstallArgs []string        `json:"install_args"`
	Events      map[string]Hook `json:"events"`
}

type Hook struct {
	Event   string            `json:"event"`
	Payload map[string]string `json:"payload,omitempty"`
}

type Option struct {
	Args []string `json:"args"`
}

type Models struct {
	Catalog  Invocation  `json:"catalog"`
	Selected *Invocation `json:"selected,omitempty"`
	Option   Option      `json:"option"`
}

type Options struct {
	Effort     *Option `json:"effort,omitempty"`
	RolePrompt *Option `json:"role_prompt,omitempty"`
}

type Invocation struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params,omitempty"`
}

type Primitives struct {
	Startup        []Invocation `json:"startup"`
	Composer       Invocation   `json:"composer"`
	Submit         Invocation   `json:"submit"`
	SubmitWitness  Invocation   `json:"submit_witness"`
	TurnBoundary   Invocation   `json:"turn_boundary"`
	Blocked        Invocation   `json:"blocked"`
	Context        Invocation   `json:"context"`
	ProviderLimits Invocation   `json:"provider_limits"`
	Wedge          Invocation   `json:"wedge"`
}

type Actions struct {
	Interrupt      Action   `json:"interrupt"`
	Compact        Action   `json:"compact"`
	CompactRecover []Action `json:"compact_recover"`
}

type Action struct {
	Text   string   `json:"text,omitempty"`
	Keys   []string `json:"keys,omitempty"`
	Submit bool     `json:"submit,omitempty"`
}

type ContextBand struct {
	Name string  `json:"name"`
	At   float64 `json:"at"`
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
	if err := validateCollar(collar); err != nil {
		return Collar{}, fmt.Errorf("validate collar %q: %w", filename, err)
	}
	return collar, nil
}

func validateCollar(collar Collar) error {
	if !argsContain(collar.Models.Option.Args, "{{value}}") {
		return fmt.Errorf("model option does not contain {{value}}")
	}
	for name, option := range map[string]*Option{"effort": collar.Options.Effort, "role prompt": collar.Options.RolePrompt} {
		if option != nil && !argsContain(option.Args, "{{value}}") {
			return fmt.Errorf("%s option does not contain {{value}}", name)
		}
	}
	if len(collar.Launch.ResumeArgs) != 0 && !argsContain(collar.Launch.ResumeArgs, "{{session_id}}") {
		return fmt.Errorf("resume arguments do not contain {{session_id}}")
	}
	if collar.Hooks != nil && !argsContain(collar.Hooks.InstallArgs, "{{hook.command.json}}") {
		return fmt.Errorf("hook install arguments do not contain {{hook.command.json}}")
	}
	if _, err := SubmitSettle(collar.Primitives.Submit); err != nil {
		return err
	}
	if _, err := SubmitInput(collar.Primitives.Submit, ""); err != nil {
		return err
	}
	checks := []struct {
		where   string
		value   Invocation
		allowed []string
	}{
		{"model catalog", collar.Models.Catalog, []string{"claude-help-models", "codex-debug-models"}},
		{"composer", collar.Primitives.Composer, []string{"claude-composer", "codex-composer"}},
		{"submit", collar.Primitives.Submit, []string{"enter-submit"}},
		{"submit witness", collar.Primitives.SubmitWitness, []string{"exact-prompt", "claude-pasted-content"}},
		{"turn boundary", collar.Primitives.TurnBoundary, []string{"hook-boundary"}},
		{"blocked", collar.Primitives.Blocked, []string{"screen-blocked"}},
		{"context", collar.Primitives.Context, []string{"claude-screen-context", "codex-screen-context"}},
		{"provider limits", collar.Primitives.ProviderLimits, []string{"claude-screen-limits", "codex-screen-limits"}},
		{"wedge", collar.Primitives.Wedge, []string{"stable-busy-screen"}},
	}
	if collar.Models.Selected != nil {
		checks = append(checks, struct {
			where   string
			value   Invocation
			allowed []string
		}{"selected model", *collar.Models.Selected, []string{"claude-screen-model", "codex-screen-model"}})
	}
	for index, startup := range collar.Primitives.Startup {
		checks = append(checks, struct {
			where   string
			value   Invocation
			allowed []string
		}{fmt.Sprintf("startup[%d]", index), startup, []string{"claude-trust-prompt", "codex-trust-prompt", "claude-composer", "codex-composer"}})
	}
	for _, check := range checks {
		known := false
		for _, allowed := range check.allowed {
			known = known || check.value.Name == allowed
		}
		if !known {
			return fmt.Errorf("%s names unknown primitive %q", check.where, check.value.Name)
		}
	}
	for _, name := range []string{"prompt", "choice"} {
		if _, err := blockedPattern(collar.Primitives.Blocked, name); err != nil {
			return err
		}
	}
	for selector, bands := range collar.ContextBands {
		seen := make(map[string]bool, len(bands))
		previous := -1.0
		for _, band := range bands {
			if seen[band.Name] || band.At <= previous {
				return fmt.Errorf("context bands for %q must have unique names and increasing thresholds", selector)
			}
			seen[band.Name] = true
			previous = band.At
		}
	}
	for name, action := range map[string]Action{
		"interrupt": collar.Actions.Interrupt,
		"compact":   collar.Actions.Compact,
	} {
		if action.Text == "" && len(action.Keys) == 0 && !action.Submit {
			return fmt.Errorf("%s action is empty", name)
		}
	}
	if len(collar.Actions.CompactRecover) == 0 {
		return fmt.Errorf("compact recovery declares no actions")
	}
	return nil
}

func argsContain(args []string, text string) bool {
	for _, arg := range args {
		if strings.Contains(arg, text) {
			return true
		}
	}
	return false
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
