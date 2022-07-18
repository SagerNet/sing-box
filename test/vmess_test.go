package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

func TestVMessSelf(t *testing.T) {
	t.Parallel()
	for _, security := range []string{
		"zero",
	} {
		t.Run(security, func(t *testing.T) {
			testVMessSelf0(t, security)
		})
	}
	for _, security := range []string{
		"aes-128-gcm", "chacha20-poly1305", "aes-128-cfb",
	} {
		t.Run(security, func(t *testing.T) {
			testVMessSelf1(t, security)
		})
	}
}

func testVMessSelf0(t *testing.T, security string) {
	t.Parallel()
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	t.Run("default", func(t *testing.T) {
		testVMessSelf2(t, security, user.String(), false, false)
	})
	t.Run("padding", func(t *testing.T) {
		testVMessSelf2(t, security, user.String(), true, false)
	})
}

func testVMessSelf1(t *testing.T, security string) {
	t.Parallel()
	user, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	t.Run("default", func(t *testing.T) {
		testVMessSelf2(t, security, user.String(), false, false)
	})
	t.Run("padding", func(t *testing.T) {
		testVMessSelf2(t, security, user.String(), true, false)
	})
	t.Run("authid", func(t *testing.T) {
		testVMessSelf2(t, security, user.String(), false, true)
	})
	t.Run("padding-authid", func(t *testing.T) {
		testVMessSelf2(t, security, user.String(), true, true)
	})
}

func testVMessSelf2(t *testing.T, security string, uuid string, globalPadding bool, authenticatedLength bool) {
	t.Parallel()
	serverPort := mkPort(t)
	clientPort := mkPort(t)
	testPort := mkPort(t)
	startInstance(t, option.Options{
		Log: &option.LogOption{
			Level: "error",
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
							Name: "sekai",
							UUID: uuid,
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
					UUID:                uuid,
					GlobalPadding:       globalPadding,
					AuthenticatedLength: authenticatedLength,
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
