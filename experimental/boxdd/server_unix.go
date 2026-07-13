//go:build !windows

package main

import (
	"net"
	"os"
	"path/filepath"
)

func listenEndpoint() (net.Listener, error) {
	path := socketPath
	if path == "" {
		path = filepath.Join(workingDirectory, serviceName+".sock")
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	err = os.Chmod(path, 0o666)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}
