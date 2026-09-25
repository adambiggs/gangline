package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/internal/prose"
)

const maximumProseBytes = 256 << 10

type startupProse struct {
	Contract []byte
	Doctrine []byte
	Role     []byte
}

func (cmd command) startupProse(role string) (startupProse, error) {
	settings, err := cmd.settings()
	if err != nil {
		return startupProse{}, err
	}
	contract, err := readOptionalProse(filepath.Join(settings.ConfigDir, "CONTRACT.md"))
	if err != nil {
		return startupProse{}, err
	}
	if contract == nil {
		contract, err = prose.Contract()
		if err != nil {
			return startupProse{}, err
		}
	}
	if err := validateProse("contract", contract); err != nil {
		return startupProse{}, err
	}
	doctrine, err := readOptionalProse(filepath.Join(settings.ConfigDir, "DOCTRINE.md"))
	if err != nil {
		return startupProse{}, err
	}
	if doctrine != nil {
		if err := validateProse("doctrine", doctrine); err != nil {
			return startupProse{}, err
		}
	}

	var roleBody []byte
	if role != "" {
		roleBody, err = readOptionalProse(filepath.Join(settings.ConfigDir, "roles", role+".md"))
		if err != nil {
			return startupProse{}, err
		}
		if roleBody == nil {
			roleBody, err = prose.Role(role)
			if err != nil {
				return startupProse{}, fmt.Errorf("role %q is not available", role)
			}
		}
		if err := validateProse("role "+role, roleBody); err != nil {
			return startupProse{}, err
		}
	}
	return startupProse{Contract: contract, Doctrine: doctrine, Role: roleBody}, nil
}

func readOptionalProse(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	defer file.Close()
	return readProse(filename, file)
}

func readProse(label string, reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maximumProseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if err := validateProse(label, data); err != nil {
		return nil, err
	}
	return data, nil
}

func validateProse(label string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("%s is empty", label)
	}
	if len(data) > maximumProseBytes {
		return fmt.Errorf("%s is larger than %d bytes", label, maximumProseBytes)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return fmt.Errorf("%s must be UTF-8 text without NUL bytes", label)
	}
	return nil
}

func startupMessages(name string, prose startupProse, assignment string, systemPrompt bool) (prompt, message string) {
	message = "No assignment was supplied."
	if assignment != "" {
		message = "Assignment:\n\n" + assignment
	}
	if systemPrompt {
		return composeStartup(name, prose), message
	}
	return "", message
}

func startupSections(name string, prose startupProse) *core.StartupSections {
	sections := &core.StartupSections{
		Contract: fmt.Sprintf("You are %s in Gangline. Read the standing contract below before anything else.\n\n%s", name, prose.Contract),
	}
	if len(prose.Doctrine) != 0 {
		sections.Doctrine = "Operator doctrine:\n\n" + string(prose.Doctrine)
	}
	if len(prose.Role) != 0 {
		sections.Role = "Role brief (operator instructions take precedence, including provider, model, effort, and staffing policy):\n\n" + string(prose.Role)
	}
	return sections
}

func composeStartup(name string, prose startupProse) string {
	var result bytes.Buffer
	fmt.Fprintf(&result, "You are %s in Gangline. Read the standing contract below before anything else.\n\n", name)
	result.Write(prose.Contract)
	if len(prose.Doctrine) != 0 {
		result.WriteString("\n\nOperator doctrine:\n\n")
		result.Write(prose.Doctrine)
	}
	if len(prose.Role) != 0 {
		result.WriteString("\n\nRole brief (operator instructions take precedence, including provider, model, effort, and staffing policy):\n\n")
		result.Write(prose.Role)
	}
	return result.String()
}
