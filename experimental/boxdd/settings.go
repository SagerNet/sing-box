package main

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/sagernet/sing-box/experimental/locale"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/tailscale/atomicfile"
)

const (
	settingsFileName               = "settings.json"
	legacySecuritySettingsFileName = "security.json"
)

type daemonSettings struct {
	InsecureModeEnabled bool   `json:"insecure_mode_enabled"`
	Locale              string `json:"locale,omitempty"`
}

var settingsAccess sync.Mutex

func loadDaemonSettings(directory string) (daemonSettings, error) {
	content, err := os.ReadFile(filepath.Join(directory, settingsFileName))
	if os.IsNotExist(err) {
		content, err = os.ReadFile(filepath.Join(directory, legacySecuritySettingsFileName))
	}
	if err != nil {
		return daemonSettings{}, err
	}
	return json.UnmarshalExtended[daemonSettings](content)
}

func updateDaemonSettings(directory string, modify func(settings *daemonSettings)) error {
	settingsAccess.Lock()
	defer settingsAccess.Unlock()
	settings, err := loadDaemonSettings(directory)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	modify(&settings)
	content, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	err = atomicfile.WriteFile(filepath.Join(directory, settingsFileName), content, 0o600)
	if err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(directory, legacySecuritySettingsFileName))
	return nil
}

func restoreLocale() {
	settings, err := loadDaemonSettings(workingDirectory)
	if err != nil {
		return
	}
	locale.Set(settings.Locale)
}
