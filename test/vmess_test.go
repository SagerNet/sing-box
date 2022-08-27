package main

import (
	"net/netip"
	"os"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/gofrs/uuid"
	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/require"
)

func TestVMessAuto(t *testing.T) {
	security := "auto"
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, false, true)
	})
	t.Run("inbound", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, user, 0, false)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, false, false, 0)
	})
}

func _TestVMess(t *testing.T) {
	for _, security := range []string{
		"zero",
	} {
		t.Run(security, func(t *testing.T) {
			testVMess0(t, security)
		})
	}
	for _, security := range []string{
		"aes-128-gcm", "chacha20-poly1305", "aes-128-cfb",
	} {
		t.Run(security, func(t *testing.T) {
			testVMess1(t, security)
		})
	}
}

func testVMess0(t *testing.T, security string) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, false, false)
	})
	t.Run("self-legacy", func(t *testing.T) {
		testVMessSelf(t, security, user, 1, false, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, false, true)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, false, false, 0)
	})
	t.Run("outbound-legacy", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, false, false, 1)
	})
}

func testVMess1(t *testing.T, security string) {
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	t.Run("self", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, false, false)
	})
	t.Run("self-padding", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, true, false, false)
	})
	t.Run("self-authid", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, true, false)
	})
	t.Run("self-padding-authid", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, true, true, false)
	})
	t.Run("self-legacy", func(t *testing.T) {
		testVMessSelf(t, security, user, 1, false, false, false)
	})
	t.Run("self-legacy-padding", func(t *testing.T) {
		testVMessSelf(t, security, user, 1, true, false, false)
	})
	t.Run("packetaddr", func(t *testing.T) {
		testVMessSelf(t, security, user, 0, false, false, true)
	})
	t.Run("inbound", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, user, 0, false)
	})
	t.Run("inbound-authid", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, user, 0, true)
	})
	t.Run("inbound-legacy", func(t *testing.T) {
		testVMessInboundWithV2Ray(t, security, user, 64, false)
	})
	t.Run("outbound", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, false, false, 0)
	})
	t.Run("outbound-padding", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, true, false, 0)
	})
	t.Run("outbound-authid", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, false, true, 0)
	})
	t.Run("outbound-padding-authid", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, true, true, 0)
	})
	t.Run("outbound-legacy", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, false, false, 1)
	})
	t.Run("outbound-legacy-padding", func(t *testing.T) {
		testVMessOutboundWithV2Ray(t, security, user, true, false, 1)
	})
}

func testVMessInboundWithV2Ray(t *testing.T, security string, uuid uuid.UUID, alterId int, authenticatedLength bool) {
	content, err := os.ReadFile("config/vmess-client.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	config.MustKey("inbounds").MustIndex(0).MustKey("port").SetNumeric(float64(clientPort))
	outbound := config.MustKey("outbounds").MustIndex(0).MustKey("settings").MustKey("vnext").MustIndex(0)
	outbound.MustKey("port").SetNumeric(float64(serverPort))
	user := outbound.MustKey("users").MustIndex(0)
	user.MustKey("id").SetString(uuid.String())
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
		Stdin:      content,
		Env:        []string{"V2RAY_VMESS_AEAD_FORCED=false"},
	})

	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeVMess,
				VMessOptions: option.VMessInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []option.VMessUser{
						{
							Name:    "sekai",
							UUID:    uuid.String(),
							AlterId: alterId,
						},
					},
				},
			},
		},
	})

	testSuit(t, clientPort, testPort)
}

func testVMessOutboundWithV2Ray(t *testing.T, security string, uuid uuid.UUID, globalPadding bool, authenticatedLength bool, alterId int) {
	content, err := os.ReadFile("config/vmess-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("id").SetString(uuid.String())
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("alterId").SetNumeric(float64(alterId))

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageV2RayCore,
		Ports:      []uint16{serverPort, testPort},
		EntryPoint: "v2ray",
		Stdin:      content,
		Env:        []string{"V2RAY_VMESS_AEAD_FORCED=false"},
	})

	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
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
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeVMess,
				VMessOptions: option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Security:            security,
					UUID:                uuid.String(),
					GlobalPadding:       globalPadding,
					AuthenticatedLength: authenticatedLength,
					AlterId:             alterId,
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func testVMessSelf(t *testing.T, security string, uuid uuid.UUID, alterId int, globalPadding bool, authenticatedLength bool, packetAddr bool) {
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
							Name:    "sekai",
							UUID:    uuid.String(),
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
				VMessOptions: option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Security:            security,
					UUID:                uuid.String(),
					AlterId:             alterId,
					GlobalPadding:       globalPadding,
					AuthenticatedLength: authenticatedLength,
					PacketAddr:          packetAddr,
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
