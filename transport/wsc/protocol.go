package wsc

import (
	"net"
	"os"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	N "github.com/sagernet/sing/common/network"
)

var _ N.EarlyConn = &ClientConn{}

var _ net.Conn = &ClientPacketConn{}
var _ net.PacketConn = &ClientPacketConn{}
var _ N.NetPacketReader = &ClientPacketConn{}
var _ N.NetPacketWriter = &ClientPacketConn{}

var _ N.NetPacketReader = &servicePacketConn{}
var _ N.NetPacketWriter = &servicePacketConn{}

type ClientConn struct {
	N.ExtendedConn
	destination M.Socksaddr
}

type ClientPacketConn struct {
	net.Conn
	ruleApplicator *WSCRuleApplicator
	writePayload   packetConnPayload
	readPayload    packetConnPayload
	packet         [buf.UDPBufferSize]byte
}

type servicePacketConn struct {
	net.Conn
	writePayload packetConnPayload
	readPayload  packetConnPayload
	packet       [buf.UDPBufferSize]byte
}

func NewClientConn(conn net.Conn, destination M.Socksaddr) (*ClientConn, error) {
	return &ClientConn{
		ExtendedConn: bufio.NewExtendedConn(conn),
		destination:  destination,
	}, nil
}

func NewClientPacketConn(conn net.Conn, ruleApplicator *WSCRuleApplicator) (*ClientPacketConn, error) {
	return &ClientPacketConn{
		Conn:           conn,
		ruleApplicator: ruleApplicator,
	}, nil
}

func (conn *ClientConn) NeedHandshake() bool {
	return false
}

func (conn *ClientConn) FrontHeadroom() int {
	return 0
}

func (conn *ClientConn) Upstream() any {
	return conn.ExtendedConn
}

func (packetConn *ClientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if buffer == nil {
		return exceptions.New("buffer is nil")
	}

	if packetConn.ruleApplicator != nil {
		ep, _ := packetConn.ruleApplicator.ApplyEndpointReplace(destination.String(), network.NetworkUDP, RuleDirectionOutbound)
		packetConn.writePayload.addrPort = metadata.ParseSocksaddr(ep).AddrPort()
	} else {
		packetConn.writePayload.addrPort = destination.AddrPort()
	}
	packetConn.writePayload.payload = buffer.Bytes()
	payloadBytes, err := packetConn.writePayload.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = packetConn.Conn.Write(payloadBytes)

	return err
}

func (packetConn *ClientPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if buffer == nil {
		return destination, exceptions.New("buffer is nil")
	}

	n, err := packetConn.Conn.Read(packetConn.packet[:])
	if err != nil {
		return destination, err
	}

	if err := packetConn.readPayload.UnmarshalBinaryUnsafe(packetConn.packet[:n]); err != nil {
		return destination, err
	}

	if _, err := buffer.Write(packetConn.readPayload.payload); err != nil {
		return destination, err
	}

	destination = metadata.SocksaddrFromNetIP(packetConn.readPayload.addrPort)

	if packetConn.ruleApplicator != nil {
		ep, _ := packetConn.ruleApplicator.ApplyEndpointReplace(destination.String(), N.NetworkUDP, RuleDirectionInbound)
		destination = metadata.ParseSocksaddr(ep)
	}

	return
}

func (packetConn *ClientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buffer := buf.With(p)
	destination, err := packetConn.ReadPacket(buffer)
	if err != nil {
		return
	}
	n = buffer.Len()
	if destination.IsFqdn() {
		addr = destination
	} else {
		addr = destination.UDPAddr()
	}
	return
}

func (packetConn *ClientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return bufio.WritePacket(packetConn, p, addr)
}

func (packetConn *ClientPacketConn) Read(b []byte) (n int, err error) {
	n, _, err = packetConn.ReadFrom(b)
	return
}

func (packetConn *ClientPacketConn) Write(b []byte) (n int, err error) {
	return 0, os.ErrInvalid
}

func (packetConn *ClientPacketConn) NeedHandshake() bool {
	return false
}

func (packetConn *ClientPacketConn) FrontHeadroom() int {
	return 0
}

func (packetConn *ClientPacketConn) Upstream() any {
	return packetConn.Conn
}

func (packetConn *servicePacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if buffer == nil {
		return exceptions.New("buffer is nil")
	}

	packetConn.writePayload.addrPort = destination.AddrPort()
	packetConn.writePayload.payload = buffer.Bytes()
	payloadBytes, err := packetConn.writePayload.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = packetConn.Conn.Write(payloadBytes)

	return err
}

func (packetConn *servicePacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return bufio.WritePacket(packetConn, p, addr)
}

func (packetConn *servicePacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buffer := buf.With(p)
	destination, err := packetConn.ReadPacket(buffer)
	if err != nil {
		return
	}
	n = buffer.Len()
	if destination.IsFqdn() {
		addr = destination
	} else {
		addr = destination.UDPAddr()
	}
	return
}

func (packetConn *servicePacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if buffer == nil {
		return destination, exceptions.New("buffer is nil")
	}

	n, err := packetConn.Conn.Read(packetConn.packet[:])
	if err != nil {
		return destination, err
	}

	if err := packetConn.readPayload.UnmarshalBinaryUnsafe(packetConn.packet[:n]); err != nil {
		return destination, err
	}

	if _, err := buffer.Write(packetConn.readPayload.payload); err != nil {
		return destination, err
	}

	destination = metadata.SocksaddrFromNetIP(packetConn.readPayload.addrPort)

	return
}

func (packetConn *servicePacketConn) NeedHandshake() bool {
	return false
}

func (packetConn *servicePacketConn) FrontHeadroom() int {
	return 0
}

func (packetConn *servicePacketConn) NeedAdditionalReadDeadline() bool {
	return false
}

func (packetConn *servicePacketConn) Upstream() any {
	return packetConn.Conn
}
