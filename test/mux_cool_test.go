package main

import (
	"net/netip"
	"os"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/spyzhov/ajson"
	"github.com/stretchr/testify/require"
)

func TestMuxCoolServer(t *testing.T) {
	userId := newUUID()
	content, err := os.ReadFile("config/vmess-mux-client.json")
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
							Name: "sekai",
							UUID: userId.String(),
						},
					},
				},
			},
		},
	})

	testSuitSimple(t, clientPort, testPort)
}

func TestMuxCoolClient(t *testing.T) {
	user := newUUID()
	content, err := os.ReadFile("config/vmess-server.json")
	require.NoError(t, err)
	config, err := ajson.Unmarshal(content)
	require.NoError(t, err)

	inbound := config.MustKey("inbounds").MustIndex(0)
	inbound.MustKey("port").SetNumeric(float64(serverPort))
	inbound.MustKey("settings").MustKey("clients").MustIndex(0).MustKey("id").SetString(user.String())

	content, err = ajson.Marshal(config)
	require.NoError(t, err)

	startDockerContainer(t, DockerOptions{
		Image:      ImageXRayCore,
		Ports:      []uint16{serverPort, testPort},
		EntryPoint: "xray",
		Stdin:      content,
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
					UUID:           user.String(),
					PacketEncoding: "xudp",
				},
			},
		},
	})
	testSuitSimple(t, clientPort, testPort)
}

func TestMuxCoolSelf(t *testing.T) {
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
							Name: "sekai",
							UUID: user.String(),
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
					UUID:           user.String(),
					PacketEncoding: "xudp",
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
	testSuitSimple(t, clientPort, testPort)
}
