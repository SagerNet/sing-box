package proxyproto

import (
	std_bufio "bufio"
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/pires/go-proxyproto"
)

type Listener struct {
	net.Listener
	AcceptNoHeader bool
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	bufReader := std_bufio.NewReader(conn)
	header, err := proxyproto.Read(bufReader)
	if err != nil && !(l.AcceptNoHeader && err == proxyproto.ErrNoProxyProtocol) {
		return nil, &Error{err}
	}
	if bufReader.Buffered() > 0 {
		cache := buf.NewSize(bufReader.Buffered())
		_, err = cache.ReadFullFrom(bufReader, cache.FreeLen())
		if err != nil {
			return nil, &Error{err}
		}
		conn = bufio.NewCachedConn(conn, cache)
	}
	if header != nil {
		return &bufio.AddrConn{Conn: conn, Metadata: M.Metadata{
			Source:      M.SocksaddrFromNet(header.SourceAddr).Unwrap(),
			Destination: M.SocksaddrFromNet(header.DestinationAddr).Unwrap(),
		}}, nil
	}
	return conn, nil
}

var _ net.Error = (*Error)(nil)

type Error struct {
	error
}

func (e *Error) Unwrap() error {
	return e.error
}

func (e *Error) Timeout() bool {
	return false
}

func (e *Error) Temporary() bool {
	return true
}
