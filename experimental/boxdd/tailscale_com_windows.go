//go:build with_tailscale

package main

import "github.com/dblohm7/wingoes/com"

func initializeTailscaleCOMRuntime(isWindowsService bool) error {
	processType := com.ConsoleApp
	if isWindowsService {
		processType = com.Service
	}
	return com.StartRuntime(processType)
}
