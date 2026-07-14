//go:build !windows

package main

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *Daemon) installUpdate(identity peerIdentity, installerPath string) (*InstallUpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "update installation is not supported on this platform")
}
