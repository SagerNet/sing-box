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

func TestV2RayWebsocket(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		testV2RayTransportSelf(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
		})
	})
	t.Run("self-early-data", func(t *testing.T) {
		testV2RayTransportSelf(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				MaxEarlyData: 2048,
			},
		})
	})
	t.Run("self-xray-early-data", func(t *testing.T) {
		testV2RayTransportSelf(t, &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeWebsocket,
			WebsocketOptions: option.V2RayWebsocketOptions{
				MaxEarlyData:        2048,
				EarlyDataHeaderName: "Sec-WebSocket-Protocol",
			},
		})
	})
	t.Run("inbound", func(t *testing.T) {
		testV2RayWebsocketInbound(t, 0, "")
	})
	t.Run("inbound-early-data", func(t *testing.T) {
		testV2RayWebsocketInbound(t, 2048, "")
	})
	t.Run("inbound-xray-early-data", func(t *testing.T) {
		testV2RayWebsocketInbound(t, 2048, "Sec-WebSocket-Protocol")
	})
	t.Run("outbound", func(t *testing.T) {
		testV2RayWebsocketOutbound(t, 0, "")
	})
	t.Run("outbound-early-data", func(t *testing.T) {
		testV2RayWebsocketOutbound(t, 2048, "")
	})
	t.Run("outbound-xray-early-data", func(t *testing.T) {
		testV2RayWebsocketOutbound(t, 2048, "Sec-WebSocket-Protocol")
	})
}

func testV2RayWebsocketInbound(t *testing.T, maxEarlyData uint32, earlyDataHeaderName string) {
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
						Type: C.V2RayTransportTypeWebsocket,
						WebsocketOptions: option.V2RayWebsocketOptions{
							MaxEarlyData:        maxEarlyData,
							EarlyDataHeaderName: earlyDataHeaderName,
						},
					},
				},
			},
		},
	})
	content, err := os.ReadFile("config/vmess-ws-client.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	config.MustKey("inbounds").MustIndex(0).MustKey("port").SetNumeric(float64(clientPort))
	outbound := config.MustKey("outbounds").MustIndex(0)
	settings := outbound.MustKey("settings").MustKey("vnext").MustIndex(0)
	settings.MustKey("port").SetNumeric(float64(serverPort))
	user := settings.MustKey("users").MustIndex(0)
	user.MustKey("id").SetString(userId.String())
	wsSettings := outbound.MustKey("streamSettings").MustKey("wsSettings")
	wsSettings.MustKey("maxEarlyData").SetNumeric(float64(maxEarlyData))
	wsSettings.MustKey("earlyDataHeaderName").SetString(earlyDataHeaderName)
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

func testV2RayWebsocketOutbound(t *testing.T, maxEarlyData uint32, earlyDataHeaderName string) {
	userId, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	content, err := os.ReadFile("config/vmess-ws-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("id").SetString(userId.String())
	wsSettings := inbound.MustKey("streamSettings").MustKey("wsSettings")
	wsSettings.MustKey("maxEarlyData").SetNumeric(float64(maxEarlyData))
	wsSettings.MustKey("earlyDataHeaderName").SetString(earlyDataHeaderName)
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
						Type: C.V2RayTransportTypeWebsocket,
						WebsocketOptions: option.V2RayWebsocketOptions{
							MaxEarlyData:        maxEarlyData,
							EarlyDataHeaderName: earlyDataHeaderName,
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}
