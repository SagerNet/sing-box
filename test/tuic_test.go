package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/gofrs/uuid/v5"
)

func TestTUICSelf(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		testTUICSelf(t, false, false)
	})
	t.Run("self-udp-stream", func(t *testing.T) {
		testTUICSelf(t, true, false)
	})
	t.Run("self-early", func(t *testing.T) {
		testTUICSelf(t, false, true)
	})
}

func testTUICSelf(t *testing.T, udpStream bool, zeroRTTHandshake bool) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	var udpRelayMode string
	if udpStream {
		udpRelayMode = "quic"
	}
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
				Type: C.TypeTUIC,
				TUICOptions: option.TUICInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.TUICUser{{
						UUID: uuid.Nil.String(),
					}},
					ZeroRTTHandshake: zeroRTTHandshake,
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
				Type: C.TypeTUIC,
				Tag:  "tuic-out",
				TUICOptions: option.TUICOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID:             uuid.Nil.String(),
					UDPRelayMode:     udpRelayMode,
					ZeroRTTHandshake: zeroRTTHandshake,
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
						Outbound: "tuic-out",
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestTUICInbound(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeTUIC,
				TUICOptions: option.TUICInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.TUICUser{{
						UUID:     "FE35D05B-8803-45C4-BAE6-723AD2CD5D3D",
						Password: "tuic",
					}},
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
	startDockerContainer(t, DockerOptions{
		Image: ImageTUICClient,
		Ports: []uint16{serverPort, clientPort},
		Bind: map[string]string{
			"tuic-client.json": "/etc/tuic/config.json",
			caPem:              "/etc/tuic/ca.pem",
		},
	})
}

func TestTUICOutbound(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startDockerContainer(t, DockerOptions{
		Image: ImageTUICServer,
		Ports: []uint16{testPort},
		Bind: map[string]string{
			"tuic-server.json": "/etc/tuic/config.json",
			certPem:            "/etc/tuic/cert.pem",
			keyPem:             "/etc/tuic/key.pem",
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
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeTUIC,
				TUICOptions: option.TUICOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID:     "FE35D05B-8803-45C4-BAE6-723AD2CD5D3D",
					Password: "tuic",
					TLS: &option.OutboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
