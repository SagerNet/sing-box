//go:build !windows

package main

import (
	"context"

	"google.golang.org/grpc"
)

func platformServerOptions(daemon *Daemon) ([]grpc.ServerOption, error) {
	return nil, nil
}

func platformFallbackPeerIdentity(ctx context.Context) (peerIdentity, error) {
	return peerIdentity{UserID: "local"}, nil
}
