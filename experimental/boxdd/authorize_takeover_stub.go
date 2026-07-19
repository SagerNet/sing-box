//go:build !linux

package main

import "context"

func authorizeTakeOver(ctx context.Context, identity peerIdentity) error {
	return nil
}
