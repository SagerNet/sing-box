package main

import (
	"net/netip"
	"os"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/gofrs/uuid/v5"
	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/require"
)

func newUUID() uuid.UUID {
	user, _ := uuid.DefaultGenerator.NewV4()
	return user
}

func TestVMessAuto(t *testing.T) {
	security := "auto"
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, true)
	})
	t.Run("inbound", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, 0, false)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 0)
	})
}

func TestVMess(t *testing.T) {
	for _, security := range []string{
		"zero",
	} {
		t.Run(security, func(t *testing.T) {
			testVMess0(t, security)
		})
	}
	for _, security := range []string{
		"none",
	} {
		t.Run(security, func(t *testing.T) {
			testVMess1(t, security)
		})
	}
	for _, security := range []string{
		"aes-128-gcm", "chacha20-poly1305", "aes-128-cfb",
	} {
		t.Run(security, func(t *testing.T) {
			testVMess2(t, security)
		})
	}
}

func testVMess0(t *testing.T, security string) {
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, false)
	})
	t.Run("self-legacy", func(t *testing.T) {
		testVMessSelf(t, security, 1, false, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, true)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 0)
	})
	t.Run("outbound-legacy", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 1)
	})
}

func testVMess1(t *testing.T, security string) {
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, false)
	})
	t.Run("self-legacy", func(t *testing.T) {
		testVMessSelf(t, security, 1, false, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, true)
	})
	t.Run("inbound", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, 0, false)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 0)
	})
	t.Run("outbound-legacy", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 1)
	})
}

func testVMess2(t *testing.T, security string) {
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, false)
	})
	t.Run("self-padding", func(t *testing.T) {
		testVMessSelf(t, security, 0, true, false, false)
	})
	t.Run("self-authid", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, true, false)
	})
	t.Run("self-padding-authid", func(t *testing.T) {
		testVMessSelf(t, security, 0, true, true, false)
	})
	t.Run("self-legacy", func(t *testing.T) {
		testVMessSelf(t, security, 1, false, false, false)
	})
	t.Run("self-legacy-padding", func(t *testing.T) {
		testVMessSelf(t, security, 1, true, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, 0, false, false, true)
	})
	t.Run("inbound", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, 0, false)
	})
	t.Run("inbound-authid", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, 0, true)
	})
	t.Run("inbound-legacy", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, 64, false)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 0)
	})
	t.Run("outbound-padding", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, true, false, 0)
	})
	t.Run("outbound-authid", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, true, 0)
	})
	t.Run("outbound-padding-authid", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, true, true, 0)
	})
	t.Run("outbound-legacy", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, false, false, 1)
	})
	t.Run("outbound-legacy-padding", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, true, false, 1)
	})
}

func testVMessInboundWithV2Ray(t *testing.T, security string, alterId int, authenticatedLength bool) {
	userId := newUUID()
	content, err := os.ReadFile("config/vmess-client.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	config.MustKey("inbounds").MustIndex(0).MustKey("port").SetNumeric(float64(clientPort))
	outbound := config.MustKey("outbounds").MustIndex(0).MustKey("settings").MustKey("vnext").MustIndex(0)
	outbound.MustKey("port").SetNumeric(float64(serverPort))
	user := outbound.MustKey("users").MustIndex(0)
	user.MustKey("id").SetString(userId.String())
	user.MustKey("alterId").SetNumeric(float64(alterId))
	user.MustKey("security").SetString(security)
	var experiments string
	if authenticatedLength {
		experiments += "AuthenticatedLength"
	}
	user.MustKey("experiments").SetString(experiments)

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageV2RayCore,
		Ports:      []uint16{serverPort, testPort},
		EntryPoint: "v2ray",
		Cmd:        []string{"run"},
		Stdin:      content,
		Env:        []string{"V2RAY_VMESS_AEAD_FORCED=false"},
	})

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeVMess,
				Options: &option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							Name:    "sekai",
							UUID:    userId.String(),
							AlterId: alterId,
						},
					},
				},
			},
		},
	})

	testSuitSimple(t, clientPort, testPort)
}

func testVMessOutboundWithV2Ray(t *testing.T, security string, globalPadding bool, authenticatedLength bool, alterId int) {
	user := newUUID()
	content, err := os.ReadFile("config/vmess-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("id").SetString(user.String())
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("alterId").SetNumeric(float64(alterId))

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageV2RayCore,
		Ports:      []uint16{serverPort, testPort},
		EntryPoint: "v2ray",
		Cmd:        []string{"run"},
		Stdin:      content,
		Env:        []string{"V2RAY_VMESS_AEAD_FORCED=false"},
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
				Type: C.TypeVMess,
				Options: &option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Security:            security,
					UUID:                user.String(),
					GlobalPadding:       globalPadding,
					AuthenticatedLength: authenticatedLength,
					AlterId:             alterId,
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func testVMessSelf(t *testing.T, security string, alterId int, globalPadding bool, authenticatedLength bool, packetAddr bool) {
	user := newUUID()
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
				Type: C.TypeVMess,
				Options: &option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							Name:    "sekai",
							UUID:    user.String(),
							AlterId: alterId,
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
				Options: &option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Security:            security,
					UUID:                user.String(),
					AlterId:             alterId,
					GlobalPadding:       globalPadding,
					AuthenticatedLength: authenticatedLength,
					PacketEncoding:      "packetaddr",
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
								Outbound: "vmess-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
