//go:build !windows

package main

func newPlatformInterface(daemon *Daemon) (daemonPlatform, error) {
	return nil, nil
}
