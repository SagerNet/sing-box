package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestProxyProtocol(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeDirect,
				DirectOptions: option.DirectInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:        option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort:    serverPort,
						ProxyProtocol: true,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeDirect,
				Tag:  "trojan-out",
				DirectOptions: option.DirectOutboundOptions{
					OverrideAddress: "127.0.0.1",
					OverridePort:    serverPort,
					ProxyProtocol:   2,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "trojan-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
