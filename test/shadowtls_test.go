package main

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	F "github.com/sagernet/sing/common/format"

	"github.com/stretchr/testify/require"
)

func TestShadowTLS(t *testing.T) {
	t.Run("v1", func(t *testing.T) {
		testShadowTLS(t, 1, "", false)
	})
	t.Run("v2", func(t *testing.T) {
		testShadowTLS(t, 2, "hello", false)
	})
	t.Run("v3", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", false)
	})
	t.Run("v2-utls", func(t *testing.T) {
		testShadowTLS(t, 2, "hello", true)
	})
	t.Run("v3-utls", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", true)
	})
}

func testShadowTLS(t *testing.T, version int, password string, utlsEanbled bool) {
	method := shadowaead_2022.List[0]
	ssPassword := mkBase64(t, 16)
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
				Type: C.TypeShadowTLS,
				Tag:  "in",
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
						Detour:     "detour",
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Version:  version,
					Password: password,
					Users:    []option.ShadowTLSUser{{Password: password}},
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: otherPort,
					},
					Method:   method,
					Password: ssPassword,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeShadowsocks,
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					Method:   method,
					Password: ssPassword,
					DialerOptions: option.DialerOptions{
						Detour: "detour",
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Tag:  "detour",
				ShadowTLSOptions: option.ShadowTLSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:    true,
							ServerName: "google.com",
							UTLS: &option.OutboundUTLSOptions{
								Enabled: utlsEanbled,
							},
						},
					},
					Version:  version,
					Password: password,
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
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"detour"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "direct",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestShadowTLSFallback(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeShadowTLS,
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Version: 3,
					Users: []option.ShadowTLSUser{
						{Password: "hello"},
					},
				},
			},
		},
	})
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, "127.0.0.1:"+F.ToString(serverPort))
			},
		},
	}
	response, err := client.Get("https://google.com")
	require.NoError(t, err)
	require.Equal(t, response.StatusCode, 200)
	response.Body.Close()
	client.CloseIdleConnections()
}

func TestShadowTLSInbound(t *testing.T) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowTLS,
		Ports:      []uint16{serverPort, otherPort},
		EntryPoint: "shadow-tls",
		Cmd:        []string{"--v3", "--threads", "1", "client", "--listen", "0.0.0.0:" + F.ToString(otherPort), "--server", "127.0.0.1:" + F.ToString(serverPort), "--sni", "google.com", "--password", password},
	})
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
				Type: C.TypeShadowTLS,
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
						Detour:     "detour",
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Version: 3,
					Users: []option.ShadowTLSUser{
						{Password: password},
					},
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen: option.NewListenAddress(netip.IPv4Unspecified()),
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
				Tag:  "out",
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: otherPort,
					},
					Method:   method,
					Password: password,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "out",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestShadowTLSOutbound(t *testing.T) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowTLS,
		Ports:      []uint16{serverPort, otherPort},
		EntryPoint: "shadow-tls",
		Cmd:        []string{"--v3", "--threads", "1", "server", "--listen", "0.0.0.0:" + F.ToString(serverPort), "--server", "127.0.0.1:" + F.ToString(otherPort), "--tls", "google.com:443", "--password", "hello"},
		Env:        []string{"RUST_LOG=trace"},
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
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
						ListenPort: otherPort,
					},
					Method:   method,
					Password: password,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeShadowsocks,
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					Method:   method,
					Password: password,
					DialerOptions: option.DialerOptions{
						Detour: "detour",
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Tag:  "detour",
				ShadowTLSOptions: option.ShadowTLSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:    true,
							ServerName: "google.com",
						},
					},
					Version:  3,
					Password: "hello",
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
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"detour"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "direct",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}
