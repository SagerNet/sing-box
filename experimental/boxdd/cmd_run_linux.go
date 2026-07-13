package main

import (
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

func runService() (bool, error) {
	if os.Getenv("INVOCATION_ID") != "" && listenAddress != "" {
		return true, E.New("--listen is not allowed in service mode")
	}
	return false, nil
}

func preparePlatformWorkingDirectory() error {
	return os.MkdirAll(workingDirectory, 0o700)
}
