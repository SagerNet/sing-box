//go:build linux

package main

import (
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authorizeDisableInsecureMode(identity peerIdentity) error {
	ownerUserID, err := loadOwner()
	if err != nil {
		if os.IsNotExist(err) {
			return status.Error(codes.PermissionDenied, "the service has no owner")
		}
		return err
	}
	if ownerUserID != identity.UserID {
		return status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	return nil
}
