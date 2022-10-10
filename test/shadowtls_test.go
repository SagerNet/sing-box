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
		testShadowTLS(t, "")
	})
	t.Run("v2", func(t *testing.T) {
		testShadowTLS(t, "hello")
	})
}

func testShadowTLS(t *testing.T, password string) {
	method := shadowaead_2022.List[0]
	ssPassword := mkBase64(t, 16)
	var version int
	if password != "" {
		version = 2
	} else {
		version = 1
	}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Tag:  "in",
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
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
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
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
					MultiplexOptions: &option.MultiplexOptions{
						Enabled: true,
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
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
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
			Rules: []option.Rule{{
				DefaultOptions: option.DefaultRule{
					Inbound:  []string{"detour"},
					Outbound: "direct",
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestShadowTLSv2Fallback(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeShadowTLS,
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Password: "hello",
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
	client.CloseIdleConnections()
}

func TestShadowTLSInbound(t *testing.T) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowTLS,
		Ports:      []uint16{serverPort, otherPort},
		EntryPoint: "shadow-tls",
		Cmd:        []string{"--threads", "1", "client", "--listen", "0.0.0.0:" + F.ToString(otherPort), "--server", "127.0.0.1:" + F.ToString(serverPort), "--sni", "google.com", "--password", password},
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "in",
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
						Detour:     "detour",
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Version:  2,
					Password: password,
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen: option.ListenAddress(netip.IPv4Unspecified()),
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
					MultiplexOptions: &option.MultiplexOptions{
						Enabled: true,
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				DefaultOptions: option.DefaultRule{
					Inbound:  []string{"in"},
					Outbound: "out",
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestShadowTLSOutbound(t *testing.T) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowTLS,
		Ports:      []uint16{serverPort, otherPort},
		EntryPoint: "shadow-tls",
		Cmd:        []string{"--threads", "1", "server", "--listen", "0.0.0.0:" + F.ToString(serverPort), "--server", "127.0.0.1:" + F.ToString(otherPort), "--tls", "google.com:443", "--password", "hello"},
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Tag:  "in",
				ShadowTLSOptions: option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
						Detour:     "detour",
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Version:  2,
					Password: password,
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				ShadowsocksOptions: option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
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
					MultiplexOptions: &option.MultiplexOptions{
						Enabled: true,
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
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "google.com",
					},
					Password: "hello",
				},
			},
			{
				Type: C.TypeDirect,
				Tag:  "direct",
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				DefaultOptions: option.DefaultRule{
					Inbound:  []string{"detour"},
					Outbound: "direct",
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}
