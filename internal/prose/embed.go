package prose

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
)

//go:embed CONTRACT.md roles/*.md
var files embed.FS

// Contract returns the shipped standing contract.
func Contract() ([]byte, error) {
	data, err := files.ReadFile("CONTRACT.md")
	if err != nil {
		return nil, fmt.Errorf("read embedded contract: %w", err)
	}
	return data, nil
}

// Role returns a shipped role brief by name.
func Role(name string) ([]byte, error) {
	if name == "" || path.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return nil, fmt.Errorf("role name %q is invalid", name)
	}
	data, err := files.ReadFile(path.Join("roles", name+".md"))
	if err != nil {
		return nil, fmt.Errorf("read embedded role %q: %w", name, err)
	}
	return data, nil
}

// RoleNames returns the names of all shipped role briefs.
func RoleNames() ([]string, error) {
	entries, err := files.ReadDir("roles")
	if err != nil {
		return nil, fmt.Errorf("list embedded roles: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".md" {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}
