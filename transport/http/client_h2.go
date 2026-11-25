package http

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/pipe"

	"golang.org/x/net/http2"
)

type http2Session struct {
	clientConn *http2ClientConn
	streams    int
}

func (c *Client) acquireHTTP2(ctx context.Context) (*http2Session, net.Conn, error) {
	for {
		c.http2Access.Lock()
		c.http2Sessions = slices.DeleteFunc(c.http2Sessions, func(it *http2Session) bool { return it.clientConn.isClosed() })
		for _, session := range c.http2Sessions {
			if session.clientConn.reserveNewRequest() {
				session.streams++
				c.http2Access.Unlock()
				return session, nil, nil
			}
		}
		offer := c.http2Dial
		if offer == nil {
			offer = &dialOffer{done: make(chan struct{})}
			c.http2Dial = offer
			c.http2Access.Unlock()
			return c.dialHTTP2(ctx, offer)
		}
		c.http2Access.Unlock()
		select {
		case <-offer.done:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
		if offer.err != nil {
			return nil, nil, offer.err
		}
	}
}

func (c *Client) dialHTTP2(ctx context.Context, offer *dialOffer) (*http2Session, net.Conn, error) {
	conn, err := c.tlsDialer.DialTLSContext(adapter.ContextForMultiplexSession(ctx), c.server)
	var clientConn *http2ClientConn
	if err == nil && conn.ConnectionState().NegotiatedProtocol == http2.NextProtoTLS {
		clientConn, err = newHTTP2ClientConn(c.http2Transport, conn)
		if err != nil {
			conn.Close()
			err = E.Cause(err, "create HTTP/2 connection")
		}
	}
	c.http2Access.Lock()
	defer c.http2Access.Unlock()
	defer close(offer.done)
	current := c.http2Dial == offer
	if current {
		c.http2Dial = nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		offer.err = err
		return nil, nil, err
	}
	if clientConn == nil {
		if !c.disableVersionFallback {
			c.http2Unsupported.Store(true)
		}
		return nil, conn, nil
	}
	if !current {
		clientConn.Close()
		return nil, nil, net.ErrClosed
	}
	clientConn.reserveNewRequest()
	session := &http2Session{clientConn: clientConn, streams: 1}
	c.http2Sessions = append(c.http2Sessions, session)
	return session, nil, nil
}

func (c *Client) releaseHTTP2(session *http2Session, keepSession bool) {
	c.http2Access.Lock()
	defer c.http2Access.Unlock()
	session.streams--
	if session.streams == 0 && !keepSession && c.closeIdle.Load() {
		session.clientConn.Close()
	}
}

func (c *Client) connectHTTP2(ctx context.Context, session *http2Session, destination M.Socksaddr) (net.Conn, error) {
	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: destination.String()},
		Host:   destination.String(),
		Header: buildRequestHeader(c.headers, c.authorization, false),
	}
	streamConn, err := c.roundTripHTTP2(ctx, session, request, destination)
	if err != nil {
		return nil, err
	}
	return newClientTunnelConn(streamConn), nil
}

func (c *Client) roundTripHTTP2(ctx context.Context, session *http2Session, request *http.Request, destination M.Socksaddr) (*clientStreamConn, error) {
	ctx, cancelHandshake := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancelHandshake()
	keepSession := adapter.KeepSessionFromContext(ctx)
	pipeReader, pipeWriter := pipe.Pipe()
	streamCtx, cancel := context.WithCancel(context.Background())
	request.Body = pipeReader
	request = request.WithContext(streamCtx)
	stop := context.AfterFunc(ctx, cancel)
	response, err := session.clientConn.roundTrip(request)
	stopped := stop()
	if err == nil && !stopped {
		response.Body.Close()
		err = ctx.Err()
	}
	if err != nil {
		cancel()
		pipeWriter.Close()
		c.releaseHTTP2(session, keepSession)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if isExtendedConnectNotSupported(err) {
			return nil, E.Cause1(errExtendedConnectUnsupported, err)
		}
		return nil, E.Cause(err, "HTTP/2 CONNECT")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		cancel()
		pipeWriter.Close()
		c.releaseHTTP2(session, keepSession)
		return nil, statusError(response)
	}
	return &clientStreamConn{
		reader:     response.Body,
		writer:     pipeWriter,
		cancel:     cancel,
		onClose:    func() { c.releaseHTTP2(session, keepSession) },
		localAddr:  M.Socksaddr{},
		remoteAddr: destination,
	}, nil
}

// golang.org/x/net/http2 and net/http/internal/http2 have no exported sentinel for a peer
// that lacks SETTINGS_ENABLE_CONNECT_PROTOCOL; RoundTrip returns errExtendedConnectNotSupported
// ("net/http: extended connect not supported by peer").
func isExtendedConnectNotSupported(err error) bool {
	return strings.Contains(err.Error(), "extended connect not supported")
}

func (c *Client) connectAndClose(ctx context.Context, conn net.Conn, destination M.Socksaddr) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
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
