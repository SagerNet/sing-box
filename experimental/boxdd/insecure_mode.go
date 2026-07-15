package main

import (
	"path/filepath"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/tailscale/atomicfile"
)

const securitySettingsFileName = "security.json"

type securitySettings struct {
	InsecureModeEnabled bool `json:"insecure_mode_enabled"`
}

func saveSecuritySettings(directory string, settings securitySettings) error {
	content, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(directory, securitySettingsFileName), content, 0o600)
}
