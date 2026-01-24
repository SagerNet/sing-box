package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

func TestShadowTLS(t *testing.T) {
	t.Run("v1", func(t *testing.T) {
		testShadowTLS(t, 1, "", false, option.ShadowTLSWildcardSNIOff)
	})
	t.Run("v2", func(t *testing.T) {
		testShadowTLS(t, 2, "hello", false, option.ShadowTLSWildcardSNIOff)
	})
	t.Run("v3", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", false, option.ShadowTLSWildcardSNIOff)
	})
	t.Run("v2-utls", func(t *testing.T) {
		testShadowTLS(t, 2, "hello", true, option.ShadowTLSWildcardSNIOff)
	})
	t.Run("v3-utls", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", true, option.ShadowTLSWildcardSNIOff)
	})
	t.Run("v3-wildcard-sni-authed", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", false, option.ShadowTLSWildcardSNIAuthed)
	})
	t.Run("v3-wildcard-sni-all", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", false, option.ShadowTLSWildcardSNIAll)
	})
	t.Run("v3-wildcard-sni-authed-utls", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", true, option.ShadowTLSWildcardSNIAll)
	})
	t.Run("v3-wildcard-sni-all-utls", func(t *testing.T) {
		testShadowTLS(t, 3, "hello", true, option.ShadowTLSWildcardSNIAll)
	})
}

func testShadowTLS(t *testing.T, version int, password string, utlsEanbled bool, wildcardSNI option.WildcardSNI) {
	method := shadowaead_2022.List[0]
	ssPassword := mkBase64(t, 16)
	var clientServerName string
	if wildcardSNI != option.ShadowTLSWildcardSNIOff {
		clientServerName = "cloudflare.com"
	} else {
		clientServerName = "google.com"
	}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Tag:  "in",
				Options: &option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,

						InboundOptions: option.InboundOptions{
							Detour: "detour",
						},
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "google.com",
							ServerPort: 443,
						},
					},
					Version:     version,
					Password:    password,
					Users:       []option.ShadowTLSUser{{Password: password}},
					WildcardSNI: wildcardSNI,
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
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
				Options: &option.ShadowsocksOutboundOptions{
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
				Options: &option.ShadowTLSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:    true,
							ServerName: clientServerName,
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
				Options: &option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "bing.com",
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
	response, err := client.Get("https://bing.com")
	require.NoError(t, err)
	require.Equal(t, response.StatusCode, 200)
	response.Body.Close()
	client.CloseIdleConnections()
}

func TestShadowTLSFallbackWildcardAll(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeShadowTLS,
				Options: &option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Version: 3,
					Users: []option.ShadowTLSUser{
						{Password: "hello"},
					},
					WildcardSNI: option.ShadowTLSWildcardSNIAll,
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
	response, err := client.Get("https://www.bing.com")
	require.NoError(t, err)
	require.Equal(t, response.StatusCode, 200)
	response.Body.Close()
	client.CloseIdleConnections()
}

func TestShadowTLSFallbackWildcardAuthedFail(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeShadowTLS,
				Options: &option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "bing.com",
							ServerPort: 443,
						},
					},
					Version: 3,
					Users: []option.ShadowTLSUser{
						{Password: "hello"},
					},
					WildcardSNI: option.ShadowTLSWildcardSNIAuthed,
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
	_, err := client.Get("https://baidu.com")
	expected := &tls.CertificateVerificationError{}
	require.ErrorAs(t, err, &expected)
	client.CloseIdleConnections()
}

func TestShadowTLSFallbackWildcardOffFail(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeShadowTLS,
				Options: &option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Handshake: option.ShadowTLSHandshakeOptions{
						ServerOptions: option.ServerOptions{
							Server:     "bing.com",
							ServerPort: 443,
						},
					},
					Version: 3,
					Users: []option.ShadowTLSUser{
						{Password: "hello"},
					},
					WildcardSNI: option.ShadowTLSWildcardSNIOff,
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
	_, err := client.Get("https://baidu.com")
	expected := &tls.CertificateVerificationError{}
	require.ErrorAs(t, err, &expected)
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
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowTLS,
				Options: &option.ShadowTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
						InboundOptions: option.InboundOptions{
							Detour: "detour",
						},
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
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
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
				Options: &option.ShadowsocksOutboundOptions{
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
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "detour",
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
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
				Options: &option.ShadowsocksOutboundOptions{
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
				Options: &option.ShadowTLSOutboundOptions{
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
