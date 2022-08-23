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
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	bufReader := std_bufio.NewReader(conn)
	header, err := proxyproto.Read(bufReader)
	if err != nil {
		return nil, err
	}
	if bufReader.Buffered() > 0 {
		cache := buf.NewSize(bufReader.Buffered())
		_, err = cache.ReadFullFrom(bufReader, cache.FreeLen())
		if err != nil {
			return nil, err
		}
		conn = bufio.NewCachedConn(conn, cache)
	}
	return &bufio.AddrConn{Conn: conn, Metadata: M.Metadata{
		Source:      M.SocksaddrFromNet(header.SourceAddr),
		Destination: M.SocksaddrFromNet(header.DestinationAddr),
	}}, nil
}
