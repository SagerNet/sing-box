package hysteria

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/lucas-clemente/quic-go"
)

var _ net.Conn = (*ClientConn)(nil)

type ClientConn struct {
	quic.Stream
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func NewClientConn(stream quic.Stream, destination M.Socksaddr) *ClientConn {
	return &ClientConn{
		Stream:      stream,
		destination: destination,
	}
}

func (c *ClientConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		var response *ServerResponse
		response, err = ReadServerResponse(c.Stream)
		if err != nil {
			return
		}
		if !response.OK {
			return 0, E.New("remote error: " + response.Message)
		}
		c.responseRead = true
	}
	return c.Stream.Read(b)
}

func (c *ClientConn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		err = WriteClientRequest(c.Stream, ClientRequest{
			UDP:  false,
			Host: c.destination.AddrString(),
			Port: c.destination.Port,
		}, b)
		if err != nil {
			return
		}
		c.requestWritten = true
		return len(b), nil
	}
	return c.Stream.Write(b)
}

func (c *ClientConn) LocalAddr() net.Addr {
	return nil
}

func (c *ClientConn) RemoteAddr() net.Addr {
	return c.destination.TCPAddr()
}

func (c *ClientConn) Upstream() any {
	return c.Stream
}

type ClientPacketConn struct {
	session     quic.Connection
	stream      quic.Stream
	sessionId   uint32
	destination M.Socksaddr
	msgCh       <-chan *UDPMessage
	closer      io.Closer
}

func NewClientPacketConn(session quic.Connection, stream quic.Stream, sessionId uint32, destination M.Socksaddr, msgCh <-chan *UDPMessage, closer io.Closer) *ClientPacketConn {
	return &ClientPacketConn{
		session:     session,
		stream:      stream,
		sessionId:   sessionId,
		destination: destination,
		msgCh:       msgCh,
		closer:      closer,
	}
}

func (c *ClientPacketConn) Hold() {
	// Hold the stream until it's closed
	buf := make([]byte, 1024)
	for {
		_, err := c.stream.Read(buf)
		if err != nil {
			break
		}
	}
	_ = c.Close()
}

func (c *ClientPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	msg := <-c.msgCh
	if msg == nil {
		err = net.ErrClosed
		return
	}
	err = common.Error(buffer.Write(msg.Data))
	destination = M.ParseSocksaddrHostPort(msg.Host, msg.Port)
	return
}

func (c *ClientPacketConn) ReadPacketThreadSafe() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	msg := <-c.msgCh
	if msg == nil {
		err = net.ErrClosed
		return
	}
	buffer = buf.As(msg.Data)
	destination = M.ParseSocksaddrHostPort(msg.Host, msg.Port)
	return
}

func (c *ClientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return WriteUDPMessage(c.session, UDPMessage{
		SessionID: c.sessionId,
		Host:      destination.AddrString(),
		Port:      destination.Port,
		FragCount: 1,
		Data:      buffer.Bytes(),
	})
}

func (c *ClientPacketConn) LocalAddr() net.Addr {
	return nil
}

func (c *ClientPacketConn) RemoteAddr() net.Addr {
	return c.destination.UDPAddr()
}

func (c *ClientPacketConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *ClientPacketConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *ClientPacketConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *ClientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	panic("invalid")
}

func (c *ClientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	panic("invalid")
}

func (c *ClientPacketConn) Read(b []byte) (n int, err error) {
	panic("invalid")
}

func (c *ClientPacketConn) Write(b []byte) (n int, err error) {
	panic("invalid")
}

func (c *ClientPacketConn) Close() error {
	return common.Close(c.stream, c.closer)
}
