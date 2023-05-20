package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestBadsocks(t *testing.T) {
	password := "test"
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeBadsocks,
				BadsocksOptions: option.BadsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Password: password,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeBadsocks,
				Tag:  "out",
				BadsocksOptions: option.BadsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password: password,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"in"},
						Outbound: "out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
