package main

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	openconnecttransport "github.com/sagernet/sing-box/transport/openconnect"
	openvpntransport "github.com/sagernet/sing-box/transport/openvpn"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type endpointUDPNATDevice struct {
	start               func() error
	writeInboundBuffers func([]*buf.Buffer) error
	setPacketWriter     func(func([]*buf.Buffer) error)
	close               func() error
}

type endpointUDPNATPacket struct {
	session     *endpointUDPNATSession
	destination M.Socksaddr
	payload     []byte
}

type endpointUDPNATSession struct {
	id     uint64
	conn   N.PacketConn
	closed chan struct{}
}

type endpointUDPNATHandler struct {
	nextSessionID atomic.Uint64
	packets       chan endpointUDPNATPacket
}

func (h *endpointUDPNATHandler) JudgeFlow(uint8, netip.AddrPort, netip.AddrPort, []byte) tun.FlowVerdict {
	return tun.FlowVerdict{Action: tun.ActionAccept}
}

func (h *endpointUDPNATHandler) NewDNSPacket([]byte, M.Socksaddr, M.Socksaddr, N.PacketWriter) {
}

func (h *endpointUDPNATHandler) NewConnectionEx(_ context.Context, conn net.Conn, _ M.Socksaddr, _ M.Socksaddr, onClose N.CloseHandlerFunc) {
	err := conn.Close()
	if onClose != nil {
		onClose(err)
	}
}

func (h *endpointUDPNATHandler) NewPacketConnectionEx(_ context.Context, conn N.PacketConn, _ M.Socksaddr, _ M.Socksaddr, onClose N.CloseHandlerFunc) {
	session := &endpointUDPNATSession{
		id:     h.nextSessionID.Add(1),
		conn:   conn,
		closed: make(chan struct{}),
	}
	go func() {
		defer close(session.closed)
		for {
			packetBuffer := buf.NewPacket()
			destination, err := conn.ReadPacket(packetBuffer)
			if err != nil {
				packetBuffer.Release()
				if onClose != nil {
					onClose(err)
				}
				return
			}
			payload := append([]byte(nil), packetBuffer.Bytes()...)
			packetBuffer.Release()
			h.packets <- endpointUDPNATPacket{
				session:     session,
				destination: destination,
				payload:     payload,
			}
		}
	}()
}

func TestOpenVPNEndpointUDPNATDataPlane(t *testing.T) {
	testEndpointUDPNATDataPlane(t, func(ctx context.Context, handler tun.Handler) (endpointUDPNATDevice, error) {
		device, err := openvpntransport.NewDevice(openvpntransport.DeviceOptions{
			Context:      ctx,
			Logger:       logger.NOP(),
			Handler:      handler,
			UDPTimeout:   time.Minute,
			UDPMapping:   tun.NATMappingAddressAndPortDependent,
			UDPFiltering: tun.NATFilteringAddressAndPortDependent,
			UDPNATMax:    1,
			MTU:          1500,
			Configuration: openvpntransport.Configuration{
				MTU:     1500,
				Address: []netip.Prefix{netip.MustParsePrefix("10.8.0.1/24")},
			},
		})
		if err != nil {
			return endpointUDPNATDevice{}, err
		}
		return endpointUDPNATDevice{
			start:               device.Start,
			writeInboundBuffers: device.WriteInboundBuffers,
			setPacketWriter: func(writer func([]*buf.Buffer) error) {
				device.SetPacketWriter(openvpntransport.PacketWriter(writer))
			},
			close: device.Close,
		}, nil
	})
}

func TestOpenConnectEndpointUDPNATDataPlane(t *testing.T) {
	testEndpointUDPNATDataPlane(t, func(ctx context.Context, handler tun.Handler) (endpointUDPNATDevice, error) {
		device, err := openconnecttransport.NewDevice(openconnecttransport.DeviceOptions{
			Context:      ctx,
			Logger:       logger.NOP(),
			Handler:      handler,
			UDPTimeout:   time.Minute,
			UDPMapping:   tun.NATMappingAddressAndPortDependent,
			UDPFiltering: tun.NATFilteringAddressAndPortDependent,
			UDPNATMax:    1,
			MTU:          1500,
			Configuration: openconnecttransport.Configuration{
				MTU:       1500,
				Addresses: []netip.Prefix{netip.MustParsePrefix("10.8.0.1/24")},
			},
		})
		if err != nil {
			return endpointUDPNATDevice{}, err
		}
		return endpointUDPNATDevice{
			start:               device.Start,
			writeInboundBuffers: device.WriteInboundBuffers,
			setPacketWriter: func(writer func([]*buf.Buffer) error) {
				device.SetPacketWriter(openconnecttransport.PacketWriter(writer))
			},
			close: device.Close,
		}, nil
	})
}

func testEndpointUDPNATDataPlane(t *testing.T, newDevice func(context.Context, tun.Handler) (endpointUDPNATDevice, error)) {
	t.Helper()
	if !tun.WithGVisor {
		t.Skip("requires gVisor")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := &endpointUDPNATHandler{packets: make(chan endpointUDPNATPacket, 4)}
	device, err := newDevice(ctx, handler)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, device.close())
	})
	outboundPackets := make(chan []byte, 4)
	device.setPacketWriter(func(packetBuffers []*buf.Buffer) error {
		for _, packetBuffer := range packetBuffers {
			payload, isUDP := endpointUDPPayload(packetBuffer.Bytes())
			packetBuffer.Release()
			if isUDP {
				outboundPackets <- payload
			}
		}
		return nil
	})

	source := netip.MustParseAddrPort("10.8.0.2:40000")
	firstDestination := netip.MustParseAddrPort("192.0.2.1:5001")
	secondDestination := netip.MustParseAddrPort("192.0.2.1:5002")
	writeEndpointUDPPacket(t, device, source, firstDestination, []byte("before-start"))
	require.NoError(t, device.start())
	writeEndpointUDPPacket(t, device, source, firstDestination, []byte("request-one"))
	firstPacket := waitEndpointUDPNATPacket(t, handler.packets)
	require.Equal(t, M.SocksaddrFromNetIP(firstDestination), firstPacket.destination)
	require.Equal(t, []byte("request-one"), firstPacket.payload)

	require.NoError(t, firstPacket.session.conn.WritePacket(buf.As([]byte("blocked")), M.SocksaddrFromNetIP(secondDestination)))
	require.NoError(t, firstPacket.session.conn.WritePacket(buf.As([]byte("allowed-one")), M.SocksaddrFromNetIP(firstDestination)))
	require.Equal(t, []byte("allowed-one"), waitEndpointUDPResponse(t, outboundPackets))

	writeEndpointUDPPacket(t, device, source, secondDestination, []byte("request-two"))
	secondPacket := waitEndpointUDPNATPacket(t, handler.packets)
	require.Equal(t, M.SocksaddrFromNetIP(secondDestination), secondPacket.destination)
	require.Equal(t, []byte("request-two"), secondPacket.payload)
	require.NotEqual(t, firstPacket.session.id, secondPacket.session.id)
	select {
	case <-firstPacket.session.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("first UDP NAT session was not evicted at max size")
	}

	require.NoError(t, secondPacket.session.conn.WritePacket(buf.As([]byte("allowed-two")), M.SocksaddrFromNetIP(secondDestination)))
	require.Equal(t, []byte("allowed-two"), waitEndpointUDPResponse(t, outboundPackets))
}

func writeEndpointUDPPacket(t *testing.T, device endpointUDPNATDevice, source netip.AddrPort, destination netip.AddrPort, payload []byte) {
	t.Helper()
	packet := make([]byte, header.IPv4MinimumSize+header.UDPMinimumSize+len(payload))
	ipHeader := header.IPv4(packet)
	ipHeader.Encode(&header.IPv4Fields{
		TotalLength: uint16(len(packet)),
		TTL:         64,
		Protocol:    uint8(header.UDPProtocolNumber),
		SrcAddr:     source.Addr(),
		DstAddr:     destination.Addr(),
	})
	ipHeader.SetChecksum(^ipHeader.CalculateChecksum())
	udpHeader := header.UDP(packet[header.IPv4MinimumSize:])
	udpHeader.Encode(&header.UDPFields{
		SrcPort: source.Port(),
		DstPort: destination.Port(),
		Length:  uint16(header.UDPMinimumSize + len(payload)),
	})
	copy(udpHeader.Payload(), payload)
	packetBuffer := buf.As(packet)
	require.NoError(t, device.writeInboundBuffers([]*buf.Buffer{packetBuffer}))
	packetBuffer.Release()
}

func endpointUDPPayload(packet []byte) ([]byte, bool) {
	if len(packet) < header.IPv4MinimumSize {
		return nil, false
	}
	ipHeader := header.IPv4(packet)
	if !ipHeader.IsValid(len(packet)) || ipHeader.Protocol() != uint8(header.UDPProtocolNumber) {
		return nil, false
	}
	udpPayload := ipHeader.Payload()
	if len(udpPayload) < header.UDPMinimumSize {
		return nil, false
	}
	udpHeader := header.UDP(udpPayload)
	return append([]byte(nil), udpHeader.Payload()...), true
}

func waitEndpointUDPNATPacket(t *testing.T, packets <-chan endpointUDPNATPacket) endpointUDPNATPacket {
	t.Helper()
	select {
	case packet := <-packets:
		return packet
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for UDP NAT packet")
		return endpointUDPNATPacket{}
	}
}

func waitEndpointUDPResponse(t *testing.T, packets <-chan []byte) []byte {
	t.Helper()
	select {
	case packet := <-packets:
		return packet
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for UDP response")
		return nil
	}
}
