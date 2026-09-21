package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

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
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
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

func composeStartup(name string, prose startupProse, assignment string) string {
	var result bytes.Buffer
	fmt.Fprintf(&result, "You are %s in Gangline. Read the standing contract below before anything else.\n\n", name)
	result.Write(prose.Contract)
	if len(prose.Doctrine) != 0 {
		result.WriteString("\n\nOperator doctrine:\n\n")
		result.Write(prose.Doctrine)
	}
	if len(prose.Role) != 0 {
		result.WriteString("\n\nRole brief:\n\n")
		result.Write(prose.Role)
	}
	if assignment != "" {
		result.WriteString("\n\nAssignment:\n\n")
		result.WriteString(assignment)
	}
	return result.String()
}
