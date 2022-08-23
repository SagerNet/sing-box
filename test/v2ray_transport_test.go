package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

func TestV2RayGRPCSelf(t *testing.T) {
	testV2RayTransportSelf(t, &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeGRPC,
		GRPCOptions: option.V2RayGRPCOptions{
			ServiceName: "TunService",
		},
	})
}

func TestV2RayWebscoketSelf(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		testV2RayTransportSelf(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
		})
	})
	t.Run("v2ray early data", func(t *testing.T) {
		testV2RayTransportSelf(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				MaxEarlyData: 2048,
			},
		})
	})
	t.Run("xray early data", func(t *testing.T) {
		testV2RayTransportSelf(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				MaxEarlyData:        2048,
				EarlyDataHeaderName: "Sec-WebSocket-Protocol",
			},
		})
	})
}

func TestV2RayHTTPSelf(t *testing.T) {
	testV2RayTransportSelf(t, &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeHTTP,
	})
}

func TestV2RayHTTPPlainSelf(t *testing.T) {
	testV2RayTransportNOTLSSelf(t, &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeHTTP,
	})
}

func testV2RayTransportSelf(t *testing.T, transport *option.V2RayTransportOptions) {
	t.Run("vmess", func(t *testing.T) {
		testVMessTransportSelf(t, transport)
	})
	t.Run("trojan", func(t *testing.T) {
		testTrojanTransportSelf(t, transport)
	})
}

func testVMessTransportSelf(t *testing.T, transport *option.V2RayTransportOptions) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
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
				Type: C.TypeVMess,
				VMessOptions: option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							Name: "sekai",
							UUID: user.String(),
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
					Transport: transport,
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
					UUID:     user.String(),
					Security: "zero",
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
					Transport: transport,
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

func testTrojanTransportSelf(t *testing.T, transport *option.V2RayTransportOptions) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
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
				Type: C.TypeTrojan,
				TrojanOptions: option.TrojanInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.TrojanUser{
						{
							Name:     "sekai",
							Password: user.String(),
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
					Transport: transport,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeTrojan,
				Tag:  "vmess-out",
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password: user.String(),
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
					Transport: transport,
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

func TestVMessQUICSelf(t *testing.T) {
	transport := &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeQUIC,
	}
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
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
				Type: C.TypeVMess,
				VMessOptions: option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							Name: "sekai",
							UUID: user.String(),
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
					Transport: transport,
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
					UUID:     user.String(),
					Security: "zero",
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
					Transport: transport,
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
	testSuitQUIC(t, clientPort, testPort)
}

func testV2RayTransportNOTLSSelf(t *testing.T, transport *option.V2RayTransportOptions) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "trace",
		},
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
				Type: C.TypeVMess,
				VMessOptions: option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							Name: "sekai",
							UUID: user.String(),
						},
					},
					Transport: transport,
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
					UUID:      user.String(),
					Security:  "zero",
					Transport: transport,
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
	testSuitQUIC(t, clientPort, testPort)
}
