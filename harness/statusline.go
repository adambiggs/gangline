package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallStatusline replaces only the deleted Gangline status-line script (or
// an absent setting). Operator-authored status lines remain operator-owned.
func InstallStatusline(path, executable string) (bool, error) {
	settings := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, fmt.Errorf("decode Claude settings: %w", err)
		}
		if settings == nil {
			return false, fmt.Errorf("Claude settings must be an object")
		}
	}
	if old, ok := settings["statusLine"]; ok {
		var setting struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		}
		if err := json.Unmarshal(old, &setting); err != nil {
			return false, fmt.Errorf("decode statusLine setting: %w", err)
		}
		command := strings.Trim(strings.TrimSpace(setting.Command), "\"'")
		if setting.Type != "command" || !strings.HasSuffix(command, "/statusline/claude-code-context.sh") {
			return false, nil
		}
	}
	replacement, err := json.Marshal(map[string]string{"type": "command", "command": shellJoin([]string{executable, "statusline"})})
	if err != nil {
		return false, err
	}
	settings["statusLine"] = replacement
	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gang-statusline-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}
