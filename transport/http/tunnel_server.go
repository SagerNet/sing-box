package http

import (
	"context"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/common/badhttp"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type TunnelHandler interface {
	NewTunnelRequest(ctx context.Context, request TunnelRequest)
}

type TunnelRequest interface {
	Request() *http.Request
	Source() M.Socksaddr
	Accept() (io.ReadWriteCloser, error)
	Reject(statusCode int, header http.Header)
}

func (s *Server) authenticate(ctx context.Context, request *http.Request, headerName string) (context.Context, error) {
	if s.authenticator == nil {
		return ctx, nil
	}
	username, password, valid := badhttp.ParseBasicAuth(request.Header.Get(headerName))
	if !valid {
		return nil, E.New("authentication failed: missing or malformed ", headerName)
	}
	if !s.authenticator.Verify(username, password) {
		return nil, E.New("authentication failed: username=", username)
	}
	return auth.ContextWithUser(ctx, username), nil
}

func (s *Server) upgradeTunnelHandler(request *http.Request) (string, TunnelHandler) {
	if len(s.tunnels) == 0 || request.Method != http.MethodGet || !requestIsUpgrade(request) {
		return "", nil
	}
	protocol := strings.ToLower(request.Header.Get("Upgrade"))
	return protocol, s.tunnels[protocol]
}

type http1TunnelRequest struct {
	conn      *serverConn
	request   *http.Request
	source    M.Socksaddr
	protocol  string
	responded bool
	accepted  bool
	result    requestResult
	err       error
}

func (r *http1TunnelRequest) Request() *http.Request {
	return r.request
}

func (r *http1TunnelRequest) Source() M.Socksaddr {
	return r.source
}

func (r *http1TunnelRequest) Accept() (io.ReadWriteCloser, error) {
	r.responded = true
	_, err := r.conn.conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: " + r.protocol + "\r\nCapsule-Protocol: ?1\r\n\r\n"))
	if err != nil {
		r.result = requestClose
		r.err = E.Cause(err, "write response")
		return nil, r.err
	}
	r.accepted = true
	return r.conn.reader.cachedConn(r.conn.conn), nil
}

func (r *http1TunnelRequest) Reject(statusCode int, header http.Header) {
	r.responded = true
	r.result, r.err = r.conn.reject(r.request, requestKeepAlive(r.request), statusCode, header, nil)
}

func (c *serverConn) serveTunnel(ctx context.Context, request *http.Request, source M.Socksaddr, protocol string, handler TunnelHandler) (requestResult, error) {
	if !request.ProtoAtLeast(1, 1) {
		return c.reject(request, requestKeepAlive(request), http.StatusBadRequest, nil, E.New("invalid ", protocol, " request"))
	}
	current := &http1TunnelRequest{
		conn:     c,
		request:  request,
		source:   source,
		protocol: protocol,
	}
	handler.NewTunnelRequest(ctx, current)
	if !current.responded {
		return c.reject(request, requestKeepAlive(request), http.StatusInternalServerError, nil, E.New(protocol, " request left unanswered"))
	}
	if current.accepted {
		return requestClose, nil
	}
	return current.result, current.err
}

type httpTunnelRequest struct {
	writer    http.ResponseWriter
	request   *http.Request
	source    M.Socksaddr
	responded bool
	wrapper   *v2rayhttp.HTTP2ConnWrapper
}

func (r *httpTunnelRequest) Request() *http.Request {
	return r.request
}

func (r *httpTunnelRequest) Source() M.Socksaddr {
	return r.source
}

func (r *httpTunnelRequest) Accept() (io.ReadWriteCloser, error) {
	r.responded = true
	r.writer.Header().Set("Capsule-Protocol", "?1")
	r.writer.WriteHeader(http.StatusOK)
	r.writer.(http.Flusher).Flush()
	if r.request.ProtoMajor == 3 && HTTP3StreamFunc != nil {
		stream, isDatagramStream := HTTP3StreamFunc(r.request.Context(), r.writer)
		if isDatagramStream {
			return stream, nil
		}
	}
	r.wrapper = v2rayhttp.NewHTTP2Wrapper(&v2rayhttp.ServerHTTPConn{
		HTTP2Conn: v2rayhttp.NewHTTPConn(r.request.Body, r.writer),
		Flusher:   r.writer.(http.Flusher),
	})
	return r.wrapper, nil
}

func (r *httpTunnelRequest) Reject(statusCode int, header http.Header) {
	r.responded = true
	maps.Copy(r.writer.Header(), header)
	r.writer.WriteHeader(statusCode)
}

func (h *httpHandler) serveTunnel(ctx context.Context, writer http.ResponseWriter, request *http.Request, source M.Socksaddr, handler TunnelHandler) {
	current := &httpTunnelRequest{
		writer:  writer,
		request: request,
		source:  source,
	}
	handler.NewTunnelRequest(ctx, current)
	if !current.responded {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	if current.wrapper != nil {
		current.wrapper.CloseWrapper()
	}
}
