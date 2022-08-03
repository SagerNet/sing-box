package mux

import (
	"io"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"

	"github.com/xtaci/smux"
)

type abstractSession interface {
	Open() (net.Conn, error)
	Accept() (net.Conn, error)
	NumStreams() int
	Close() error
	IsClosed() bool
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

type protocolConn struct {
	net.Conn
	protocol        Protocol
	protocolWritten bool
}

func (c *protocolConn) Write(p []byte) (n int, err error) {
	if c.protocolWritten {
		return c.Conn.Write(p)
	}
	_buffer := buf.StackNewSize(2 + len(p))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	EncodeRequest(buffer, Request{
		Protocol: c.protocol,
	})
	common.Must(common.Error(buffer.Write(p)))
	n, err = c.Conn.Write(buffer.Bytes())
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
