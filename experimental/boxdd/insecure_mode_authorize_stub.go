//go:build !linux

package main

func authorizeDisableInsecureMode(identity peerIdentity) error {
	return nil
}
