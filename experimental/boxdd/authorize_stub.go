//go:build !linux

package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authorizeTakeOver(ctx context.Context, identity peerIdentity) error {
	return nil
}

func authorizeSetInsecureMode(ctx context.Context, identity peerIdentity, enabled bool) error {
	if enabled {
		return status.Error(codes.PermissionDenied, "enabling insecure mode requires an elevated service command")
	}
	return nil
}
