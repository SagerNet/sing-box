package main

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestV2RayXHTTP(t *testing.T) {
	for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) {
			testV2RayTransportSelf(t, &option.V2RayTransportOptions{
				Type: C.V2RayTransportTypeXHTTP,
				XHTTPOptions: option.V2RayXHTTPOptions{
					Path: "/xhttp",
					Mode: mode,
				},
			})
		})
	}
}
