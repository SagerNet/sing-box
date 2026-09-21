package http

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/pipe"

	"golang.org/x/net/http2"
)

type http2ClientConn struct {
	*http2.ClientConn
}

func (c *Client) acquireHTTP2(ctx context.Context) (*http2ClientConn, net.Conn, error) {
	c.http2Access.Lock()
	defer c.http2Access.Unlock()
	c.http2Conns = slices.DeleteFunc(c.http2Conns, func(it *http2ClientConn) bool { return it.State().Closed })
	for _, clientConn := range c.http2Conns {
		if clientConn.ReserveNewRequest() {
			return clientConn, nil, nil
		}
	}
	conn, err := c.tlsDialer.DialTLSContext(ctx, c.server)
	if err != nil {
		return nil, nil, err
	}
	if conn.ConnectionState().NegotiatedProtocol != http2.NextProtoTLS {
		if !c.disableVersionFallback {
			c.http2Unsupported.Store(true)
		}
		return nil, conn, nil
	}
	rawClientConn, err := c.http2Transport.NewClientConn(conn)
	if err != nil {
		conn.Close()
		return nil, nil, E.Cause(err, "create HTTP/2 connection")
	}
	clientConn := &http2ClientConn{ClientConn: rawClientConn}
	clientConn.ReserveNewRequest()
	c.http2Conns = append(c.http2Conns, clientConn)
	return clientConn, nil, nil
}

func (c *Client) connectHTTP2(ctx context.Context, clientConn *http2ClientConn, destination M.Socksaddr) (net.Conn, error) {
	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: destination.String()},
		Host:   destination.String(),
		Header: c.headers.Clone(),
	}
	streamConn, err := c.roundTripHTTP2(ctx, clientConn, request, destination)
	if err != nil {
		return nil, err
	}
	return newClientTunnelConn(streamConn), nil
}

func (c *Client) roundTripHTTP2(ctx context.Context, clientConn *http2ClientConn, request *http.Request, destination M.Socksaddr) (*clientStreamConn, error) {
	pipeReader, pipeWriter := pipe.Pipe()
	streamCtx, cancel := context.WithCancel(context.Background())
	request.Body = pipeReader
	request = request.WithContext(streamCtx)
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	if _, loaded := request.Header["User-Agent"]; !loaded {
		request.Header["User-Agent"] = nil
	}
	if c.authorization != "" {
		request.Header.Set("Proxy-Authorization", c.authorization)
	}
	stop := context.AfterFunc(ctx, cancel)
	response, err := clientConn.RoundTrip(request)
	stopped := stop()
	if err == nil && !stopped {
		response.Body.Close()
		err = ctx.Err()
	}
	if err != nil {
		cancel()
		pipeWriter.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// golang.org/x/net/http2 has no exported sentinel for a peer that lacks
		// SETTINGS_ENABLE_CONNECT_PROTOCOL; it returns errExtendedConnectNotSupported
		// ("net/http: extended connect not supported by peer") from RoundTrip.
		if strings.Contains(err.Error(), "extended connect not supported") {
			return nil, E.Cause1(errExtendedConnectUnsupported, err)
		}
		return nil, E.Cause(err, "HTTP/2 CONNECT")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		cancel()
		pipeWriter.Close()
		return nil, statusError(response)
	}
	return &clientStreamConn{
		reader:     response.Body,
		writer:     pipeWriter,
		cancel:     cancel,
		localAddr:  M.Socksaddr{},
		remoteAddr: destination,
	}, nil
}

func (c *Client) connectAndClose(ctx context.Context, conn net.Conn, destination M.Socksaddr) (net.Conn, error) {
	result, err := c.connect(ctx, conn, destination)
	if err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return result, nil
}
