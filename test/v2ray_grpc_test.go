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

func TestV2RayGRPCInbound(t *testing.T) {
	t.Run("origin", func(t *testing.T) {
		testV2RayGRPCInbound(t, false)
	})
	t.Run("lite", func(t *testing.T) {
		testV2RayGRPCInbound(t, true)
	})
}

func testV2RayGRPCInbound(t *testing.T, forceLite bool) {
	userId, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
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
							Name: "sekai",
							UUID: userId.String(),
						},
					},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Transport: &option.V2RayTransportOptions{
						Type: C.V2RayTransportTypeGRPC,
						GRPCOptions: option.V2RayGRPCOptions{
							ServiceName: "TunService",
							ForceLite:   forceLite,
						},
					},
				},
			},
		},
	})
	content, err := os.ReadFile("config/vmess-grpc-client.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	config.MustKey("inbounds").MustIndex(0).MustKey("port").SetNumeric(float64(clientPort))
	outbound := config.MustKey("outbounds").MustIndex(0).MustKey("settings").MustKey("vnext").MustIndex(0)
	outbound.MustKey("port").SetNumeric(float64(serverPort))
	user := outbound.MustKey("users").MustIndex(0)
	user.MustKey("id").SetString(userId.String())
	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageV2RayCore,
		Ports:      []uint16{serverPort, testPort},
		EntryPoint: "v2ray",
		Cmd:        []string{"run"},
		Stdin:      content,
		Bind: map[string]string{
			certPem: "/path/to/certificate.crt",
			keyPem:  "/path/to/private.key",
		},
	})

	testSuitSimple(t, clientPort, testPort)
}

func TestV2RayGRPCOutbound(t *testing.T) {
	t.Run("origin", func(t *testing.T) {
		testV2RayGRPCOutbound(t, false)
	})
	t.Run("lite", func(t *testing.T) {
		testV2RayGRPCOutbound(t, true)
	})
}

func testV2RayGRPCOutbound(t *testing.T, forceLite bool) {
	userId, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	content, err := os.ReadFile("config/vmess-grpc-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("id").SetString(userId.String())
	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageV2RayCore,
		Ports:      []uint16{serverPort, testPort},
		EntryPoint: "v2ray",
		Cmd:        []string{"run"},
		Stdin:      content,
		Env:        []string{"V2RAY_VMESS_AEAD_FORCED=false"},
		Bind: map[string]string{
			certPem: "/path/to/certificate.crt",
			keyPem:  "/path/to/private.key",
		},
	})
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
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeVMess,
				Tag:  "vmess-out",
				Options: &option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					UUID:     userId.String(),
					Security: "zero",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
					Transport: &option.V2RayTransportOptions{
						Type: C.V2RayTransportTypeGRPC,
						GRPCOptions: option.V2RayGRPCOptions{
							ServiceName: "TunService",
							ForceLite:   forceLite,
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestV2RayGRPCLite(t *testing.T) {
	t.Run("server", func(t *testing.T) {
		testV2RayTransportSelfWith(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: "TunService",
				ForceLite:   true,
			},
		}, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: "TunService",
			},
		})
	})
	t.Run("client", func(t *testing.T) {
		testV2RayTransportSelfWith(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: "TunService",
			},
		}, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: "TunService",
				ForceLite:   true,
			},
		})
	})
	t.Run("self", func(t *testing.T) {
		testV2RayTransportSelfWith(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: "TunService",
				ForceLite:   true,
			},
		}, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{
				ServiceName: "TunService",
				ForceLite:   true,
			},
		})
	})
}
