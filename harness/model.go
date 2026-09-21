package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type Model struct {
	ID      string
	Efforts []string
}

type ModelCatalog struct {
	Models   []Model
	Complete bool
}

type ModelValidation string

const (
	ModelRecognized   ModelValidation = "recognized"
	ModelUnrecognized ModelValidation = "unrecognized"
	ModelUnknown      ModelValidation = "unknown"
)

func DiscoverModels(ctx context.Context, collar Collar) (ModelCatalog, error) {
	command := collar.Models.Catalog.Params["command"]
	if command == "" {
		return ModelCatalog{}, fmt.Errorf("model catalog primitive %q declares no command", collar.Models.Catalog.Name)
	}
	args := strings.Fields(collar.Models.Catalog.Params["args"])
	output, err := exec.CommandContext(ctx, command, args...).Output()
	if err != nil {
		return ModelCatalog{}, fmt.Errorf("run %s model catalog: %w", collar.Name, err)
	}
	return ParseModelCatalog(collar.Models.Catalog, output)
}

func ParseModelCatalog(invocation Invocation, output []byte) (ModelCatalog, error) {
	switch invocation.Name {
	case "codex-debug-models":
		return parseCodexModels(output)
	case "claude-help-models":
		return parseClaudeModels(string(output))
	default:
		return ModelCatalog{}, fmt.Errorf("unknown model catalog primitive %q", invocation.Name)
	}
}

func ValidateModel(catalog ModelCatalog, id string) ModelValidation {
	for _, model := range catalog.Models {
		if model.ID == id {
			return ModelRecognized
		}
	}
	if catalog.Complete {
		return ModelUnrecognized
	}
	return ModelUnknown
}

func ValidateEffort(catalog ModelCatalog, id, effort string) ModelValidation {
	for _, model := range catalog.Models {
		if model.ID != id {
			continue
		}
		for _, supported := range model.Efforts {
			if supported == effort {
				return ModelRecognized
			}
		}
		return ModelUnrecognized
	}
	if catalog.Complete {
		return ModelUnrecognized
	}
	return ModelUnknown
}

func ReadSelectedModel(invocation Invocation, screen substrate.Screen) (string, error) {
	lines := screenLines(screen, true)
	switch invocation.Name {
	case "codex-screen-model":
		pattern := regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)\s+(?:[A-Za-z0-9_-]+\s+)?·`)
		for index := len(lines) - 1; index >= 0; index-- {
			if match := pattern.FindStringSubmatch(strings.TrimSpace(lines[index])); len(match) != 0 {
				return match[1], nil
			}
		}
	case "claude-screen-model":
		pattern := regexp.MustCompile(`(?i)^(?:current )?model:[[:space:]]+([A-Za-z0-9][A-Za-z0-9._-]*)`)
		header := regexp.MustCompile(`(?i)\b(fable|opus|sonnet|haiku)\b(?:[[:space:]]+[0-9.]+)?[[:space:]]+·`)
		for index := len(lines) - 1; index >= 0; index-- {
			if match := pattern.FindStringSubmatch(strings.TrimSpace(lines[index])); len(match) != 0 {
				return match[1], nil
			}
			if match := header.FindStringSubmatch(lines[index]); len(match) != 0 {
				return strings.ToLower(match[1]), nil
			}
		}
	default:
		return "", fmt.Errorf("unknown selected-model primitive %q", invocation.Name)
	}
	return "", errors.New("screen carries no selected model id")
}

func parseCodexModels(output []byte) (ModelCatalog, error) {
	var document struct {
		Models []struct {
			ID      string `json:"slug"`
			Efforts []struct {
				Name string `json:"effort"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(output, &document); err != nil {
		return ModelCatalog{}, fmt.Errorf("decode codex model catalog: %w", err)
	}
	if len(document.Models) == 0 {
		return ModelCatalog{}, errors.New("codex model catalog is empty")
	}
	seen := make(map[string]bool, len(document.Models))
	models := make([]Model, 0, len(document.Models))
	for _, source := range document.Models {
		if source.ID == "" || seen[source.ID] {
			return ModelCatalog{}, errors.New("codex model catalog has an empty or duplicate id")
		}
		seen[source.ID] = true
		model := Model{ID: source.ID, Efforts: make([]string, 0, len(source.Efforts))}
		efforts := make(map[string]bool, len(source.Efforts))
		for _, effort := range source.Efforts {
			if effort.Name == "" || efforts[effort.Name] {
				return ModelCatalog{}, fmt.Errorf("codex model %q has an empty or duplicate effort", source.ID)
			}
			efforts[effort.Name] = true
			model.Efforts = append(model.Efforts, effort.Name)
		}
		models = append(models, model)
	}
	return ModelCatalog{Models: models, Complete: true}, nil
}

func parseClaudeModels(output string) (ModelCatalog, error) {
	modelBlock := regexp.MustCompile(`(?s)--model <model>.*?(?:\n\s*-|$)`).FindString(output)
	if modelBlock == "" {
		return ModelCatalog{}, errors.New("claude help has no model option")
	}
	aliases := regexp.MustCompile(`'([a-z][a-z0-9-]*)'`).FindAllStringSubmatch(modelBlock, -1)
	if len(aliases) == 0 {
		return ModelCatalog{}, errors.New("claude help has no model aliases")
	}
	efforts, err := parseClaudeEfforts(output)
	if err != nil {
		return ModelCatalog{}, err
	}
	seen := make(map[string]bool, len(aliases))
	models := make([]Model, 0, len(aliases))
	for _, alias := range aliases {
		if strings.HasPrefix(alias[1], "claude-") {
			continue
		}
		if seen[alias[1]] {
			continue
		}
		seen[alias[1]] = true
		models = append(models, Model{ID: alias[1], Efforts: append([]string(nil), efforts...)})
	}
	sort.Slice(models, func(left, right int) bool { return models[left].ID < models[right].ID })
	return ModelCatalog{Models: models, Complete: false}, nil
}

func parseClaudeEfforts(output string) ([]string, error) {
	block := regexp.MustCompile(`(?s)--effort <level>.*?(?:\n\s*-|$)`).FindString(output)
	if block == "" {
		return nil, errors.New("claude help has no effort option")
	}
	list := regexp.MustCompile(`\(([a-z0-9-]+(?:\s*,\s*[a-z0-9-]+)+)\)`).FindStringSubmatch(block)
	if len(list) != 2 {
		return nil, errors.New("claude help effort vocabulary is unreadable")
	}
	parts := strings.Split(list[1], ",")
	seen := make(map[string]bool, len(parts))
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
		if parts[index] == "" || seen[parts[index]] {
			return nil, errors.New("claude help effort vocabulary has an empty or duplicate value")
		}
		seen[parts[index]] = true
	}
	return parts, nil
}
