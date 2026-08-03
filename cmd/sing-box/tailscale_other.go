//go:build !windows || !with_tailscale

package main

import "github.com/sagernet/sing-box/option"

func initializeTailscaleCOM(option.Options) error {
	return nil
}
