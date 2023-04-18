package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/vless"
)

func TestVLESSVisionReality(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	userUUID := newUUID()
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
				Type: C.TypeVLESS,
				VLESSOptions: option.VLESSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VLESSUser{
						{
							Name: "sekai",
							UUID: userUUID.String(),
							Flow: vless.FlowVision,
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						Reality: &option.InboundRealityOptions{
							Enabled: true,
							Handshake: option.InboundRealityHandshakeOptions{
								ServerOptions: option.ServerOptions{
									Server:     "google.com",
									ServerPort: 443,
								},
							},
							ShortID:    []string{"0123456789abcdef"},
							PrivateKey: "UuMBgl7MXTPx9inmQp2UC7Jcnwc6XYbwDNebonM-FCc",
						},
					},
				},
			},
			{
				Type: C.TypeTrojan,
				Tag:  "trojan",
				TrojanOptions: option.TrojanInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: otherPort,
					},
					Users: []option.TrojanUser{
						{
							Name:     "sekai",
							Password: userUUID.String(),
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeTrojan,
				Tag:  "trojan-out",
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: otherPort,
					},
					Password: userUUID.String(),
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
					DialerOptions: option.DialerOptions{
						Detour: "vless-out",
					},
				},
			},
			{
				Type: C.TypeVLESS,
				Tag:  "vless-out",
				VLESSOptions: option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID: userUUID.String(),
					Flow: vless.FlowVision,
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						Reality: &option.OutboundRealityOptions{
							Enabled:   true,
							ShortID:   "0123456789abcdef",
							PublicKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
						},
						UTLS: &option.OutboundUTLSOptions{
							Enabled: true,
						},
					},
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

func TestVLESSVisionRealityPlain(t *testing.T) {
	userUUID := newUUID()
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
				Type: C.TypeVLESS,
				VLESSOptions: option.VLESSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VLESSUser{
						{
							Name: "sekai",
							UUID: userUUID.String(),
							Flow: vless.FlowVision,
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						Reality: &option.InboundRealityOptions{
							Enabled: true,
							Handshake: option.InboundRealityHandshakeOptions{
								ServerOptions: option.ServerOptions{
									Server:     "google.com",
									ServerPort: 443,
								},
							},
							ShortID:    []string{"0123456789abcdef"},
							PrivateKey: "UuMBgl7MXTPx9inmQp2UC7Jcnwc6XYbwDNebonM-FCc",
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
				Type: C.TypeVLESS,
				Tag:  "vless-out",
				VLESSOptions: option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID: userUUID.String(),
					Flow: vless.FlowVision,
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						Reality: &option.OutboundRealityOptions{
							Enabled:   true,
							ShortID:   "0123456789abcdef",
							PublicKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
						},
						UTLS: &option.OutboundUTLSOptions{
							Enabled: true,
						},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"mixed-in"},
						Outbound: "vless-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestVLESSRealityTransport(t *testing.T) {
	t.Run("grpc", func(t *testing.T) {
		testVLESSRealityTransport(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
		})
	})
	t.Run("websocket", func(t *testing.T) {
		testVLESSRealityTransport(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
		})
	})
	t.Run("h2", func(t *testing.T) {
		testVLESSRealityTransport(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeHTTP,
		})
	})
}

func testVLESSRealityTransport(t *testing.T, transport *option.V2RayTransportOptions) {
	userUUID := newUUID()
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
				Type: C.TypeVLESS,
				VLESSOptions: option.VLESSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VLESSUser{
						{
							Name: "sekai",
							UUID: userUUID.String(),
						},
					},
					TLS: &option.InboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						Reality: &option.InboundRealityOptions{
							Enabled: true,
							Handshake: option.InboundRealityHandshakeOptions{
								ServerOptions: option.ServerOptions{
									Server:     "google.com",
									ServerPort: 443,
								},
							},
							ShortID:    []string{"0123456789abcdef"},
							PrivateKey: "UuMBgl7MXTPx9inmQp2UC7Jcnwc6XYbwDNebonM-FCc",
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
				Type: C.TypeVLESS,
				Tag:  "vless-out",
				VLESSOptions: option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID: userUUID.String(),
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
						Reality: &option.OutboundRealityOptions{
							Enabled:   true,
							ShortID:   "0123456789abcdef",
							PublicKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
						},
						UTLS: &option.OutboundUTLSOptions{
							Enabled: true,
						},
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
						Outbound: "vless-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
