package mux

import (
	"io"
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/smux"

	"github.com/hashicorp/yamux"
)

type abstractSession interface {
	Open() (net.Conn, error)
	Accept() (net.Conn, error)
	NumStreams() int
	Close() error
	IsClosed() bool
	CanTakeNewRequest() bool
}

var _ abstractSession = (*smuxSession)(nil)

type smuxSession struct {
	*smux.Session
}

func (s *smuxSession) Open() (net.Conn, error) {
	return s.OpenStream()
}

func (s *smuxSession) Accept() (net.Conn, error) {
	return s.AcceptStream()
}

func (s *smuxSession) CanTakeNewRequest() bool {
	return true
}

type yamuxSession struct {
	*yamux.Session
}

func (y *yamuxSession) CanTakeNewRequest() bool {
	return true
}

type protocolConn struct {
	net.Conn
	request         Request
	protocolWritten bool
}

func newProtocolConn(conn net.Conn, request Request) net.Conn {
	writer, isVectorised := bufio.CreateVectorisedWriter(conn)
	if isVectorised {
		return &vectorisedProtocolConn{
			protocolConn{
				Conn:    conn,
				request: request,
			},
			writer,
		}
	} else {
		return &protocolConn{
			Conn:    conn,
			request: request,
		}
	}
}

func (c *protocolConn) Write(p []byte) (n int, err error) {
	if c.protocolWritten {
		return c.Conn.Write(p)
	}
	buffer := EncodeRequest(c.request, p)
	n, err = c.Conn.Write(buffer.Bytes())
	buffer.Release()
	if err == nil {
		n--
	}
	c.protocolWritten = true
	return n, err
}

func (c *protocolConn) ReadFrom(r io.Reader) (n int64, err error) {
	if !c.protocolWritten {
		return bufio.ReadFrom0(c, r)
	}
	return bufio.Copy(c.Conn, r)
}

func (c *protocolConn) Upstream() any {
	return c.Conn
}

type vectorisedProtocolConn struct {
	protocolConn
	writer N.VectorisedWriter
}

func (c *vectorisedProtocolConn) WriteVectorised(buffers []*buf.Buffer) error {
	if c.protocolWritten {
		return c.writer.WriteVectorised(buffers)
	}
	c.protocolWritten = true
	buffer := EncodeRequest(c.request, nil)
	return c.writer.WriteVectorised(append([]*buf.Buffer{buffer}, buffers...))
}
