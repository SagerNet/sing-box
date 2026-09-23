//go:build !go1.27 || http2legacy || !badlinkname

package http

import (
	"net"
	"net/http"

	"golang.org/x/net/http2"
)

type http2ClientConn struct {
	*http2.ClientConn
}

func newHTTP2ClientConn(transport *http2.Transport, conn net.Conn) (*http2ClientConn, error) {
	clientConn, err := transport.NewClientConn(conn)
	if err != nil {
		return nil, err
	}
	return &http2ClientConn{ClientConn: clientConn}, nil
}

func (c *http2ClientConn) isClosed() bool {
	return c.State().Closed
}

func (c *http2ClientConn) reserveNewRequest() bool {
	return c.ReserveNewRequest()
}

func (c *http2ClientConn) roundTrip(request *http.Request) (*http.Response, error) {
	return c.RoundTrip(request)
}
