//go:build !windows && !(linux && !android)

package main

func newPlatformInterface(daemon *Daemon) (daemonPlatform, error) {
	return nil, nil
}
