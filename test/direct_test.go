package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

// Since this is a feature one-off added by outsiders, I won't address these anymore.
func _TestProxyProtocol(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeDirect,
				Options: &option.DirectInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:        common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
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
				Tag:  "proxy-out",
				Options: &option.DirectOutboundOptions{
					OverrideAddress: "127.0.0.1",
					OverridePort:    serverPort,
					ProxyProtocol:   2,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "proxy-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
