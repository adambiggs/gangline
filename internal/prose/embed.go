package prose

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
)

//go:embed CONTRACT.md roles/*.md
var prose embed.FS

func Contract() ([]byte, error) {
	data, err := prose.ReadFile("CONTRACT.md")
	if err != nil {
		return nil, fmt.Errorf("read embedded contract: %w", err)
	}
	return data, nil
}

func Role(name string) ([]byte, error) {
	if name == "" || path.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return nil, fmt.Errorf("role name %q is invalid", name)
	}
	data, err := prose.ReadFile(path.Join("roles", name+".md"))
	if err != nil {
		return nil, fmt.Errorf("read embedded role %q: %w", name, err)
	}
	return data, nil
}

func RoleNames() ([]string, error) {
	entries, err := prose.ReadDir("roles")
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
