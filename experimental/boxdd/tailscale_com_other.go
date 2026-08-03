//go:build !windows || !with_tailscale

package main

func initializeTailscaleCOMRuntime(bool) error {
	return nil
}
