package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

func TestV2RayHTTPSelf(t *testing.T) {
	testV2RayTransportSelf(t, &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeHTTP,
		HTTPOptions: option.V2RayHTTPOptions{
			Method: "POST",
		},
	})
}

func TestV2RayHTTPPlainSelf(t *testing.T) {
	testV2RayTransportNOTLSSelf(t, &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeHTTP,
	})
}

func testV2RayTransportSelf(t *testing.T, transport *option.V2RayTransportOptions) {
	testV2RayTransportSelfWith(t, transport, transport)
}

func testV2RayTransportSelfWith(t *testing.T, server, client *option.V2RayTransportOptions) {
	t.Run("vmess", func(t *testing.T) {
		testVMessTransportSelf(t, server, client)
	})
	t.Run("trojan", func(t *testing.T) {
		testTrojanTransportSelf(t, server, client)
	})
}

func testVMessTransportSelf(t *testing.T, server *option.V2RayTransportOptions, client *option.V2RayTransportOptions) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
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
					Transport: server,
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
					Transport: client,
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

func testTrojanTransportSelf(t *testing.T, server *option.V2RayTransportOptions, client *option.V2RayTransportOptions) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
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
					Transport: server,
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
					Transport: client,
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

func testV2RayTransportNOTLSSelf(t *testing.T, transport *option.V2RayTransportOptions) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
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
	testSuit(t, clientPort, testPort)
}
