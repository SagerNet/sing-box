//go:build linux

package main

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
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

func (d *Daemon) insecureModeEnabled() bool {
	settings, err := loadDaemonSettings(workingDirectory)
	if err != nil {
		return false
	}
	return settings.InsecureModeEnabled
}
