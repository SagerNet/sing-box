package main

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

func TestSudoku_E2EOutboundProvidedConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in -short mode")
	}
	if os.Getenv("SUDOKU_E2E") != "1" {
		t.Skip("set SUDOKU_E2E=1 to enable")
	}

	serverAddress := os.Getenv("SUDOKU_E2E_SERVER")
	if serverAddress == "" {
		serverAddress = "sub.393633.xyz:80"
	}
	key := os.Getenv("SUDOKU_E2E_KEY")
	if key == "" {
		t.Skip("set SUDOKU_E2E_KEY to enable (example: export SUDOKU_E2E_KEY='aa61f4ff...')")
	}

	localPort := uint16(8444)
	if s := os.Getenv("SUDOKU_E2E_LOCAL_PORT"); s != "" {
		n, err := strconv.ParseUint(s, 10, 16)
		require.NoError(t, err)
		localPort = uint16(n)
	}

	targetAddress := os.Getenv("SUDOKU_E2E_TARGET")
	if targetAddress == "" {
		targetAddress = "example.com:80"
	}

	serverHost, serverPortStr, err := net.SplitHostPort(serverAddress)
	require.NoError(t, err)
	serverPort64, err := strconv.ParseUint(serverPortStr, 10, 16)
	require.NoError(t, err)

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						ListenPort: localPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
				Tag:  "direct",
			},
			{
				Type: C.TypeSudoku,
				Tag:  "sudoku-out",
				Options: &option.SudokuOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     serverHost,
						ServerPort: uint16(serverPort64),
					},
					Key:                key,
					AEADMethod:         "aes-128-gcm",
					PaddingMin:         ptr(1),
					PaddingMax:         ptr(9),
					ASCII:              "prefer_ascii",
					EnablePureDownlink: ptr(false),
					DisableHTTPMask:    false,
					HTTPMaskMode:       "auto",
					HTTPMaskTLS:        false,
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
								Outbound: "sudoku-out",
							},
						},
					},
				},
			},
		},
	})

	target := M.ParseSocksaddr(targetAddress)
	require.True(t, target.IsValid(), "invalid SUDOKU_E2E_TARGET: %q", targetAddress)

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", localPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), N.NetworkTCP, target)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: " + target.AddrString() + "\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)

	var buf [64]byte
	n, err := conn.Read(buf[:])
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(buf[:n]), "HTTP/"), "unexpected response: %q", string(buf[:n]))
}

func ptr[T any](v T) *T { return &v }
