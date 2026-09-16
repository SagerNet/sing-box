//go:build with_tailscale

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/tailscale"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/tailscale/derp"
	"github.com/sagernet/tailscale/derp/derphttp"
	"github.com/sagernet/tailscale/net/netmon"
	"github.com/sagernet/tailscale/types/key"

	"github.com/stretchr/testify/require"
	"go4.org/mem"
)

func TestTailcat(t *testing.T) {
	loopback := netip.MustParseAddr("127.0.0.1")
	derpPort := reserveTCPPort(t, loopback)
	stunPort := reserveUDPPort(t, loopback)
	_, certPath, keyPath := createSelfSignedCertificate(t, "localhost")
	certPEM, err := os.ReadFile(certPath)
	require.NoError(t, err)
	certBlock, _ := pem.Decode(certPEM)
	certificate, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)
	certHash := sha256.Sum256(certificate.Raw)

	serverKey := tailscale.NewTailcatPrivateKey()
	clientKey := tailscale.NewTailcatPrivateKey()
	var presharedKey [32]byte
	_, err = rand.Read(presharedKey[:])
	require.NoError(t, err)
	derpServers := badoption.Listable[option.TailcatDERPServer]{{
		Host:     "localhost",
		IPv4:     loopback.String(),
		IPv6:     "none",
		DERPPort: derpPort,
		STUNPort: stunPort,
		CertName: "sha256-raw:" + hex.EncodeToString(certHash[:]),
	}}

	startInstance(t, option.Options{
		Services: []option.Service{{
			Type: C.TypeDERP,
			Tag:  "derp",
			Options: &option.DERPServiceOptions{
				ListenOptions: option.ListenOptions{
					Listen:     (*badoption.Addr)(common.Ptr(loopback)),
					ListenPort: derpPort,
				},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						CertificatePath: certPath,
						KeyPath:         keyPath,
					},
				},
				ConfigPath:          filepath.Join(t.TempDir(), "derp.key"),
				VerifyClientInbound: []string{"tailcat-in"},
				STUN: &option.DERPSTUNListenOptions{
					Enabled: true,
					ListenOptions: option.ListenOptions{
						Listen:     (*badoption.Addr)(common.Ptr(loopback)),
						ListenPort: stunPort,
					},
				},
			},
		}},
		Inbounds: []option.Inbound{{
			Type: C.TypeTailcat,
			Tag:  "tailcat-in",
			Options: &option.TailcatInboundOptions{
				PrivateKey:   tailscale.EncodeTailcatKey(serverKey),
				PreSharedKey: tailscale.EncodeTailcatKey(presharedKey),
				Users: []option.TailcatUser{{
					Name:      "client",
					PublicKey: tailscale.EncodeTailcatKey(tailscale.TailcatPublicKey(clientKey)),
				}},
				TailcatDERPOptions: option.TailcatDERPOptions{DERPServers: derpServers},
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}}},
		Route:     &option.RouteOptions{Final: "direct"},
	})
	client := startInstance(t, option.Options{
		Outbounds: []option.Outbound{{
			Type: C.TypeTailcat,
			Tag:  "tailcat-out",
			Options: &option.TailcatOutboundOptions{
				PrivateKey:         tailscale.EncodeTailcatKey(clientKey),
				ServerPublicKey:    tailscale.EncodeTailcatKey(tailscale.TailcatPublicKey(serverKey)),
				ServerDiscoKey:     tailscale.EncodeTailcatKey(tailscale.TailcatPublicKey(tailscale.TailcatDiscoPrivateKey(serverKey))),
				PreSharedKey:       tailscale.EncodeTailcatKey(presharedKey),
				TailcatDERPOptions: option.TailcatDERPOptions{DERPServers: derpServers},
			},
		}},
		Route: &option.RouteOptions{Final: "tailcat-out"},
	})
	outbound, loaded := client.Outbound().Outbound("tailcat-out")
	require.True(t, loaded)

	// IPv4 destinations exercise the NAT64 exit-node path; the server's
	// own tailcat address exercises the locally served IPv6 path.
	targets := []tailcatTarget{
		{name: "exit-node", listen: loopback, dial: loopback},
		{name: "server", listen: netip.IPv6Loopback(), dial: tailscale.TailcatAddress(tailscale.TailcatPublicKey(serverKey))},
	}
	t.Run("data-plane", func(t *testing.T) {
		for _, target := range targets {
			for _, network := range []string{"tcp", "udp", "packet"} {
				t.Run(target.name+"/"+network, func(t *testing.T) {
					t.Parallel()
					testTailcatEcho(t, outbound, target, network)
				})
			}
		}
	})
	t.Run("on-demand", func(t *testing.T) {
		outbound.(adapter.IdleConnectionKeeper).SetKeepIdleConnections(false)
		testTailcatEcho(t, outbound, targets[0], "tcp")
	})
	t.Run("verify-client", func(t *testing.T) {
		rootCAs := x509.NewCertPool()
		rootCAs.AddCert(certificate)
		serverURL := "https://" + net.JoinHostPort(loopback.String(), F.ToString(derpPort)) + "/derp"
		for _, testCase := range []struct {
			name    string
			key     key.NodePrivate
			allowed bool
		}{
			{"listed", key.NodePrivateFromRaw32(mem.B(clientKey[:])), true},
			{"unlisted", key.NewNode(), false},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				defer cancel()
				derpClient, err := derphttp.NewClient(testCase.key, serverURL, t.Logf, netmon.NewStatic())
				require.NoError(t, err)
				defer derpClient.Close()
				derpClient.TLSConfig = &tls.Config{ServerName: "localhost", RootCAs: rootCAs}
				err = derpClient.Connect(ctx)
				require.NoError(t, err)
				type recvResult struct {
					message derp.ReceivedMessage
					err     error
				}
				received := make(chan recvResult, 1)
				go func() {
					message, recvErr := derpClient.Recv()
					received <- recvResult{message, recvErr}
				}()
				var result recvResult
				select {
				case result = <-received:
				case <-ctx.Done():
					require.FailNow(t, "DERP server did not answer")
				}
				if testCase.allowed {
					require.NoError(t, result.err)
					require.IsType(t, derp.ServerInfoMessage{}, result.message)
				} else {
					require.Error(t, result.err)
				}
			})
		}
	})
}

type tailcatTarget struct {
	name   string
	listen netip.Addr
	dial   netip.Addr
}

func testTailcatEcho(t *testing.T, outbound adapter.Outbound, target tailcatTarget, network string) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	deadline := time.Now().Add(15 * time.Second)
	if network == "tcp" {
		listener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(target.listen, 0)))
		require.NoError(t, err)
		defer listener.Close()
		listener.SetDeadline(deadline)
		destination := M.SocksaddrFrom(target.dial, uint16(listener.Addr().(*net.TCPAddr).Port))
		client, err := outbound.DialContext(ctx, "tcp", destination)
		require.NoError(t, err)
		defer client.Close()
		server, err := listener.Accept()
		require.NoError(t, err)
		defer server.Close()
		client.SetDeadline(deadline)
		server.SetDeadline(deadline)
		payload := bytes.Repeat([]byte("tailcat-encrypted-tcp"), 8192)
		for _, pair := range [][2]net.Conn{{client, server}, {server, client}} {
			result := make(chan error, 1)
			go func() {
				_, writeErr := pair[0].Write(payload)
				result <- writeErr
			}()
			received := make([]byte, len(payload))
			_, err = io.ReadFull(pair[1], received)
			require.NoError(t, err)
			require.Equal(t, payload, received)
			require.NoError(t, <-result)
		}
		return
	}
	server, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(target.listen, 0)))
	require.NoError(t, err)
	defer server.Close()
	server.SetDeadline(deadline)
	destination := M.SocksaddrFrom(target.dial, uint16(server.LocalAddr().(*net.UDPAddr).Port))
	var send func([]byte) (int, error)
	var receive func([]byte) (int, error)
	if network == "udp" {
		client, err := outbound.DialContext(ctx, "udp", destination)
		require.NoError(t, err)
		defer client.Close()
		client.SetDeadline(deadline)
		send, receive = client.Write, client.Read
	} else {
		client, err := outbound.ListenPacket(ctx, destination)
		require.NoError(t, err)
		defer client.Close()
		client.SetDeadline(deadline)
		send = func(payload []byte) (int, error) { return client.WriteTo(payload, destination.UDPAddr()) }
		receive = func(payload []byte) (int, error) {
			count, source, readErr := client.ReadFrom(payload)
			if readErr == nil {
				require.Equal(t, destination.String(), source.String())
			}
			return count, readErr
		}
	}
	for _, size := range []int{64, 1200} {
		payload := bytes.Repeat([]byte{0x65}, size)
		_, err = send(payload)
		require.NoError(t, err)
		received := make([]byte, 65535)
		count, source, err := server.ReadFromUDP(received)
		require.NoError(t, err)
		require.Equal(t, payload, received[:count])
		_, err = server.WriteToUDP(payload, source)
		require.NoError(t, err)
		count, err = receive(received)
		require.NoError(t, err)
		require.Equal(t, payload, received[:count])
	}
}

func reserveTCPPort(t *testing.T, address netip.Addr) uint16 {
	listener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(address, 0)))
	require.NoError(t, err)
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	require.NoError(t, listener.Close())
	return port
}

func reserveUDPPort(t *testing.T, address netip.Addr) uint16 {
	conn, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(address, 0)))
	require.NoError(t, err)
	port := uint16(conn.LocalAddr().(*net.UDPAddr).Port)
	require.NoError(t, conn.Close())
	return port
}
