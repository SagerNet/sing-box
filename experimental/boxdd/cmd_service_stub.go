//go:build !windows && !linux

package main

import (
	E "github.com/sagernet/sing/common/exceptions"
)

const defaultServiceWorkingDirectory = ""

func addPlatformServiceCommands() {
}

func serviceStart() error {
	return E.New("service management is not supported on this platform")
}

func serviceStop() error {
	return E.New("service management is not supported on this platform")
}

func serviceStatus() (*serviceStatusResult, error) {
	return nil, E.New("service management is not supported on this platform")
}
