package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/common/badhttp"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type requestResult uint8

const (
	requestContinue requestResult = iota
	requestClose
	requestHandedOff
)

type serverConn struct {
	server   *Server
	ctx      context.Context
	conn     net.Conn
	reader   *Reader
	handler  Handler
	source   M.Socksaddr
	onClose  N.CloseHandlerFunc
	upstream *upstreamConn
}

func (c *serverConn) serve() {
	for {
		result, err := c.serveRequest()
		switch result {
		case requestContinue:
			continue
		case requestHandedOff:
			c.closeUpstream()
			return
		case requestClose:
			c.closeUpstream()
			c.server.finishConnection(c.ctx, c.conn, c.source, c.onClose, err)
			return
		}
	}
}

func (c *serverConn) serveRequest() (requestResult, error) {
	c.conn.SetReadDeadline(time.Now().Add(idleTimeout))
	c.reader.setLimit(maxHeaderBytes)
	request, err := badhttp.ReadRequest(c.reader.Reader)
	c.reader.setLimit(-1)
	c.conn.SetReadDeadline(time.Time{})
	if err != nil {
		switch {
		case errors.Is(err, io.EOF), E.IsTimeout(err), E.IsClosed(err):
			return requestClose, err
		case errors.Is(err, errHeaderTooLarge):
			return c.rejectAndClose(nil, http.StatusRequestHeaderFieldsTooLarge, err)
		default:
			return c.rejectAndClose(nil, http.StatusBadRequest, E.Cause(err, "read request"))
		}
	}
	tunnelProtocol, tunnelHandler := c.server.upgradeTunnelHandler(request)
	if tunnelHandler != nil {
		tunnelCtx, tunnelAuthErr := c.server.authenticate(c.ctx, request, "Authorization")
		if tunnelAuthErr != nil {
			c.server.logger.ErrorContext(c.ctx, E.Cause(tunnelAuthErr, "process connection from ", c.source))
			return c.reject(request, requestKeepAlive(request), http.StatusUnauthorized, nil, tunnelAuthErr)
		}
		return c.serveTunnel(tunnelCtx, request, badhttp.ForwardedSource(request, c.source), tunnelProtocol, tunnelHandler)
	}
	if c.handler == nil {
		return c.reject(request, requestKeepAlive(request), http.StatusNotFound, nil, E.New("unexpected request: ", request.Method, " ", request.URL))
	}
	ctx, authErr := c.server.authenticate(c.ctx, request, "Proxy-Authorization")
	if authErr != nil {
		c.server.logger.ErrorContext(c.ctx, E.Cause(authErr, "process connection from ", c.source))
		return c.reject(request, requestKeepAlive(request), http.StatusProxyAuthRequired, nil, authErr)
	}
	source := badhttp.ForwardedSource(request, c.source)
	switch {
	case request.Method == http.MethodConnect:
		return c.serveConnect(ctx, request, source)
	case c.server.udp && requestIsConnectUDP(request):
		return c.serveConnectUDP(ctx, request, source)
	case requestIsUpgrade(request):
		return c.serveForward(ctx, request, source, true)
	default:
		return c.serveForward(ctx, request, source, false)
	}
}

func (c *serverConn) serveConnect(ctx context.Context, request *http.Request, source M.Socksaddr) (requestResult, error) {
	destination := connectDestination(request)
	if !destination.IsValid() {
		return c.reject(request, requestKeepAlive(request), http.StatusBadRequest, nil, E.New("invalid CONNECT target: ", request.URL.Host))
	}
	_, err := c.conn.Write([]byte(F.ToString("HTTP/", request.ProtoMajor, ".", request.ProtoMinor, " 200 Connection established\r\n\r\n")))
	if err != nil {
		return requestClose, E.Cause(err, "write response")
	}
	c.handler.NewConnectionEx(ctx, c.reader.cachedConn(c.conn), source, destination, c.onClose)
	return requestHandedOff, nil
}

func (c *serverConn) reject(request *http.Request, keepAlive bool, statusCode int, header http.Header, cause error) (requestResult, error) {
	if keepAlive && request.Body != nil && request.Body != http.NoBody {
		keepAlive = c.discardBody(request.Body)
	}
	err := c.writeReject(request, statusCode, header, keepAlive)
	if err != nil {
		return requestClose, E.Errors(cause, err)
	}
	if keepAlive {
		return requestContinue, nil
	}
	return requestClose, cause
}

func (c *serverConn) rejectAndClose(request *http.Request, statusCode int, cause error) (requestResult, error) {
	err := c.writeReject(request, statusCode, nil, false)
	if err != nil {
		return requestClose, E.Errors(cause, err)
	}
	return requestClose, cause
}

func (c *serverConn) writeReject(request *http.Request, statusCode int, header http.Header, keepAlive bool) error {
	response := &http.Response{
		StatusCode: statusCode,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header.Clone(),
		Request:    request,
		Close:      !keepAlive,
	}
	if response.Header == nil {
		response.Header = make(http.Header)
	}
	if request != nil {
		response.ProtoMajor = request.ProtoMajor
		response.ProtoMinor = request.ProtoMinor
	}
	switch statusCode {
	case http.StatusProxyAuthRequired:
		response.Header.Set("Proxy-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
	case http.StatusUnauthorized:
		response.Header.Set("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
	}
	if keepAlive && request.ProtoMajor == 1 && request.ProtoMinor == 0 {
		response.Header.Set("Connection", "keep-alive")
		response.Header.Set("Proxy-Connection", "keep-alive")
	}
	err := response.Write(c.conn)
	if err != nil {
		return E.Cause(err, "write response")
	}
	return nil
}

func (c *serverConn) discardBody(body io.Reader) bool {
	c.conn.SetReadDeadline(time.Now().Add(discardBodyTimeout))
	_, err := io.CopyN(io.Discard, body, maxDiscardBodyBytes)
	c.conn.SetReadDeadline(time.Time{})
	return errors.Is(err, io.EOF)
}

func (c *serverConn) closeUpstream() {
	if c.upstream != nil {
		c.upstream.Close()
		c.upstream = nil
	}
}
