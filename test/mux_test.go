package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"

	"github.com/gofrs/uuid/v5"
)

var muxProtocols = []string{
	"h2mux",
	"smux",
	"yamux",
}

func TestVMessSMux(t *testing.T) {
	testVMessMux(t, option.MultiplexOptions{
		Enabled:  true,
		Protocol: "smux",
	})
}

func TestShadowsocksMux(t *testing.T) {
	for _, protocol := range muxProtocols {
		t.Run(protocol, func(t *testing.T) {
			testShadowsocksMux(t, option.MultiplexOptions{
				Enabled:  true,
				Protocol: protocol,
			})
		})
	}
}

func TestShadowsockH2Mux(t *testing.T) {
	testShadowsocksMux(t, option.MultiplexOptions{
		Enabled:  true,
		Protocol: "h2mux",
		Padding:  true,
	})
}

func TestShadowsockSMuxPadding(t *testing.T) {
	testShadowsocksMux(t, option.MultiplexOptions{
		Enabled:  true,
		Protocol: "smux",
		Padding:  true,
	})
}

func testShadowsocksMux(t *testing.T, options option.MultiplexOptions) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowsocks,
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Method:   method,
					Password: password,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "ss-out",
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Method:           method,
					Password:         password,
					MultiplexOptions: &options,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "ss-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func testVMessMux(t *testing.T, options option.MultiplexOptions) {
	user, _ := uuid.NewV4()
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeVMess,
				VMessOptions: option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							UUID: user.String(),
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeVMess,
				Tag:  "vmess-out",
				VMessOptions: option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Security:  "auto",
					UUID:      user.String(),
					Multiplex: &options,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "vmess-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
