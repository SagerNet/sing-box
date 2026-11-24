//go:build go1.27 && !http2legacy && badlinkname

package http

import (
	"net"
	"net/http"
	_ "unsafe"

	"golang.org/x/net/http2"
)

// Same method set as net/http.genericClientConn, the interface returned by
// (*net/http.Transport).http2NewClientConn.
type http2GenericClientConn interface {
	Available() int
	Close() error
	Err() error
	InFlight() int
	Release()
	Reserve() error
	RoundTrip(request *http.Request) (*http.Response, error)
}

type http2ClientConn struct {
	http2GenericClientConn
}

func newHTTP2ClientConn(transport *http2.Transport, conn net.Conn) (*http2ClientConn, error) {
	clientConn, err := transportHTTP2NewClientConn(http2TransportInit(transport), conn, nil)
	if err != nil {
		return nil, err
	}
	return &http2ClientConn{clientConn}, nil
}

func (c *http2ClientConn) isClosed() bool {
	return c.Err() != nil
}

func (c *http2ClientConn) reserveNewRequest() bool {
	return c.Reserve() == nil
}

func (c *http2ClientConn) roundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.RoundTrip(request)
	// net/http/internal/http2 returns errExtendedConnectNotSupported before consuming the reservation.
	if err != nil && isExtendedConnectNotSupported(err) {
		c.Release()
	}
	return response, err
}

//go:linkname http2TransportInit golang.org/x/net/http2.(*Transport).init
func http2TransportInit(transport *http2.Transport) *http.Transport

//go:linkname transportHTTP2NewClientConn net/http.(*Transport).http2NewClientConn
func transportHTTP2NewClientConn(transport *http.Transport, conn net.Conn, internalStateHook func()) (http2GenericClientConn, error)
