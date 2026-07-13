//go:build !windows

package main

import (
	"io"
	"net"
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

type unixWorkerParent struct{}

func (unixWorkerParent) Close() error {
	return nil
}

func prepareWorkerParent(parentProcessID uint32) (workerParent, error) {
	if uint32(os.Getppid()) != parentProcessID {
		return nil, E.New("worker was not started by the expected application process")
	}
	return unixWorkerParent{}, nil
}

func listenWorkerEndpoint(path string, parent workerParent) (net.Listener, error) {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	err = os.Chmod(path, 0o600)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func startWorkerDaemonRelay(path string, parent workerParent, onFailure func(error)) (io.Closer, error) {
	if path != "" {
		return nil, E.New("daemon relay is only supported on Windows")
	}
	return nil, nil
}
