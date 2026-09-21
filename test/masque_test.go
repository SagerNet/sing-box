package main

import (
	std_bufio "bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

const (
	masqueServerAddress = "10.8.0.1"
	masqueSiteAddress   = "10.9.0.5"
)

type masqueEnvironment struct {
	serverPort      uint16
	serverProxyPort uint16
	clientProxyPort uint16
}

func startMASQUE(t *testing.T, serverVersions []int, clientVersion int, disableVersionFallback bool, mtu uint32, serverRoutes []netip.Prefix, clientRoutes []netip.Prefix) masqueEnvironment {
	t.Helper()
	environment := masqueEnvironment{
		serverProxyPort: reserveOpenVPNTCPPort(t),
		clientProxyPort: reserveOpenVPNTCPPort(t),
	}
	masquePort := reserveOpenVPNEchoPort(t)
	environment.serverPort = masquePort
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	users := []auth.User{{Username: "sekai", Password: "password"}}
	startInstance(t, masqueInstanceOptions(C.TypeMASQUEServer, &option.MASQUEServerEndpointOptions{
		ListenOptions: option.ListenOptions{
			Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
			ListenPort: masquePort,
		},
		MASQUEEndpointOptions: option.MASQUEEndpointOptions{MTU: mtu},
		Users:                 users,
		Version:               serverVersions,
		InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
			TLS: &option.InboundTLSOptions{
				Enabled:         true,
				ServerName:      "example.org",
				CertificatePath: certPem,
				KeyPath:         keyPem,
			},
		},
		Address:         []netip.Prefix{netip.MustParsePrefix(masqueServerAddress + "/24")},
		AdvertiseRoutes: serverRoutes,
	}, environment.serverProxyPort, ""))
	startInstance(t, masqueInstanceOptions(C.TypeMASQUEClient, &option.MASQUEClientEndpointOptions{
		ServerOptions: option.ServerOptions{
			Server:     "127.0.0.1",
			ServerPort: masquePort,
		},
		MASQUEEndpointOptions: option.MASQUEEndpointOptions{MTU: mtu},
		Username:              users[0].Username,
		Password:              users[0].Password,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:         true,
				ServerName:      "example.org",
				CertificatePath: certPem,
			},
		},
		Version:                clientVersion,
		DisableVersionFallback: disableVersionFallback,
		AdvertiseRoutes:        clientRoutes,
	}, environment.clientProxyPort, "127.0.0.1"))
	waitForOpenVPNClientReady(t, environment.clientProxyPort, reserveOpenVPNEchoPort(t), masqueServerAddress)
	return environment
}

func masqueInstanceOptions(endpointType string, endpointOptions any, proxyPort uint16, overrideAddress string) option.Options {
	return option.Options{
		Endpoints: []option.Endpoint{
			{
				Type:    endpointType,
				Tag:     "masque",
				Options: endpointOptions,
			},
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeSOCKS,
				Tag:  "socks-in",
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
						ListenPort: proxyPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
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
							Inbound: []string{"socks-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "masque",
							},
						},
					},
				},
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"masque"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "direct",
								RawRouteOptionsActionOptions: option.RawRouteOptionsActionOptions{
									OverrideAddress: overrideAddress,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestMASQUESelfToSelf(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		version int
	}{
		{"HTTP3", 3},
		{"HTTP2", 2},
		{"HTTP1", 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			environment := startMASQUE(t, nil, testCase.version, true, 0, nil, nil)
			testSuitOpenVPN(t, environment.clientProxyPort, reserveOpenVPNEchoPort(t), masqueServerAddress)
		})
	}
}

func TestMASQUEVersionFallback(t *testing.T) {
	environment := startMASQUE(t, []int{1, 2}, 0, false, 0, nil, nil)
	testSuitOpenVPN(t, environment.clientProxyPort, reserveOpenVPNEchoPort(t), masqueServerAddress)
}

func TestMASQUEAdvertiseRoutes(t *testing.T) {
	environment := startMASQUE(t, nil, 0, false, 0, nil, []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")})
	readinessPort := reserveOpenVPNEchoPort(t)
	closeEcho := startOpenVPNReadinessEcho(t, readinessPort)
	waitForOpenVPNRemoteReady(t, environment.serverProxyPort, masqueSiteAddress, readinessPort, 30*time.Second)
	closeEcho()
	testSuitOpenVPN(t, environment.serverProxyPort, reserveOpenVPNEchoPort(t), masqueSiteAddress)
}

func TestMASQUENoRoute(t *testing.T) {
	environment := startMASQUE(t, nil, 0, false, 0, []netip.Prefix{netip.MustParsePrefix("10.9.0.0/24")}, nil)
	for _, proxyPort := range []uint16{environment.clientProxyPort, environment.serverProxyPort} {
		dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("10.10.0.1", 80))
		if err == nil {
			conn.Close()
		}
		require.Error(t, err)
		require.NoError(t, ctx.Err())
		cancel()
	}
}

func TestMASQUEPacketTooBig(t *testing.T) {
	environment := startMASQUE(t, nil, 3, true, 1500, nil, nil)
	testSuitOpenVPN(t, environment.clientProxyPort, reserveOpenVPNEchoPort(t), masqueServerAddress)
}

func TestMASQUEReconnectBackoff(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var accepted atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted.Add(1)
			conn.Close()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
	})
	startInstance(t, masqueInstanceOptions(C.TypeMASQUEClient, &option.MASQUEClientEndpointOptions{
		ServerOptions: option.ServerOptions{
			Server:     "127.0.0.1",
			ServerPort: uint16(listener.Addr().(*net.TCPAddr).Port),
		},
		Version: 1,
	}, reserveOpenVPNTCPPort(t), ""))
	time.Sleep(2500 * time.Millisecond)
	require.Positive(t, accepted.Load())
	require.LessOrEqual(t, accepted.Load(), int32(3))
}

func TestMASQUEStuckClient(t *testing.T) {
	environment := startMASQUE(t, nil, 0, false, 0, nil, nil)
	stuckConn, err := tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(environment.serverPort)), &tls.Config{
		ServerName:         "example.org",
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		stuckConn.Close()
	})
	_, err = stuckConn.Write([]byte("GET /.well-known/masque/ip/*/*/ HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: Upgrade\r\nUpgrade: connect-ip\r\nCapsule-Protocol: ?1\r\nAuthorization: Basic " + base64.StdEncoding.EncodeToString([]byte("sekai:password")) + "\r\n\r\n"))
	require.NoError(t, err)
	reader := std_bufio.NewReader(stuckConn)
	response, err := http.ReadResponse(reader, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.Equal(t, uint64(1), readVarint(t, reader))
	capsuleLength := readVarint(t, reader)
	capsule := make([]byte, capsuleLength)
	_, err = io.ReadFull(reader, capsule)
	require.NoError(t, err)
	require.Equal(t, byte(4), capsule[1])
	stuckAddress := netip.AddrFrom4([4]byte(capsule[2:6]))

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", environment.serverProxyPort), socks.Version5, "", "")
	floodConn, err := dialer.DialContext(context.Background(), N.NetworkUDP, M.SocksaddrFrom(stuckAddress, 9))
	require.NoError(t, err)
	t.Cleanup(func() {
		floodConn.Close()
	})
	floodDone := make(chan struct{})
	go func() {
		defer close(floodDone)
		payload := make([]byte, 1200)
		for range 4096 {
			_, writeErr := floodConn.Write(payload)
			if writeErr != nil {
				return
			}
		}
	}()
	select {
	case <-floodDone:
	case <-time.After(10 * time.Second):
	}

	echoPort := reserveOpenVPNEchoPort(t)
	closeEcho := startOpenVPNReadinessEcho(t, echoPort)
	defer closeEcho()
	require.NoError(t, probeOpenVPNTCPWithTimeout(environment.clientProxyPort, masqueServerAddress, echoPort, 5*time.Second))
}
