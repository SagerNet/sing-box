//go:build with_wireguard

package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

func TestWireGuardEndpointDataPlane(t *testing.T) {
	for _, system := range []bool{false, true} {
		t.Run(fmt.Sprintf("system=%v", system), func(test *testing.T) {
			serverKey, err := ecdh.X25519().GenerateKey(rand.Reader)
			require.NoError(test, err)
			clientKey, err := ecdh.X25519().GenerateKey(rand.Reader)
			require.NoError(test, err)
			portReservation, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			require.NoError(test, err)
			serverPort := uint16(portReservation.LocalAddr().(*net.UDPAddr).Port)
			require.NoError(test, portReservation.Close())
			serverAddresses := []netip.Prefix{netip.MustParsePrefix("198.18.17.1/32"), netip.MustParsePrefix("fd17::1/128")}
			clientAddresses := []netip.Prefix{netip.MustParsePrefix("198.18.17.2/32"), netip.MustParsePrefix("fd17::2/128")}
			instance := startInstance(test, option.Options{
				Endpoints: []option.Endpoint{
					{
						Type: C.TypeWireGuard,
						Tag:  "server",
						Options: &option.WireGuardEndpointOptions{
							MTU: 1280, Address: serverAddresses, Workers: 2,
							PrivateKey: base64.StdEncoding.EncodeToString(serverKey.Bytes()),
							ListenPort: serverPort,
							Peers: []option.WireGuardPeer{{
								PublicKey:  base64.StdEncoding.EncodeToString(clientKey.PublicKey().Bytes()),
								AllowedIPs: clientAddresses,
							}},
						},
					},
					{
						Type: C.TypeWireGuard,
						Tag:  "client",
						Options: &option.WireGuardEndpointOptions{
							System: system, MTU: 1280, Address: clientAddresses, Workers: 2,
							PrivateKey: base64.StdEncoding.EncodeToString(clientKey.Bytes()),
							Peers: []option.WireGuardPeer{{
								Address: "127.0.0.1", Port: serverPort,
								PublicKey:  base64.StdEncoding.EncodeToString(serverKey.PublicKey().Bytes()),
								AllowedIPs: serverAddresses,
							}},
						},
					},
				},
				Outbounds: []option.Outbound{{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}}},
				Route:     &option.RouteOptions{Final: "direct"},
			})
			endpoint, loaded := instance.Endpoint().Get("client")
			require.True(test, loaded)
			for index, address := range serverAddresses {
				for _, network := range []string{"tcp", "udp", "packet"} {
					test.Run(fmt.Sprintf("ipv6=%v/%s", address.Addr().Is6(), network), func(subtest *testing.T) {
						subtest.Parallel()
						loopback := netip.MustParseAddr("127.0.0.1")
						if index == 1 {
							loopback = netip.IPv6Loopback()
						}
						ctx, cancel := context.WithTimeout(subtest.Context(), 3*time.Second)
						defer cancel()
						if network == "tcp" {
							listener, listenErr := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(loopback, 0)))
							require.NoError(subtest, listenErr)
							defer listener.Close()
							listener.SetDeadline(time.Now().Add(3 * time.Second))
							destination := M.SocksaddrFrom(address.Addr(), uint16(listener.Addr().(*net.TCPAddr).Port))
							client, dialErr := endpoint.DialContext(ctx, "tcp", destination)
							require.NoError(subtest, dialErr)
							defer client.Close()
							server, acceptErr := listener.Accept()
							require.NoError(subtest, acceptErr)
							defer server.Close()
							client.SetDeadline(time.Now().Add(3 * time.Second))
							server.SetDeadline(time.Now().Add(3 * time.Second))
							payload := bytes.Repeat([]byte("wireguard-encrypted-tcp"), 8192)
							for _, pair := range [][2]net.Conn{{client, server}, {server, client}} {
								result := make(chan error, 1)
								go func() {
									_, writeErr := pair[0].Write(payload)
									result <- writeErr
								}()
								received := make([]byte, len(payload))
								_, readErr := io.ReadFull(pair[1], received)
								require.NoError(subtest, readErr)
								require.Equal(subtest, payload, received)
								require.NoError(subtest, <-result)
							}
							return
						}
						server, listenErr := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(loopback, 0)))
						require.NoError(subtest, listenErr)
						defer server.Close()
						server.SetDeadline(time.Now().Add(3 * time.Second))
						destination := M.SocksaddrFrom(address.Addr(), uint16(server.LocalAddr().(*net.UDPAddr).Port))
						var send func([]byte) (int, error)
						var receive func([]byte) (int, error)
						if network == "udp" {
							client, dialErr := endpoint.DialContext(ctx, "udp", destination)
							require.NoError(subtest, dialErr)
							defer client.Close()
							client.SetDeadline(time.Now().Add(3 * time.Second))
							send, receive = client.Write, client.Read
						} else {
							client, dialErr := endpoint.ListenPacket(ctx, destination)
							require.NoError(subtest, dialErr)
							defer client.Close()
							client.SetDeadline(time.Now().Add(3 * time.Second))
							send = func(payload []byte) (int, error) { return client.WriteTo(payload, destination.UDPAddr()) }
							receive = func(payload []byte) (int, error) {
								count, source, readErr := client.ReadFrom(payload)
								if readErr == nil {
									require.Equal(subtest, destination.String(), source.String())
								}
								return count, readErr
							}
						}
						for _, size := range []int{64, 4096} {
							payload := bytes.Repeat([]byte{0x65}, size)
							request := payload
							if system && size > 1200 {
								request = request[:1200]
							}
							_, writeErr := send(request)
							require.NoError(subtest, writeErr)
							received := make([]byte, 65535)
							count, source, readErr := server.ReadFromUDP(received)
							require.NoError(subtest, readErr)
							require.Equal(subtest, request, received[:count])
							_, writeErr = server.WriteToUDP(payload, source)
							require.NoError(subtest, writeErr)
							count, readErr = receive(received)
							require.NoError(subtest, readErr)
							require.Equal(subtest, payload, received[:count])
						}
					})
				}
			}
		})
	}
}
