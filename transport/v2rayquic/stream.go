package v2rayquic

import (
	"net"

	"github.com/sagernet/quic-go"
	qtls "github.com/sagernet/sing-quic"
)

type StreamWrapper struct {
	Conn *quic.Conn
	*quic.Stream
}

func (s *StreamWrapper) Read(p []byte) (n int, err error) {
	n, err = s.Stream.Read(p)
	return n, qtls.WrapError(err)
}

func (s *StreamWrapper) Write(p []byte) (n int, err error) {
	n, err = s.Stream.Write(p)
	return n, qtls.WrapError(err)
}

func (s *StreamWrapper) LocalAddr() net.Addr {
	return s.Conn.LocalAddr()
}

func (s *StreamWrapper) RemoteAddr() net.Addr {
	return s.Conn.RemoteAddr()
}

func (s *StreamWrapper) Upstream() any {
	return s.Stream
}

func (s *StreamWrapper) Close() error {
	s.CancelRead(0)
	s.Stream.Close()
	return nil
}
