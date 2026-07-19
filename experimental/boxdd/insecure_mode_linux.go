//go:build linux

package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
)

func registerSecurityPolicy(ctx context.Context, daemon *Daemon) {
	service.MustRegister[adapter.SecurityPolicy](ctx, &daemonSecurityPolicy{daemon})
	service.MustRegister[filemanager.Manager](ctx, &restrictedFileManager{daemon})
}

func insecureModeAvailable() bool {
	return true
}

func insecureModePlatformName() string {
	return "Linux"
}

func loadSecuritySettings(directory string) (securitySettings, error) {
	content, err := os.ReadFile(filepath.Join(directory, securitySettingsFileName))
	if err != nil {
		return securitySettings{}, err
	}
	settings, err := json.UnmarshalExtended[securitySettings](content)
	if err != nil {
		return securitySettings{}, err
	}
	return settings, nil
}

func (d *Daemon) insecureModeEnabled() bool {
	settings, err := loadSecuritySettings(workingDirectory)
	if err != nil {
		return false
	}
	return settings.InsecureModeEnabled
}
