//go:build !windows && !linux

package main

import "context"

func registerSecurityPolicy(ctx context.Context, daemon *Daemon) {
}

func insecureModeAvailable() bool {
	return false
}

func (d *Daemon) insecureModeEnabled() bool {
	return false
}
