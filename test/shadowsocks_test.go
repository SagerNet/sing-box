package main

import (
	"crypto/rand"
	"encoding/base64"
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

const (
	serverPort uint16 = 10000 + iota
	clientPort
	testPort
	otherPort
	otherClientPort
)

func TestShadowsocks(t *testing.T) {
	for _, method := range []string{
		"aes-128-gcm",
		"aes-256-gcm",
		"chacha20-ietf-poly1305",
	} {
		t.Run(method+"-inbound", func(t *testing.T) {
			testShadowsocksInboundWithShadowsocksRust(t, method, mkBase64(t, 16))
		})
		t.Run(method+"-outbound", func(t *testing.T) {
			testShadowsocksOutboundWithShadowsocksRust(t, method, mkBase64(t, 16))
		})
		t.Run(method+"-self", func(t *testing.T) {
			testShadowsocksSelf(t, method, mkBase64(t, 16))
		})
	}
}

func TestShadowsocksNone(t *testing.T) {
	testShadowsocksSelf(t, "none", "")
}

func TestShadowsocks2022(t *testing.T) {
	for _, method16 := range []string{
		"2022-blake3-aes-128-gcm",
	} {
		t.Run(method16+"-inbound", func(t *testing.T) {
			testShadowsocksInboundWithShadowsocksRust(t, method16, mkBase64(t, 16))
		})
		t.Run(method16+"-outbound", func(t *testing.T) {
			testShadowsocksOutboundWithShadowsocksRust(t, method16, mkBase64(t, 16))
		})
		t.Run(method16+"-self", func(t *testing.T) {
			testShadowsocksSelf(t, method16, mkBase64(t, 16))
		})
	}
	for _, method32 := range []string{
		"2022-blake3-aes-256-gcm",
		"2022-blake3-chacha20-poly1305",
	} {
		t.Run(method32+"-inbound", func(t *testing.T) {
			testShadowsocksInboundWithShadowsocksRust(t, method32, mkBase64(t, 32))
		})
		t.Run(method32+"-outbound", func(t *testing.T) {
			testShadowsocksOutboundWithShadowsocksRust(t, method32, mkBase64(t, 32))
		})
		t.Run(method32+"-self", func(t *testing.T) {
			testShadowsocksSelf(t, method32, mkBase64(t, 32))
		})
	}
}

func TestShadowsocks2022EIH(t *testing.T) {
	for _, method16 := range []string{
		"2022-blake3-aes-128-gcm",
	} {
		t.Run(method16, func(t *testing.T) {
			testShadowsocks2022EIH(t, method16, mkBase64(t, 16))
		})
	}
	for _, method32 := range []string{
		"2022-blake3-aes-256-gcm",
	} {
		t.Run(method32, func(t *testing.T) {
			testShadowsocks2022EIH(t, method32, mkBase64(t, 32))
		})
	}
}

func testShadowsocksInboundWithShadowsocksRust(t *testing.T, method string, password string) {
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowsocksRustClient,
		EntryPoint: "sslocal",
		Ports:      []uint16{serverPort, clientPort},
		Cmd:        []string{"-s", F.ToString("127.0.0.1:", serverPort), "-b", F.ToString("0.0.0.0:", clientPort), "-m", method, "-k", password, "-U"},
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeShadowsocks,
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Method:   method,
					Password: password,
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func testShadowsocksOutboundWithShadowsocksRust(t *testing.T, method string, password string) {
	startDockerContainer(t, DockerOptions{
		Image:      ImageShadowsocksRustServer,
		EntryPoint: "ssserver",
		Ports:      []uint16{serverPort, testPort},
		Cmd:        []string{"-s", F.ToString("0.0.0.0:", serverPort), "-m", method, "-k", password, "-U"},
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
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeShadowsocks,
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Method:   method,
					Password: password,
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func testShadowsocksSelf(t *testing.T, method string, password string) {
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
				Type: C.TypeShadowsocks,
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
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
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
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
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "ss-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestShadowsocksUoT(t *testing.T) {
	method := shadowaead_2022.List[0]
	password := mkBase64(t, 16)
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
				Type: C.TypeShadowsocks,
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
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
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Method:   method,
					Password: password,
					UDPOverTCP: &option.UDPOverTCPOptions{
						Enabled: true,
					},
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
								Outbound: "ss-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func testShadowsocks2022EIH(t *testing.T, method string, password string) {
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
				Type: C.TypeShadowsocks,
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Method:   method,
					Password: password,
					Users: []option.ShadowsocksUser{
						{
							Password: password,
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
				Type: C.TypeShadowsocks,
				Tag:  "ss-out",
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Method:   method,
					Password: password + ":" + password,
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
								Outbound: "ss-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func mkBase64(t *testing.T, length int) string {
	psk := make([]byte, length)
	_, err := rand.Read(psk)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(psk)
}
