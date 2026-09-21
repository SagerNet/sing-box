package http

import (
	std_bufio "bufio"
	"context"
	"crypto/tls"
	"maps"
	"net"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/common/badhttp"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/pipe"

	"golang.org/x/net/http2"
)

const http2Preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

func (r *Reader) isHTTP2Preface() (bool, error) {
	head, err := r.Peek(3)
	if err != nil {
		return false, err
	}
	if string(head) != http2Preface[:3] {
		return false, nil
	}
	preface, err := r.Peek(len(http2Preface))
	if err != nil {
		return false, err
	}
	return string(preface) == http2Preface, nil
}

type connectionStater interface {
	ConnectionState() tls.ConnectionState
}

type statedConn struct {
	net.Conn
	stater connectionStater
}

func (c *statedConn) ConnectionState() tls.ConnectionState {
	return c.stater.ConnectionState()
}

func (s *Server) serveHTTP2(ctx context.Context, conn net.Conn, reader *Reader, handler Handler, source M.Socksaddr, onClose N.CloseHandlerFunc) {
	serveConn := reader.cachedConn(conn)
	stater, isStater := conn.(connectionStater)
	if isStater && serveConn != conn {
		serveConn = &statedConn{Conn: serveConn, stater: stater}
	}
	s.http2Server.ServeConn(serveConn, &http2.ServeConnOpts{
		Context: ctx,
		BaseConfig: &http.Server{
			MaxHeaderBytes: maxHeaderBytes,
		},
		Handler: &httpHandler{
			server:    s,
			handler:   handler,
			source:    source,
			plaintext: !isStater,
		},
	})
	if onClose != nil {
		onClose(nil)
	}
}

type httpHandler struct {
	server    *Server
	handler   Handler
	source    M.Socksaddr
	plaintext bool
}

func (h *httpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := log.ContextWithNewID(request.Context())
	connectionSource := h.source
	if !connectionSource.IsValid() {
		connectionSource = M.ParseSocksaddr(request.RemoteAddr).Unwrap()
	}
	var protocol string
	if request.Method == http.MethodConnect {
		protocol = request.Header.Get(":protocol")
		if protocol == "" && request.ProtoMajor == 3 && !strings.HasPrefix(request.Proto, "HTTP/") {
			protocol = request.Proto
		}
	}
	tunnelHandler := h.server.tunnels[protocol]
	if tunnelHandler != nil {
		tunnelCtx, authErr := h.server.authenticate(ctx, request, "Authorization")
		if authErr != nil {
			h.server.logger.ErrorContext(ctx, E.Cause(authErr, "process connection from ", connectionSource))
			writer.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.serveTunnel(tunnelCtx, writer, request, badhttp.ForwardedSource(request, connectionSource), tunnelHandler)
		return
	}
	if h.handler == nil {
		h.server.logger.ErrorContext(ctx, "process connection from ", connectionSource, ": unexpected request: ", request.Method, " ", request.URL)
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	proxyCtx, authErr := h.server.authenticate(ctx, request, "Proxy-Authorization")
	if authErr != nil {
		h.server.logger.ErrorContext(ctx, E.Cause(authErr, "process connection from ", connectionSource))
		writer.Header().Set("Proxy-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
		writer.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	ctx = proxyCtx
	source := badhttp.ForwardedSource(request, connectionSource)
	if request.Method == http.MethodConnect {
		switch {
		case protocol == "":
			h.serveConnect(ctx, writer, request, source)
		case protocol == connectUDPProtocol && h.server.udp:
			h.serveConnectUDP(ctx, writer, request, source)
		default:
			h.server.logger.ErrorContext(ctx, "process connection from ", source, ": unsupported CONNECT protocol: ", protocol)
			writer.WriteHeader(http.StatusNotImplemented)
		}
		return
	}
	h.serveForward(ctx, writer, request, source)
}

func (h *httpHandler) serveConnect(ctx context.Context, writer http.ResponseWriter, request *http.Request, source M.Socksaddr) {
	destination := connectDestination(request)
	if !destination.IsValid() {
		h.server.logger.ErrorContext(ctx, "process connection from ", source, ": invalid CONNECT target: ", request.Host)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.(http.Flusher).Flush()
	conn := v2rayhttp.NewHTTP2Wrapper(&v2rayhttp.ServerHTTPConn{
		HTTP2Conn: v2rayhttp.NewHTTPConn(request.Body, writer),
		Flusher:   writer.(http.Flusher),
	})
	done := make(chan struct{})
	h.handler.NewConnectionEx(ctx, conn, source, destination, N.OnceClose(func(it error) {
		close(done)
	}))
	<-done
	conn.CloseWrapper()
}

func (h *httpHandler) serveForward(ctx context.Context, writer http.ResponseWriter, request *http.Request, source M.Socksaddr) {
	// golang.org/x/net/http2 builds the request URL from :path only (internal/httpcommon.NewServerRequest);
	// :scheme survives only as request.TLS, which it sets when :scheme is https and the served conn
	// exposes ConnectionState, so on a plaintext conn the scheme is unknowable. quic-go/http3 sets URL.Scheme.
	if request.URL.Scheme == "" {
		if h.plaintext {
			h.server.logger.ErrorContext(ctx, "process connection from ", source, ": forward request over cleartext HTTP/2")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.TLS != nil {
			request.URL.Scheme = "https"
		} else {
			request.URL.Scheme = "http"
		}
	}
	request.URL.Host = request.Host
	destination, valid := forwardDestination(request)
	if !valid {
		h.server.logger.ErrorContext(ctx, "process connection from ", source, ": invalid forward target: ", request.URL.Scheme, "://", request.Host)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	removeHopByHopHeaders(request.Header)
	if _, loaded := request.Header["User-Agent"]; !loaded {
		request.Header["User-Agent"] = nil
	}
	if request.ContentLength == 0 {
		request.Body = http.NoBody
	}
	request.Close = false

	upstream := newUpstreamConn(ctx, h.handler, source, destination)
	stop := context.AfterFunc(request.Context(), func() {
		upstream.Close()
	})
	defer stop()
	writeDone := make(chan error, 1)
	go func() {
		err := request.Write(upstream)
		if err != nil {
			upstream.Close()
		}
		writeDone <- err
	}()
	defer func() {
		upstream.Close()
		request.Body.Close()
		<-writeDone
	}()

	var response *http.Response
	for {
		var err error
		response, err = upstream.readResponse(request)
		if err != nil {
			upstream.Close()
			h.server.logger.ErrorContext(ctx, E.Cause(E.Errors(upstream.closeErr(), err), "process connection from ", source, ": read upstream response"))
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		if response.StatusCode >= 200 {
			break
		}
		if response.StatusCode == http.StatusSwitchingProtocols {
			upstream.Close()
			h.server.logger.ErrorContext(ctx, "process connection from ", source, ": unexpected 101 response")
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		removeHopByHopHeaders(response.Header)
		maps.Copy(writer.Header(), response.Header)
		writer.WriteHeader(response.StatusCode)
		clear(writer.Header())
	}
	removeHopByHopHeaders(response.Header)
	maps.Copy(writer.Header(), response.Header)
	if !responseHasBody(request, response) && request.Method != http.MethodHead {
		writer.Header().Del("Content-Length")
	}
	for name := range response.Trailer {
		writer.Header().Add("Trailer", name)
	}
	writer.WriteHeader(response.StatusCode)
	if !responseHasBody(request, response) {
		response.Body.Close()
		return
	}
	writer.(http.Flusher).Flush()
	_, err := bufio.Copy(flushWriter{writer}, response.Body)
	if err != nil {
		upstream.Close()
		response.Body.Close()
		h.server.logger.DebugContext(ctx, "process connection from ", source, ": relay response: ", err)
		panic(http.ErrAbortHandler)
	}
	response.Body.Close()
	maps.Copy(writer.Header(), response.Trailer)
}

func newUpstreamConn(ctx context.Context, handler Handler, source M.Socksaddr, destination M.Socksaddr) *upstreamConn {
	serverSide, clientSide := pipe.Pipe()
	limiter := &readLimiter{reader: clientSide, remaining: -1}
	upstream := &upstreamConn{
		Conn:        clientSide,
		reader:      std_bufio.NewReader(limiter),
		limiter:     limiter,
		source:      source,
		destination: destination,
		done:        make(chan struct{}),
	}
	go handler.NewConnectionEx(ctx, serverSide, source, destination, N.OnceClose(func(it error) {
		upstream.err = it
		close(upstream.done)
	}))
	return upstream
}

type flushWriter struct {
	http.ResponseWriter
}

func (w flushWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if err != nil {
		return n, err
	}
	w.ResponseWriter.(http.Flusher).Flush()
	return n, nil
}
