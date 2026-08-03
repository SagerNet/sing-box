//go:build with_tailscale

package main

import (
	"sync"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/dblohm7/wingoes/com"
)

var startTailscaleCOMRuntime = sync.OnceValue(func() error {
	return com.StartRuntime(com.ConsoleApp)
})

func initializeTailscaleCOM(options option.Options) error {
	for _, endpoint := range options.Endpoints {
		if endpoint.Type != C.TypeTailscale {
			continue
		}
		endpointOptions, loaded := endpoint.Options.(*option.TailscaleEndpointOptions)
		if loaded && endpointOptions.SystemInterface {
			return startTailscaleCOMRuntime()
		}
	}
	return nil
}
