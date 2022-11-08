package wireguard

import (
	"io"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/conn"
)

var _ conn.Bind = (*ServerBind)(nil)

type ServerBind struct {
	inbound   chan serverPacket
	done      chan struct{}
	writeBack N.PacketWriter
}

func NewServerBind(writeBack N.PacketWriter) *ServerBind {
	return &ServerBind{
		inbound:   make(chan serverPacket, 256),
		done:      make(chan struct{}),
		writeBack: writeBack,
	}
}

func (s *ServerBind) Abort() error {
	select {
	case <-s.done:
		return io.ErrClosedPipe
	default:
		close(s.done)
	}
	return nil
}

type serverPacket struct {
	buffer *buf.Buffer
	source M.Socksaddr
}

func (s *ServerBind) Open(port uint16) (fns []conn.ReceiveFunc, actualPort uint16, err error) {
	fns = []conn.ReceiveFunc{s.receive}
	return
}

func (s *ServerBind) receive(b []byte) (n int, ep conn.Endpoint, err error) {
	select {
	case packet := <-s.inbound:
		defer packet.buffer.Release()
		n = copy(b, packet.buffer.Bytes())
		ep = Endpoint(packet.source)
		return
	case <-s.done:
		err = io.ErrClosedPipe
		return
	}
}

func (s *ServerBind) WriteIsThreadUnsafe() {
}

func (s *ServerBind) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	select {
	case s.inbound <- serverPacket{
		buffer: buffer,
		source: destination,
	}:
		return nil
	case <-s.done:
		return io.ErrClosedPipe
	}
}

func (s *ServerBind) Close() error {
	return nil
}

func (s *ServerBind) SetMark(mark uint32) error {
	return nil
}

func (s *ServerBind) Send(b []byte, ep conn.Endpoint) error {
	return s.writeBack.WritePacket(buf.As(b), M.Socksaddr(ep.(Endpoint)))
}

func (s *ServerBind) ParseEndpoint(addr string) (conn.Endpoint, error) {
	destination := M.ParseSocksaddr(addr)
	if !destination.IsValid() || destination.Port == 0 {
		return nil, E.New("invalid endpoint: ", addr)
	}
	return Endpoint(destination), nil
}
