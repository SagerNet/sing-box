package main

import (
	"net/netip"
	"os"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/vless"

	"github.com/gofrs/uuid/v5"
	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/require"
)

func TestVLESS(t *testing.T) {
	content, err := os.ReadFile("config/vless-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	user := newUUID()
	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("id").SetString(user.String())

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageV2RayCore,
		Ports:      []uint16{serverPort},
		EntryPoint: "v2ray",
		Cmd:        []string{"run"},
		Stdin:      content,
	})

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeVLESS,
				VLESSOptions: option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID: user.String(),
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestVLESSXRay(t *testing.T) {
	t.Run("origin", func(t *testing.T) {
		testVLESSXrayOutbound(t, "", "")
	})
	t.Run("xudp", func(t *testing.T) {
		testVLESSXrayOutbound(t, "xudp", "")
	})
	t.Run("vision", func(t *testing.T) {
		testVLESSXrayOutbound(t, "xudp", vless.FlowVision)
	})
}

func testVLESSXrayOutbound(t *testing.T, packetEncoding string, flow string) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	content, err := os.ReadFile("config/vless-tls-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	userID := newUUID()
	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	user := inbound.MustKey("settings").MustKey("clients").MustIndex(0)
	user.MustKey("id").SetString(userID.String())
	user.MustKey("flow").SetString(flow)

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageXRayCore,
		Ports:      []uint16{serverPort},
		EntryPoint: "xray",
		Stdin:      content,
		Bind: map[string]string{
			certPem: "/path/to/certificate.crt",
			keyPem:  "/path/to/private.key",
		},
	})

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
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
							Password: userID.String(),
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
				Type: C.TypeTrojan,
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "host.docker.internal",
						ServerPort: otherPort,
					},
					Password: userID.String(),
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
					DialerOptions: option.DialerOptions{
						Detour: "vless",
					},
				},
			},
			{
				Type: C.TypeVLESS,
				Tag:  "vless",
				VLESSOptions: option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID:           userID.String(),
					Flow:           flow,
					PacketEncoding: &packetEncoding,
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
				},
			},
			{
				Type: C.TypeDirect,
				Tag:  "direct",
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					DefaultOptions: option.DefaultRule{
						Inbound:  []string{"trojan"},
						Outbound: "direct",
					},
				},
			},
		},
	})

	testSuit(t, clientPort, testPort)
}

func TestVLESSSelf(t *testing.T) {
	t.Run("origin", func(t *testing.T) {
		testVLESSSelf(t, "")
	})
	t.Run("vision", func(t *testing.T) {
		testVLESSSelf(t, vless.FlowVision)
	})
	t.Run("vision-tls", func(t *testing.T) {
		testVLESSSelfTLS(t, vless.FlowVision)
	})
}

func testVLESSSelf(t *testing.T, flow string) {
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
							Flow: flow,
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
				Type: C.TypeVLESS,
				Tag:  "vless-out",
				VLESSOptions: option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID: userUUID.String(),
					Flow: flow,
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
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

func testVLESSSelfTLS(t *testing.T, flow string) {
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
							Flow: flow,
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
					Flow: flow,
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
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

func TestVLESSXrayInbound(t *testing.T) {
	testVLESSXrayInbound(t, vless.FlowVision)
}

func testVLESSXrayInbound(t *testing.T, flow string) {
	userId, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
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
							UUID: userId.String(),
							Flow: flow,
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
							Password: userId.String(),
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
	})

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: otherClientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeTrojan,
				Tag:  "trojan-out",
				TrojanOptions: option.TrojanOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: otherPort,
					},
					Password: userId.String(),
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
				Type: C.TypeSOCKS,
				Tag:  "vless-out",
				SocksOptions: option.SocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: clientPort,
					},
				},
			},
		},
	})

	content, err := os.ReadFile("config/vless-tls-client.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	config.MustKey("inbounds").MustIndex(0).MustKey("port").SetNumeric(float64(clientPort))
	outbound := config.MustKey("outbounds").MustIndex(0)
	settings := outbound.MustKey("settings").MustKey("vnext").MustIndex(0)
	settings.MustKey("port").SetNumeric(float64(serverPort))
	user := settings.MustKey("users").MustIndex(0)
	user.MustKey("id").SetString(userId.String())
	user.MustKey("flow").SetString(flow)
	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageXRayCore,
		Ports:      []uint16{clientPort},
		EntryPoint: "xray",
		Stdin:      content,
		Bind: map[string]string{
			certPem: "/path/to/certificate.crt",
			keyPem:  "/path/to/private.key",
		},
	})
	testTCP(t, otherClientPort, testPort)
}
