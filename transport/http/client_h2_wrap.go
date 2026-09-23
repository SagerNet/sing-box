//go:build go1.27 && !http2legacy && badlinkname

package http

import (
	"net"
	"net/http"
	"unsafe"

	"golang.org/x/net/http2"
)

type http2ClientConn struct {
	*http.ClientConn
}

func newHTTP2ClientConn(transport *http2.Transport, conn net.Conn) (*http2ClientConn, error) {
	clientConn, err := transport.NewClientConn(conn)
	if err != nil {
		return nil, err
	}
	return &http2ClientConn{ClientConn: *(**http.ClientConn)(unsafe.Pointer(clientConn))}, nil
}

func (c *http2ClientConn) isClosed() bool {
	return c.Err() != nil
}

func (c *http2ClientConn) reserveNewRequest() bool {
	return c.Reserve() == nil
}

func (c *http2ClientConn) roundTrip(request *http.Request) (*http.Response, error) {
	if request.Header.Get(":protocol") == "" {
		return c.ClientConn.RoundTrip(request)
	}
	internalClientConn := (*[2]unsafe.Pointer)(unsafe.Pointer(c.ClientConn))[1]
	return http2RoundTrip(request, func(clientRequest unsafe.Pointer) (unsafe.Pointer, error) {
		return netHTTPClientConnRoundTrip(internalClientConn, clientRequest)
	})
}

//go:linkname http2RoundTrip net/http.http2RoundTrip
func http2RoundTrip(request *http.Request, roundTrip func(clientRequest unsafe.Pointer) (unsafe.Pointer, error)) (*http.Response, error)

//go:linkname netHTTPClientConnRoundTrip net/http/internal/http2.NetHTTPClientConn.RoundTrip
func netHTTPClientConnRoundTrip(clientConn unsafe.Pointer, clientRequest unsafe.Pointer) (unsafe.Pointer, error)
